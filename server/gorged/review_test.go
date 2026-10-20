package gorged

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
	"github.com/adams-shaun/gorge/testkit/testutil/feedback"
)

// The committed feedback fixtures (cmd/repro/testdata/feedback): fb01 is a
// short constructed capture, e484f1db a 14-turn commander game whose
// recording ends in a burst's overshoot tail.
const (
	reviewFixtureShort = "../../cmd/repro/testdata/feedback/20260914T120000Z-fb01"
	reviewFixtureLong  = "../../cmd/repro/testdata/feedback/20260915T094418Z-e484f1db"
)

// feedbackReviewLoader is cmd/gorged's loader, rebuilt here: server/gorged
// itself never imports testkit, but its tests may.
func feedbackReviewLoader(dir string) (host.ReviewMatch, error) {
	l, cfg, meta, err := feedback.Load(dir)
	if err != nil {
		return host.ReviewMatch{}, err
	}
	return host.ReviewMatch{Config: cfg, Log: l, Label: meta.ID}, nil
}

// reviewServer serves both committed fixtures through review mode's own
// handler.
func reviewServer(t *testing.T) *httptest.Server {
	t.Helper()
	testutil.CorpusRegistry(t) // Skips when .cards/ is absent
	r, err := buildReviewRegistry([]string{reviewFixtureShort, reviewFixtureLong}, feedbackReviewLoader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	srv := httptest.NewServer(reviewHandler(r))
	t.Cleanup(srv.Close)
	return srv
}

func reviewGet(t *testing.T, url string, into any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if into != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
			t.Fatalf("%s: %v", url, err)
		}
	}
	return resp.StatusCode
}

// TestReviewServesTheFinishedMatchRouteWithEveryHand is the design's T2
// done-means, browser-free: each fixture is a review table the DVR's
// finished-match route can open (/api/tables marks it review, its match
// lists with its events), every view?seq= at an early, a middle and the last
// seq fans both hands face up with no library contents, and events?since=
// pages through to the head.
func TestReviewServesTheFinishedMatchRouteWithEveryHand(t *testing.T) {
	t.Parallel()
	srv := reviewServer(t)
	var tables []protocol.TableInfo
	if code := reviewGet(t, srv.URL+"/api/tables", &tables); code != http.StatusOK || len(tables) != 2 {
		t.Fatalf("/api/tables = %d, %+v", code, tables)
	}
	for _, ti := range tables {
		if !ti.Review || ti.Spectator != "omniscient" || ti.Match != 1 || ti.Seats != 2 {
			t.Fatalf("table %+v", ti)
		}
		var ms []protocol.MatchInfo
		base := fmt.Sprintf("%s/api/tables/%s/matches", srv.URL, ti.ID)
		if code := reviewGet(t, base, &ms); code != http.StatusOK || len(ms) != 1 || ms[0].Match != 1 || ms[0].Events == 0 || len(ms[0].Seats) != 2 {
			t.Fatalf("%s: %d %+v", ti.ID, code, ms)
		}
		head := uint64(ms[0].Events - 1)
		for _, seq := range []uint64{10, head / 2, head} {
			var v view.View
			if code := reviewGet(t, fmt.Sprintf("%s/1/view?seq=%d", base, seq), &v); code != http.StatusOK {
				t.Fatalf("%s view seq %d: %d", ti.ID, seq, code)
			}
			if len(v.Players) != 2 {
				t.Fatalf("%s seq %d: %d players", ti.ID, seq, len(v.Players))
			}
			for _, p := range v.Players {
				if p.Hand == nil || len(p.Hand) != p.HandSize || p.Library != nil || p.LibraryTop != nil {
					t.Fatalf("%s seq %d seat %d: hand %v (size %d) library %v top %v", ti.ID, seq, p.ID, p.Hand, p.HandSize, p.Library, p.LibraryTop)
				}
			}
		}
		if seq := head / 2; seq > 0 {
			var v view.View
			reviewGet(t, fmt.Sprintf("%s/1/view?seq=%d", base, seq), &v)
			hands := 0
			for _, p := range v.Players {
				hands += len(p.Hand)
			}
			if hands == 0 {
				t.Fatalf("%s seq %d: no card in any hand", ti.ID, seq)
			}
		}
		total := uint64(0)
		for since := uint64(0); since <= head; {
			var page []protocol.EventBody
			if code := reviewGet(t, fmt.Sprintf("%s/1/events?since=%d", base, since), &page); code != http.StatusOK || len(page) == 0 {
				t.Fatalf("%s events since %d: %d (%d bodies)", ti.ID, since, code, len(page))
			}
			if page[0].Event.Seq != since {
				t.Fatalf("%s events since %d start at %d", ti.ID, since, page[0].Event.Seq)
			}
			total += uint64(len(page))
			since += uint64(len(page))
		}
		if total != head+1 {
			t.Fatalf("%s: paged %d events, head %d", ti.ID, total, head)
		}
		if code := reviewGet(t, fmt.Sprintf("%s/1/events?since=%d", base, head+1), nil); code != http.StatusConflict {
			t.Fatalf("%s events past head: %d, want 409", ti.ID, code)
		}
	}
}

