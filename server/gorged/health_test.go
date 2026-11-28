package gorged

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
)

func TestHealthzIsOK(t *testing.T) {
	rec := httptest.NewRecorder()
	healthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "ok\n" {
		t.Fatalf("body %q, want %q", got, "ok\n")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("content type %q", ct)
	}
}

func TestMetricsListsTheGauges(t *testing.T) {
	r, err := host.New(host.Options{LoadDeck: gameLoader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, id := range []host.TableID{"t1", "t2"} {
		if err := r.AddTable(host.TableConfig{ID: id, Name: string(id), Seats: 2, Decks: []string{"a", "b"}, Seed: 1, Spectator: view.Public}); err != nil {
			t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	metricsHandler(r)(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("content type %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"gorged_tables{state=\"idle\"} 2\n",
		"gorged_tables{state=\"live\"} 0\n",
		"gorged_tables{state=\"cooldown\"} 0\n",
		"gorged_tables{state=\"halted\"} 0\n",
		"gorged_on_demand_tables 0\n",
		"gorged_matches_stored ",
		"gorged_search_slots_busy 0\n",
		"# TYPE gorged_matches_stored gauge\n",
		"# HELP gorged_tables ",
		"# TYPE gorged_tables gauge\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q:\n%s", want, body)
		}
	}
	// the series come in the documented order
	prev := -1
	for _, s := range []string{`state="live"`, `state="cooldown"`, `state="idle"`, `state="halted"`, "gorged_on_demand_tables ", "gorged_matches_stored ", "gorged_search_slots_busy "} {
		i := strings.Index(body, s)
		if i <= prev {
			t.Errorf("%q out of order in:\n%s", s, body)
		}
		prev = i
	}
}

func TestMetricsIsOffByDefault(t *testing.T) {
	fs := ServeFlagSet()
	f := fs.Lookup("metrics")
	if f == nil {
		t.Fatal("no -metrics flag")
	}
	if f.DefValue != "false" {
		t.Fatalf("default %q, want false", f.DefValue)
	}
	if err := fs.Parse([]string{"-metrics"}); err != nil {
		t.Fatal(err)
	}
	if f.Value.String() != "true" {
		t.Fatalf("-metrics parsed to %q", f.Value.String())
	}
}
