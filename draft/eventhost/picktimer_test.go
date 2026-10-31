package eventhost

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// fakeClock is an Options.Clock whose timers fire synchronously from
// Advance, earliest due first, in insertion order on a tie.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	at time.Time
	f  func()
}

func newFakeClock() *fakeClock { return &fakeClock{now: time.Unix(1700000000, 0)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) AfterFunc(d time.Duration, f func()) (stop func() bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i, x := range c.timers {
			if x == t {
				c.timers = append(c.timers[:i], c.timers[i+1:]...)
				return true
			}
		}
		return false
	}
}

// Advance moves the clock d forward and fires every due timer, one at a
// time, re-scanning after each (a callback may arm new timers).
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
	for {
		c.mu.Lock()
		var due *fakeTimer
		idx := -1
		for i, t := range c.timers {
			if t.at.After(c.now) {
				continue
			}
			if due == nil || t.at.Before(due.at) {
				due, idx = t, i
			}
		}
		if due != nil {
			c.timers = append(c.timers[:idx], c.timers[idx+1:]...)
		}
		c.mu.Unlock()
		if due == nil {
			return
		}
		due.f()
	}
}

// twoHumansReq is the standard request with a second human at seat 3.
func twoHumansReq() protocol.CreateEventRequest {
	req := oneHumanReq()
	req.Seats[3] = protocol.CreateEventSeat{Kind: "human"}
	return req
}

// seatToken is the invite token of one seat.
func seatToken(t *testing.T, resp protocol.CreateEventResponse, seat int) string {
	t.Helper()
	for _, inv := range resp.Invites {
		if strings.Contains(inv.Join, fmt.Sprintf("seat=%d", seat)) {
			return inv.Token
		}
	}
	t.Fatalf("no seat-%d invite", seat)
	return ""
}

// pickedEntries lists the log's Picked entries. It takes and releases
// run.mu itself.
func pickedEntries(t *testing.T, r *run) []event.Entry {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []event.Entry
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindPicked {
			out = append(out, en)
		}
	}
	return out
}

// assertArmed checks that seat's pick timer is armed for exactly the pack
// at its queue head, and returns the timer and that view.
func assertArmed(t *testing.T, h *Host, id string, seat int) (pickTimer, draft.PickView) {
	t.Helper()
	r := runOf(t, h, id)
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.ev.Next(seat)
	if !ok {
		t.Fatalf("seat %d has no pack to pick from", seat)
	}
	tm := r.timers[seat]
	if tm.stop == nil {
		t.Fatalf("seat %d has no armed timer", seat)
	}
	if tm.round != v.Round || tm.pickNo != v.PickNo {
		t.Fatalf("seat %d armed for round %d pick %d, queue head is round %d pick %d",
			seat, tm.round, tm.pickNo, v.Round, v.PickNo)
	}
	return tm, v
}

// assertDisarmed checks that seat's pick timer is not armed.
func assertDisarmed(t *testing.T, h *Host, id string, seat int) {
	t.Helper()
	r := runOf(t, h, id)
	r.mu.Lock()
	defer r.mu.Unlock()
	if tm := r.timers[seat]; tm.stop != nil {
		t.Fatalf("seat %d timer armed for round %d pick %d, want none", seat, tm.round, tm.pickNo)
	}
}

func TestPickLimitTable(t *testing.T) {
	h := &Host{}
	want := [15]int{75, 70, 65, 60, 55, 50, 45, 40, 30, 25, 20, 15, 12, 10, 7}
	for cardsLeft := 15; cardsLeft >= 1; cardsLeft-- {
		got := h.pickLimit("standard", cardsLeft)
		if wantd := time.Duration(want[15-cardsLeft]) * time.Second; got != wantd {
			t.Fatalf("pickLimit(standard, %d cards) = %v, want %v", cardsLeft, got, wantd)
		}
	}
	if got := h.pickLimit("relaxed", 14); got != 105*time.Second {
		t.Fatalf("pickLimit(relaxed, 14 cards) = %v, want 105s", got)
	}
	if got := h.pickLimit("standard", 0); got != 7*time.Second {
		t.Fatalf("pickLimit(standard, 0 cards) = %v, want 7s", got)
	}
	if got := h.pickLimit("standard", 20); got != 75*time.Second {
		t.Fatalf("pickLimit(standard, 20 cards) = %v, want 75s", got)
	}
	if got := h.pickLimit("off", 14); got != 0 {
		t.Fatalf("pickLimit(off, 14 cards) = %v, want 0", got)
	}
	if got := h.pickLimit("", 14); got != 0 {
		t.Fatalf("pickLimit(\"\", 14 cards) = %v, want 0", got)
	}
	pt := &Host{opts: Options{PickTimeout: 3 * time.Second}}
	for cardsLeft := 0; cardsLeft <= 20; cardsLeft++ {
		for _, mode := range []string{"standard", "relaxed"} {
			if got := pt.pickLimit(mode, cardsLeft); got != 3*time.Second {
				t.Fatalf("pickLimit(%s, %d cards) with PickTimeout = %v, want 3s", mode, cardsLeft, got)
			}
		}
	}
	if pt.pickLimit("off", 14) != 0 {
		t.Fatal("PickTimeout must not arm a timer in the off mode")
	}
	_, err := New(Options{Registry: newRegistry(t), Store: mustStore(t, ""), Cards: testutil.CorpusRegistry(t), PickTimeout: -time.Second})
	if err == nil {
		t.Fatal("negative PickTimeout accepted")
	}
}