// TestReviewHandlerServesNoSeatRoute: review mode mounts httpapi with no seat
// resolver, so every seat-scoped route is refused 403 -- reads and writes
// alike -- and neither play-vs-bot nor a decisions route exists.
func TestReviewHandlerServesNoSeatRoute(t *testing.T) {
	t.Parallel()
	srv := reviewServer(t)
	m := srv.URL + "/api/tables/20260914T120000Z-fb01/matches/1"
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"GET", m + "/pending?seat=0", http.StatusForbidden},
		{"GET", m + "/pending?seat=1", http.StatusForbidden},
		{"GET", m + "/view?seq=5&seat=0", http.StatusForbidden},
		{"GET", m + "/events?since=0&seat=1", http.StatusForbidden},
		{"POST", m + "/intent?seat=0", http.StatusForbidden},
		{"POST", m + "/undo?seat=0", http.StatusForbidden},
		{"POST", srv.URL + "/api/games", http.StatusNotFound},
		{"GET", m + "/decisions", http.StatusNotFound},
	} {
		req, err := http.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer anything")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatalf("%s %s = %d %s, want %d", tc.method, tc.path, resp.StatusCode, body, tc.want)
		}
	}
}

// TestReviewRefusesANonLoopbackAddr: review mode refuses a wildcard or
// public listener before loading anything, unless -review-public.
func TestReviewRefusesANonLoopbackAddr(t *testing.T) {
	t.Parallel()
	ln := &fakeListener{addr: &net.TCPAddr{IP: net.IPv4zero, Port: 1}}
	loaded := false
	ext := Extensions{Review: func(string) (host.ReviewMatch, error) {
		loaded = true
		return host.ReviewMatch{}, fmt.Errorf("unreached")
	}}
	err := ServeReview(context.Background(), []string{"-records", "x"}, ln, ext)
	if err == nil || !strings.Contains(err.Error(), "loopback") || loaded {
		t.Fatalf("ServeReview on 0.0.0.0 = %v (loaded %t), want the loopback refusal before any load", err, loaded)
	}
	// -review-public lifts the guard: the start now gets as far as the
	// loader (whose failure refuses the whole start).
	err = ServeReview(context.Background(), []string{"-review-public", "x"}, ln, ext)
	if err == nil || !strings.Contains(err.Error(), "unreached") || !loaded {
		t.Fatalf("ServeReview -review-public = %v (loaded %t), want the loader's error", err, loaded)
	}
	if code := Main([]string{"review", "-addr", "127.0.0.1:0"}, ext); code != 2 {
		t.Fatalf("review with no records exited %d, want 2", code)
	}
	if err := ServeReview(context.Background(), []string{"x"}, &fakeListener{addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}}, Extensions{}); err == nil || !strings.Contains(err.Error(), "no record loader") {
		t.Fatalf("ServeReview with no loader = %v", err)
	}
}

// TestReviewRefusesAnInputThatDoesNotReplay: one bad input refuses the whole
// start, naming it.
func TestReviewRefusesAnInputThatDoesNotReplay(t *testing.T) {
	t.Parallel()
	testutil.CorpusRegistry(t)
	broken := func(dir string) (host.ReviewMatch, error) {
		m, err := feedbackReviewLoader(dir)
		if err != nil {
			return m, err
		}
		m.Log.Seed++ // a different deal: the replay diverges at the first shuffle
		return m, nil
	}
	_, err := buildReviewRegistry([]string{reviewFixtureShort}, broken)
	if err == nil || !strings.Contains(err.Error(), "does not replay") || !strings.Contains(err.Error(), "20260914T120000Z-fb01") {
		t.Fatalf("buildReviewRegistry = %v, want the replay refusal naming the input", err)
	}
}

// TestLiveServerHasNoReviewBehaviour: the live server knows nothing of
// review mode -- its flag set has no review flag, its /api/tables carries no
// review key, and no decisions route exists on it.
func TestLiveServerHasNoReviewBehaviour(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"records", "review-public"} {
		if ServeFlagSet().Lookup(f) != nil {
			t.Fatalf("the live flag set defines -%s", f)
		}
	}
	url, cancel, done := startServe(t, config{tables: 1, seats: 2, pace: 0})
	defer cancel()
	waitTables(t, url, 1)
	resp, err := http.Get(url + "/api/tables")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(body), `"review"`) {
		t.Fatalf("live /api/tables carries a review key: %s", body)
	}
	expectStatus(t, url, "/api/tables/t1/matches/1/decisions", http.StatusNotFound)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

func TestReviewTableIDs(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"cmd/repro/testdata/feedback/20260914T120000Z-fb01/": "20260914T120000Z-fb01",
		"/x/run7/g12.match.json.gz":                          "g12",
		"a b?c":                                              "a-b-c",
	} {
		if got := reviewTableID(in); got != want {
			t.Errorf("reviewTableID(%q) = %q, want %q", in, got, want)
		}
	}
}
