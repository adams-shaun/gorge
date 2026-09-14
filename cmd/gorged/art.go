package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	// sem serializes this process's outbound Scryfall work to one fetch at a
	// time. Every request in that work — named metadata, requested-face
	// image, sibling-face images, and every retry — goes through doScryfall,
	// whose limiter spaces their starts (Scryfall asks for <=10 req/s).
	sem chan struct{}
	// namedBaseURL is "https://api.scryfall.com/cards/named?exact=" in
	// production; tests point it at an httptest server instead so this
	// package's tests never touch the real network.
	namedBaseURL string
	// pace is the minimum spacing between two outbound Scryfall request
	// starts — the <=10 req/s courtesy Scryfall asks for (50-100ms between
	// requests). It is enforced by paceWait as a LIMITER over the request
	// stream (measured against a clock, not a fixed sleep). On Flock-capable
	// systems it is shared by EVERY process using this cache directory through
	// a locked stamp file (paceFile); elsewhere it is shared by every cache in
	// this process through synchronized memory. Thus the deploy's one-shot fill
	// and every lock-aware Unix demo server stay within one pace, including old
	// servers a deploy replaces once they run a binary that knows the stamp
	// file. The FIRST deploy after that code lands overlaps old-binary servers,
	// which pace only themselves; its fill budget bounds the duration of that
	// unshared window, not its aggregate request rate. It is a field, not a
	// constant, for
	// the same reason namedBaseURL
	// is: a test seam, so a package whose budget is measured in whole
	// seconds can exercise the prewarm without paying the real-world pacing
	// (never the other way round — production keeps the 100ms). With pace 0
	// the stamp file is never touched.
	pace time.Duration
	// now is the clock the limiter and the 429 backoff read (production:
	// time.Now; tests inject a fake clock so pacing and backoff are
	// exercised with no real sleeps).
	now func() time.Time
	// sleep pauses for d, or until ctx is cancelled, whichever first.
	// Production uses a timer; tests replace it to advance a fake clock, so
	// no test ever waits real time for a rate limit.
	sleep func(context.Context, time.Duration) error
	// afterFunc runs f once d has passed on this cache's clock and returns a
	// stop function (production: time.AfterFunc). The fill's time budget is
	// armed through it, so a test trips the budget on the fake clock.
	afterFunc func(d time.Duration, f func()) (stop func() bool)
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
		afterFunc:    func(d time.Duration, f func()) func() bool { return time.AfterFunc(d, f).Stop },
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

// paceFile is the cross-process limiter's stamp on Flock-capable systems,
// inside the cache dir. Its first 8 bytes hold the most recent request start
// (big-endian Unix nanoseconds). On other systems the same bytes live in the
// processPaceState keyed by cache directory. The blob route serves only
// 64-hex keys, so this name is never reachable over HTTP.
const paceFile = ".scryfall-pace"

// paceLock is the common locked-stamp contract. Unix implementations back it
// with an *os.File and flock; fallback targets back it with synchronized,
// process-local memory. Close releases the lock in either case.
type paceLock interface {
	io.ReaderAt
	io.WriterAt
	io.Closer
}

// paceWait spaces Scryfall request starts at least a.pace apart — the <=10
// req/s limiter. lockPace supplies cross-process exclusion through flock on
// supported systems and process-local exclusion everywhere else. While the
// lock is held, paceWait reads the most recent start, waits out the rest of
// the gap, stamps its own start, and releases. The lock is held for at most
// one pace. With pace == 0 (tests) it does nothing and touches no state.
func (a *artCache) paceWait(ctx context.Context) error {
	if a.pace <= 0 {
		return nil
	}
	f, err := a.lockPace(ctx)
	if err != nil {
		return fmt.Errorf("art pace lock: %w", err)
	}
	defer f.Close() // releases the flock or process-local lock
	var stamp [8]byte
	var last time.Time
	if n, _ := f.ReadAt(stamp[:], 0); n == len(stamp) {
		last = time.Unix(0, int64(binary.BigEndian.Uint64(stamp[:])))
	}
	wait := a.pace - a.now().Sub(last)
	if wait > a.pace {
		wait = a.pace // a stamp from the future (the clock stepped back) costs one gap, never more
	}
	if wait > 0 {
		if err := a.sleep(ctx, wait); err != nil {
			return err
		}
	}
	binary.BigEndian.PutUint64(stamp[:], uint64(a.now().UnixNano()))
	if _, err := f.WriteAt(stamp[:], 0); err != nil {
		return fmt.Errorf("art pace stamp: %w", err)
	}
	return nil
}

