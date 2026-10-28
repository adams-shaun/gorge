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
