package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// pairedServer is a two-seat table with one human, seat 0, on a server with a
// paired bot, parked on that seat's first decision. "s0" is its token, and "s1"
// claims the bot's seat on the same table.
func pairedServer(t *testing.T, entry *bots.Entry) (*httptest.Server, *host.Registry) {
	t.Helper()
	r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}, PairedBot: entry})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: "t1", Name: "paired", Seats: 2, Decks: []string{"a", "b"}, Seed: 5,
		Spectator: view.Omniscient, Humans: []int{0}, BotPolicy: "bot"}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewHandler(r, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		switch req.Header.Get("X-Seat-Session") {
		case "s0":
			return SeatClaim{Table: "t1", Seat: 0}, true
		case "s1":
			return SeatClaim{Table: "t1", Seat: 1}, true
		case "elsewhere":
			return SeatClaim{Table: "t2", Seat: 0}, true
		}
		return SeatClaim{}, false
	}}))
	t.Cleanup(srv.Close)
	return srv, r
}

func pairedEntry(t *testing.T) *bots.Entry {
	t.Helper()
	_, decks := testutil.SampleDecks(t, 4)
	b, err := searchprobe.NewBelief(decks)
	if err != nil {
		t.Fatal(err)
	}
	return &bots.Entry{Info: bots.Info{Name: "test-paired"}, New: azredeal.ClosedFactory(b, 4)}
}

func parkedSeq(t *testing.T, r *host.Registry, seat state.PlayerID) uint64 {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if d, err := r.Pending("t1", 1, seat); err == nil {
			return d.Seq
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("seat parked nothing")
	return 0
}

// Advice goes to the seat the token names and to nobody else.
func TestAdviceIsForTheClaimedSeatOnly(t *testing.T) {
	srv, r := pairedServer(t, pairedEntry(t))
	seq := parkedSeq(t, r, 0)
	url := fmt.Sprintf("%s/api/tables/t1/matches/1/advice?seq=%d", srv.URL, seq)

	for _, c := range []struct {
		name, session string
		want          int
	}{
		{"a spectator", "", http.StatusUnauthorized},
		{"an unknown token", "nobody", http.StatusUnauthorized},
		{"another table's token", "elsewhere", http.StatusForbidden},
		{"another seat's token", "s1", http.StatusConflict},
	} {
		code, _, raw := seatReq(t, http.MethodGet, url, c.session, nil)
		if code != c.want {
			t.Errorf("%s: status %d, want %d: %s", c.name, code, c.want, raw)
		}
		var a protocol.Advice
		if json.Unmarshal(raw, &a) == nil && (a.State != "" || len(a.Explain) > 0) {
			t.Errorf("%s got advice: %s", c.name, raw)
		}
	}
	if code, _, raw := seatReq(t, http.MethodGet, srv.URL+"/api/tables/t1/matches/1/advice?seq=x", "s0", nil); code != http.StatusBadRequest {
		t.Errorf("bad seq: status %d: %s", code, raw)
	}
	stale := fmt.Sprintf("%s/api/tables/t1/matches/1/advice?seq=%d", srv.URL, seq+1)
	if code, _, raw := seatReq(t, http.MethodGet, stale, "s0", nil); code != http.StatusConflict || !strings.Contains(string(raw), `"advice_stale"`) {
		t.Errorf("a decision that is not pending: status %d: %s", code, raw)
	}
	code, _, raw := seatReq(t, http.MethodGet, url, "s0", nil)
	if code != http.StatusOK {
		t.Fatalf("the seat itself: status %d: %s", code, raw)
	}
	var a protocol.Advice
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	if a.AskSeq != seq || a.Seat != 0 || a.Table != "t1" || a.State == "" {
		t.Fatalf("advice %+v", a)
	}
}

// A server without the option answers plainly and never searches.
func TestPairedBotOffByDefault(t *testing.T) {
	srv, r := pairedServer(t, nil)
	seq := parkedSeq(t, r, 0)
	code, _, raw := seatReq(t, http.MethodGet, srv.URL+"/api/tables/t1/paired-bot", "", nil)
	var st protocol.PairedBotStatus
	if err := json.Unmarshal(raw, &st); err != nil || code != http.StatusOK || st.Offered || st.Reason != "" {
		t.Fatalf("status %d %s %v", code, raw, err)
	}
	url := fmt.Sprintf("%s/api/tables/t1/matches/1/advice?seq=%d", srv.URL, seq)
	if code, _, raw := seatReq(t, http.MethodGet, url, "s0", nil); code != http.StatusConflict {
		t.Fatalf("advice on a server without it: status %d: %s", code, raw)
	}
}
