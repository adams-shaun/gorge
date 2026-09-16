package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is the pacing/backoff clock a test injects into artCache.now,
// artCache.sleep and artCache.afterFunc: sleep RECORDS the pause and advances
// the fake clock, firing any timer it passes, so a test exercises the
// limiter, the 429 backoff and the fill budget without waiting real time.
// Like sleepCtx, a sleep returns ctx's error when ctx is already done or is
// cancelled while it waits (here: by a timer firing during the advance).
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
	timers []*fakeTimer
}

type fakeTimer struct {
	at   time.Time
	f    func()
	done bool
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	c.sleeps = append(c.sleeps, d)
	target := c.now.Add(d)
	c.mu.Unlock()
	for {
		c.mu.Lock()
		var next *fakeTimer
		for _, tm := range c.timers {
			if !tm.done && !tm.at.After(target) && (next == nil || tm.at.Before(next.at)) {
				next = tm
			}
		}
		if next == nil {
			c.now = target
			c.mu.Unlock()
			return nil
		}
		next.done = true
		if next.at.After(c.now) {
			c.now = next.at
		}
		c.mu.Unlock()
		next.f()
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

func (c *fakeClock) AfterFunc(d time.Duration, f func()) func() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	tm := &fakeTimer{at: c.now.Add(d), f: f}
	c.timers = append(c.timers, tm)
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		pending := !tm.done
		tm.done = true
		return pending
	}
}

func (c *fakeClock) recorded() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.sleeps...)
}

// tWriter routes the one-shot fill's stderr into the test log.
type tWriter struct{ t *testing.T }

