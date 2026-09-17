package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// The fb-20260917T004304Z smoke-gate wedge, as a unit test. The upstream
// catalog hangs (the review run's degraded network): without the wait budget
// the /art/named handler holds the browser's connection for the full
// serialized fetch chain — each request up to the outbound client's 15s
// timeout, seven cold-cache cards one after another — and the browser's
// six-per-origin connection budget pins behind the burst, starving the page's
// OWN same-origin API traffic (pending/view/events) so the client can never
// answer the engine and the game wedges. The budget must release the
// connection with 503 quickly while the fetch keeps running detached and
// settles the cache, so a later request is a plain 200 hit.

// blockingArtFixture is artFixture whose upstream blocks every /cards/named
// request until release is closed (and then answers like the plain fixture).
func blockingArtFixture(t *testing.T, name, imgPath string) (*artCache, *sync.Mutex, *bool, func()) {
	t.Helper()
	var mu sync.Mutex
	released := false
	var img string // rewritten onto srv once its address is known
	unblock := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/cards/named", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-unblock:
		case <-r.Context().Done():
			return
		}
		mu.Lock()
		released = true
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":       name,
			"image_uris": map[string]string{"normal": img},
		})
	})
	mux.HandleFunc("/img/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg-bytes:" + r.URL.Path))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ac, err := newArtCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ac.client = srv.Client()
	ac.namedBaseURL = srv.URL + "/cards/named?exact="
	ac.pace = 0
	ac.waitBudget = 100 * time.Millisecond
	// Like artFixture: the fixture names a relative image path; the real
	// Scryfall response carries absolute URLs, so rewrite it onto srv before
	// any lookup can fire (the client routes here run after this returns).
	img = srv.URL + imgPath
	return ac, &mu, &released, sync.OnceFunc(func() { close(unblock) })
}

func TestArtRouteAnswersWithinTheWaitBudgetWhileTheUpstreamHangs(t *testing.T) {
	ac, _, _, release := blockingArtFixture(t, "Goblin Guide", "/img/gg.jpg")
	t.Cleanup(release) // the detached fetch must be able to finish before the temp dir goes

	start := time.Now()
	w := httptest.NewRecorder()
	ac.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Goblin+Guide", nil))
	elapsed := time.Since(start)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("hung upstream: /art/named answered %d, want 503", w.Code)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("hung upstream: /art/named held its connection %v, want ~the 100ms budget", elapsed)
	}

	// The fetch was NOT cancelled: release the upstream and wait for the
	// detached goroutine to settle the cache on disk.
	release()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ac.mu.Lock()
		_, live := ac.inflight[artKey("Goblin Guide")]
		ac.mu.Unlock()
		if !live && fileExists(ac.jpgPath(artKey("Goblin Guide"))) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !fileExists(ac.jpgPath(artKey("Goblin Guide"))) {
		t.Fatal("the detached fetch never settled the image on disk")
	}

	// And the settled entry now serves as a plain hit.
	w2 := httptest.NewRecorder()
	ac.named(w2, httptest.NewRequest(http.MethodGet, "/art/named?exact=Goblin+Guide", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("settled cache: /art/named answered %d, want 200", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), "/art/blob/") {
		t.Fatalf("settled cache: body %q does not point at the blob route", w2.Body.String())
	}
}

// The same contract on the facts route, which the client's oracle text reads.
func TestTextRouteAnswersWithinTheWaitBudgetWhileTheUpstreamHangs(t *testing.T) {
	ac, _, _, release := blockingArtFixture(t, "Llanowar Elves", "/img/le.jpg")
	t.Cleanup(release)

	start := time.Now()
	w := httptest.NewRecorder()
	ac.text(w, httptest.NewRequest(http.MethodGet, "/cards/named?exact=Llanowar+Elves", nil))
	elapsed := time.Since(start)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("hung upstream: /cards/named answered %d, want 503", w.Code)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("hung upstream: /cards/named held its connection %v, want ~the 100ms budget", elapsed)
	}

	release()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ac.mu.Lock()
		_, live := ac.inflight[artKey("Llanowar Elves")]
		ac.mu.Unlock()
		if !live && fileExists(ac.factsPath(artKey("Llanowar Elves"))) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !fileExists(ac.factsPath(artKey("Llanowar Elves"))) {
		t.Fatal("the detached fetch never settled the facts on disk")
	}
	w2 := httptest.NewRecorder()
	ac.text(w2, httptest.NewRequest(http.MethodGet, "/cards/named?exact=Llanowar+Elves", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("settled cache: /cards/named answered %d, want 200", w2.Code)
	}
}

// A fast upstream must be untouched by the budget: the ordinary cold-cache
// fetch completes inside the budget and answers 200 on the very first
// request — the pre-fix behaviour every existing art test already pins, so
// this guard only exists to say out loud that the budget does not cost the
// healthy path its answer.
func TestArtRouteStillAnswersAHealthyUpstreamInsideTheBudget(t *testing.T) {
	ac, hits := artFixture(t, map[string]string{"Goblin Guide": "/img/gg.jpg"})
	w := httptest.NewRecorder()
	ac.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Goblin+Guide", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("healthy upstream: /art/named answered %d, want 200", w.Code)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("healthy upstream: fetch count %d, want 1", got)
	}
}
