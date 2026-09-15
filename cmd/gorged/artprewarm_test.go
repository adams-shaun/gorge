package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/internal/testutil"
)

// prewarmFixture writes two minimal deck files (deck.Parse needs no corpus —
// names are read straight off the JSON, so no .cards dependency) naming
// Alpha, Beta and Gamma, with Beta shared between the decks and Delta only
// as a commander. Returns the deck dir.
func prewarmFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("deck-a.json", `{"name":"A","cards":[{"name":"Beta Card","count":4},{"name":"Alpha Card","count":20}]}`)
	write("deck-b.json", `{"name":"B","commander":"Delta Card","cards":[{"name":"Gamma Card","count":4},{"name":"Beta Card","count":4},{"name":"Unknown Card","count":4}]}`)
	return dir
}

// TestDeckCardNamesIsSortedDistinctAndIncludesCommanders pins the prewarm
// name set: every card name of every deck file plus each commander, deduped
// across decks and sorted (the prewarm walks it in this order).
func TestDeckCardNamesIsSortedDistinctAndIncludesCommanders(t *testing.T) {
	dir := prewarmFixture(t)
	names, err := deckCardNames(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Alpha Card", "Beta Card", "Delta Card", "Gamma Card", "Unknown Card"}
	if len(names) != len(want) {
		t.Fatalf("deckCardNames = %q, want %q", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("deckCardNames[%d] = %q, want %q (full list %q)", i, names[i], want[i], names)
		}
	}
}

// TestPrewarmFillsColdCacheWithoutAnyClientRequest proves the prewarm fills
// a cold cache for every deck name on its own, and that a server restarted
// on that cache then serves every deck card — art AND facts — with ZERO
// Scryfall requests. The fixture's httptest server stands in for Scryfall
// (artFixture) and counts named lookups.
func TestPrewarmFillsColdCacheWithoutAnyClientRequest(t *testing.T) {
	cards := map[string]string{
		"Alpha Card": "/img/a.jpg",
		"Beta Card":  "/img/b.jpg",
		"Delta Card": "/img/d.jpg",
		"Gamma Card": "/img/g.jpg",
		// "Unknown Card" is deliberately absent: the fixture answers its named
		// lookup 404, exactly as Scryfall would.
	}
	ac, hits := artFixture(t, cards)
	ac.pace = 0 // the prewarm tests pay no Scryfall pacing; see artCache.pace
	decks := prewarmFixture(t)

	// No client request has been made — the cold cache is filled by prewarm
	// alone. "Unknown Card" is a genuine 404: prewarm records its .miss and
	// moves on, exactly as it must (a miss is a fact, not a failure).
	prewarmArt(context.Background(), ac, decks, t.Logf)
	for _, name := range []string{"Alpha Card", "Beta Card", "Delta Card", "Gamma Card"} {
		key := artKey(name)
		if _, err := os.Stat(ac.jpgPath(key)); err != nil {
			t.Errorf("prewarm left no art for %q: %v", name, err)
		}
		if _, err := os.Stat(ac.factsPath(key)); err != nil {
			t.Errorf("prewarm left no facts sidecar for %q: %v", name, err)
		}
	}
	if _, err := os.Stat(ac.missPath(artKey("Unknown Card"))); err != nil {
		t.Errorf("prewarm recorded no .miss for the unknown card: %v", err)
	}
	before := hits.Load()
	if before == 0 {
		t.Fatalf("fixture counted no lookups at all; the prewarm never fetched")
	}

	// A restarted server on the same (now warm) cache directory: every deck
	// name answers over HTTP with zero further Scryfall requests.
	ac2, err := newArtCache(ac.dir)
	if err != nil {
		t.Fatal(err)
	}
	ac2.client = ac.client
	ac2.namedBaseURL = ac.namedBaseURL
	hits.Store(0)
	for _, name := range []string{"Alpha Card", "Beta Card", "Delta Card", "Gamma Card"} {
		w := httptest.NewRecorder()
		ac2.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact="+url.QueryEscape(name), nil))
		if w.Code != http.StatusOK {
			t.Errorf("warm /art/named %q: want 200, got %d: %s", name, w.Code, w.Body.String())
		}
		var body struct {
			ImageURIs struct {
				Normal string `json:"normal"`
			} `json:"image_uris"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Errorf("warm /art/named %q body: %v", name, err)
		}
		w = httptest.NewRecorder()
		ac2.text(w, httptest.NewRequest(http.MethodGet, "/cards/named?exact="+url.QueryEscape(name), nil))
		if w.Code != http.StatusOK {
			t.Errorf("warm /cards/named %q: want 200, got %d", name, w.Code)
		}
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("warm cache made %d Scryfall named requests on restart, want 0", got)
	}
}

// TestArtCacheDirDefaultsUnderDir pins the -art-dir contract: empty keeps the
// historical <dir>/art (every existing deployment, test and local run is
// unchanged), an explicit value wins.
func TestArtCacheDirDefaultsUnderDir(t *testing.T) {
	if got := (config{dir: "/data"}).artCacheDir(); got != filepath.Join("/data", "art") {
		t.Errorf("empty -art-dir: got %q, want <dir>/art", got)
	}
	if got := (config{dir: "/data", artDir: "/mnt/sata/gorge-data/art"}).artCacheDir(); got != "/mnt/sata/gorge-data/art" {
		t.Errorf("set -art-dir: got %q, want the flag's value", got)
	}
}

// TestPrewarmSurvivesACancelledFetch checks the self-heal claim THROUGH the
// prewarm loop itself (a round-2 review finding): prewarmArt run against a
// dead context — a server shutting down mid-prewarm — logs every fetch's
// error, wedges on nothing, and writes no .miss behind (only a genuine
// Scryfall 404 does); the SAME name is then fetched for real by a retried
// prewarmArt on a live context, art and facts both. The test fails if the
// prewarm loop is removed entirely (the retry fills nothing) as well as if
// a cancelled fetch recorded a miss.
func TestPrewarmSurvivesACancelledFetch(t *testing.T) {
	ac, _ := artFixture(t, map[string]string{"Alpha Card": "/img/a.jpg"})
	ac.pace = 0
	dir := prewarmNamedDir(t, "Alpha Card")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	prewarmArt(ctx, ac, dir, t.Logf)
	if _, err := os.Stat(ac.missPath(artKey("Alpha Card"))); err == nil {
		t.Fatal("a cancelled prewarm wrote a .miss; the name would never self-heal")
	}
	prewarmArt(context.Background(), ac, dir, t.Logf)
	if _, err := os.Stat(ac.jpgPath(artKey("Alpha Card"))); err != nil {
		t.Fatalf("the retried prewarm did not fill the cache: %v", err)
	}
	if _, err := os.Stat(ac.factsPath(artKey("Alpha Card"))); err != nil {
		t.Fatalf("the retried prewarm left no facts sidecar: %v", err)
	}
}

// TestPrewarmBackfillsFactsForALegacyArtOnlyCache covers the second prewarm
// arm: a name whose art was cached before sidecars were kept (jpg present,
// json absent) gets its sidecar from prewarm's ensureText without the image
// being fetched again.
func TestPrewarmBackfillsFactsForALegacyArtOnlyCache(t *testing.T) {
	ac, hits := artFixture(t, map[string]string{"Alpha Card": "/img/a.jpg"})
	ac.pace = 0
	key := artKey("Alpha Card")
	// Forge a legacy entry: art on disk, no sidecar, as if fetched by the
	// pre-sidecar build.
	if err := os.WriteFile(ac.jpgPath(key), []byte("legacy-jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	prewarmArt(context.Background(), ac, prewarmNamedDir(t, "Alpha Card"), t.Logf)
	if _, err := os.Stat(ac.factsPath(key)); err != nil {
		t.Fatalf("prewarm did not backfill the facts sidecar: %v", err)
	}
	if got := hits.Load(); got != 1 { // exactly the one named lookup the backfill needs; no image download
		t.Errorf("legacy backfill made %d named lookups, want 1 (metadata only)", got)
	}
}

// TestServePrewarmsTheDealtDecks is the serve-level wiring gate (the round-2
// review's MAJOR finding): every other prewarm test drives prewarmArt
// directly, so deleting the `go prewarmArt(...)` block in serve — or the
// -prewarm flag default — would leave them all green while the live deploy
// regressed. This one drives serve() itself, through the newServeArtCache
// seam, with a fixture deck of three distinct names.
//
// The fixture HOLDS its first named lookup until the test releases it. That
// makes both halves of the contract observed, not inferred:
//
//  1. a prewarm fetch is in flight (and blocked) before anything else —
//     deleting the `go prewarmArt(...)` block, or defaulting -prewarm off,
//     leaves the lookup counter at zero and this poll times out; and
//  2. the server answers /api/tables while that fetch is still held —
//     serving never waits on a prewarm fetch. (serve starts its listener
//     before the prewarm block, so this is the strongest readiness claim
//     the real code makes; a synchronous prewarm after listener start is
//     not distinguished, and does not need to be — it delays nothing a
//     client can observe.)
//  3. after release, every deck name lands in the cache dir — a partial
//     prewarm (one name fetched, the rest skipped) leaves the missing
//     markers behind and fails.
func TestServePrewarmsTheDealtDecks(t *testing.T) {
	testutil.CorpusRegistry(t) // Skips when .cards/ is absent
	release := make(chan struct{})
	var lookups atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if lookups.Add(1) == 1 {
			<-release
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	decks := t.TempDir()
	// A constructed deck has no size floor (deck.Parse) and the tables here
	// are never dealt (tables: 0 — the deck is validated by splitDecks and
	// its names collected by deckCardNames, which is all prewarm reads), so
	// three named basic lands are the smallest honest fixture.
	body := `{"name":"One","cards":[{"name":"Island","count":1},{"name":"Mountain","count":1},{"name":"Plains","count":1}]}`
	if err := os.WriteFile(filepath.Join(decks, "one.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	artDir := t.TempDir()
	orig := newServeArtCache
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
	t.Cleanup(func() { newServeArtCache = orig })

	cfg := config{cards: "../../.cards", decks: decks, tables: 0, pace: 0, cooldown: 0,
		dir: t.TempDir(), spectator: "omniscient", seed: 1, perpetual: false,
		prewarm: true, artDir: artDir}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, ln) }()

	// 1. A prewarm fetch is in flight (and blocked on release).
	deadline := time.Now().Add(10 * time.Second)
	for lookups.Load() == 0 {
		if time.Now().After(deadline) {
			select {
			case serr := <-done:
				t.Fatalf("no prewarm lookup arrived and serve returned %v: the serve-level prewarm wiring is not running", serr)
			default:
				t.Fatalf("no prewarm lookup arrived (serve still up): the serve-level prewarm wiring is not running")
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	// 2. Prompt readiness while that first fetch is still held.
	waitTables(t, "http://"+ln.Addr().String(), 0)
	// 3. Release: the blocked fetch finishes and every deck name lands.
	close(release)
	for _, name := range []string{"Island", "Mountain", "Plains"} {
		marker := filepath.Join(artDir, artKey(name)+".miss")
		deadline = time.Now().Add(10 * time.Second)
		for {
			if _, err := os.Stat(marker); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("prewarm left no .miss marker for %q (lookups=%d)", name, lookups.Load())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

// TestServeFlagPrewarmDefaultsOn pins the flag default the deploy depends
// on: a gorged started with no flags prewarms the art cache. The default
// lives in serveFlags' registration (not in a statement main runs after
// Parse), so this is the pin on the whole wiring's on-switch.
func TestServeFlagPrewarmDefaultsOn(t *testing.T) {
	fs, c := serveFlags()
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if !c.prewarm {
		t.Fatal("-prewarm defaults off: a fresh deploy would refill the art cache lazily again")
	}
}

// TestServeFlagsMutateReturnedConfig is the non-default counterpart to the
// default pin above. serveFlags must return the same config whose fields were
// handed to flag.StringVar/BoolVar: returning it by value leaves Parse mutating
// an escaped original and main sees every default instead (including :8080,
// which made every smoke server collide with the live demo).
func TestServeFlagsMutateReturnedConfig(t *testing.T) {
	fs, c := serveFlags()
	if err := fs.Parse([]string{"-addr", "127.0.0.1:8097", "-art-dir", "/durable/art", "-prewarm=false"}); err != nil {
		t.Fatal(err)
	}
	if c.addr != "127.0.0.1:8097" {
		t.Errorf("-addr = %q, want parsed value", c.addr)
	}
	if c.artDir != "/durable/art" {
		t.Errorf("-art-dir = %q, want parsed value", c.artDir)
	}
	if c.prewarm {
		t.Error("-prewarm=false was ignored")
	}
}

func prewarmNamedDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	var cards string
	for i, n := range names {
		if i > 0 {
			cards += ","
		}
		cards += `{"name":"` + n + `","count":1}`
	}
	if err := os.WriteFile(filepath.Join(dir, "deck.json"), []byte(`{"name":"D","cards":[`+cards+`]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
