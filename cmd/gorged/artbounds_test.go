package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// lockedBuffer collects the one-shot fill's stderr for assertions.
type lockedBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// seamCache points serve's art-cache seam at a fixture for one test: every
// cache the one-shot builds talks to namedBase with the given clock.
func seamCache(t *testing.T, client *http.Client, namedBase string, clk *fakeClock) {
	t.Helper()
	orig := newServeArtCache
	t.Cleanup(func() { newServeArtCache = orig })
	newServeArtCache = func(dir string) (*artCache, error) {
		ac, err := newArtCache(dir)
		if err != nil {
			return nil, err
		}
		ac.client = client
		ac.namedBaseURL = namedBase
		ac.pace = 0
		if clk != nil {
			ac.now, ac.sleep, ac.afterFunc = clk.Now, clk.Sleep, clk.AfterFunc
		}
		return ac, nil
	}
}

// TestArtFillBudgetStopsA429StormAndReportsTheSummary is the opus1 MAJOR's
// Go-side regression. Ten deck names against a Scryfall that answers every
// request 429 cost 60.5s each on the retry budget — 605s for ten, about 9.7h
// for the real 579 — and nothing bounded the pass. With
// -prewarm-art-budget 240s the one-shot must stop at exactly 240s of FAKE
// time (no real waiting), exit 1, and print the loud summary with the names
// it never reached, so the deploy can start its servers on time.
func TestArtFillBudgetStopsA429StormAndReportsTheSummary(t *testing.T) {
	script := make([]string, 200)
	for i := range script {
		script[i] = "429"
	}
	f := newScryFixture(t, script)
	clk := newFakeClock()
	seamCache(t, f.srv.Client(), f.srv.URL+"/cards/named?exact=", clk)
	var names []string
	for i := 1; i <= 10; i++ {
		names = append(names, fmt.Sprintf("Card %02d", i))
	}
	decks := prewarmNamedDir(t, names...)

	var out lockedBuffer
	fakeStart, realStart := clk.Now(), time.Now()
	code := runPrewarmArtOnly(context.Background(), config{decks: decks, artDir: t.TempDir(),
		prewarmArtBudget: 240 * time.Second}, &out)
	if real := time.Since(realStart); real > 5*time.Second {
		t.Errorf("the budgeted fill took %v of REAL time; the fake clock leaked", real)
	}
	t.Log(out.String())
	if code != 1 {
		t.Errorf("exit = %d, want 1 (the budget stopped an incomplete fill)", code)
	}
	if spent := clk.Now().Sub(fakeStart); spent != 240*time.Second {
		t.Errorf("fill ran %v of fake time, want exactly the 240s budget (unbounded: 605s)", spent)
	}
	for _, want := range []string{
		"STOPPED (fill time budget spent (4m0s)) after 4 of 10 names",
		"ART FILL INCOMPLETE (fill time budget spent (4m0s)): cached 0, already present 0, genuine 404 0, failed 4, 6 names unattempted",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

// TestArtFillStopsAfterAFailureStreak pins -prewarm-art-max-consecutive-failures:
// against a Scryfall erroring on most names, the one-shot stops once N names
// IN A ROW have failed instead of sending one request per deck name, and a
// name that completes in between resets the streak. Sorted order here is
// fail, fail, OK, fail, fail, fail, fail, fail: with N=3 the pass must reach
// exactly six names (six lookups) and leave two unattempted.
func TestArtFillStopsAfterAFailureStreak(t *testing.T) {
	var lookups atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cards/named" {
			_, _ = io.WriteString(w, "img")
			return
		}
		lookups.Add(1)
		name := r.URL.Query().Get("exact")
		if strings.Contains(name, "Bad") {
			http.Error(w, "maintenance", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintf(w, `{"name":%q,"image_uris":{"normal":"http://%s/img/good.jpg"}}`, name, r.Host)
	}))
	t.Cleanup(srv.Close)
	seamCache(t, srv.Client(), srv.URL+"/cards/named?exact=", nil)
	decks := prewarmNamedDir(t, "A1 Bad", "A2 Bad", "B Good", "C1 Bad", "C2 Bad", "C3 Bad", "C4 Bad", "C5 Bad")

	var out lockedBuffer
	code := runPrewarmArtOnly(context.Background(), config{decks: decks, artDir: t.TempDir(),
		prewarmArtMaxConsecutiveFailures: 3}, &out)
	t.Log(out.String())
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if got := lookups.Load(); got != 6 {
		t.Errorf("named lookups = %d, want 6 (A1, A2, B resets the streak, C1..C3 trip it)", got)
	}
	want := "ART FILL INCOMPLETE (too many consecutive failures (3 in a row)): cached 1, already present 0, genuine 404 0, failed 5, 2 names unattempted"
	if !strings.Contains(out.String(), want) {
		t.Errorf("output lacks %q", want)
	}
}

