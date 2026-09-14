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
	"strconv"
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
	// pace is the minimum spacing between two api.scryfall.com request
	// starts — the <=10 req/s courtesy Scryfall asks for (50-100ms between
	// requests). It is enforced by paceWait as a LIMITER over the request
	// stream (measured against a clock, not a fixed sleep), so prewarm and
	// browser-driven fetches — which share this one artCache — can never
	// race each other into 429s: every outbound named lookup is spaced from
	// the previous one whoever asked for it. It is a field, not a constant,
	// for the same reason namedBaseURL is: a test seam, so a package whose
	// budget is measured in whole seconds can exercise the prewarm without
	// paying the real-world pacing (never the other way round — production
	// keeps the 100ms).
	pace time.Duration
	// lastAPI is the start time of the most recent api.scryfall.com request,
	// paceWait's reference point. Guarded by sem (capacity 1): every caller
	// that touches it holds the semaphore across its whole fetch, so reads
	// and writes are serialized without a lock of their own.
	lastAPI time.Time
	// now is the clock the limiter and the 429 backoff read (production:
	// time.Now; tests inject a fake clock so pacing and backoff are
	// exercised with no real sleeps).
	now func() time.Time
	// sleep pauses for d, or until ctx is cancelled, whichever first.
	// Production uses a timer; tests replace it to advance a fake clock, so
	// no test ever waits real time for a rate limit.
	sleep func(context.Context, time.Duration) error
	// logf, when non-nil, gets one line per 429 retry so an operator can see
	// the limiter working in the demo log (nil → silent; the fill and serve
	// set it).
	logf func(string, ...any)

	mu       sync.Mutex
	inflight map[string]chan struct{} // key -> closed when that key's fetch finishes
}

const artUserAgent = "gorge-art-cache/1 (+https://github.com/adams-shaun/gorge)"
const scryfallNamedURL = "https://api.scryfall.com/cards/named?exact="

var hexKey = regexp.MustCompile(`^[0-9a-f]{64}$`)

// newServeArtCache is the seam serve() builds its art cache through. It is a
// package variable so the serve-level prewarm wiring (the `go prewarmArt`
// block in serve) can be exercised end to end: a test overrides it to hand
// back a cache whose outbound client points at an httptest fixture, then
// asserts on what serve's own goroutine fetched. Production never touches
// it — the default is newArtCache and nothing reassigns it outside tests.
var newServeArtCache = newArtCache

func newArtCache(dir string) (*artCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("art cache dir: %w", err)
	}
	return &artCache{
		dir:          dir,
		client:       &http.Client{Timeout: 15 * time.Second},
		sem:          make(chan struct{}, 1),
		namedBaseURL: scryfallNamedURL,
		pace:         100 * time.Millisecond,
		now:          time.Now,
		sleep:        sleepCtx,
		inflight:     map[string]chan struct{}{},
	}, nil
}

// sleepCtx is the production sleep: a timer raced against ctx, so a shutdown
// cancels a pending rate-limit pause cleanly.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// paceWait spaces api.scryfall.com request starts at least a.pace apart —
// the <=10 req/s limiter. It must be called with the pacing semaphore held
// (fetch and fetchFacts hold it across the whole fetch), so lastAPI needs no
// lock of its own. With pace == 0 (tests) it does nothing.
func (a *artCache) paceWait(ctx context.Context) error {
	if a.pace <= 0 {
		return nil
	}
	if wait := a.pace - a.now().Sub(a.lastAPI); wait > 0 {
		if err := a.sleep(ctx, wait); err != nil {
			return err
		}
	}
	a.lastAPI = a.now()
	return nil
}

// scryMaxRetries caps the 429 RETRIES of the loop in lookupNamed/download:
// an exhausted budget is nine total attempts (the initial request plus eight
// retries) — exhaustion is only checked when the ninth attempt has already
// received a 429. Nine attempts with the worst-case backoff below is on the
// order of a minute, past which a genuinely wedged Scryfall is better left
// to the next prewarm pass than held under the pacing semaphore.
const scryMaxRetries = 8

// scryRetryAfter turns a 429 into a backoff delay: the Retry-After header
// (seconds; Scryfall sends whole seconds, but a fraction parses too) when
// present and positive, otherwise exponential from 500ms doubling per
// attempt, capped at 15s.
func scryRetryAfter(header string, attempt int) time.Duration {
	if h := strings.TrimSpace(header); h != "" {
		if f, err := strconv.ParseFloat(h, 64); err == nil && f > 0 {
			return time.Duration(f * float64(time.Second))
		}
	}
	d := 500 * time.Millisecond << attempt
	if d > 15*time.Second {
		d = 15 * time.Second
	}
	return d
}

