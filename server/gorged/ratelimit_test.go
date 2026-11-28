package gorged

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/protocol"
)

// rlClock is an injected clock for the limiter tests.
type rlClock struct{ t time.Time }

func (c *rlClock) now() time.Time { return c.t }

// newTestLimiter builds a limiter on an injected clock and the handler it
// wraps around a stub that counts the requests it lets through.
func newTestLimiter(t *testing.T, header string) (*rateLimiter, *rlClock, http.Handler, *int) {
	t.Helper()
	l := newRateLimiter(10, 10, header)
	if l == nil {
		t.Fatal("newRateLimiter(10, 10) = nil, want a limiter")
	}
	clk := &rlClock{t: time.Unix(1_800_000_000, 0)}
	l.now = clk.now
	passed := new(int)
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*passed++
		w.WriteHeader(http.StatusOK)
	})
	return l, clk, l.wrap(stub), passed
}

func rlDo(h http.Handler, method, path, remote string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remote
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateLimitEleventhCreateIs429(t *testing.T) {
	_, clk, h, passed := newTestLimiter(t, "")
	for i := 0; i < 10; i++ {
		if rec := rlDo(h, http.MethodPost, "/api/games", "198.51.100.7:1000", nil); rec.Code != http.StatusOK {
			t.Fatalf("request %d: status %d, want 200", i+1, rec.Code)
		}
	}
	if *passed != 10 {
		t.Fatalf("stub saw %d requests, want 10", *passed)
	}
	rec := rlDo(h, http.MethodPost, "/api/games", "198.51.100.7:1000", nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("11th request: status %d, want 429", rec.Code)
	}
	if *passed != 10 {
		t.Fatalf("the 429 reached the handler (stub saw %d)", *passed)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	retry, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retry != 6 {
		t.Errorf("Retry-After = %q, want 6", rec.Header().Get("Retry-After"))
	}
	var body protocol.ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("429 body %q: %v", rec.Body.String(), err)
	}
	want := "too many requests from this client: at most 10 a minute; retry in 6s"
	if body.Code != "rate_limited" || body.Message != want {
		t.Errorf("429 body = %+v, want code rate_limited, message %q", body, want)
	}
	if rec := rlDo(h, http.MethodPost, "/api/games", "198.51.100.8:1000", nil); rec.Code != http.StatusOK {
		t.Errorf("another client: status %d, want 200", rec.Code)
	}
	clk.t = clk.t.Add(6 * time.Second)
	if rec := rlDo(h, http.MethodPost, "/api/games", "198.51.100.7:1000", nil); rec.Code != http.StatusOK {
		t.Errorf("after 6s: status %d, want 200", rec.Code)
	}
}

func TestRateLimitIgnoresUntrustedHeader(t *testing.T) {
	_, _, h, _ := newTestLimiter(t, "")
	for i := 0; i < 11; i++ {
		hdr := map[string]string{"CF-Connecting-IP": fmt.Sprintf("203.0.113.%d", i+1)}
		rec := rlDo(h, http.MethodPost, "/api/events", "198.51.100.7:1000", hdr)
		want := http.StatusOK
		if i == 10 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Fatalf("request %d: status %d, want %d", i+1, rec.Code, want)
		}
	}
}

func TestRateLimitTrustedHeaderKeys(t *testing.T) {
	l, _, h, _ := newTestLimiter(t, "cf-connecting-ip")
	for _, ip := range []string{"203.0.113.1", "203.0.113.2"} {
		hdr := map[string]string{"CF-Connecting-IP": ip}
		for i := 0; i < 10; i++ {
			if rec := rlDo(h, http.MethodPost, "/api/decks/check", "198.51.100.7:1000", hdr); rec.Code != http.StatusOK {
				t.Fatalf("%s request %d: status %d, want 200", ip, i+1, rec.Code)
			}
		}
		if rec := rlDo(h, http.MethodPost, "/api/decks/check", "198.51.100.7:1000", hdr); rec.Code != http.StatusTooManyRequests {
			t.Fatalf("%s 11th request: status %d, want 429", ip, rec.Code)
		}
	}
	// The remote address's own bucket is untouched, so an unparsable header
	// falls back to it and passes.
	bad := map[string]string{"CF-Connecting-IP": "not-an-ip"}
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", nil)
	req.RemoteAddr = "198.51.100.7:1000"
	req.Header.Set("CF-Connecting-IP", "not-an-ip")
	if k := l.key(req); k != "198.51.100.7" {
		t.Fatalf("key with unparsable header = %q, want the remote address", k)
	}
	if rec := rlDo(h, http.MethodPost, "/api/feedback", "198.51.100.7:1000", bad); rec.Code != http.StatusOK {
		t.Errorf("unparsable header: status %d, want 200", rec.Code)
	}
	// An IPv6 key is its /64.
	req.Header.Set("CF-Connecting-IP", "2001:db8:1:2:3:4:5:6")
	if k := l.key(req); k != "2001:db8:1:2::/64" {
		t.Errorf("IPv6 key = %q, want 2001:db8:1:2::/64", k)
	}
}

func TestRateLimitLeavesReadsAlone(t *testing.T) {
	_, _, h, passed := newTestLimiter(t, "")
	for i := 0; i < 100; i++ {
		if rec := rlDo(h, http.MethodGet, "/api/tables", "198.51.100.7:1000", nil); rec.Code != http.StatusOK {
			t.Fatalf("GET %d: status %d", i+1, rec.Code)
		}
		if rec := rlDo(h, http.MethodPost, "/api/subscribe", "198.51.100.7:1000", nil); rec.Code != http.StatusOK {
			t.Fatalf("POST /api/subscribe %d: status %d", i+1, rec.Code)
		}
	}
	if *passed != 200 {
		t.Fatalf("stub saw %d requests, want 200", *passed)
	}
	// Precondition: the same client IS limited on a creating route.
	for i := 0; i < 10; i++ {
		rlDo(h, http.MethodPost, "/api/games", "198.51.100.7:1000", nil)
	}
	if rec := rlDo(h, http.MethodPost, "/api/games", "198.51.100.7:1000", nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("11th POST /api/games: status %d, want 429", rec.Code)
	}
}

func TestRateLimitBucketCap(t *testing.T) {
	l, clk, _, _ := newTestLimiter(t, "")
	for i := 0; i < 5000; i++ {
		clk.t = clk.t.Add(time.Millisecond)
		if ok, _ := l.allow(fmt.Sprintf("10.%d.%d.%d", i>>16, (i>>8)&0xff, i&0xff)); !ok {
			t.Fatalf("key %d refused on its first request", i)
		}
	}
	l.mu.Lock()
	n := len(l.buckets)
	l.mu.Unlock()
	if n > rateLimitMaxBuckets || rateLimitMaxBuckets != 4096 {
		t.Fatalf("%d buckets after 5000 keys, want at most 4096 (cap %d)", n, rateLimitMaxBuckets)
	}
	if n == 0 {
		t.Fatal("no buckets kept")
	}
}

func TestRateLimitOffIsNil(t *testing.T) {
	var l *rateLimiter = newRateLimiter(0, 10, "")
	if l != nil {
		t.Fatal("newRateLimiter(0, ...) != nil")
	}
	stub := http.NotFoundHandler()
	if got := l.wrap(stub); fmt.Sprintf("%p", got) != fmt.Sprintf("%p", stub) {
		t.Error("nil limiter's wrap did not return next")
	}
}
