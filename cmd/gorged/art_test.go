package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
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

func TestArtCacheUsesTheFrontFaceOfADoubleFacedCard(t *testing.T) {
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