// retry429 sleeps out one 429 backoff and reports whether the caller should
// retry (true) or the retry budget is exhausted (false). rerr is non-nil
// when the wait itself was cancelled. resp is drained and closed here.
func (a *artCache) retry429(ctx context.Context, resp *http.Response, what string, attempt int) (bool, error) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if attempt >= scryMaxRetries {
		return false, nil
	}
	after := scryRetryAfter(resp.Header.Get("Retry-After"), attempt)
	if a.logf != nil {
		a.logf("art: %s: 429, retrying in %s (attempt %d/%d)", what, after, attempt+1, scryMaxRetries)
	}
	if err := a.sleep(ctx, after); err != nil {
		return false, err
	}
	return true, nil
}

// artKeyVersion busts the whole cache whenever the meaning of the bytes a
// key's fetch writes changes — round 2 of task fb-20260914T033246Z-3f1cc033:
// the round-1 fix made a back-face name (Insectile Aberration) fetch its own
// face's art, but the live server had already cached that name under the
// OLD code with the FRONT face's bytes, and ensure() treats an existing JPG
// as a permanent hit while blob() marks the URL immutable — so after deploy
// the same key kept serving the wrong art from both the server's disk and
// every browser's year-long cache. Folding the version into the hash gives
// every name a NEW key, so the first request after deploy re-fetches at the
// new key and the old key's URL (and the browser entry pinning it) is
// simply never requested again. Legacy files at old keys are never read;
// they are dead weight until the cache dir is cleared. The next change to
// what a fetch writes bumps this string again.
const artKeyVersion = "gorge-art-v2"