// TestRetryAfterIsCapped is the opus1 Retry-After regression: every 429
// backoff sleeps holding the pacing semaphore, so a server-sent
// `Retry-After: 86400` used to park every cold art fetch in the process for
// a day. The delay is capped at scryMaxRetryAfter, including for values a
// Duration cannot hold, and a real 429 through the fetch path sleeps the cap.
func TestRetryAfterIsCapped(t *testing.T) {
	for header, want := range map[string]time.Duration{
		"2":     2 * time.Second,
		"0.25":  250 * time.Millisecond,
		"60":    60 * time.Second,
		"61":    scryMaxRetryAfter,
		"86400": scryMaxRetryAfter,
		"1e30":  scryMaxRetryAfter,
		"Inf":   scryMaxRetryAfter,
		"NaN":   500 * time.Millisecond, // not a positive number: the exponential fallback
	} {
		if got := scryRetryAfter(header, 0); got != want {
			t.Errorf("scryRetryAfter(%q) = %v, want %v", header, got, want)
		}
	}

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cards/named" {
			_, _ = io.WriteString(w, "img")
			return
		}
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "86400")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		fmt.Fprintf(w, `{"name":"Alpha Card","image_uris":{"normal":"http://%s/img/a.jpg"}}`, r.Host)
	}))
	t.Cleanup(srv.Close)
	ac, err := newArtCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clk := newFakeClock()
	ac.client, ac.namedBaseURL, ac.pace = srv.Client(), srv.URL+"/cards/named?exact=", 0
	ac.now, ac.sleep = clk.Now, clk.Sleep
	if ok, err := ac.ensure(context.Background(), artKey("Alpha Card"), "Alpha Card"); !ok || err != nil {
		t.Fatalf("ensure = %v, %v; want the card cached after one capped backoff", ok, err)
	}
	if got := clk.recorded(); len(got) != 1 || got[0] != scryMaxRetryAfter {
		t.Errorf("backoff sleeps = %v, want [%v]", got, scryMaxRetryAfter)
	}
}

// TestLegacyEntryWhoseNameNow404sSettles is the opus1 re-lookup regression:
// an entry cached with art and facts (no face manifest) whose name Scryfall
// no longer knows was looked up again on EVERY pass, because only a .miss
// WITHOUT a .jpg counted as settled. Three passes must make one lookup, each
// reporting the genuine 404, while the client route keeps serving the old
// image from disk.
func TestLegacyEntryWhoseNameNow404sSettles(t *testing.T) {
	ac, hits := artFixture(t, map[string]string{}) // Scryfall knows no name
	ac.pace = 0
	key := artKey("Renamed Card")
	if err := os.WriteFile(ac.jpgPath(key), []byte("legacy-jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ac.writeFacts(key, cardFacts{Name: "Renamed Card"}); err != nil {
		t.Fatal(err)
	}
	decks := prewarmNamedDir(t, "Renamed Card")
	for pass := 1; pass <= 3; pass++ {
		st, err := fillArt(context.Background(), ac, decks, "art fill", t.Logf, fillLimits{})
		if err != nil {
			t.Fatal(err)
		}
		if want := (artFillStats{Names: 1, NotFound: 1}); st != want {
			t.Errorf("pass %d stats = %+v, want %+v", pass, st, want)
		}
		if got := hits.Load(); got != 1 {
			t.Errorf("after pass %d: %d named lookups, want 1 (the 404 settles the entry)", pass, got)
		}
	}
	w := httptest.NewRecorder()
	ac.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Renamed+Card", nil))
	if w.Code != http.StatusOK || hits.Load() != 1 {
		t.Errorf("/art/named on the legacy image = %d with %d lookups, want 200 from disk", w.Code, hits.Load())
	}
}

// TestFillRefusesAnUnwritableArtDirBeforeAnyLookup is the opus1 read-only
// regression: from a jail that binds the art dir read-only, the one-shot
// used to spend one Scryfall lookup per deck name before each first write
// failed (579 on the real decks). One probe write now fails the pass with
// zero requests and a message naming the directory.
func TestFillRefusesAnUnwritableArtDirBeforeAnyLookup(t *testing.T) {
	fx, hits := artFixture(t, map[string]string{"Alpha Card": "/img/a.jpg", "Beta Card": "/img/b.jpg"})
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if f, err := os.CreateTemp(dir, "probe"); err == nil {
		// Privileged (e.g. root in a user namespace) ignores the mode bits; a
		// path under a regular file is unwritable for everyone.
		_ = f.Close()
		_ = os.Remove(f.Name())
		file := filepath.Join(t.TempDir(), "plain-file")
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		dir = filepath.Join(file, "art")
	}
	orig := newServeArtCache
	t.Cleanup(func() { newServeArtCache = orig })
	newServeArtCache = func(string) (*artCache, error) {
		ac, err := newArtCache(t.TempDir())
		if err != nil {
			return nil, err
		}
		ac.dir = dir
		ac.client, ac.namedBaseURL, ac.pace = fx.client, fx.namedBaseURL, 0
		return ac, nil
	}

	var out lockedBuffer
	code := runPrewarmArtOnly(context.Background(), config{decks: prewarmNamedDir(t, "Alpha Card", "Beta Card"), artDir: dir}, &out)
	t.Log(out.String())
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("named lookups = %d, want 0 (nothing can be stored, so nothing is asked)", got)
	}
	if !strings.Contains(out.String(), "is not writable") || !strings.Contains(out.String(), "ART FILL INCOMPLETE") {
		t.Errorf("output does not name the unwritable dir loudly")
	}
}

