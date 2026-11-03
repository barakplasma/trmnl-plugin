package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeUpstream serves testdata fixtures in place of api.bus.gov.il.
func fakeUpstream(t *testing.T, routeCalls *atomic.Int32) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file := "testdata/times.json"
		switch {
		case strings.HasSuffix(r.URL.Path, "RefreshStopTimesAtStop"):
			routeCalls.Add(1)
			if r.URL.Query().Get("day") == "" {
				t.Errorf("routes call without day param")
			}
			file = "testdata/routes.json"
		case r.URL.Query().Get("routeDesc") == "12155-2-0":
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	apiBase = srv.URL
	cache = map[string]cachedRoutes{}
}

func get(t *testing.T, query string) (int, Response) {
	t.Helper()
	rec := httptest.NewRecorder()
	handleStops(rec, httptest.NewRequest("GET", "/stops?"+query, nil))
	var resp Response
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
	}
	return rec.Code, resp
}

func TestStops(t *testing.T) {
	var routeCalls atomic.Int32
	fakeUpstream(t, &routeCalls)

	// Slot 2 is blank (unused TRMNL field); slot 3 asks for a line not at the stop.
	code, resp := get(t, "stop=27997&lines=124,155&label=Home&stop=&lines=&label=&stop=27997&lines=999&label=Work")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if len(resp.Stops) != 2 {
		t.Fatalf("want 2 stops (blank skipped), got %d", len(resp.Stops))
	}

	home := resp.Stops[0]
	if home.Label != "Home" || home.Error != "" {
		t.Fatalf("home: %+v", home)
	}
	// 124 and 155 (direction 1) share the fixture times; 155 direction 0 fails upstream (partial failure).
	if len(home.Lines) != 2 {
		t.Fatalf("want 2 lines, got %+v", home.Lines)
	}
	got := home.Lines[0]
	if got.Line != "124" || got.Operator != "מטרופולין" || strings.Contains(got.Direction, "_") {
		t.Errorf("line: %+v", got)
	}
	// Negative minutes dropped, scheduled duplicate of a live 20:13 dropped, rounded, sorted.
	want := []Arrival{{13, "20:13", true}, {39, "20:40", false}}
	if len(got.Arrivals) != 2 || got.Arrivals[0] != want[0] || got.Arrivals[1] != want[1] {
		t.Errorf("arrivals: %+v", got.Arrivals)
	}

	if work := resp.Stops[1]; !strings.Contains(work.Error, "124, 155") {
		t.Errorf("unknown line should list available lines: %+v", work)
	}

	// Routes list is cached per stop + day.
	before := routeCalls.Load()
	get(t, "stop=27997&lines=124")
	if n := routeCalls.Load(); n != before {
		t.Errorf("routes refetched despite cache: %d -> %d", before, n)
	}
}

// testdata/routes.json: 27997 is the last stop of every route, except that
// line 124 got a made-up stop 99999 after it.
func TestDirection(t *testing.T) {
	var routeCalls atomic.Int32
	fakeUpstream(t, &routeCalls)
	_, resp := get(t, "stop=27997&to=99999&stop=27997&to=21534")
	toward := resp.Stops[0]
	if toward.Error != "" || len(toward.Lines) != 1 || toward.Lines[0].Line != "124" {
		t.Errorf("only 124 continues to 99999: %+v", toward)
	}
	if toward.Name == "" || toward.ToName != "תחנת בדיקה" {
		t.Errorf("stop names: %q -> %q", toward.Name, toward.ToName)
	}
	// 21534 is before 27997 on 124: wrong direction, so nothing qualifies.
	if back := resp.Stops[1]; back.Error == "" || len(back.Lines) != 0 {
		t.Errorf("wrong direction should error: %+v", back)
	}
	if code, _ := get(t, "stop=27997&to=abc"); code != http.StatusBadRequest {
		t.Errorf("non-numeric to: %d", code)
	}
}

func TestAllRouteLookupsFailIsAnError(t *testing.T) {
	var routeCalls atomic.Int32
	fakeUpstream(t, &routeCalls)
	_, resp := get(t, "stop=27997&lines=155&label=x")
	// 155 has two routes; one fails, one succeeds -> no error.
	if resp.Stops[0].Error != "" {
		t.Fatalf("partial failure should not error: %+v", resp.Stops[0])
	}
	apiBase = "http://127.0.0.1:1" // nothing listening; cache still holds routes
	_, resp = get(t, "stop=27997&lines=155&label=x")
	if resp.Stops[0].Error == "" {
		t.Fatalf("all lookups failed, want error: %+v", resp.Stops[0])
	}
}

func TestBadInput(t *testing.T) {
	var routeCalls atomic.Int32
	fakeUpstream(t, &routeCalls)
	if code, _ := get(t, "stop=12a"); code != http.StatusBadRequest {
		t.Errorf("non-numeric stop: %d", code)
	}
	if code, _ := get(t, "stop=1&stop=2&stop=3&stop=4&stop=5"); code != http.StatusBadRequest {
		t.Errorf("too many stops: %d", code)
	}
}