func TestPickTimerOffForSingleHuman(t *testing.T) {
	clk := newFakeClock()
	h, _ := newHost(t, func(o *Options) { o.Clock = clk })
	req := oneHumanReq()
	req.PickTimer = "standard"
	resp := createOK(t, h, req)
	tok, _ := humanToken(t, resp)
	if st := stateOf(t, h, resp.Event, tok); st.PickTimer != "off" {
		t.Fatalf("state pick_timer = %q, want off", st.PickTimer)
	}
	joinOK(t, h, resp.Event, tok, "Ann")
	p := packOf(t, h, resp.Event, tok)
	if p.Waiting || len(p.Pack) == 0 || p.Round != 1 || p.PickNo != 1 {
		t.Fatalf("precondition: no live pick-1 pack: %+v", p)
	}
	if p.TimeLimitMS != 0 || p.RemainingMS != 0 {
		t.Fatalf("pack timer fields = %d/%d, want 0/0", p.TimeLimitMS, p.RemainingMS)
	}
	r := runOf(t, h, resp.Event)
	r.mu.Lock()
	defer r.mu.Unlock()
	for s, tm := range r.timers {
		if tm.stop != nil {
			t.Fatalf("seat %d timer armed, want none", s)
		}
	}
}

func TestPickTimerAutoPicksOnExpiry(t *testing.T) {
	clk := newFakeClock()
	h, _ := newHost(t, func(o *Options) { o.Clock = clk })
	resp := createOK(t, h, twoHumansReq())
	tok0, _ := humanToken(t, resp)
	tok3 := seatToken(t, resp, 3)
	joinOK(t, h, resp.Event, tok0, "Ann")
	joinOK(t, h, resp.Event, tok3, "Eve")

	p0 := packOf(t, h, resp.Event, tok0)
	p3 := packOf(t, h, resp.Event, tok3)
	if p0.Round != 1 || p0.PickNo != 1 || p3.Round != 1 || p3.PickNo != 1 {
		t.Fatalf("precondition: packs at %d/%d and %d/%d, want both 1/1",
			p0.Round, p0.PickNo, p3.Round, p3.PickNo)
	}
	if p0.TimeLimitMS != 70000 || p3.TimeLimitMS != 70000 {
		t.Fatalf("pack time limits = %d/%d, want 70000/70000", p0.TimeLimitMS, p3.TimeLimitMS)
	}
	if st := stateOf(t, h, resp.Event, tok0); st.PickTimer != "standard" {
		t.Fatalf("state pick_timer = %q, want standard", st.PickTimer)
	}

	// The id seat 3's timer will log, computed on a copy of the view it
	// is armed for, before the fire.
	r := runOf(t, h, resp.Event)
	r.mu.Lock()
	v3, ok := r.ev.Next(3)
	if !ok {
		r.mu.Unlock()
		t.Fatal("seat 3 has no pack")
	}
	cp := draft.PickView{Set: v3.Set, Round: v3.Round, PickNo: v3.PickNo, Seats: v3.Seats,
		Pack: slices.Clone(v3.Pack), Picks: slices.Clone(v3.Picks)}
	for _, seen := range v3.Seen {
		cp.Seen = append(cp.Seen, slices.Clone(seen))
	}
	expected := r.drafters[3].Pick(cp, pickRNG(r.meta.Seed, 3, v3.Round, v3.PickNo))
	r.mu.Unlock()

	// Seat 0 picks by hand at +10 s.
	clk.Advance(10 * time.Second)
	p0 = packOf(t, h, resp.Event, tok0)
	if p0.Waiting || p0.PickNo != 1 {
		t.Fatalf("precondition: seat 0 pack before hand pick: %+v", p0)
	}
	if _, err := h.Pick(resp.Event, tok0, protocol.PickRequest{
		Round: p0.Round, PickNo: p0.PickNo, ID: p0.Pack[0].ID,
	}); err != nil {
		t.Fatalf("seat 0 hand pick: %v", err)
	}

	// +71 s: seat 3's 70 s deadline (armed at the join) has passed.
	clk.Advance(61 * time.Second)
	picks := pickedEntries(t, r)
	var auto *event.Entry
	var hand int
	for i := range picks {
		en := picks[i]
		if en.Seat == 3 && en.Round == 1 && en.PickNo == 1 {
			if en.Auto {
				auto = &picks[i]
			}
			continue
		}
		if en.Seat == 0 && en.Round == 1 && en.PickNo == 1 {
			hand++
			if en.Auto {
				t.Fatal("seat 0's hand pick logged as Auto")
			}
		}
	}
	if hand != 1 {
		t.Fatalf("seat 0 logged %d pick-1 entries, want 1", hand)
	}
	if auto == nil {
		t.Fatalf("no auto Picked for seat 3 round 1 pick 1; log: %+v", picks)
	}
	if auto.ID != expected {
		t.Fatalf("auto pick id %q, want the drafter's own %q", auto.ID, expected)
	}

	// Seat 0 received seat 3's passed pack and is re-armed for 13 cards.
	tm0, v0 := assertArmed(t, h, resp.Event, 0)
	if len(v0.Pack) != 13 {
		t.Fatalf("seat 0 re-armed for a %d-card pack, want 13", len(v0.Pack))
	}
	if tm0.limit != 65*time.Second {
		t.Fatalf("seat 0 re-armed with limit %v, want 65s", tm0.limit)
	}
	p0 = packOf(t, h, resp.Event, tok0)
	if p0.TimeLimitMS != 65000 {
		t.Fatalf("seat 0 pack time_limit_ms = %d, want 65000", p0.TimeLimitMS)
	}
}

