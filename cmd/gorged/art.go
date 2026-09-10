package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// artCache serves card art from this server's own origin instead of sending
// every viewer's browser to Scryfall directly. It is a transparent
// proxy-and-cache: GET /art/named?exact=<name> mirrors the shape of
// Scryfall's own /cards/named endpoint (just enough of it — name and
// image_uris.normal) so the client's existing image-resolution logic needs
// no change beyond the base URL, and GET /art/blob/{key}.jpg serves the
// cached bytes. A name this server has never seen is fetched from Scryfall
// once, written to disk, and served from disk on every request after —
// including from a different viewer's browser and across a server restart.
//
// This is unlike oracle text (see oracle.ts): oracle text is withheld
// because putting Forge's GPL-3.0 script text on the wire raises a licensing
// question. Card art has no such issue, so gorge is free to cache and serve
// it itself rather than sending every browser to a third party.
type artCache struct {
	dir    string
	client *http.Client
	// sem serializes outbound Scryfall requests to one at a time, with a
	// pause after each — the same discipline images.ts used to keep
	// client-side (Scryfall asks for <=10 req/s); centralising it here
	// means the whole server, not one browser, is what's paced.
	sem chan struct{}
	// namedBaseURL is "https://api.scryfall.com/cards/named?exact=" in
	// production; tests point it at an httptest server instead so this
	// package's tests never touch the real network.
	namedBaseURL string

	mu       sync.Mutex
	inflight map[string]chan struct{} // key -> closed when that key's fetch finishes
}

const artUserAgent = "gorge-art-cache/1 (+https://github.com/adams-shaun/gorge)"
const scryfallNamedURL = "https://api.scryfall.com/cards/named?exact="

var hexKey = regexp.MustCompile(`^[0-9a-f]{64}$`)

func newArtCache(dir string) (*artCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("art cache dir: %w", err)
	}
	return &artCache{
		dir:          dir,
		client:       &http.Client{Timeout: 15 * time.Second},
		sem:          make(chan struct{}, 1),
		namedBaseURL: scryfallNamedURL,
		inflight:     map[string]chan struct{}{},
	}, nil
}

// artKey derives the cache filename from the exact card name so an arbitrary
// name never reaches a filesystem path directly (no traversal, no encoding
// surprises from commas, apostrophes, or non-ASCII names).
func artKey(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}

func (a *artCache) jpgPath(key string) string  { return filepath.Join(a.dir, key+".jpg") }
func (a *artCache) missPath(key string) string { return filepath.Join(a.dir, key+".miss") }

// named answers GET /art/named?exact=<name> with the same two fields the
// client's image-resolution logic already reads out of a Scryfall response —
// name and image_uris.normal — except normal now points at this server's own
// /art/blob/ route. A name Scryfall has never heard of answers 404, exactly
// like Scryfall's own endpoint, so a client written against the real API
// needs no new "unknown" case.
func (a *artCache) named(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("exact"))
	if name == "" {
		http.Error(w, "exact is required", http.StatusBadRequest)
		return
	}
	key := artKey(name)
	hit, err := a.ensure(r.Context(), key, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if !hit {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Name      string `json:"name"`
		ImageURIs struct {
			Normal string `json:"normal"`
		} `json:"image_uris"`
	}{
		Name: name,
		ImageURIs: struct {
			Normal string `json:"normal"`
		}{Normal: "/art/blob/" + key + ".jpg"},
	})
}

// blob serves a cached image's bytes. key is validated as an exact 64-hex
// sha256 digest before it ever reaches filepath.Join — the one place this
// package turns a request path into a filesystem path.
func (a *artCache) blob(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSuffix(r.PathValue("key"), ".jpg")
	if !hexKey.MatchString(key) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(a.jpgPath(key))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	// Content-addressed by name+printing choice, not by a URL that can be
	// reused for different bytes later — safe to cache forever.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = io.Copy(w, f)
}

// ensure reports whether key has art, fetching and caching it from Scryfall
// on a first request. Concurrent requests for the same never-seen name join
// the one in-flight fetch rather than each starting their own.
func (a *artCache) ensure(ctx context.Context, key, name string) (bool, error) {
	if _, err := os.Stat(a.jpgPath(key)); err == nil {
		return true, nil
	}
	if _, err := os.Stat(a.missPath(key)); err == nil {
		return false, nil
	}

	a.mu.Lock()
	ch, inflight := a.inflight[key]
	if !inflight {
		ch = make(chan struct{})
		a.inflight[key] = ch
	}
	a.mu.Unlock()

	if inflight {
		select {
		case <-ch:
			return a.ensure(ctx, key, name) // re-check disk now that the fetch that was running has finished
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}

	defer func() {
		a.mu.Lock()
		delete(a.inflight, key)
		a.mu.Unlock()
		close(ch)
	}()
	return a.fetch(ctx, key, name)
}

// fetch does the actual Scryfall round trip: one paced request for the
// card's metadata, then (on a hit) one more for the image bytes. Both go
// through a. sem so only one such pair is ever in flight across the whole
// server.
func (a *artCache) fetch(ctx context.Context, key, name string) (bool, error) {
	select {
	case a.sem <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() {
		time.Sleep(100 * time.Millisecond)
		<-a.sem
	}()

	u := a.namedBaseURL + url.QueryEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", artUserAgent)
	resp, err := a.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, a.writeMiss(key)
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("scryfall named lookup: status %d", resp.StatusCode)
	}

	var card struct {
		ImageURIs struct {
			Normal string `json:"normal"`
		} `json:"image_uris"`
		CardFaces []struct {
			ImageURIs struct {
				Normal string `json:"normal"`
			} `json:"image_uris"`
		} `json:"card_faces"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return false, err
	}
	imgURL := card.ImageURIs.Normal
	if imgURL == "" && len(card.CardFaces) > 0 {
		imgURL = card.CardFaces[0].ImageURIs.Normal
	}
	if imgURL == "" {
		return false, a.writeMiss(key)
	}
	if err := a.download(ctx, key, imgURL); err != nil {
		return false, err
	}
	return true, nil
}

func (a *artCache) download(ctx context.Context, key, imgURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", artUserAgent)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("scryfall image: status %d", resp.StatusCode)
	}
	tmp := a.jpgPath(key) + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	// Atomic within one filesystem: a concurrent named() request never
	// observes a partially-written .jpg.
	return os.Rename(tmp, a.jpgPath(key))
}

func (a *artCache) writeMiss(key string) error {
	return os.WriteFile(a.missPath(key), nil, 0o644)
}
