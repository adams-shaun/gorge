package gorged

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/server/protocol"
)

// rateLimitMaxBuckets bounds the limiter's memory: past it, buckets that
// have refilled (idle clients) go first, else the least recently used one.
const rateLimitMaxBuckets = 4096

// rateLimiter is per-client HTTP admission for the creating endpoints
// (limitedRoute): a token bucket per client key, capacity burst, refilled
// at perMinute tokens a minute. It sits outside the game: its clock never
// reaches an event, a seed or a table.
type rateLimiter struct {
	perMinute, burst int
	header           string           // canonical header name, "" = remote address only
	now              func() time.Time // time.Now in production; tests inject a clock
	mu               sync.Mutex
	buckets          map[string]*bucket // key -> tokens, last
}

// bucket is one client's tokens as of last.
type bucket struct {
	tokens float64
	last   time.Time
}

// newRateLimiter returns nil when perMinute == 0 (the limiter is off).
func newRateLimiter(perMinute, burst int, header string) *rateLimiter {
	if perMinute == 0 {
		return nil
	}
	if header != "" {
		header = http.CanonicalHeaderKey(header)
	}
	return &rateLimiter{
		perMinute: perMinute,
		burst:     burst,
		header:    header,
		now:       time.Now,
		buckets:   make(map[string]*bucket),
	}
}

// limitedRoute reports whether r is one of the creating requests the
// limiter counts.
func limitedRoute(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	switch r.URL.Path {
	case "/api/games", "/api/events", "/api/decks/check", "/api/feedback":
		return true
	}
	return false
}

// key is the client r counts against: the trusted proxy header's address
// when the flag names one and it parses, else the connection's remote
// address. An IPv6 address keys on its /64.
func (l *rateLimiter) key(r *http.Request) string {
	if l.header != "" {
		if a, err := netip.ParseAddr(r.Header.Get(l.header)); err == nil {
			return addrKey(a)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if a, err := netip.ParseAddr(host); err == nil {
		return addrKey(a)
	}
	return host
}

func addrKey(a netip.Addr) string {
	a = a.Unmap().WithZone("")
	if a.Is4() {
		return a.String()
	}
	return netip.PrefixFrom(a, 64).Masked().String()
}

// allow takes one token from key's bucket; when it is empty it returns
// false and how long until a token is back.
func (l *rateLimiter) allow(key string) (ok bool, retry time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b == nil {
		if len(l.buckets) >= rateLimitMaxBuckets {
			l.evict(now)
		}
		b = &bucket{tokens: float64(l.burst), last: now}
		l.buckets[key] = b
	}
	l.refill(b, now)
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	perSecond := float64(l.perMinute) / 60
	return false, time.Duration((1 - b.tokens) / perSecond * float64(time.Second))
}

func (l *rateLimiter) refill(b *bucket, now time.Time) {
	if el := now.Sub(b.last); el > 0 {
		b.tokens = math.Min(float64(l.burst), b.tokens+el.Seconds()*float64(l.perMinute)/60)
	}
	b.last = now
}

// evict makes room for one bucket: every bucket that is full again goes,
// and when none is, the least recently used one. Which bucket goes never
// reaches a game, so the map order here is harmless.
func (l *rateLimiter) evict(now time.Time) {
	var lruKey string
	var lru *bucket
	dropped := false
	for k, b := range l.buckets {
		l.refill(b, now)
		if b.tokens >= float64(l.burst) {
			delete(l.buckets, k)
			dropped = true
			continue
		}
		if lru == nil || b.last.Before(lru.last) {
			lruKey, lru = k, b
		}
	}
	if !dropped && lru != nil {
		delete(l.buckets, lruKey)
	}
}

// wrap admits next's creating requests through the limiter and answers
// 429 rate_limited for an empty bucket. A nil limiter returns next.
func (l *rateLimiter) wrap(next http.Handler) http.Handler {
	if l == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limitedRoute(r) {
			next.ServeHTTP(w, r)
			return
		}
		ok, retry := l.allow(l.key(r))
		if ok {
			next.ServeHTTP(w, r)
			return
		}
		secs := int((retry + time.Second - 1) / time.Second)
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(protocol.ErrorBody{
			Code:    "rate_limited",
			Message: fmt.Sprintf("too many requests from this client: at most %d a minute; retry in %ds", l.perMinute, secs),
		})
	})
}