func (w tWriter) Write(p []byte) (int, error) {
	w.t.Helper()
	w.t.Log(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// scryFixture builds an httptest server standing in for Scryfall with a
// scripted /cards/named response per call and a plain image endpoint. Each
// element of script is either a status code for a non-200 answer or the JSON
// body for a 200 one. Returns the server, the wired cache and the lookup
// count.
type scryFixture struct {
	srv     *httptest.Server
	ac      *artCache
	lookups int
	images  int
}

func newScryFixture(t *testing.T, script []string) *scryFixture {
	t.Helper()
	f := &scryFixture{}
	mux := http.NewServeMux()
	mux.HandleFunc("/cards/named", func(w http.ResponseWriter, r *http.Request) {
		f.lookups++
		if f.lookups > len(script) {
			http.Error(w, "script exhausted", http.StatusInternalServerError)
			return
		}
		switch s := script[f.lookups-1]; {
		case strings.HasPrefix(s, "{"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, s)
		case s == "429-retry":
			w.Header().Set("Retry-After", "1")
			http.Error(w, "slow down", http.StatusTooManyRequests)
		case s == "429":
			http.Error(w, "slow down", http.StatusTooManyRequests)
		default:
			http.Error(w, "scripted failure "+s, http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/img/", func(w http.ResponseWriter, r *http.Request) {
		f.images++
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg-bytes:" + r.URL.Path))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	ac, err := newArtCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ac.client = f.srv.Client()
	ac.namedBaseURL = f.srv.URL + "/cards/named?exact="
	ac.pace = 0 // prewarm tests pay no real pacing; see artCache.pace
	f.ac = ac
	return f
}

func (f *scryFixture) namedJSON(name string) string {
	return fmt.Sprintf(`{"name":%q,"image_uris":{"normal":"%s/img/%s.jpg"}}`,
		name, f.srv.URL, url.PathEscape(name))
}

// TestPrewarmSurvives429sAndCachesEveryName is the brief's 429 gate: a fake
// Scryfall answering 429 twice (once with Retry-After: 1, once without) then
// 200 must end with the name's art AND facts cached and the name NOT skipped.
// The fake clock records the backoff pauses — honouring Retry-After on the
// first (1s) and the exponential fallback on the second (500ms<<1 = 1s) — so
// no real time is waited.
func TestPrewarmSurvives429sAndCachesEveryName(t *testing.T) {
	script := []string{"429-retry", "429", ""}
	f := newScryFixture(t, script)
	script[2] = f.namedJSON("Alpha Card")
	clk := newFakeClock()
	f.ac.now = clk.Now
	f.ac.sleep = clk.Sleep

	prewarmArt(context.Background(), f.ac, prewarmNamedDir(t, "Alpha Card"), t.Logf)

	if got := clk.recorded(); len(got) != 2 || got[0] != time.Second || got[1] != time.Second {
		t.Errorf("backoff sleeps = %v, want [1s 1s] (Retry-After, then exponential)", got)
	}
	if f.lookups != 3 {
		t.Errorf("named lookups = %d, want 3 (two 429s, then the 200)", f.lookups)
	}
	if _, err := os.Stat(f.ac.jpgPath(artKey("Alpha Card"))); err != nil {
		t.Errorf("429 pass left no art on disk: %v", err)
	}
	if _, err := os.Stat(f.ac.factsPath(artKey("Alpha Card"))); err != nil {
		t.Errorf("429 pass left no facts sidecar on disk: %v", err)
	}
}

// TestExhausted429RetriesAreAFailureNotAMiss pins the other side of the
// retry loop: a Scryfall that 429s past scryMaxRetries leaves the name
// UNFETCHED — no .miss (a rate limit is not a fact about the card), the name
// counted failed, so the next pass or browser request retries it.
func TestExhausted429RetriesAreAFailureNotAMiss(t *testing.T) {
	script := make([]string, scryMaxRetries+1)
	for i := range script {
		script[i] = "429"
	}
	f := newScryFixture(t, script)
	clk := newFakeClock()
	f.ac.now = clk.Now
	f.ac.sleep = clk.Sleep

	dir := prewarmNamedDir(t, "Alpha Card")
	st, err := fillArt(context.Background(), f.ac, dir, "art fill", t.Logf, fillLimits{})
	if err != nil {
		t.Fatalf("fillArt: %v", err)
	}
	if st.Failed != 1 || st.Cached != 0 {
		t.Errorf("stats = %+v, want Failed=1 Cached=0", st)
	}
	if _, err := os.Stat(f.ac.missPath(artKey("Alpha Card"))); err == nil {
		t.Error("an exhausted 429 recorded a .miss; the name would never self-heal")
	}
	if got := len(clk.recorded()); got != scryMaxRetries {
		t.Errorf("backoff sleeps = %d, want %d (one per retry)", got, scryMaxRetries)
	}
}

// TestLimiterSpacesEveryScryfallRequest is the brief's limiter gate: with
// the REAL 100ms pace on a fake clock, every outbound Scryfall request start
// is spaced at least a.pace after the previous one. The sequence deliberately
// includes named metadata, the requested image, a multi-face sibling image,
// then another card's metadata and image; a limiter applied only to named
// lookups cannot pass. Real wall time must not move because the clock is fake.
func TestLimiterSpacesEveryScryfallRequest(t *testing.T) {
	clk := newFakeClock()
	var mu sync.Mutex
	var starts []time.Time
	var paths []string
	record := func(r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		starts = append(starts, clk.Now())
		paths = append(paths, r.URL.Path)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/cards/named", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("exact") {
		case "Alpha Card":
			fmt.Fprintf(w, `{"name":"Alpha Card // Alpha Back","card_faces":[
				{"name":"Alpha Card","image_uris":{"normal":"http://%s/img/alpha-front.jpg"}},
				{"name":"Alpha Back","image_uris":{"normal":"http://%s/img/alpha-back.jpg"}}]}`,
				r.Host, r.Host)
		case "Beta Card":
			fmt.Fprintf(w, `{"name":"Beta Card","image_uris":{"normal":"http://%s/img/beta.jpg"}}`, r.Host)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/img/", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = io.WriteString(w, "image:"+r.URL.Path)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ac, err := newArtCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ac.client = srv.Client()
	ac.namedBaseURL = srv.URL + "/cards/named?exact="
	ac.now = clk.Now
	ac.sleep = clk.Sleep
	ac.pace = 100 * time.Millisecond // the production pace, on the fake clock

	start := time.Now()
	prewarmArt(context.Background(), ac, prewarmNamedDir(t, "Alpha Card", "Beta Card"), t.Logf)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("pacing waited %v of REAL time; the clock injection leaked", elapsed)
	}

	mu.Lock()
	gotStarts := append([]time.Time(nil), starts...)
	gotPaths := append([]string(nil), paths...)
	mu.Unlock()
	wantPaths := []string{"/cards/named", "/img/alpha-front.jpg", "/img/alpha-back.jpg", "/cards/named", "/img/beta.jpg"}
	if fmt.Sprint(gotPaths) != fmt.Sprint(wantPaths) {
		t.Fatalf("request paths = %v, want %v", gotPaths, wantPaths)
	}
	for i := 1; i < len(gotStarts); i++ {
		if gap := gotStarts[i].Sub(gotStarts[i-1]); gap < ac.pace {
			t.Errorf("request %d (%s) started %v after %s, want >= %v", i, gotPaths[i], gap, gotPaths[i-1], ac.pace)
		}
	}

	// Five outbound requests need four fake 100ms waits. This assertion
	// catches both unpaced ordinary images and an unpaced sibling-face image.
	sleeps := clk.recorded()
	if len(sleeps) != 4 {
		t.Fatalf("recorded sleeps = %v, want four 100ms gaps (5 total requests)", sleeps)
	}
	for i, d := range sleeps {
		if d < ac.pace {
			t.Errorf("sleep[%d] = %v, want >= %v", i, d, ac.pace)
		}
	}
	if fakeElapsed := clk.Now().Sub(newFakeClock().now); fakeElapsed != 400*time.Millisecond {
		t.Errorf("fake clock advanced %v, want exactly 400ms of pacing", fakeElapsed)
	}
	if _, err := os.Stat(ac.jpgPath(artKey("Alpha Back"))); err != nil {
		t.Errorf("multi-face sibling image was not cached: %v", err)
	}
}

// TestFillArtCachesEveryDeckNameAndReports runs the fill loop over a temp
// deck dir: the first pass caches every fetchable name (stats say so), and a
// second pass over the same directory makes ZERO fetches — the fill is a
// no-op on a complete cache.
func TestFillArtCachesEveryDeckNameAndReports(t *testing.T) {
	ac, hits := artFixture(t, map[string]string{
		"Alpha Card": "/img/a.jpg",
		"Beta Card":  "/img/b.jpg",
		"Delta Card": "/img/d.jpg",
		"Gamma Card": "/img/g.jpg",
		// "Unknown Card" is deliberately absent: a genuine 404.
	})
	ac.pace = 0
	decks := prewarmFixture(t)

	st, err := fillArt(context.Background(), ac, decks, "art fill", t.Logf, fillLimits{})
	if err != nil {
		t.Fatalf("fillArt: %v", err)
	}
	want := artFillStats{Names: 5, Cached: 4, NotFound: 1}
	if st != want {
		t.Errorf("first pass stats = %+v, want %+v", st, want)
	}

	// Second pass: nothing to fetch, everything already present (the 404 is
	// re-measured from its .miss, still a fact).
	hits.Store(0)
	ac2, err := newArtCache(ac.dir)
	if err != nil {
		t.Fatal(err)
	}
	ac2.client = ac.client
	ac2.namedBaseURL = ac.namedBaseURL
	ac2.pace = 0
	st2, err := fillArt(context.Background(), ac2, decks, "art fill", t.Logf, fillLimits{})
	if err != nil {
		t.Fatalf("second fillArt: %v", err)
	}
	want2 := artFillStats{Names: 5, Present: 4, NotFound: 1}
	if st2 != want2 {
		t.Errorf("second pass stats = %+v, want %+v", st2, want2)
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("second pass made %d Scryfall requests, want 0", got)
	}
}

// TestWarmCacheServesWithoutScryfall is brief item 3's gate: on a warm cache
// every client route — /art/named, /cards/named, /art/blob — answers from
// disk with a fetcher that CANNOT reach the network at all (every transport
// round trip fails), proving a cached file is served without touching
// Scryfall.
func TestWarmCacheServesWithoutScryfall(t *testing.T) {
	ac, _ := artFixture(t, map[string]string{"Goblin Guide": "/img/gg.jpg"})
	ac.pace = 0
	dir := prewarmNamedDir(t, "Goblin Guide")
	prewarmArt(context.Background(), ac, dir, t.Logf)
	key := artKey("Goblin Guide")

	dead := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network is down")
	})}
	ac.client = dead
	// Also prove the serving routes through a FRESH cache object on the same
	// directory — a restarted server with a dead network still serves.
	ac2, err := newArtCache(ac.dir)
	if err != nil {
		t.Fatal(err)
	}
	ac2.client = dead

	w := httptest.NewRecorder()
	ac2.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Goblin+Guide", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/art/named on a warm cache with a dead network: want 200, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	ac2.text(w, httptest.NewRequest(http.MethodGet, "/cards/named?exact=Goblin+Guide", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/cards/named on a warm cache with a dead network: want 200, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	rb := httptest.NewRequest(http.MethodGet, "/art/blob/"+key+".jpg", nil)
	rb.SetPathValue("key", key+".jpg")
	ac2.blob(w, rb)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "fake-jpeg-bytes") {
		t.Fatalf("/art/blob on a warm cache with a dead network: got %d %q", w.Code, w.Body.String())
	}
}

// TestRunPrewarmArtOnlyExitCodeWiring drives the -prewarm-art-only body end
// to end: a complete fill exits 0, and a fill whose lookups all fail (a dead
// Scryfall) exits 1 with nothing recorded as a miss.
func TestRunPrewarmArtOnlyExitCodeWiring(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cards/named" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"name":"Alpha Card","image_uris":{"normal":"`+srvImg(r)+`"}}`)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("img:" + r.URL.Path))
	}))
	t.Cleanup(srv.Close)
	orig := newServeArtCache
	t.Cleanup(func() { newServeArtCache = orig })

	decks := prewarmNamedDir(t, "Alpha Card")

	// Healthy Scryfall: exit 0, artifact on disk.
	newServeArtCache = func(dir string) (*artCache, error) {
		ac, err := newArtCache(dir)
		if err != nil {
			return nil, err
		}
		ac.client = srv.Client()
		ac.namedBaseURL = srv.URL + "/cards/named?exact="
		ac.pace = 0
		return ac, nil
	}
	artDir := t.TempDir()
	if code := runPrewarmArtOnly(context.Background(), config{decks: decks, artDir: artDir}, tWriter{t}); code != 0 {
		t.Errorf("runPrewarmArtOnly on a healthy Scryfall = exit %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(artDir, artKey("Alpha Card")+".jpg")); err != nil {
		t.Errorf("one-shot fill left no art on disk: %v", err)
	}
	var facts cardFacts
	b, err := os.ReadFile(filepath.Join(artDir, artKey("Alpha Card")+".json"))
	if err != nil {
		t.Fatalf("one-shot fill left no facts sidecar: %v", err)
	}
	if err := json.Unmarshal(b, &facts); err != nil {
		t.Fatalf("facts sidecar: %v", err)
	}
	if facts.Name != "Alpha Card" {
		t.Errorf("facts.Name = %q, want the card's name", facts.Name)
	}

	// Dead Scryfall: exit 1, and no .miss (a network failure self-heals).
	newServeArtCache = func(dir string) (*artCache, error) {
		ac, err := newArtCache(dir)
		if err != nil {
			return nil, err
		}
		ac.client = srv.Client()
		ac.namedBaseURL = "http://127.0.0.1:1/cards/named?exact=" // nothing listens
		ac.pace = 0
		return ac, nil
	}
	if code := runPrewarmArtOnly(context.Background(), config{decks: decks, artDir: t.TempDir()}, tWriter{t}); code != 1 {
		t.Errorf("runPrewarmArtOnly on a dead Scryfall = exit %d, want 1", code)
	}
}