// artKey derives the cache filename from the exact card name so an arbitrary
// name never reaches a filesystem path directly (no traversal, no encoding
// surprises from commas, apostrophes, or non-ASCII names). The version is
// hashed in (see artKeyVersion), so a bump rotates every key at once.
func artKey(name string) string {
	sum := sha256.Sum256([]byte(artKeyVersion + "\x00" + name))
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

// scryFace is one entry of Scryfall's card_faces: the face's printed name
// (the face picker's key, task fb-20260914T033246Z-3f1cc033) plus the
// printed facts and image the front-face fallback and the matching-face
// path both read.
type scryFace struct {
	Name       string `json:"name"`
	ManaCost   string `json:"mana_cost"`
	TypeLine   string `json:"type_line"`
	OracleText string `json:"oracle_text"`
	Power      string `json:"power"`
	Toughness  string `json:"toughness"`
	ImageURIs  struct {
		Normal string `json:"normal"`
	} `json:"image_uris"`
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
	CardFaces []scryFace `json:"card_faces"`
}

// faceFor returns the card_face whose printed name is the requested one, or
// nil when no face matches (a single-faced card, or a name Scryfall answers
// with a combined multi-face name). Comparison is case-insensitive on the
// trimmed face name: Scryfall prints faces exactly as the client requests
// them ("Insectile Aberration"), but being liberal here costs nothing and
// the fallback below is the front face either way.
func (c *scryNamed) faceFor(name string) *scryFace {
	for i := range c.CardFaces {
		if strings.EqualFold(strings.TrimSpace(c.CardFaces[i].Name), strings.TrimSpace(name)) {
			return &c.CardFaces[i]
		}
	}
	return nil
}

// faceImage picks the image bytes to cache for the requested name: the
// top-level image when Scryfall carries one (single-faced cards),
// otherwise the face whose printed name is the requested one — task
// fb-20260914T033246Z-3f1cc033: Scryfall's named lookup lists BOTH faces
// for either name of a transform card and leaves the top-level image_uris
// empty, so a back-face name (Insectile Aberration) must pick
// card_faces[1], not the front face card_faces[0] the old code took —
// falling back to the front face for a name that matches no face. The
// cache is keyed by the exact requested name, so the back face's name gets
// its own .jpg and its own blob route entry; the client needs no change.
func (c *scryNamed) faceImage(name string) string {
	if c.ImageURIs.Normal != "" {
		return c.ImageURIs.Normal
	}
	if match := c.faceFor(name); match != nil {
		return match.ImageURIs.Normal
	}
	if len(c.CardFaces) > 0 {
		return c.CardFaces[0].ImageURIs.Normal
	}
	return ""
}

// facts collapses a Scryfall named response to the six-field record, taking
// the FRONT face's printed facts for a multi-faced card (Scryfall leaves the
// top-level fields empty there) — EXCEPT when the requested name is one
// face's own printed name (task fb-20260914T033246Z-3f1cc033: a transformed
// Delver of Secrets is on the wire as "Insectile Aberration", and its
// sidecar must carry THAT face's oracle text and P/T, not the front
// face's). One face is still the contract for a name that matches no face.
func (c *scryNamed) facts(name string) cardFacts {
	f := cardFacts{Name: c.Name, ManaCost: c.ManaCost, TypeLine: c.TypeLine,
		OracleText: c.OracleText, Power: c.Power, Toughness: c.Toughness}
	if len(c.CardFaces) > 0 {
		face := &c.CardFaces[0]
		if match := c.faceFor(name); match != nil {
			face = match
			f.Name = face.Name
		}
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

// fetch does the actual Scryfall round trip: one paced, 429-retried request
// for the card's metadata, then (on a hit) one more for the image bytes. Both
// go through a.sem so only one such pair is ever in flight across the whole
// server, and the named lookup is additionally spaced by paceWait's limiter
// (<=10 api.scryfall.com req/s no matter who — prewarm or a browser — asked).
func (a *artCache) fetch(ctx context.Context, key, name string) (bool, error) {
	// The whole pair — named lookup, then image download — is paced as one
	// unit under the semaphore, exactly as it was before facts were kept.
	select {
	case a.sem <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() { <-a.sem }()

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
	if err := a.writeFacts(key, card.facts(name)); err != nil {
		return false, err
	}
	imgURL := card.faceImage(name)
	if imgURL == "" {
		return false, a.writeMiss(key)
	}
	if err := a.download(ctx, key, imgURL); err != nil {
		return false, err
	}
	// One named response carries every face of the card: cache the OTHER
	// faces too, so a transformed split/DFC the deck only names on one side
	// is already on disk when a client asks for the face it transformed
	// into — zero extra api.scryfall.com requests.
	a.cacheSiblingFaces(ctx, card, name)
	return true, nil
}

// cacheSiblingFaces writes the art and facts of every face of a multi-faced
// card other than the name that was asked for, from the SAME named response
// (no extra Scryfall API request; each face's image is one CDN download).
// Failures are skipped, not fatal: the name stays unfetched on disk and the
// ordinary ensure path self-heals it on the next request or prewarm pass.
func (a *artCache) cacheSiblingFaces(ctx context.Context, card *scryNamed, requested string) {
	for i := range card.CardFaces {
		face := &card.CardFaces[i]
		fname := strings.TrimSpace(face.Name)
		if fname == "" || strings.EqualFold(fname, requested) || face.ImageURIs.Normal == "" {
			continue
		}
		fkey := artKey(fname)
		if _, err := os.Stat(a.jpgPath(fkey)); err != nil {
			if err := a.download(ctx, fkey, face.ImageURIs.Normal); err != nil {
				continue
			}
		}
		if _, err := os.Stat(a.factsPath(fkey)); err != nil {
			_ = a.writeFacts(fkey, card.facts(fname))
		}
	}
}

// fetchFacts backfills ONLY the facts sidecar for a name that already has
// cached art but no sidecar (see ensureText). Same pacing discipline as
// fetch: one paced request through the semaphore — and the same 429 retry —
// plus the sibling-face caching fetch does, since this named response is
// just as capable of filling the other faces' art for free.
func (a *artCache) fetchFacts(ctx context.Context, key, name string) (bool, error) {
	select {
	case a.sem <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() { <-a.sem }()
	card, known, err := a.lookupNamed(ctx, name)
	if err != nil {
		return false, err
	}
	if !known {
		return false, a.writeMiss(key)
	}
	if err := a.writeFacts(key, card.facts(name)); err != nil {
		return false, err
	}
	a.cacheSiblingFaces(ctx, card, name)
	return true, nil
}

// lookupNamed does the one Scryfall named round trip both fetch and
// fetchFacts need: card is nil with known=false exactly when Scryfall
// answered 404 (the caller records the miss), and a non-200 anything else
// is an error — except 429, the rate limit, which is backed off (honouring
// Retry-After when the header carries it, exponential otherwise) and
// retried up to scryMaxRetries times, so a prewarm pass over the whole deck
// pool ends with every fetchable name on disk instead of skipping whatever
// it hit the limit on. It acquires NO semaphore of its own — the caller
// holds the pacing semaphore across the whole fetch, including the image
// download — but it DOES consult the limiter (paceWait) before each
// attempt, which is what spaces the request stream whoever asked.
func (a *artCache) lookupNamed(ctx context.Context, name string) (*scryNamed, bool, error) {
	for attempt := 0; ; attempt++ {
		if err := a.paceWait(ctx); err != nil {
			return nil, false, err
		}
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
		if resp.StatusCode == http.StatusTooManyRequests {
			more, rerr := a.retry429(ctx, resp, "named lookup "+name, attempt)
			if rerr != nil {
				return nil, false, rerr
			}
			if !more {
				return nil, false, fmt.Errorf("scryfall named lookup: status 429 after %d retries", attempt)
			}
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			_ = resp.Body.Close()
			return nil, false, nil
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, false, fmt.Errorf("scryfall named lookup: status %d", resp.StatusCode)
		}
		var card scryNamed
		derr := json.NewDecoder(resp.Body).Decode(&card)
		_ = resp.Body.Close()
		if derr != nil {
			return nil, false, derr
		}
		return &card, true, nil
	}
}

// createTemp opens a uniquely named staging file in the cache directory.
// Uniqueness matters across processes: deploy-demo runs two gorged instances
// against one art directory, while artCache.inflight only coordinates callers
// within one process. Each writer therefore needs its own inode until the
// completed artifact is atomically published with Rename.
func (a *artCache) createTemp(pattern string) (*os.File, error) {
	f, err := os.CreateTemp(a.dir, pattern)
	if err != nil {
		return nil, err
	}
	// CreateTemp deliberately defaults to 0600. Cached artifacts have always
	// been 0644 (subject to the process umask), so preserve that contract.
	if err := f.Chmod(0o644); err != nil {
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
		return nil, err
	}
	return f, nil
}

// download fetches one image URL (the Scryfall CDN, not the rate-limited
// API) and publishes it atomically at the key. A 429 here is backed off and
// retried the same way lookupNamed does — the CDN honours the same header —
// so one name's fetch never ends half-done because of a rate limit.
func (a *artCache) download(ctx context.Context, key, imgURL string) error {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", artUserAgent)
		resp, err := a.client.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			more, rerr := a.retry429(ctx, resp, "image "+key[:8], attempt)
			if rerr != nil {
				return rerr
			}
			if !more {
				return fmt.Errorf("scryfall image: status 429 after %d retries", attempt)
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return fmt.Errorf("scryfall image: status %d", resp.StatusCode)
		}
		f, err := a.createTemp("." + key + "-*.jpg.tmp")
		if err != nil {
			_ = resp.Body.Close()
			return err
		}
		tmp := f.Name()
		defer os.Remove(tmp)
		if _, err := io.Copy(f, resp.Body); err != nil {
			_ = resp.Body.Close()
			_ = f.Close()
			return err
		}
		_ = resp.Body.Close()
		if err := f.Close(); err != nil {
			return err
		}
		// Atomic within one filesystem: neither a concurrent named() request
		// nor the other demo process can observe a partially-written .jpg.
		// Unique staging names let concurrent processes both publish the
		// same key safely.
		return os.Rename(tmp, a.jpgPath(key))
	}
}

func (a *artCache) writeMiss(key string) error {
	return os.WriteFile(a.missPath(key), nil, 0o644)
}

// writeFacts writes the sidecar atomically within one filesystem, the same
// discipline download() uses: a concurrent text() request never observes a
// partially-written .json, including when two gorged processes share a cache.
func (a *artCache) writeFacts(key string, facts cardFacts) error {
	b, err := json.Marshal(facts)
	if err != nil {
		return err
	}
	f, err := a.createTemp("." + key + "-*.json.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, a.factsPath(key))
}

// artFillStats is what one fill pass over a deck directory did. String()
// is the exact summary shape the one-shot fill prints and the deploy log
// carries.
type artFillStats struct {
	// Names is how many distinct deck names the pass walked.
	Names int
	// Cached counts names this pass fetched (art or facts was missing).
	Cached int
	// Present counts names already complete on disk before this pass.
	Present int
	// NotFound counts names Scryfall genuinely does not know (a .miss —
	// a fact, not a failure; a miss recorded by an EARLIER pass counts
	// here too, honestly re-measured each run).
	NotFound int
	// Failed counts names whose fetch errored (network, exhausted 429
	// retries) and so is NOT on disk in any form — the one-shot fill
	// exits non-zero when this is non-zero, and the next pass retries them.
	Failed int
}

func (s artFillStats) String() string {
	return fmt.Sprintf("cached %d, already present %d, genuine 404 %d, failed %d",
		s.Cached, s.Present, s.NotFound, s.Failed)
}

// fillArt walks every distinct card name (plus each commander) referenced by
// the deck files in dir, in sorted order, and leaves each one complete in the
// cache — art, facts sidecar, and (via fetch's sibling-face caching) every
// other face of a multi-faced card. It is the ONE fill loop: prewarmArt
// (serve's background pass) and the -prewarm-art-only one-shot both go
// through it, so their guarantees cannot drift apart. label is the log-line
// prefix distinguishing the caller ("art prewarm" / "art fill").
//
// A name already complete on disk costs nothing (ensure/ensureText are
// disk-checked and single-flight), which is what makes the fill idempotent:
// a second pass makes zero fetches. Every per-name error is logged through
// logf and left to self-heal — a network failure writes no .miss (only a
// genuine Scryfall 404 does), so the next pass or browser request retries
// the fetch. A cancelled ctx aborts the walk with the partial stats and
// ctx.Err(); names never attempted are not counted as failures.
func fillArt(ctx context.Context, ac *artCache, dir, label string, logf func(string, ...any)) (artFillStats, error) {
	names, err := deckCardNames(dir)
	if err != nil {
		return artFillStats{}, err
	}
	st := artFillStats{Names: len(names)}
	logf("%s: %d distinct card names from %s", label, len(names), dir)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return st, err
		}
		key := artKey(name)
		complete := fileExists(ac.jpgPath(key)) && fileExists(ac.factsPath(key))
		ok, err := ac.ensure(ctx, key, name)
		if err != nil {
			st.Failed++
			logf("%s %q: %v", label, name, err)
			continue
		}
		if !ok {
			st.NotFound++ // a genuine Scryfall 404 recorded a .miss; nothing to backfill
			continue
		}
		if _, err := ac.ensureText(ctx, key, name); err != nil {
			st.Failed++
			logf("%s facts %q: %v", label, name, err)
			continue
		}
		if complete {
			st.Present++
		} else {
			st.Cached++
		}
	}
	logf("%s: done (%d names): %s", label, st.Names, st)
	return st, nil
}

// fileExists is os.Stat's presence check, named so the call sites read.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// prewarmArt fills the cache for every deck name in the background at serve
// startup (see fillArt for the loop itself). Never blocks or fails serving:
// every error is logged through logf and left to self-heal. ctx is serve()'s
// own, so shutdown cancels an in-flight prewarm cleanly.
func prewarmArt(ctx context.Context, ac *artCache, dir string, logf func(string, ...any)) {
	st, err := fillArt(ctx, ac, dir, "art prewarm", logf)
	if err != nil {
		if ctx.Err() != nil {
			logf("art prewarm: cancelled with %d names unattempted", st.Names-st.Cached-st.Present-st.NotFound-st.Failed)
			return
		}
		logf("art prewarm: reading decks in %s: %v", dir, err)
	}
}

// runPrewarmArtOnly is the -prewarm-art-only body: fill the card-art cache
// from every deck in -decks (c.artCacheDir() is the destination), log the
// per-name failures, and return the process exit code — non-zero when any
// name FAILED (a genuine 404 is a fact, not a failure). No listener, no
// corpus, no tables: the deploy runs this before any server starts so a
// deploy never serves a cold cache, and it is a no-op (zero fetches) when
// the cache is already complete.
func runPrewarmArtOnly(ctx context.Context, c config) int {
	ac, err := newServeArtCache(c.artCacheDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "gorged: %v\n", err)
		return 1
	}
	ac.logf = func(f string, a ...any) { fmt.Fprintf(os.Stderr, "gorged: "+f+"\n", a...) }
	st, err := fillArt(ctx, ac, c.decks, "art fill", ac.logf)
	if err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "gorged: art fill cancelled")
		} else {
			fmt.Fprintf(os.Stderr, "gorged: reading decks in %s: %v\n", c.decks, err)
		}
		return 1
	}
	if st.Failed > 0 {
		return 1
	}
	return 0
}
