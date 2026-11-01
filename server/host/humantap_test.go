package host

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// fakeTap answers every decision its seat parks with the first legal intent,
// from a goroutine, as the v2 codec does.
type fakeTap struct {
	attached atomic.Int32
	parked   atomic.Int32
	engine   atomic.Bool
	native   atomic.Int32
}

func (f *fakeTap) AttachMatch(mc *bots.MatchContext) error {
	f.attached.Add(1)
	f.engine.Store(mc.Engine != nil && mc.Projection != nil)
	return nil
}

func (f *fakeTap) Parked(_ context.Context, _ view.View, d decision.Decision, answer func(decision.Intent) error, _ <-chan struct{}) {
	f.parked.Add(1)
	go func() { _ = answer(legalIntent(&d)) }()
}

func (f *fakeTap) NativeAnswer() { f.native.Add(1) }

func tapTable(t *testing.T, autoMana bool, tap HumanTap) (*Registry, TableID) {
	t.Helper()
	names := testutil.RepoDeckNames()
	o := testOptions(t)
	o.LoadDeck = repoDeckLoader(t)
	o.Seats = func(ns []string, seed uint64) []seat.Seat {
		out := defaultSeats(BotPolicy, ns, seed)
		hs := NewHumanSeat()
		hs.SetTap(tap)
		out[0] = hs
		return out
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := TableConfig{ID: "tap", Name: "tap", Seats: 2, Decks: []string{names[0], names[1]}, Seed: 7,
		Spectator: view.Omniscient, AutoMana: autoMana, BotAutoPayMana: autoMana}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(cfg.ID); err != nil {
		t.Fatal(err)
	}
	return r, cfg.ID
}

func waitMatch(t *testing.T, r *Registry, id TableID) protocol.MatchInfo {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if ms, err := r.Matches(id); err == nil && len(ms) == 1 && (ms[0].State == protocol.MatchFinished || ms[0].State == protocol.MatchCrashed) {
			return ms[0]
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("match did not end")
	return protocol.MatchInfo{}
}

// TestHumanTapSeesTheLiveEngineAndAnswersThroughTheSeat: a tapped HumanSeat
// attaches its tap with the live engine and projection lock, announces each
// parked decision, and a match answered only through the tap finishes.
func TestHumanTapSeesTheLiveEngineAndAnswersThroughTheSeat(t *testing.T) {
	tap := &fakeTap{}
	r, id := tapTable(t, true, tap)
	if m := waitMatch(t, r, id); m.State != protocol.MatchFinished {
		t.Fatalf("match %+v", m)
	}
	if tap.attached.Load() != 1 || !tap.engine.Load() {
		t.Fatalf("attach: %d calls, engine+projection %v", tap.attached.Load(), tap.engine.Load())
	}
	if tap.parked.Load() < 10 {
		t.Fatalf("tap saw %d parked decisions", tap.parked.Load())
	}
	if tap.native.Load() != 0 {
		t.Fatalf("the tap's own answers reported as %d native answers", tap.native.Load())
	}
}

// TestHumanTapHearsANativeAnswerBeforeTheNextPark: an answer through the
// registry tells the tap once it is accepted for delivery, and a refused one
// doesn't.
func TestHumanTapHearsANativeAnswerBeforeTheNextPark(t *testing.T) {
	tap := &nativeTap{}
	r, id := tapTable(t, true, tap)
	d := waitPending(t, r, id, 1, 0)
	bad := legalIntent(d)
	bad.Seq++
	if err := r.SubmitIntent(id, 1, 0, bad); err == nil {
		t.Fatal("a stale answer was accepted")
	}
	if n := tap.native.Load(); n != 0 {
		t.Fatalf("a refused answer told the tap %d times", n)
	}
	parked := tap.parked.Load()
	tap.onNative = func() {
		if p := tap.parked.Load(); p != parked {
			t.Errorf("parked moved from %d to %d before NativeAnswer", parked, p)
		}
	}
	if err := r.SubmitIntent(id, 1, 0, legalIntent(d)); err != nil {
		t.Fatal(err)
	}
	if n := tap.native.Load(); n != 1 {
		t.Fatalf("an accepted answer told the tap %d times", n)
	}
}

// An accepted undo tells the tap, so a client coming back over the native wire
// takes the seat with it; a refused one doesn't.
func TestHumanTapHearsANativeUndo(t *testing.T) {
	tap := &nativeTap{}
	names := testutil.RepoDeckNames()
	o := testOptions(t)
	o.LoadDeck = repoDeckLoader(t)
	o.TapHuman = func(TableID, int, bool, *bots.MatchContext) (HumanTap, error) { return tap, nil }
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	const id = TableID("undo")
	if err := r.AddTable(TableConfig{ID: id, Name: "undo", Seats: 2, Decks: []string{names[0], names[1]}, Seed: 7,
		Spectator: view.Omniscient, AutoMana: true, BotAutoPayMana: true, Humans: []int{0}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(id); err != nil {
		t.Fatal(err)
	}
	d := waitPending(t, r, id, 1, 0)
	if err := r.Undo(id, 1, 0); err == nil {
		t.Fatal("an undo with nothing to undo was accepted")
	}
	if n := tap.native.Load(); n != 0 {
		t.Fatalf("a refused undo told the tap %d times", n)
	}
	if err := r.SubmitIntent(id, 1, 0, legalIntent(d)); err != nil {
		t.Fatal(err)
	}
	if err := r.Undo(id, 1, 0); err != nil {
		t.Fatal(err)
	}
	if n := tap.native.Load(); n != 2 {
		t.Fatalf("an answer and an undo told the tap %d times, want 2", n)
	}
}

// nativeTap watches decisions park and answers none of them.
type nativeTap struct {
	fakeTap
	onNative func()
}

func (n *nativeTap) Parked(_ context.Context, _ view.View, _ decision.Decision, _ func(decision.Intent) error, _ <-chan struct{}) {
	n.parked.Add(1)
}

func (n *nativeTap) NativeAnswer() {
	if n.onNative != nil {
		n.onNative()
	}
	n.native.Add(1)
}

// TestHumanTapRefusesAManualManaTable: the tap answers from payment plans, so
// a table that does not build them crashes at attach instead of posing no cast.
func TestHumanTapRefusesAManualManaTable(t *testing.T) {
	tap := &fakeTap{}
	r, id := tapTable(t, false, tap)
	if m := waitMatch(t, r, id); m.State != protocol.MatchCrashed {
		t.Fatalf("want a crash at attach, got %+v", m)
	}
	if tap.attached.Load() != 0 || tap.parked.Load() != 0 {
		t.Fatalf("tap used on a manual-mana table: attached %d parked %d", tap.attached.Load(), tap.parked.Load())
	}
}