func srvImg(r *http.Request) string { return "http://" + r.Host + "/img/alpha.jpg" }

// TestSiblingImageFailureFailsTheFillAndRemainsRetryable is the sol2 review
// regression: a successful requested-face download followed by a failed
// sibling CDN download must make the one-shot exit non-zero, not claim the
// deck name was cached. No face manifest is published on that failure, so a
// later pass retries the SAME deck name instead of letting its warm primary
// face hide the missing sibling forever. Both entry shapes are exercised: a
// fully cold fetch and a legacy JPG whose facts need backfilling.
func TestSiblingImageFailureFailsTheFillAndRemainsRetryable(t *testing.T) {
	var failBack atomic.Bool
	failBack.Store(true)
	var frontHits, backHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/cards/named", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":"Alpha Card // Alpha Back","card_faces":[
			{"name":"Alpha Card","image_uris":{"normal":"http://%s/img/front.jpg"}},
			{"name":"Alpha Back","image_uris":{"normal":"http://%s/img/back.jpg"}}]}`,
			r.Host, r.Host)
	})
	mux.HandleFunc("/img/front.jpg", func(w http.ResponseWriter, _ *http.Request) {
		frontHits.Add(1)
		_, _ = io.WriteString(w, "front")
	})
	mux.HandleFunc("/img/back.jpg", func(w http.ResponseWriter, _ *http.Request) {
		backHits.Add(1)
		if failBack.Load() {
			http.Error(w, "expired sibling image", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, "back")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	newFixtureCache := func(dir string) (*artCache, error) {
		ac, err := newArtCache(dir)
		if err != nil {
			return nil, err
		}
		ac.client = srv.Client()
		ac.namedBaseURL = srv.URL + "/cards/named?exact="
		ac.pace = 0
		return ac, nil
	}
	decks := prewarmNamedDir(t, "Alpha Card")

	t.Run("cold fetch controls one-shot exit", func(t *testing.T) {
		orig := newServeArtCache
		newServeArtCache = newFixtureCache
		t.Cleanup(func() { newServeArtCache = orig })
		artDir := t.TempDir()

		if code := runPrewarmArtOnly(context.Background(), config{decks: decks, artDir: artDir}, tWriter{t}); code != 1 {
			t.Fatalf("one-shot with failed sibling image = exit %d, want 1", code)
		}
		if frontHits.Load() == 0 || backHits.Load() == 0 {
			t.Fatalf("downloads after first fill: front=%d back=%d, want both attempted", frontHits.Load(), backHits.Load())
		}
		if fileExists(filepath.Join(artDir, artKey("Alpha Back")+".jpg")) {
			t.Fatal("failed sibling image unexpectedly exists")
		}

		failBack.Store(false)
		if code := runPrewarmArtOnly(context.Background(), config{decks: decks, artDir: artDir}, tWriter{t}); code != 0 {
			t.Fatalf("one-shot retry after sibling recovers = exit %d, want 0", code)
		}
		for _, name := range []string{"Alpha Card", "Alpha Back"} {
			key := artKey(name)
			if !fileExists(filepath.Join(artDir, key+".jpg")) || !fileExists(filepath.Join(artDir, key+".json")) {
				t.Errorf("retry left %q incomplete", name)
			}
		}
	})

	t.Run("facts backfill propagates sibling failure", func(t *testing.T) {
		failBack.Store(true)
		artDir := t.TempDir()
		ac, err := newFixtureCache(artDir)
		if err != nil {
			t.Fatal(err)
		}
		frontKey := artKey("Alpha Card")
		if err := os.WriteFile(ac.jpgPath(frontKey), []byte("legacy front"), 0o644); err != nil {
			t.Fatal(err)
		}

		st, err := fillArt(context.Background(), ac, decks, "art fill", t.Logf, fillLimits{})
		if err != nil {
			t.Fatal(err)
		}
		if st.Failed != 1 || st.Cached != 0 {
			t.Fatalf("facts-backfill stats = %+v, want Failed=1 Cached=0", st)
		}
		if ac.complete(frontKey) || fileExists(ac.facesPath(frontKey)) {
			t.Fatal("a face manifest was published despite the failed sibling; the next pass would count the name present")
		}

		failBack.Store(false)
		st, err = fillArt(context.Background(), ac, decks, "art fill", t.Logf, fillLimits{})
		if err != nil {
			t.Fatal(err)
		}
		if st.Failed != 0 || st.Cached != 1 {
			t.Fatalf("facts-backfill retry stats = %+v, want Cached=1 Failed=0", st)
		}
		if !fileExists(ac.jpgPath(artKey("Alpha Back"))) || !fileExists(ac.factsPath(artKey("Alpha Back"))) {
			t.Fatal("facts-backfill retry did not complete the sibling cache")
		}
	})
}

// TestFetchCachesBothFacesOfAMultiFacedCard is brief item 2's DFC gate: one
// named lookup for a transform card's front face also lands the BACK face's
// art and facts sidecar (from the same response, zero extra API requests),
// so the fill covers both faces of every DFC/split card a deck names — and a
// later request for the back face is served entirely from disk.
func TestFetchCachesBothFacesOfAMultiFacedCard(t *testing.T) {
	ac, hits := artFixture(t, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/cards/named", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":"Delver of Secrets // Insectile Aberration","card_faces":[
			{"name":"Delver of Secrets","mana_cost":"{U}","type_line":"Creature — Human","power":"1","toughness":"1","image_uris":{"normal":"http://%s/img/front.jpg"}},
			{"name":"Insectile Aberration","mana_cost":"{2}{U}","type_line":"Creature — Insect","power":"3","toughness":"2","image_uris":{"normal":"http://%s/img/back.jpg"}}]}`,
			r.Host, r.Host)
	})
	mux.HandleFunc("/img/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("bytes:" + r.URL.Path))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ac.client = srv.Client()
	ac.namedBaseURL = srv.URL + "/cards/named?exact="
	ac.pace = 0

	w := httptest.NewRecorder()
	ac.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Delver+of+Secrets", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("front lookup: want 200, got %d", w.Code)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("named lookups = %d, want 1 (both faces come from one response)", got)
	}
	front, back := artKey("Delver of Secrets"), artKey("Insectile Aberration")
	if _, err := os.ReadFile(ac.jpgPath(front)); err != nil {
		t.Fatalf("front face art missing: %v", err)
	}
	if _, err := os.ReadFile(ac.jpgPath(back)); err != nil {
		t.Fatalf("back face art not cached alongside the front: %v", err)
	}
	b, err := os.ReadFile(ac.factsPath(back))
	if err != nil {
		t.Fatalf("back face facts not cached: %v", err)
	}
	var facts cardFacts
	if err := json.Unmarshal(b, &facts); err != nil {
		t.Fatal(err)
	}
	if facts.Name != "Insectile Aberration" || facts.TypeLine != "Creature — Insect" {
		t.Errorf("back face facts = %+v, want the back face's printed facts", facts)
	}

	// A client that now asks for the back face is served from disk: no new
	// Scryfall round trip, and the blob route serves the back bytes.
	ac.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network is down")
	})}
	w = httptest.NewRecorder()
	ac.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Insectile+Aberration", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("back lookup from disk: want 200, got %d", w.Code)
	}
	wb := httptest.NewRecorder()
	rb := httptest.NewRequest(http.MethodGet, "/art/blob/"+back+".jpg", nil)
	rb.SetPathValue("key", back+".jpg")
	ac.blob(wb, rb)
	if wb.Code != http.StatusOK || !strings.Contains(wb.Body.String(), "img/back.jpg") {
		t.Fatalf("back blob: got %d %q", wb.Code, wb.Body.String())
	}
}

