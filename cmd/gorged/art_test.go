package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// artFixture builds an httptest server standing in for Scryfall, serving
// /cards/named for the given name->image-path map and the images themselves
// at whatever path each entry names, plus ac wired to it via namedBaseURL.
func artFixture(t *testing.T, cards map[string]string) (*artCache, *atomic.Int32) {
	t.Helper()
	var namedHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/cards/named", func(w http.ResponseWriter, r *http.Request) {
		namedHits.Add(1)
		name := r.URL.Query().Get("exact")
		img, ok := cards[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
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

	// Image paths in the fixture map are relative (e.g. "/img/gg.jpg"); the
	// real Scryfall response carries absolute URLs, so rewrite them onto
	// srv's own address before the handler above ever serves them.
	for name, path := range cards {
		cards[name] = srv.URL + path
	}

	dir := t.TempDir()
	ac, err := newArtCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	ac.client = srv.Client()
	ac.namedBaseURL = srv.URL + "/cards/named?exact="
	return ac, &namedHits
}

func TestArtCacheServesAndCachesAHit(t *testing.T) {
	ac, hits := artFixture(t, map[string]string{"Goblin Guide": "/img/gg.jpg"})
	name := "Goblin Guide"
	key := artKey(name)

	w1 := httptest.NewRecorder()
	ac.named(w1, httptest.NewRequest(http.MethodGet, "/art/named?exact="+url.QueryEscape(name), nil))
	if w1.Code != http.StatusOK {
		t.Fatalf("first lookup: want 200, got %d: %s", w1.Code, w1.Body.String())
	}
	var body struct {
		Name      string `json:"name"`
		ImageURIs struct {
			Normal string `json:"normal"`
		} `json:"image_uris"`
	}
	if err := json.Unmarshal(w1.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	wantBlob := "/art/blob/" + key + ".jpg"
	if body.ImageURIs.Normal != wantBlob {
		t.Fatalf("image_uris.normal = %q, want %q", body.ImageURIs.Normal, wantBlob)
	}
	if _, err := os.Stat(ac.jpgPath(key)); err != nil {
		t.Fatalf("art not written to disk: %v", err)
	}

	// The blob route serves what got cached.
	wb := httptest.NewRecorder()
	rb := httptest.NewRequest(http.MethodGet, wantBlob, nil)
	rb.SetPathValue("key", key+".jpg")
	ac.blob(wb, rb)
	if wb.Code != http.StatusOK || wb.Body.String() != "fake-jpeg-bytes:/img/gg.jpg" {
		t.Fatalf("blob: got %d %q", wb.Code, wb.Body.String())
	}

	// A second lookup of the same name must not re-hit Scryfall.
	ac.named(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/art/named?exact="+url.QueryEscape(name), nil))
	if got := hits.Load(); got != 1 {
		t.Fatalf("want exactly 1 named hit across two lookups, got %d", got)
	}
}

func TestArtCacheMissIsRememberedOnDisk(t *testing.T) {
	ac, hits := artFixture(t, map[string]string{"Goblin Guide": "/img/gg.jpg"})

	wMiss := httptest.NewRecorder()
	ac.named(wMiss, httptest.NewRequest(http.MethodGet, "/art/named?exact=Nonexistent", nil))
	if wMiss.Code != http.StatusNotFound {
		t.Fatalf("miss: want 404, got %d", wMiss.Code)
	}
	if _, err := os.Stat(ac.missPath(artKey("Nonexistent"))); err != nil {
		t.Fatalf("miss not recorded on disk: %v", err)
	}

	// A repeat lookup of the same unknown name must not re-hit Scryfall.
	ac.named(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/art/named?exact=Nonexistent", nil))
	if got := hits.Load(); got != 1 {
		t.Fatalf("want exactly 1 named hit for the repeated unknown name, got %d", got)
	}
}

func TestArtCacheBlobRejectsANonHexKey(t *testing.T) {
	dir := t.TempDir()
	ac, err := newArtCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	// A file outside the cache dir that a traversal-shaped key would reach
	// if the hex-key gate in blob() were ever skipped.
	escaped := filepath.Join(dir, "..", "escaped.jpg")
	if err := os.WriteFile(escaped, []byte("nope"), 0o644); err == nil {
		defer os.Remove(escaped)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /art/blob/{key}", ac.blob)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/art/blob/..%2f..%2fescaped.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("traversal-shaped key: want 404, got %d", resp.StatusCode)
	}
}

func TestArtCacheConcurrentRequestsJoinOneFetch(t *testing.T) {
	ac, hits := artFixture(t, map[string]string{"Goblin Guide": "/img/gg.jpg"})

	var wg sync.WaitGroup
	codes := make([]int, 8)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			ac.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Goblin+Guide", nil))
			codes[i] = w.Code
		}(i)
	}
	wg.Wait()
	for _, c := range codes {
		if c != http.StatusOK {
			t.Fatalf("concurrent request got %d, want 200", c)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("8 concurrent requests for the same never-seen name should join one fetch, got %d Scryfall hits", got)
	}
}

// roundTripFunc and downloadReadBarrier let the cross-process regression
// below hold both image copies inside io.Copy. At that point download has
// already opened its staging file, which makes a reused deterministic temp
// pathname fail reliably rather than depending on scheduler timing.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type downloadReadBarrier struct {
	mu      sync.Mutex
	arrived int
	release chan struct{}
}

func (b *downloadReadBarrier) wait(ctx context.Context) error {
	b.mu.Lock()
	b.arrived++
	if b.arrived == 2 {
		close(b.release)
	}
	release := b.release
	b.mu.Unlock()
	select {
	case <-release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type barrierBody struct {
	ctx     context.Context
	barrier *downloadReadBarrier
	body    *strings.Reader
	entered bool
}

func (b *barrierBody) Read(p []byte) (int, error) {
	if !b.entered {
		b.entered = true
		if err := b.barrier.wait(b.ctx); err != nil {
			return 0, err
		}
	}
	return b.body.Read(p)
}

func (*barrierBody) Close() error { return nil }

// TestArtCachesSharingADirectoryPublishAtomically is the deploy topology:
// public and omniscient are separate processes, so their per-process
// inflight maps cannot join first fetches, but both point at one ART_DIR.
// Both writers must succeed, and a reader must see only complete immutable
// image and facts artifacts. Unique staging files are load-bearing: fixed
// <key>.jpg.tmp / <key>.json.tmp names race at rename and can expose the
// inode one process is still writing.
func TestArtCachesSharingADirectoryPublishAtomically(t *testing.T) {
	dir := t.TempDir()
	caches := make([]*artCache, 2)
	for i := range caches {
		var err error
		caches[i], err = newArtCache(dir)
		if err != nil {
			t.Fatal(err)
		}
		caches[i].pace = 0
	}

	const name = "Shared Card"
	const namedJSON = `{"name":"Shared Card","mana_cost":"{G}","type_line":"Creature — Test","oracle_text":"Complete facts.","power":"2","toughness":"3","image_uris":{"normal":"https://fixture.invalid/image.jpg"}}`
	image := strings.Repeat("complete-image-bytes-", 16*1024)
	barrier := &downloadReadBarrier{release: make(chan struct{})}
	var namedHits, imageHits atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/cards/named":
			namedHits.Add(1)
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(namedJSON))}, nil
		case "/image.jpg":
			imageHits.Add(1)
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: &barrierBody{
				ctx: r.Context(), barrier: barrier, body: strings.NewReader(image),
			}}, nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: http.NoBody}, nil
		}
	})}
	for _, ac := range caches {
		ac.client = client
		ac.namedBaseURL = "https://fixture.invalid/cards/named?exact="
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type result struct {
		hit bool
		err error
	}
	results := make(chan result, len(caches))
	key := artKey(name)
	for _, ac := range caches {
		go func(ac *artCache) {
			hit, err := ac.ensure(ctx, key, name)
			results <- result{hit: hit, err: err}
		}(ac)
	}
	for range caches {
		got := <-results
		if got.err != nil || !got.hit {
			t.Errorf("shared-directory ensure = (%v, %v), want (true, nil)", got.hit, got.err)
		}
	}
	if got := namedHits.Load(); got != 2 {
		t.Errorf("named lookups = %d, want 2 independent process fetches", got)
	}
	if got := imageHits.Load(); got != 2 {
		t.Errorf("image downloads = %d, want 2 independent process fetches", got)
	}

	gotImage, err := os.ReadFile(caches[0].jpgPath(key))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotImage) != image {
		t.Fatalf("published image is incomplete: got %d bytes, want %d", len(gotImage), len(image))
	}
	gotFacts, err := os.ReadFile(caches[0].factsPath(key))
	if err != nil {
		t.Fatal(err)
	}
	var facts cardFacts
	if err := json.Unmarshal(gotFacts, &facts); err != nil {
		t.Fatalf("published facts are incomplete: %v (bytes %q)", err, gotFacts)
	}
	wantFacts := cardFacts{Name: name, ManaCost: "{G}", TypeLine: "Creature — Test", OracleText: "Complete facts.", Power: "2", Toughness: "3"}
	if facts != wantFacts {
		t.Errorf("published facts = %+v, want %+v", facts, wantFacts)
	}
	if temps, err := filepath.Glob(filepath.Join(dir, ".*.tmp")); err != nil {
		t.Fatal(err)
	} else if len(temps) != 0 {
		t.Errorf("staging files left behind: %v", temps)
	}
}

