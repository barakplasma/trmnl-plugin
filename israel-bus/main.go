// Command israel-bus serves upcoming Israeli bus arrivals as small JSON for a
// TRMNL polling plugin. Data comes from the Ministry of Transport API behind
// route.bus.gov.il (anonymous, no key).
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
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
	maxLines         = 6  // what fits the full layout; smaller sizes hide more via CSS
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
	Stops     []stopRef // in route order
}

type stopRef struct {
	Code string
	Name string
}

// seq is the position of a stop code on the route, or -1.
func (r route) seq(code string) int {
	return slices.IndexFunc(r.Stops, func(s stopRef) bool { return s.Code == code })
}

// stopName finds a stop's Hebrew name in any route's stop list.
func stopName(routes []route, code string) string {
	for _, r := range routes {
		if i := r.seq(code); i >= 0 {
			return r.Stops[i].Name
		}
	}
	return ""
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
	Code   string `json:"code"`
	Name   string `json:"name"`
	To     string `json:"to,omitempty"`
	ToName string `json:"to_name,omitempty"`
	Label  string `json:"label"`
	Lines  []Line `json:"lines"`
	Error  string `json:"error,omitempty"`
}

type Response struct {
	Title   string `json:"title"`
	Updated string `json:"updated"`
	Stops   []Stop `json:"stops"`
	HTML    string `json:"html"` // the whole board, ready for the plugin to print
}

//go:embed board.html
var boardHTML string

var board = template.Must(template.New("board").Parse(boardHTML))

// Header is the stop column's title, "from ← to", with names cut at "/" (the street part).
func (s Stop) Header() string {
	short := func(name string) string { before, _, _ := strings.Cut(name, "/"); return before }
	h := s.Label
	if h == "" {
		h = short(s.Name)
	}
	if h == "" {
		h = s.Code
	}
	if to := short(s.ToName); to != "" {
		h += " ← " + to
	}
	return h
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
			Stops      []struct {
				Stopcode int               `json:"stopcode"`
				Name     map[string]string `json:"name"`
			} `json:"stops"`
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
		stops := make([]stopRef, len(r.Stops))
		for i, st := range r.Stops {
			stops[i] = stopRef{fmt.Sprint(st.Stopcode), st.Name["he"]}
		}
		routes = append(routes, route{r.RouteName, r.RouteDesc, strings.ReplaceAll(dir, "_", " "), r.AgencyName, stops})
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
	live := map[string]bool{}
	if s := data[code]; s != nil {
		for _, t := range s.StopTimes {
			if t.MinutesToArrival == nil || *t.MinutesToArrival < 0 {
				continue
			}
			out = append(out, Arrival{int(*t.MinutesToArrival + 0.5), t.ArrivalTimeString, t.IsRealTime})
			if t.IsRealTime {
				live[t.ArrivalTimeString] = true
			}
		}
	}
	// Upstream sometimes lists a bus both live and scheduled at the same time; keep the live one.
	out = slices.DeleteFunc(out, func(a Arrival) bool { return !a.Realtime && live[a.At] })
	sort.Slice(out, func(i, j int) bool { return out[i].Min < out[j].Min })
	if len(out) > maxArrivals {
		out = out[:maxArrivals]
	}
	return out, nil
}

// fetchStop returns the lines at a stop with upcoming arrivals, soonest first.
// to, if set, keeps only lines that reach that stop after this one (the direction);
// wanted filters by line number ("1" does not match "1א"); empty means all.
func fetchStop(ctx context.Context, code, to, label string, wanted []string, now time.Time) Stop {
	stop := Stop{Code: code, To: to, Label: label, Lines: []Line{}}
	routes, err := routesAtStop(ctx, code, now)
	if err != nil {
		stop.Error = err.Error()
		return stop
	}
	stop.Name = stopName(routes, code)
	if to != "" {
		stop.ToName = stopName(routes, to)
		var kept []route
		for _, r := range routes {
			if from, dest := r.seq(code), r.seq(to); from >= 0 && dest > from {
				kept = append(kept, r)
			}
		}
		if len(kept) == 0 && len(routes) > 0 {
			stop.Error = "אף קו מתחנה " + code + " לא מגיע לתחנה " + to
			return stop
		}
		routes = kept
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
			stop.Error = "הקווים בתחנה: " + strings.Join(avail, ", ")
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
	if len(stop.Lines) > maxLines {
		stop.Lines = stop.Lines[:maxLines]
	}
	return stop
}

// handleStops serves GET /stops?title=T&stop=CODE&to=CODE&lines=1,2&label=X[&stop=...&to=...&lines=...&label=...].
// Params pair up by position; a blank stop (an unused TRMNL slot) is skipped.
func handleStops(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	codes, tos, linesQ, labels := q["stop"], q["to"], q["lines"], q["label"]
	at := func(s []string, i int) string {
		if i < len(s) {
			return strings.TrimSpace(s[i])
		}
		return ""
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestBudget)
	defer cancel()
	now := time.Now()
	resp := Response{Title: strings.TrimSpace(q.Get("title")), Updated: now.In(israel).Format("15:04")}
	if resp.Title == "" {
		resp.Title = "אוטובוסים"
	}
	type ask struct {
		code, to, label string
		wanted          []string
	}
	var asks []ask
	for i := range codes {
		code := at(codes, i)
		if code == "" {
			continue
		}
		to := at(tos, i)
		if !stopCode.MatchString(code) || (to != "" && !stopCode.MatchString(to)) {
			http.Error(w, "stop and to must be numeric stop codes", http.StatusBadRequest)
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
		asks = append(asks, ask{code, to, at(labels, i), wanted})
	}
	resp.Stops = make([]Stop, len(asks))
	var wg sync.WaitGroup
	for i, a := range asks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp.Stops[i] = fetchStop(ctx, a.code, a.to, a.label, a.wanted, now)
		}()
	}
	wg.Wait()

	var buf bytes.Buffer
	if err := board.Execute(&buf, resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp.HTML = buf.String()
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