// TestFillCompletesAWarmFrontFaceWhoseSiblingWasNeverCached is the sol2
// review regression: a deck name whose OWN art and facts were cached before
// sibling faces were kept (or by a fetch killed between the requested image
// and a sibling) has a .jpg and .json yet the client can still request an
// uncached back face. Completeness is decided by every face the named
// response lists — recorded in the face manifest — not by the requested
// name's files, so the fill must fetch the back face, count the name cached
// (or failed, exiting 1), and only then become a zero-request no-op.
func TestFillCompletesAWarmFrontFaceWhoseSiblingWasNeverCached(t *testing.T) {
	var failBack atomic.Bool
	var named, frontImg, backImg atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/cards/named", func(w http.ResponseWriter, r *http.Request) {
		named.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":"Alpha Card // Alpha Back","card_faces":[
			{"name":"Alpha Card","type_line":"Creature","image_uris":{"normal":"http://%s/img/front.jpg"}},
			{"name":"Alpha Back","type_line":"Creature — Back","image_uris":{"normal":"http://%s/img/back.jpg"}}]}`,
			r.Host, r.Host)
	})
	mux.HandleFunc("/img/front.jpg", func(w http.ResponseWriter, _ *http.Request) {
		frontImg.Add(1)
		_, _ = io.WriteString(w, "front")
	})
	mux.HandleFunc("/img/back.jpg", func(w http.ResponseWriter, _ *http.Request) {
		backImg.Add(1)
		if failBack.Load() {
			http.Error(w, "sibling unavailable", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, "back")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	orig := newServeArtCache
	t.Cleanup(func() { newServeArtCache = orig })
	newServeArtCache = func(dir string) (*artCache, error) {
		ac, err := newArtCache(dir)
		if err != nil {
			return nil, err
		}
		ac.client = srv.Client()
		ac.namedBaseURL = srv.URL + "/cards/named?exact="
		ac.pace = 0
		return ac, nil
	}
	decks := prewarmNamedDir(t, "Alpha Card")
	front, back := artKey("Alpha Card"), artKey("Alpha Back")
	// seedFrontOnly lays down exactly the pre-feature shape: the requested
	// name's v2 JPG and JSON, no sibling files, no face manifest.
	seedFrontOnly := func(t *testing.T) (string, *artCache) {
		t.Helper()
		dir := t.TempDir()
		ac, err := newServeArtCache(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ac.jpgPath(front), []byte("legacy front"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := ac.writeFacts(front, cardFacts{Name: "Alpha Card"}); err != nil {
			t.Fatal(err)
		}
		return dir, ac
	}
	reset := func() { named.Store(0); frontImg.Store(0); backImg.Store(0) }

	t.Run("fill fetches the missing sibling, then is a no-op", func(t *testing.T) {
		failBack.Store(false)
		reset()
		dir, ac := seedFrontOnly(t)
		st, err := fillArt(context.Background(), ac, decks, "art fill", t.Logf, fillLimits{})
		if err != nil {
			t.Fatal(err)
		}
		if want := (artFillStats{Names: 1, Cached: 1}); st != want {
			t.Fatalf("front-only cache stats = %+v, want %+v (not already present)", st, want)
		}
		if named.Load() != 1 || frontImg.Load() != 0 || backImg.Load() != 1 {
			t.Errorf("requests named=%d front=%d back=%d, want 1/0/1 (the warm front image is not re-downloaded)",
				named.Load(), frontImg.Load(), backImg.Load())
		}
		if b, err := os.ReadFile(ac.jpgPath(back)); err != nil || string(b) != "back" {
			t.Fatalf("back face art = %q %v, want the sibling image", b, err)
		}
		var facts cardFacts
		if b, err := os.ReadFile(ac.factsPath(back)); err != nil || json.Unmarshal(b, &facts) != nil || facts.TypeLine != "Creature — Back" {
			t.Fatalf("back face facts = %+v %v, want the back face's printed facts", facts, err)
		}

		reset()
		if code := runPrewarmArtOnly(context.Background(), config{decks: decks, artDir: dir}, tWriter{t}); code != 0 {
			t.Fatalf("second one-shot = exit %d, want 0", code)
		}
		if n := named.Load() + frontImg.Load() + backImg.Load(); n != 0 {
			t.Errorf("second one-shot made %d Scryfall requests, want 0", n)
		}

		// A sibling removed after completion (manifest present, file gone) is
		// incomplete again — the manifest is re-verified, not trusted blindly.
		if err := os.Remove(ac.jpgPath(back)); err != nil {
			t.Fatal(err)
		}
		reset()
		st, err = fillArt(context.Background(), ac, decks, "art fill", t.Logf, fillLimits{})
		if err != nil {
			t.Fatal(err)
		}
		if st.Cached != 1 || backImg.Load() != 1 {
			t.Errorf("removed-sibling pass stats = %+v back downloads = %d, want Cached=1 and one back download", st, backImg.Load())
		}
	})

	t.Run("failed sibling of a warm front face exits 1 and stays retryable", func(t *testing.T) {
		failBack.Store(true)
		reset()
		dir, ac := seedFrontOnly(t)
		if code := runPrewarmArtOnly(context.Background(), config{decks: decks, artDir: dir}, tWriter{t}); code != 1 {
			t.Fatalf("one-shot over a warm front with a failing sibling = exit %d, want 1", code)
		}
		if ac.complete(front) {
			t.Fatal("entry marked complete while its sibling is absent")
		}
		// A client that asks for the front face itself is still served from
		// disk: only the fill's completeness question fails.
		w := httptest.NewRecorder()
		ac.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Alpha+Card", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("/art/named for the warm front face = %d, want 200", w.Code)
		}

		failBack.Store(false)
		if code := runPrewarmArtOnly(context.Background(), config{decks: decks, artDir: dir}, tWriter{t}); code != 0 {
			t.Fatalf("retry after the sibling recovers = exit %d, want 0", code)
		}
		if !ac.complete(front) || !fileExists(ac.jpgPath(back)) {
			t.Fatal("retry did not complete the sibling face")
		}
	})
}

// TestFillCachesTheImagelessHalfOfASplitCard pins "all faces" for the layouts
// whose halves carry no image of their own — split cards, Rooms, adventures:
// Scryfall puts their art on the top level and lists the halves in
// card_faces without image_uris. A deck naming one half ("Boom") must leave
// the other half ("Bust") complete too, with the shared art and that half's
// own printed facts, because the wire can name either half; an image-bearing
// face filter would skip it and leave a client cold fetch behind.
func TestFillCachesTheImagelessHalfOfASplitCard(t *testing.T) {
	var named atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/cards/named", func(w http.ResponseWriter, r *http.Request) {
		named.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":"Boom // Bust","image_uris":{"normal":"http://%s/img/boom-bust.jpg"},"card_faces":[
			{"name":"Boom","type_line":"Sorcery","oracle_text":"Destroy target land you control and target land you don't control."},
			{"name":"Bust","type_line":"Sorcery","oracle_text":"Destroy all lands."}]}`, r.Host)
	})
	mux.HandleFunc("/img/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "bytes:"+r.URL.Path)
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
	decks := prewarmNamedDir(t, "Boom")

	st, err := fillArt(context.Background(), ac, decks, "art fill", t.Logf, fillLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if want := (artFillStats{Names: 1, Cached: 1}); st != want {
		t.Fatalf("stats = %+v, want %+v", st, want)
	}
	bust := artKey("Bust")
	if b, err := os.ReadFile(ac.jpgPath(bust)); err != nil || string(b) != "bytes:/img/boom-bust.jpg" {
		t.Fatalf("Bust art = %q %v, want the card's shared top-level image", b, err)
	}
	var facts cardFacts
	if b, err := os.ReadFile(ac.factsPath(bust)); err != nil || json.Unmarshal(b, &facts) != nil || facts.OracleText != "Destroy all lands." {
		t.Fatalf("Bust facts = %+v %v, want Bust's own printed facts", facts, err)
	}
	if !ac.complete(artKey("Boom")) {
		t.Fatal("Boom not complete after the fill")
	}

	named.Store(0)
	w := httptest.NewRecorder()
	ac.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Bust", nil))
	if w.Code != http.StatusOK || named.Load() != 0 {
		t.Fatalf("/art/named Bust after the fill = %d with %d Scryfall lookups, want 200 from disk", w.Code, named.Load())
	}
}