// doScryfall is the single outbound Scryfall-client path. Keeping the limiter
// beside client.Do makes it impossible for a new metadata, image, or retry
// caller to use this client without pacing unless it explicitly bypasses this
// helper. The caller holds a.sem, serializing this cache's request starts;
// paceWait extends that spacing to every cache in the process and, where flock
// is available, every process on the same cache directory.
func (a *artCache) doScryfall(req *http.Request) (*http.Response, error) {
	if err := a.paceWait(req.Context()); err != nil {
		return nil, err
	}
	return a.client.Do(req)
}

// scryMaxRetries caps the 429 RETRIES of the loop in lookupNamed/download:
// an exhausted budget is nine total attempts (the initial request plus eight
// retries) — exhaustion is only checked when the ninth attempt has already
// received a 429. Nine attempts with the exponential backoff below is about a
// minute (60.5s of sleeps); a server-sent Retry-After is capped at
// scryMaxRetryAfter per retry, so no request can stay in backoff past eight
// minutes, and the deploy's bounded fill (-prewarm-art-budget) cuts even
// that short. A genuinely wedged Scryfall is better left to the next prewarm
// pass than held under the pacing semaphore.
const scryMaxRetries = 8

// scryMaxRetryAfter caps one 429 backoff. Every backoff sleeps with the
// pacing semaphore held, so an uncapped `Retry-After: 86400` would stall
// every browser's cold art fetch in the process for a day.
const scryMaxRetryAfter = 60 * time.Second