// TestArtCacheDoesNotServeALegacyPreBumpEntry is round-2 finding 2's
// regression pin: the live server had already cached "Insectile Aberration"
// under the OLD unversioned key (sha256 of the bare name) with the FRONT
// face's bytes, and ensure() treats an existing JPG as a permanent hit while
// blob() marks the URL immutable — so after the face-matched fix deployed,
// that name would have kept resolving to the stale art from both the
// server's disk and every browser's cache. The versioned key (artKeyVersion)
// moves every name to a NEW key, so a legacy entry must be ignored: the
// lookup re-fetches from Scryfall and the blob route serves the fresh bytes
// under the new key, which no browser has cached.
func TestArtCacheDoesNotServeALegacyPreBumpEntry(t *testing.T) {
	ac, hits := artFixture(t, map[string]string{"Insectile Aberration": "/img/back.jpg"})

	// The legacy entry, exactly as the pre-fix server left it: a JPG at
	// sha256(name) — the unversioned key — holding the wrong bytes.
	sum := sha256.Sum256([]byte("Insectile Aberration"))
	legacyKey := hex.EncodeToString(sum[:])
	if legacyKey == artKey("Insectile Aberration") {
		t.Fatal("the version bump did not move the key — the legacy entry would be served")
	}
	legacyPath := filepath.Join(ac.dir, legacyKey+".jpg")
	if err := os.WriteFile(legacyPath, []byte("stale-front-face-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	ac.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Insectile+Aberration", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("lookup over a legacy entry: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Name      string `json:"name"`
		ImageURIs struct {
			Normal string `json:"normal"`
		} `json:"image_uris"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	newKey := artKey("Insectile Aberration")
	if want := "/art/blob/" + newKey + ".jpg"; body.ImageURIs.Normal != want {
		t.Fatalf("blob url = %q, want the versioned key %q", body.ImageURIs.Normal, want)
	}

	// The blob route serves the FRESH bytes at the new key, never the
	// legacy entry's.
	wb := httptest.NewRecorder()
	rb := httptest.NewRequest(http.MethodGet, body.ImageURIs.Normal, nil)
	rb.SetPathValue("key", newKey+".jpg")
	ac.blob(wb, rb)
	if wb.Code != http.StatusOK || wb.Body.String() != "fake-jpeg-bytes:/img/back.jpg" {
		t.Fatalf("blob: got %d %q, want the freshly fetched back-face bytes", wb.Code, wb.Body.String())
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("want exactly 1 named hit (the legacy entry must not suppress the fetch), got %d", got)
	}
	// The legacy file itself is untouched dead weight: never read, never
	// served, cleared only by wiping the cache dir.
	if got, err := os.ReadFile(legacyPath); err != nil || string(got) != "stale-front-face-bytes" {
		t.Fatalf("the legacy file should be left alone: %q %v", got, err)
	}
}

func TestArtCacheUsesTheFrontFaceOfADoubleFacedCard(t *testing.T) {
	// The requested name matches no face (the fixture faces carry no
	// printed name), so the front-face fallback still governs.
	ac, _ := artFixture(t, nil)
	// artFixture's JSON handler only emits image_uris, so build the
	// card_faces shape by hand against the same test server.
	mux := http.NewServeMux()
	mux.HandleFunc("/cards/named", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "Delver of Secrets",
			"card_faces": []map[string]any{
				{"image_uris": map[string]string{"normal": "http://" + r.Host + "/img/front.jpg"}},
				{"image_uris": map[string]string{"normal": "http://" + r.Host + "/img/back.jpg"}},
			},
		})
	})
	mux.HandleFunc("/img/front.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("front"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ac.client = srv.Client()
	ac.namedBaseURL = srv.URL + "/cards/named?exact="

	w := httptest.NewRecorder()
	ac.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Delver+of+Secrets", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	key := artKey("Delver of Secrets")
	got, err := os.ReadFile(ac.jpgPath(key))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "front" {
		t.Fatalf("cached bytes = %q, want the front face's image", got)
	}
}

// TestArtCacheUsesTheMatchedFaceForABackFaceName is task
// fb-20260914T033246Z-3f1cc033 defect 3: a transform card's back-face name
// must cache and serve the BACK face's art. Scryfall's named lookup lists
// both faces for either name and leaves the top-level image_uris empty, so
// the old code — which took card_faces[0] unconditionally — resolved
// "Insectile Aberration" to Delver of Secrets' art forever, and a
// transformed Delver never displayed its back side. The cache is keyed by
// the exact requested name, so the back name gets its own jpg.
func TestArtCacheUsesTheMatchedFaceForABackFaceName(t *testing.T) {
	ac, _ := artFixture(t, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/cards/named", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "Delver of Secrets // Insectile Aberration",
			"card_faces": []map[string]any{
				{"name": "Delver of Secrets", "image_uris": map[string]string{"normal": "http://" + r.Host + "/img/front.jpg"}},
				{"name": "Insectile Aberration", "image_uris": map[string]string{"normal": "http://" + r.Host + "/img/back.jpg"}},
			},
		})
	})
	mux.HandleFunc("/img/front.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("front"))
	})
	mux.HandleFunc("/img/back.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("back"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ac.client = srv.Client()
	ac.namedBaseURL = srv.URL + "/cards/named?exact="

	w := httptest.NewRecorder()
	ac.named(w, httptest.NewRequest(http.MethodGet, "/art/named?exact=Insectile+Aberration", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	key := artKey("Insectile Aberration")
	got, err := os.ReadFile(ac.jpgPath(key))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "back" {
		t.Fatalf("cached bytes = %q, want the back face's image", got)
	}
}
