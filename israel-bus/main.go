// Command israel-bus serves upcoming Israeli bus arrivals as small JSON for a
// TRMNL polling plugin. Data comes from the Ministry of Transport API behind
// route.bus.gov.il (anonymous, no key).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // stock alpine has no zoneinfo
)

const (
	maxStops         = 4
	maxArrivals      = 3
	maxRoutesNoLines = 12 // busy stops serve 50+ routes; times cost one request each
	maxConcurrent    = 5
	routesTTL        = time.Hour
	requestBudget    = 9 * time.Second // BYOS Laravel gives a poll 10s
)

var (
	apiBase  = "https://api.bus.gov.il/prod/mot-scheduler-prod/api/he"
	israel   = mustTZ("Asia/Jerusalem")
	stopCode = regexp.MustCompile(`^\d{1,6}$`)
	client   = &http.Client{Timeout: 8 * time.Second}

	cacheMu sync.Mutex
	cache   = map[string]cachedRoutes{} // key: stop code + Israel-local date
)

type cachedRoutes struct {
	at     time.Time
	routes []route
}

type route struct {
	Line      string
	Desc      string
	Direction string
	Operator  string
}

type Arrival struct {
	Min      int    `json:"min"`
	At       string `json:"at"`
	Realtime bool   `json:"realtime"`
}

type Line struct {
	Line      string    `json:"line"`
	Direction string    `json:"direction"`
	Operator  string    `json:"operator"`
	Arrivals  []Arrival `json:"arrivals"`
}

type Stop struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Lines []Line `json:"lines"`
	Error string `json:"error,omitempty"`
}

type Response struct {
	Updated string `json:"updated"`
	Stops   []Stop `json:"stops"`
}