// TestScryfallPaceIsSharedByEveryCacheOnTheDirectory is the opus1 pacing
// regression: the limiter was per process, so the deploy fill, the old
// servers it overlaps and both new servers could each send 10 req/s. Two
// caches on ONE directory (two processes, as far as the lock is concerned)
// with the real 100ms pace on a shared fake clock must space all four of
// their requests — B's first request waits on A's last — so the combined
// stream never exceeds one pace.
func TestScryfallPaceIsSharedByEveryCacheOnTheDirectory(t *testing.T) {
	clk := newFakeClock()
	var mu sync.Mutex
	var starts []time.Time
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		starts = append(starts, clk.Now())
		paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()
		if r.URL.Path == "/cards/named" {
			name := r.URL.Query().Get("exact")
			fmt.Fprintf(w, `{"name":%q,"image_uris":{"normal":"http://%s/img/%d.jpg"}}`, name, r.Host, len(name))
			return
		}
		_, _ = io.WriteString(w, "img")
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	caches := make([]*artCache, 2)
	for i := range caches {
		ac, err := newArtCache(dir)
		if err != nil {
			t.Fatal(err)
		}
		ac.client, ac.namedBaseURL = srv.Client(), srv.URL+"/cards/named?exact="
		ac.now, ac.sleep = clk.Now, clk.Sleep
		ac.pace = 100 * time.Millisecond
		caches[i] = ac
	}
	for i, name := range []string{"Alpha Card", "Beta Card"} {
		if ok, err := caches[i].ensure(context.Background(), artKey(name), name); !ok || err != nil {
			t.Fatalf("cache %d ensure(%q) = %v, %v", i, name, ok, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(starts) != 4 {
		t.Fatalf("requests = %v, want named+image for each cache", paths)
	}
	for i := 1; i < len(starts); i++ {
		if gap := starts[i].Sub(starts[i-1]); gap < 100*time.Millisecond {
			t.Errorf("request %d (%s) started %v after %s, want >= 100ms across caches", i, paths[i], gap, paths[i-1])
		}
	}
	if got := clk.recorded(); len(got) != 3 {
		t.Errorf("pace sleeps = %v, want three 100ms gaps for four requests", got)
	}
}

// TestPaceLockExcludesOtherDescriptorsAndCancelsCleanly pins the lock under
// the shared pace. flock belongs to the open file description, so a second
// open of the stamp file — which is what another process has — cannot take
// it while it is held. A waiter whose context ends gives up promptly, and the
// lock its abandoned flock call later acquires is released, not leaked: the
// next waiter still gets it once the holder lets go.
func TestPaceLockExcludesOtherDescriptorsAndCancelsCleanly(t *testing.T) {
	dir := t.TempDir()
	a, err := newArtCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := newArtCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	held, err := a.lockPace(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	other, err := os.OpenFile(filepath.Join(dir, paceFile), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(other.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Errorf("a second descriptor took the held pace lock (err %v); another process would too", err)
	}
	_ = other.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if f, err := b.lockPace(ctx); !errors.Is(err, context.Canceled) {
		if f != nil {
			_ = f.Close()
		}
		t.Fatalf("waiter with a done context = %v, want context.Canceled", err)
	}

	_ = held.Close()
	got := make(chan error, 1)
	go func() {
		f, err := b.lockPace(context.Background())
		if f != nil {
			_ = f.Close()
		}
		got <- err
	}()
	select {
	case err := <-got:
		if err != nil {
			t.Fatalf("lock after release: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the pace lock stayed held after its holder closed: the cancelled waiter leaked it")
	}
}