// scryRetryAfter turns a 429 into a backoff delay: the Retry-After header
// (seconds; Scryfall sends whole seconds, but a fraction parses too) when
// present and positive, capped at scryMaxRetryAfter (the cap is applied
// before the conversion, so a huge or infinite value cannot overflow a
// Duration); otherwise exponential from 500ms doubling per attempt, capped
// at 15s.
func scryRetryAfter(header string, attempt int) time.Duration {
	if h := strings.TrimSpace(header); h != "" {
		if f, err := strconv.ParseFloat(h, 64); err == nil && f > 0 {
			if f > scryMaxRetryAfter.Seconds() {
				return scryMaxRetryAfter
			}
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
func (a *artCache) facesPath(key string) string { return filepath.Join(a.dir, key+".faces") }

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
// reads: the image fields fetchCard always wanted, plus the printed facts the
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
// request serves both. A card already fetched for art before facts were kept
// (a .jpg but no .json) is backfilled by the same fetchCard, which skips the
// image download because the .jpg is already on disk.
func (a *artCache) ensureText(ctx context.Context, key, name string) (bool, error) {
	return a.singleFlight(ctx, key, func() (bool, bool) {
		if fileExists(a.factsPath(key)) {
			return true, true
		}
		if fileExists(a.missPath(key)) {
			return false, true
		}
		return false, false
	}, func() (bool, error) {
		return a.servedDespiteSiblings(a.factsPath(key))(a.fetchCard(ctx, key, name))
	})
}

// singleFlight is the one join-or-run discipline ensure, ensureText and
// ensureComplete share. check reports (hit, decided) from disk; when it is
// undecided, the first caller for key runs work while every concurrent caller
// for the same key waits and then re-checks disk (and becomes the runner if
// the earlier work failed to settle it).
func (a *artCache) singleFlight(ctx context.Context, key string, check func() (hit, decided bool), work func() (bool, error)) (bool, error) {
	for {
		if hit, decided := check(); decided {
			return hit, nil
		}
		a.mu.Lock()
		ch, inflight := a.inflight[key]
		if !inflight {
			ch = make(chan struct{})
			a.inflight[key] = ch
			a.mu.Unlock()
			return a.runFlight(key, ch, check, work)
		}
		a.mu.Unlock()
		select {
		case <-ch:
			// re-check disk now that the fetch that was running has finished
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}

// runFlight runs work as key's single in-flight fetch and releases the slot
// when it returns. It re-checks disk first: a runner that finished between
// this caller's check and its claiming the slot may already have settled it.
func (a *artCache) runFlight(key string, ch chan struct{}, check func() (bool, bool), work func() (bool, error)) (bool, error) {
	defer func() {
		a.mu.Lock()
		delete(a.inflight, key)
		a.mu.Unlock()
		close(ch)
	}()
	if hit, decided := check(); decided {
		return hit, nil
	}
	return work()
}

// servedDespiteSiblings adapts fetchCard's result for a CLIENT route, which
// asked for exactly one face: when only a sibling face failed
// (incompleteFacesError)
// and the requested artifact at path is on disk, the request is a hit — the
// sibling is fetched when a client asks for it by name, and the one-shot fill
// (ensureComplete) still sees the entry as incomplete because no face
// manifest was written.
func (a *artCache) servedDespiteSiblings(path string) func(bool, error) (bool, error) {
	return func(hit bool, err error) (bool, error) {
		var inc *incompleteFacesError
		if errors.As(err, &inc) && fileExists(path) {
			if a.logf != nil {
				a.logf("art: %v", err)
			}
			return true, nil
		}
		return hit, err
	}
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
	return a.singleFlight(ctx, key, func() (bool, bool) {
		if fileExists(a.jpgPath(key)) {
			return true, true
		}
		if fileExists(a.missPath(key)) {
			return false, true
		}
		return false, false
	}, func() (bool, error) {
		return a.servedDespiteSiblings(a.jpgPath(key))(a.fetchCard(ctx, key, name))
	})
}

// ensureComplete is the FILL's question, stricter than ensure's: is every
// artifact a client can request for this card on disk — the requested name's
// art and facts AND every sibling face's art and facts? A client can only be
// told which siblings exist by a Scryfall named response, so a .jpg+.json pair
// alone cannot answer it: an entry cached before sibling faces were kept, or
// by a fetch killed between the requested image and a sibling, looks exactly
// like a complete single-faced card. complete() therefore trusts only the face
// manifest fetchCard writes LAST, after every sibling is on disk; anything
// else re-runs fetchCard, which makes one named lookup and downloads only the
// artifacts still missing. (hit=false only for a genuine Scryfall 404.)
//
// A .miss settles the question even beside a .jpg. That pair is a legacy
// entry whose name Scryfall no longer knows: the lookup that wrote the .miss
// is the answer, so the entry is a genuine 404 for the fill, not a lookup
// repeated on every pass (and by both servers' prewarms each deploy). The
// client routes still serve the old image — ensure checks the .jpg first.
func (a *artCache) ensureComplete(ctx context.Context, key, name string) (bool, error) {
	return a.singleFlight(ctx, key, func() (bool, bool) {
		if a.complete(key) {
			return true, true
		}
		if fileExists(a.missPath(key)) {
			return false, true
		}
		return false, false
	}, func() (bool, error) {
		return a.fetchCard(ctx, key, name)
	})
}

// faceManifest is the <key>.faces record: the printed names of every OTHER
// named face of the card this key's named response described (empty
// for a single-faced card). Its presence means fetchCard finished that
// response completely; complete() re-verifies each listed sibling on disk.
type faceManifest struct {
	Siblings []string `json:"siblings"`
}

// complete reports whether key's requested artifacts and every sibling its
// face manifest names are on disk. No manifest (a legacy or interrupted
// entry) or an unreadable one is incomplete.
func (a *artCache) complete(key string) bool {
	if !fileExists(a.jpgPath(key)) || !fileExists(a.factsPath(key)) {
		return false
	}
	b, err := os.ReadFile(a.facesPath(key))
	if err != nil {
		return false
	}
	var m faceManifest
	if err := json.Unmarshal(b, &m); err != nil || m.Siblings == nil {
		return false
	}
	for _, sib := range m.Siblings {
		sk := artKey(sib)
		if !fileExists(a.jpgPath(sk)) || !fileExists(a.factsPath(sk)) {
			return false
		}
	}
	return true
}

// incompleteFacesError is fetchCard's failure when the requested name's own
// artifacts are on disk but a sibling face's are not. The fill counts it as a
// failure; a client route that asked only for the requested face does not
// (see servedDespiteSiblings).
type incompleteFacesError struct{ err error }

func (e *incompleteFacesError) Error() string { return e.err.Error() }
func (e *incompleteFacesError) Unwrap() error { return e.err }

// fetchCard does the actual Scryfall round trip for every caller — client art,
// client text, legacy facts backfill, and the fill: one paced, 429-retried
// named request, then whichever of the requested image, the facts sidecar and
// the sibling faces' art/facts are still missing, then the face manifest.
// Everything runs under a.sem so only one card's requests are ever in flight
// across the whole server, and every request is spaced by doScryfall's
// limiter (<=10 Scryfall req/s no matter who — prewarm or a browser — asked).
//
// The manifest is written only after every other artifact is on disk, so a
// failure or a kill at any earlier point leaves the entry incomplete for the
// next fill, which retries it rather than reporting it present.
func (a *artCache) fetchCard(ctx context.Context, key, name string) (bool, error) {
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
	// A legacy entry already holding the requested image (an art-only cache
	// being backfilled with facts, or a front face being completed with its
	// siblings) never re-downloads it.
	if !fileExists(a.jpgPath(key)) {
		imgURL := card.faceImage(name)
		if imgURL == "" {
			return false, a.writeMiss(key)
		}
		if err := a.download(ctx, key, imgURL); err != nil {
			return false, err
		}
	}
	// One named response carries every face of the card: cache the OTHER
	// faces too, so a transformed split/DFC the deck only names on one side
	// is already on disk when a client asks for the face it transformed
	// into — zero extra api.scryfall.com requests.
	siblings, err := a.cacheSiblingFaces(ctx, card, name)
	if err != nil {
		return false, &incompleteFacesError{err: err}
	}
	if err := a.writeFaceManifest(key, faceManifest{Siblings: siblings}); err != nil {
		return false, err
	}
	return true, nil
}

// cacheSiblingFaces writes the art and facts of every named face of a
// multi-faced card other than the name that was asked for, from the SAME named
// response (no extra Scryfall API request; each face's image is one CDN
// download), and returns those faces' names for the manifest. A face's image
// is chosen by faceImage — exactly what a cold client request for that face's
// name would cache — so a split, Room or adventure half, whose art is the
// card's top-level image rather than its own, is covered too. Every failure is
// returned: sibling faces are part of a complete fill, so silently skipping one
// would make the summary and one-shot exit status claim success for an
// incomplete cache.
func (a *artCache) cacheSiblingFaces(ctx context.Context, card *scryNamed, requested string) ([]string, error) {
	siblings := []string{}
	for i := range card.CardFaces {
		face := &card.CardFaces[i]
		fname := strings.TrimSpace(face.Name)
		if fname == "" || strings.EqualFold(fname, requested) {
			continue
		}
		img := card.faceImage(fname)
		if img == "" {
			continue // no art anywhere in the response; a request would record a miss
		}
		fkey := artKey(fname)
		if !fileExists(a.jpgPath(fkey)) {
			if err := a.download(ctx, fkey, img); err != nil {
				return nil, fmt.Errorf("cache sibling image %q: %w", fname, err)
			}
		}
		if !fileExists(a.factsPath(fkey)) {
			if err := a.writeFacts(fkey, card.facts(fname)); err != nil {
				return nil, fmt.Errorf("cache sibling facts %q: %w", fname, err)
			}
		}
		siblings = append(siblings, fname)
	}
	return siblings, nil
}

// lookupNamed does the one Scryfall named round trip fetchCard needs: card is nil with known=false exactly when Scryfall
// answered 404 (the caller records the miss), and a non-200 anything else
// is an error — except 429, the rate limit, which is backed off (honouring
// Retry-After when the header carries it, exponential otherwise) and
// retried up to scryMaxRetries times, so a prewarm pass over the whole deck
// pool ends with every fetchable name on disk instead of skipping whatever
// it hit the limit on. It acquires NO semaphore of its own — the caller
// holds the pacing semaphore across the whole fetch, including the image
// download — but every attempt DOES go through doScryfall, which spaces the
// shared request stream whoever asked.
func (a *artCache) lookupNamed(ctx context.Context, name string) (*scryNamed, bool, error) {
	for attempt := 0; ; attempt++ {
		u := a.namedBaseURL + url.QueryEscape(name)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, false, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", artUserAgent)
		resp, err := a.doScryfall(req)
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

// download fetches one Scryfall image URL and publishes it atomically at the
// key. Every attempt goes through the same limiter as named metadata, so a
// cold name's API request, requested-face image, sibling-face images, and any
// retries cannot burst independently. A 429 here is backed off and retried the
// same way lookupNamed does — the CDN honours the same header — so one name's
// fetch never ends half-done because of a rate limit.
func (a *artCache) download(ctx context.Context, key, imgURL string) error {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", artUserAgent)
		resp, err := a.doScryfall(req)
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
	return a.writeJSONAtomic(a.factsPath(key), "."+key+"-*.json.tmp", facts)
}

// writeFaceManifest publishes key's face manifest atomically (see complete).
func (a *artCache) writeFaceManifest(key string, m faceManifest) error {
	return a.writeJSONAtomic(a.facesPath(key), "."+key+"-*.faces.tmp", m)
}

func (a *artCache) writeJSONAtomic(dst, pattern string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := a.createTemp(pattern)
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
	return os.Rename(tmp, dst)
}

// artFillStats is what one fill pass over a deck directory did. String()
// is the exact summary shape the one-shot fill prints and the deploy log
// carries.
type artFillStats struct {
	// Names is how many distinct deck names the pass walked.
	Names int
	// Cached counts names this pass fetched or completed: art, facts, a
	// sibling face, or the face manifest was missing (a legacy entry cached
	// before sibling faces were kept costs one named lookup here, once).
	Cached int
	// Present counts names already complete on disk before this pass — every
	// face, per the face manifest (see artCache.complete).
	Present int
	// NotFound counts names Scryfall genuinely does not know (a .miss —
	// a fact, not a failure; a miss recorded by an EARLIER pass counts
	// here too, honestly re-measured each run).
	NotFound int
	// Failed counts names whose fetch errored (network, exhausted 429,
	// or an incomplete sibling face) and so is not COMPLETE on disk — the
	// one-shot fill exits non-zero when this is non-zero, and the next pass
	// retries them.
	Failed int
}

// attempted is how many names the pass reached, whatever their outcome.
func (s artFillStats) attempted() int { return s.Cached + s.Present + s.NotFound + s.Failed }

func (s artFillStats) String() string {
	return fmt.Sprintf("cached %d, already present %d, genuine 404 %d, failed %d",
		s.Cached, s.Present, s.NotFound, s.Failed)
}

// fillLimits bounds one fill pass. The zero value is unbounded — serve's
// background prewarm, which exists to finish whatever a bounded pass left.
type fillLimits struct {
	// Budget stops the pass once this much time has passed on the cache's
	// clock (0 = no limit). It cancels the pass's context, which aborts an
	// in-flight request, a 429 backoff and a pace wait alike.
	Budget time.Duration
	// MaxConsecutiveFailures stops the pass once this many names in a row
	// have failed (0 = never): a dead or erroring Scryfall costs N requests,
	// not one per deck name. Any completed or genuine-404 name resets it.
	MaxConsecutiveFailures int
}

// errFillBudget and errFillTripped are the two ways fillLimits stop a pass.
var (
	errFillBudget  = errors.New("fill time budget spent")
	errFillTripped = errors.New("too many consecutive failures")
)

// fillArt walks every distinct card name (plus each commander) referenced by
// the deck files in dir, in sorted order, and leaves each one complete in the
// cache — art, facts sidecar, and (via fetchCard's sibling-face caching and
// face manifest) every other face of a multi-faced card, whatever the
// requested name's own files already were. It is the ONE fill loop: prewarmArt
// (serve's background pass) and the -prewarm-art-only one-shot both go
// through it, so their guarantees cannot drift apart. label is the log-line
// prefix distinguishing the caller ("art prewarm" / "art fill").
//
// A name already complete on disk costs nothing (ensureComplete is
// disk-checked and single-flight), which is what makes the fill idempotent:
// a second pass makes zero fetches. Every per-name error is logged through
// logf and left to self-heal — a network failure writes no .miss (only a
// genuine Scryfall 404 does), so the next pass or browser request retries
// the fetch.
//
// Before any name is walked the cache dir must accept a new file: a
// read-only dir would otherwise spend one Scryfall lookup per name before
// each first write failed. The pass ends early — with the partial stats and
// an error naming why — when ctx is cancelled or a fillLimits bound trips;
// names never attempted are not counted as failures. Every outcome, early or
// not, is logged as one summary line.
func fillArt(ctx context.Context, ac *artCache, dir, label string, logf func(string, ...any), lim fillLimits) (artFillStats, error) {
	if lim.Budget > 0 {
		bctx, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		stop := ac.afterFunc(lim.Budget, func() { cancel(fmt.Errorf("%w (%s)", errFillBudget, lim.Budget)) })
		defer stop()
		ctx = bctx
	}
	names, err := deckCardNames(dir)
	if err != nil {
		err = fmt.Errorf("reading decks in %s: %w", dir, err)
		logf("%s: %v", label, err)
		return artFillStats{}, err
	}
	st := artFillStats{Names: len(names)}
	if err := ac.checkWritable(); err != nil {
		logf("%s: %v", label, err)
		return st, err
	}
	logf("%s: %d distinct card names from %s", label, len(names), dir)
	stopped := func(err error) (artFillStats, error) {
		logf("%s: STOPPED (%v) after %d of %d names: %s; %d names unattempted",
			label, err, st.attempted(), st.Names, st, st.Names-st.attempted())
		return st, err
	}
	streak := 0
	for _, name := range names {
		if ctx.Err() != nil {
			return stopped(context.Cause(ctx))
		}
		key := artKey(name)
		complete := ac.complete(key)
		ok, err := ac.ensureComplete(ctx, key, name)
		if err != nil {
			st.Failed++
			if ctx.Err() != nil {
				return stopped(context.Cause(ctx)) // the name was cut off, not failed on its own
			}
			logf("%s %q: %v", label, name, err)
			if streak++; lim.MaxConsecutiveFailures > 0 && streak >= lim.MaxConsecutiveFailures {
				return stopped(fmt.Errorf("%w (%d in a row)", errFillTripped, streak))
			}
			continue
		}
		streak = 0
		if !ok {
			st.NotFound++ // a genuine Scryfall 404 recorded a .miss; nothing to backfill
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

// checkWritable proves the cache dir accepts a new file — the same staging
// create every artifact write starts with — and removes the probe.
func (a *artCache) checkWritable() error {
	f, err := a.createTemp(".write-probe-*.tmp")
	if err != nil {
		return fmt.Errorf("art cache dir %s is not writable: %w", a.dir, err)
	}
	name := f.Name()
	cerr := f.Close()
	_ = os.Remove(name)
	if cerr != nil {
		return fmt.Errorf("art cache dir %s is not writable: %w", a.dir, cerr)
	}
	return nil
}

// fileExists is os.Stat's presence check, named so the call sites read.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// prewarmArt fills the cache for every deck name in the background at serve
// startup (see fillArt for the loop itself). Never blocks or fails serving:
// fillArt logs every error and the summary, and leaves each failure to
// self-heal. It is deliberately unbounded (zero fillLimits) — it is what
// completes a cache the deploy's bounded one-shot left partial. ctx is
// serve()'s own, so shutdown cancels an in-flight prewarm cleanly.
func prewarmArt(ctx context.Context, ac *artCache, dir string, logf func(string, ...any)) {
	_, _ = fillArt(ctx, ac, dir, "art prewarm", logf, fillLimits{})
}

// runPrewarmArtOnly is the -prewarm-art-only body: fill the card-art cache
// from every deck in -decks (c.artCacheDir() is the destination) within the
// -prewarm-art-budget and -prewarm-art-max-consecutive-failures bounds, log
// to stderr, and return the process exit code. No listener, no corpus, no
// tables: the deploy runs this before any server starts, and it is a no-op
// (zero fetches) when the cache is already complete.
//
// Exit 0 means every name is complete or a genuine 404. Anything else — a
// failed name, a tripped bound, a cancel, an unreadable deck dir or an
// unwritable cache dir — exits 1 after one loud ART FILL INCOMPLETE line
// carrying the summary, so an operator running `make prewarm-art` by hand
// sees a strict result, and the deploy (which starts its servers regardless)
// leaves the reason in its log.
func runPrewarmArtOnly(ctx context.Context, c config, stderr io.Writer) int {
	logf := func(f string, a ...any) { fmt.Fprintf(stderr, "gorged: "+f+"\n", a...) }
	ac, err := newServeArtCache(c.artCacheDir())
	if err != nil {
		logf("%v", err)
		return 1
	}
	ac.logf = logf
	st, err := fillArt(ctx, ac, c.decks, "art fill", logf, fillLimits{
		Budget:                 c.prewarmArtBudget,
		MaxConsecutiveFailures: c.prewarmArtMaxConsecutiveFailures,
	})
	if err == nil && st.Failed == 0 {
		return 0
	}
	reason := fmt.Sprintf("%d names failed", st.Failed)
	if err != nil {
		reason = err.Error()
	}
	logf("ART FILL INCOMPLETE (%s): %s, %d names unattempted", reason, st, st.Names-st.attempted())
	return 1
}