func mustTZ(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// getData GETs path under apiBase and decodes the envelope's "data" into out.
func getData(ctx context.Context, path string, q url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/"+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", path, resp.StatusCode)
	}
	// A removed endpoint still answers 200 with the portal's HTML page.
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "json") {
		return fmt.Errorf("%s: got %q, not JSON", path, ct)
	}
	var env struct {
		Data    json.RawMessage `json:"data"`
		Success *bool           `json:"success"`
		Message string          `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if env.Success != nil && !*env.Success {
		return fmt.Errorf("%s: API failure: %s", path, env.Message)
	}
	return json.Unmarshal(env.Data, out)
}

// routesAtStop lists the routes serving a stop today, cached for routesTTL.
func routesAtStop(ctx context.Context, code string, now time.Time) ([]route, error) {
	local := now.In(israel)
	// The service day must be Israel-local: a past date returns zero routes.
	key := code + "@" + local.Format(time.DateOnly)
	cacheMu.Lock()
	c, ok := cache[key]
	cacheMu.Unlock()
	if ok && now.Sub(c.at) < routesTTL {
		return c.routes, nil
	}

	var data struct {
		RoutesInStop []struct {
			RouteName  string            `json:"routeName"`
			RouteDesc  string            `json:"routeDesc"`
			Headsign   map[string]string `json:"headsign"`
			AgencyName string            `json:"agencyName"`
		} `json:"routesInStop"`
	}
	q := url.Values{"stopCode": {code}, "day": {local.Format("2006-01-02T15:04:05")}}
	if err := getData(ctx, "Stops/RefreshStopTimesAtStop", q, &data); err != nil {
		return nil, err
	}
	var routes []route
	for _, r := range data.RoutesInStop {
		if r.RouteName == "" || r.RouteDesc == "" {
			continue
		}
		dir := r.Headsign["he"]
		if dir == "" {
			dir = r.Headsign["en"]
		}
		routes = append(routes, route{r.RouteName, r.RouteDesc, strings.ReplaceAll(dir, "_", " "), r.AgencyName})
	}
	// An empty list is usually a night gap or an upstream hiccup; don't pin it.
	if len(routes) > 0 {
		cacheMu.Lock()
		for k := range cache {
			if !strings.HasSuffix(k, local.Format(time.DateOnly)) {
				delete(cache, k)
			}
		}
		cache[key] = cachedRoutes{now, routes}
		cacheMu.Unlock()
	}
	return routes, nil
}

func routeTimes(ctx context.Context, code, desc string) ([]Arrival, error) {
	var data map[string]*struct {
		StopTimes []struct {
			ArrivalTimeString string   `json:"arrivalTimeString"`
			MinutesToArrival  *float64 `json:"minutesToArrival"`
			IsRealTime        bool     `json:"isRealTime"`
		} `json:"stopTimes"`
	}
	q := url.Values{"stopCodes": {code}, "routeDesc": {desc}}
	if err := getData(ctx, "Calendar/GetRouteCalendarAtStopsByStopCodes", q, &data); err != nil {
		return nil, err
	}
	var out []Arrival
	if s := data[code]; s != nil {
		for _, t := range s.StopTimes {
			if t.MinutesToArrival == nil || *t.MinutesToArrival < 0 {
				continue
			}
			out = append(out, Arrival{int(*t.MinutesToArrival + 0.5), t.ArrivalTimeString, t.IsRealTime})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Min < out[j].Min })
	if len(out) > maxArrivals {
		out = out[:maxArrivals]
	}
	return out, nil
}

// fetchStop returns the lines at a stop with upcoming arrivals, soonest first.
// wanted filters by line number ("1" does not match "1א"); empty means all.
func fetchStop(ctx context.Context, code, label string, wanted []string, now time.Time) Stop {
	stop := Stop{Code: code, Label: label, Lines: []Line{}}
	routes, err := routesAtStop(ctx, code, now)
	if err != nil {
		stop.Error = err.Error()
		return stop
	}
	if len(wanted) > 0 {
		var kept []route
		for _, r := range routes {
			for _, w := range wanted {
				if r.Line == w {
					kept = append(kept, r)
					break
				}
			}
		}
		if len(kept) == 0 && len(routes) > 0 {
			var avail []string
			for _, r := range routes {
				if !slices.Contains(avail, r.Line) {
					avail = append(avail, r.Line)
				}
			}
			stop.Error = "no such line here; lines at this stop: " + strings.Join(avail, ",")
			return stop
		}
		routes = kept
	} else if len(routes) > maxRoutesNoLines {
		routes = routes[:maxRoutesNoLines]
	}

	lines := make([]Line, len(routes))
	errs := make([]error, len(routes))
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	for i, r := range routes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			arr, err := routeTimes(ctx, code, r.Desc)
			lines[i], errs[i] = Line{r.Line, r.Direction, r.Operator, arr}, err
		}()
	}
	wg.Wait()

	var failed []error
	for i := range lines {
		if errs[i] != nil {
			failed = append(failed, errs[i])
		} else if len(lines[i].Arrivals) > 0 {
			stop.Lines = append(stop.Lines, lines[i])
		}
	}
	// If every lookup failed we don't know the timetable; say so instead of "no buses".
	if len(failed) > 0 && len(failed) == len(routes) {
		stop.Error = errors.Join(failed...).Error()
	} else if len(failed) > 0 {
		log.Printf("stop %s: %d/%d route lookups failed: %v", code, len(failed), len(routes), failed[0])
	}
	sort.SliceStable(stop.Lines, func(i, j int) bool { return stop.Lines[i].Arrivals[0].Min < stop.Lines[j].Arrivals[0].Min })
	return stop
}

// handleStops serves GET /stops?stop=CODE&lines=1,2&label=Home[&stop=...&lines=...&label=...].
// Params pair up by position; a blank stop (an unused TRMNL slot) is skipped.
func handleStops(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	codes, linesQ, labels := q["stop"], q["lines"], q["label"]
	at := func(s []string, i int) string {
		if i < len(s) {
			return strings.TrimSpace(s[i])
		}
		return ""
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	now := time.Now()
	resp := Response{Updated: now.In(israel).Format("15:04")}
	type ask struct {
		code, label string
		wanted      []string
	}
	var asks []ask
	for i := range codes {
		code := at(codes, i)
		if code == "" {
			continue
		}
		if !stopCode.MatchString(code) {
			http.Error(w, "stop must be a numeric stop code", http.StatusBadRequest)
			return
		}
		if len(asks) == maxStops {
			http.Error(w, fmt.Sprintf("at most %d stops", maxStops), http.StatusBadRequest)
			return
		}
		var wanted []string
		for _, l := range strings.Split(at(linesQ, i), ",") {
			if l = strings.TrimSpace(l); l != "" {
				wanted = append(wanted, l)
			}
		}
		asks = append(asks, ask{code, at(labels, i), wanted})
	}
	resp.Stops = make([]Stop, len(asks))
	var wg sync.WaitGroup
	for i, a := range asks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp.Stops[i] = fetchStop(ctx, a.code, a.label, a.wanted, now)
		}()
	}
	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	addr := os.Getenv("LISTEN")
	if addr == "" {
		addr = ":8080"
	}
	http.HandleFunc("GET /stops", handleStops)
	http.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