func TestPickTimerStaleFireIsIgnored(t *testing.T) {
	clk := newFakeClock()
	h, _ := newHost(t, func(o *Options) { o.Clock = clk })
	resp := createOK(t, h, twoHumansReq())
	tok0, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok0, "Ann")
	joinOK(t, h, resp.Event, seatToken(t, resp, 3), "Eve")

	// Seat 0 picks its (1,1) pack by hand, so a timer fire for (1,1) is
	// stale by construction.
	clk.Advance(10 * time.Second)
	p0 := packOf(t, h, resp.Event, tok0)
	if p0.Waiting || p0.PickNo != 1 {
		t.Fatalf("precondition: seat 0 pack: %+v", p0)
	}
	if _, err := h.Pick(resp.Event, tok0, protocol.PickRequest{
		Round: p0.Round, PickNo: p0.PickNo, ID: p0.Pack[0].ID,
	}); err != nil {
		t.Fatalf("seat 0 hand pick: %v", err)
	}
	r := runOf(t, h, resp.Event)
	before := pickedEntries(t, r)

	// The stale fire: seat 0's (1,1) deadline, already answered by hand.
	h.pickExpired(resp.Event, 0, p0.Round, p0.PickNo)

	after := pickedEntries(t, r)
	if len(after) != len(before) {
		t.Fatalf("stale fire added entries: before %d, after %d", len(before), len(after))
	}
	for i := range after {
		if !reflect.DeepEqual(after[i], before[i]) {
			t.Fatalf("stale fire changed entry %d: %+v -> %+v", i, before[i], after[i])
		}
	}
}

