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
// Alongside the art, the cache keeps the card's six printed facts — name,
// mana_cost, type_line, oracle_text, power, toughness — as a JSON sidecar
// (<key>.json) written from the very same Scryfall named response the image
// comes from, so a card already fetched for art costs ZERO extra Scryfall
// requests when its text is asked for. GET /cards/named?exact=<name> serves
// that sidecar: this is gorge acting as its own embedder-chosen catalog (see
// oracle.ts — the client reads the <meta name="gorge-cards" tag gorged
// injects into the served index.html). The same posture the art cache
// already took — third-party printed material proxied and cached from this
// origin — with the standing line unchanged: Forge's GPL-3.0 script text is
// NEVER put on the wire; only the six printed facts are.
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

func (a *artCache) jpgPath(key string) string   { return filepath.Join(a.dir, key+".jpg") }
func (a *artCache) missPath(key string) string  { return filepath.Join(a.dir, key+".miss") }
func (a *artCache) factsPath(key string) string { return filepath.Join(a.dir, key+".json") }

// cardFacts is the exact record GET /cards/named serves — the six fields
// web/src/lib/oracle.ts normalises a catalog entry to, and nothing else.
type cardFacts struct {
	Name       string `json:"name"`
	ManaCost   string `json:"mana_cost,omitempty"`
	TypeLine   string `json:"type_line,omitempty"`
	OracleText string `json:"oracle_text,omitempty"`
	Power      string `json:"power,omitempty"`
	Toughness  string `json:"toughness,omitempty"`
}

// scryNamed is the slice of Scryfall's named-lookup response the cache
// reads: the image fields fetch() always wanted, plus the printed facts the
// /cards/named sidecar keeps. A multi-faced card carries its printed facts
// on the faces, not the top level — see facts().
type scryNamed struct {
	Name       string `json:"name"`
	ManaCost   string `json:"mana_cost"`
	TypeLine   string `json:"type_line"`
	OracleText string `json:"oracle_text"`
	Power      string `json:"power"`
	Toughness  string `json:"toughness"`
	ImageURIs  struct {
		Normal string `json:"normal"`
	} `json:"image_uris"`
	CardFaces []struct {
		ManaCost   string `json:"mana_cost"`
		TypeLine   string `json:"type_line"`
		OracleText string `json:"oracle_text"`
		Power      string `json:"power"`
		Toughness  string `json:"toughness"`
		ImageURIs  struct {
			Normal string `json:"normal"`
		} `json:"image_uris"`
	} `json:"card_faces"`
}

// facts collapses a Scryfall named response to the six-field record, taking
// the FRONT face's printed facts for a multi-faced card (Scryfall leaves the
// top-level fields empty there). One face is the contract — see the report:
// no face picker.
func (c *scryNamed) facts() cardFacts {
	f := cardFacts{Name: c.Name, ManaCost: c.ManaCost, TypeLine: c.TypeLine,
		OracleText: c.OracleText, Power: c.Power, Toughness: c.Toughness}
	if len(c.CardFaces) > 0 {
		face := c.CardFaces[0]
		if f.ManaCost == "" {
			f.ManaCost = face.ManaCost
		}
		if f.TypeLine == "" {
			f.TypeLine = face.TypeLine
		}
		if f.OracleText == "" {
			f.OracleText = face.OracleText
		}
		if f.Power == "" {
			f.Power = face.Power
		}
		if f.Toughness == "" {
			f.Toughness = face.Toughness
		}
	}
	return f
}

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

// text answers GET /cards/named?exact=<name> with the card's six printed
// facts (cardFacts) from the sidecar written when the card was fetched for
// art — or, for a card cached before facts were kept, from one paced
// metadata-only Scryfall request that backfills the sidecar. A name Scryfall
// has never heard of answers 404 (a .miss, the same record the art path
// writes), exactly like the real API, so oracle.ts treats it as a known
// cached null rather than an error.
func (a *artCache) text(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("exact"))
	if name == "" {
		http.Error(w, "exact is required", http.StatusBadRequest)
		return
	}
	key := artKey(name)
	hit, err := a.ensureText(r.Context(), key, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if !hit {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(a.factsPath(key))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, f)
}

// ensureText reports whether key has a facts sidecar, fetching it if not.
// It joins the SAME single-flight map as ensure: a text lookup arriving
// while the art fetch for the same name is still running waits for it and
// then reads the sidecar the fetch wrote, so one never-fired Scryfall named
// request serves both.
func (a *artCache) ensureText(ctx context.Context, key, name string) (bool, error) {
	if _, err := os.Stat(a.factsPath(key)); err == nil {
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
			return a.ensureText(ctx, key, name) // re-check disk now that the fetch that was running has finished
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
	// A card already fetched for art before facts were kept (a legacy cache
	// dir from before the sidecar existed, or an image download that never
	// completed) has a .jpg but no .json: backfill it with ONE paced
	// metadata-only request rather than re-fetching the image.
	if _, err := os.Stat(a.jpgPath(key)); err == nil {
		return a.fetchFacts(ctx, key, name)
	}
	return a.fetch(ctx, key, name)
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
	// The whole pair — named lookup, then image download — is paced as one
	// unit under the semaphore, exactly as it was before facts were kept.
	select {
	case a.sem <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() {
		time.Sleep(100 * time.Millisecond)
		<-a.sem
	}()

	card, known, err := a.lookupNamed(ctx, name)
	if err != nil {
		return false, err
	}
	if !known {
		return false, a.writeMiss(key)
	}
	// The sidecar is written BEFORE the image download: a card whose image
	// fetch fails still has its text on disk, and a text lookup that raced
	// this fetch finds the facts rather than firing a second named request.
	if err := a.writeFacts(key, card.facts()); err != nil {
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

// fetchFacts backfills ONLY the facts sidecar for a name that already has
// cached art but no sidecar (see ensureText). Same pacing discipline as
// fetch: one request through the semaphore, then the 100ms pause.
func (a *artCache) fetchFacts(ctx context.Context, key, name string) (bool, error) {
	select {
	case a.sem <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() {
		time.Sleep(100 * time.Millisecond)
		<-a.sem
	}()
	card, known, err := a.lookupNamed(ctx, name)
	if err != nil {
		return false, err
	}
	if !known {
		return false, a.writeMiss(key)
	}
	if err := a.writeFacts(key, card.facts()); err != nil {
		return false, err
	}
	return true, nil
}

// lookupNamed does the one Scryfall named round trip both fetch and
// fetchFacts need: card is nil with known=false exactly when Scryfall
// answered 404 (the caller records the miss), and a non-200 anything else
// is an error. It acquires NO semaphore of its own — the caller holds the
// pacing semaphore across the whole fetch, including the image download.
func (a *artCache) lookupNamed(ctx context.Context, name string) (*scryNamed, bool, error) {
	u := a.namedBaseURL + url.QueryEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", artUserAgent)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("scryfall named lookup: status %d", resp.StatusCode)
	}
	var card scryNamed
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, false, err
	}
	return &card, true, nil
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

// writeFacts writes the sidecar atomically within one filesystem, the same
// discipline download() uses: a concurrent text() request never observes a
// partially-written .json.
func (a *artCache) writeFacts(key string, facts cardFacts) error {
	b, err := json.Marshal(facts)
	if err != nil {
		return err
	}
	tmp := a.factsPath(key) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, a.factsPath(key))
}
