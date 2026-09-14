package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
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

// TestPrewarmSurvivesACancelledFetch checks the self-heal claim: a fetch
// whose context dies mid-flight (a server shutdown during prewarm) leaves no
// .miss behind, so the next request for that name retries instead of
// believing a permanent miss.
func TestPrewarmSurvivesACancelledFetch(t *testing.T) {
	ac, _ := artFixture(t, map[string]string{"Alpha Card": "/img/a.jpg"})
	ac.pace = 0
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ok, err := ac.ensure(ctx, artKey("Alpha Card"), "Alpha Card")
	if ok || err == nil {
		t.Fatalf("ensure on a dead ctx: ok=%v err=%v, want false + ctx error", ok, err)
	}
	if _, statErr := os.Stat(ac.missPath(artKey("Alpha Card"))); statErr == nil {
		t.Fatal("a cancelled fetch wrote a .miss; the name would never self-heal")
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
	if got := hits.Load(); got != 1 { // exactly the one named lookup fetchFacts needs; no image download path
		t.Errorf("legacy backfill made %d named lookups, want 1 (metadata only)", got)
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