func TestAutoPicksReplay(t *testing.T) {
	clk := newFakeClock()
	h, _ := newHost(t, func(o *Options) { o.Clock = clk })
	resp := createOK(t, h, twoHumansReq())
	tok0, _ := humanToken(t, resp)
	tok3 := seatToken(t, resp, 3)
	joinOK(t, h, resp.Event, tok0, "Ann")
	joinOK(t, h, resp.Event, tok3, "Eve")

	r := runOf(t, h, resp.Event)
	handPick := func(at time.Duration) {
		t.Helper()
		clk.Advance(at)
		p := packOf(t, h, resp.Event, tok0)
		if p.Waiting {
			t.Fatalf("seat 0 has no pack to hand-pick at %v", at)
		}
		if _, err := h.Pick(resp.Event, tok0, protocol.PickRequest{
			Round: p.Round, PickNo: p.PickNo, ID: p.Pack[0].ID,
		}); err != nil {
			t.Fatalf("seat 0 hand pick: %v", err)
		}
	}
	handPick(10 * time.Second)
	clk.Advance(61 * time.Second) // seat 3's first deadline fires
	if n := autoPicks(t, r); n != 1 {
		t.Fatalf("auto picks after the first deadline = %d, want 1", n)
	}
	handPick(10 * time.Second)
	clk.Advance(66 * time.Second) // seat 3's second deadline fires
	if n := autoPicks(t, r); n < 2 {
		t.Fatalf("auto picks after two deadlines = %d, want >= 2", n)
	}

	r.mu.Lock()
	wire, err := event.MarshalLog(r.ev.Log())
	if err != nil {
		r.mu.Unlock()
		t.Fatalf("MarshalLog: %v", err)
	}
	before, err := r.ev.StateJSON()
	r.mu.Unlock()
	if err != nil {
		t.Fatalf("StateJSON: %v", err)
	}
	re, err := event.ReplayLog(wire)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	after, err := re.StateJSON()
	if err != nil {
		t.Fatalf("replayed StateJSON: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("replayed event state diverged from the live one")
	}
}

// autoPicks counts the log's Auto Picked entries.
func autoPicks(t *testing.T, r *run) int {
	t.Helper()
	n := 0
	for _, en := range pickedEntries(t, r) {
		if en.Auto {
			n++
		}
	}
	return n
}

func TestPickTimersRearmOnResume(t *testing.T) {
	clk := newFakeClock()
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(Options{Registry: newRegistry(t), Store: store, Cards: testutil.CorpusRegistry(t),
		Seed: func() uint64 { return 7 }, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	resp := createOK(t, h, twoHumansReq())
	tok0, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok0, "Ann")
	joinOK(t, h, resp.Event, seatToken(t, resp, 3), "Eve")
	p0 := packOf(t, h, resp.Event, tok0)
	if p0.Waiting || p0.Round != 1 || p0.PickNo != 1 || p0.TimeLimitMS != 70000 {
		t.Fatalf("precondition: seat 0 pack before close: %+v", p0)
	}
	h.Close()

	// Reopen half a minute later: the deadline restarts at its full
	// limit for the same pick.
	clk.Advance(30 * time.Second)
	h2, err := New(Options{Registry: newRegistry(t), Store: store, Cards: testutil.CorpusRegistry(t),
		Seed: func() uint64 { return 7 }, Clock: clk})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	t.Cleanup(func() { h2.Close() })
	p0b := packOf(t, h2, resp.Event, tok0)
	if p0b.Round != p0.Round || p0b.PickNo != p0.PickNo || !reflect.DeepEqual(p0b.Pack, p0.Pack) {
		t.Fatalf("pack changed across resume: %+v -> %+v", p0, p0b)
	}
	if p0b.TimeLimitMS != 70000 || p0b.RemainingMS != 70000 {
		t.Fatalf("re-armed timer = %d/%d ms, want the full 70000/70000", p0b.TimeLimitMS, p0b.RemainingMS)
	}
	tm0, _ := assertArmed(t, h2, resp.Event, 0)
	if !tm0.started.Equal(clk.Now()) {
		t.Fatalf("re-armed timer started at %v, want the resume time %v", tm0.started, clk.Now())
	}
}

func TestRemainingMSCountsDown(t *testing.T) {
	clk := newFakeClock()
	h, _ := newHost(t, func(o *Options) { o.Clock = clk })
	resp := createOK(t, h, twoHumansReq())
	tok0, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok0, "Ann")
	joinOK(t, h, resp.Event, seatToken(t, resp, 3), "Eve")

	p0 := packOf(t, h, resp.Event, tok0)
	if p0.TimeLimitMS != 70000 || p0.RemainingMS != 70000 {
		t.Fatalf("precondition: pack timer = %d/%d, want 70000/70000", p0.TimeLimitMS, p0.RemainingMS)
	}
	clk.Advance(10 * time.Second)
	p0 = packOf(t, h, resp.Event, tok0)
	if p0.TimeLimitMS != 70000 {
		t.Fatalf("time_limit_ms = %d, want 70000", p0.TimeLimitMS)
	}
	if p0.RemainingMS != 60000 {
		t.Fatalf("remaining_ms = %d, want 60000", p0.RemainingMS)
	}
}
