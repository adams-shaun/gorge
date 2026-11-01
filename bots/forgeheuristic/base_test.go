package forgeheuristic

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/bot"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func unwrapHeuristic(t *testing.T, s seat.Seat) *heuristic.Bot {
	t.Helper()
	for {
		if hb, ok := s.(*heuristic.Bot); ok {
			return hb
		}
		u, ok := s.(interface{ UnwrapSeat() seat.Seat })
		if !ok {
			t.Fatalf("seat %T does not unwrap to *heuristic.Bot", s)
		}
		s = u.UnwrapSeat()
	}
}

// TestForgeHeuristicUsesCombatSimOnAutoPay pins that forge-heuristic is built
// on the production constructor: the combat sim is present with the production
// parameters on auto-pay tables and absent otherwise.
func TestForgeHeuristicUsesCombatSimOnAutoPay(t *testing.T) {
	s, err := New(bots.Options{Seed: 7, AutoPayMana: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	hb := unwrapHeuristic(t, s)
	if !hb.CombatSim() {
		t.Fatal("auto-pay: CombatSim() = false, want true")
	}
	p, ok := hb.AttackSimParams()
	if !ok || p != simParams() {
		t.Errorf("AttackSimParams() = %+v, %v; want simParams() (the production params plus the combat rules), true", p, ok)
	}

	s, err = New(bots.Options{Seed: 7})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	hb = unwrapHeuristic(t, s)
	if hb.CombatSim() {
		t.Error("no auto-pay: CombatSim() = true, want false")
	}
	if _, ok := hb.AttackSimParams(); ok {
		t.Error("no auto-pay: AttackSimParams ok = true, want false")
	}
}

// TestForgeHeuristicZeroRulesMatchesBot proves the shared base: the
// forge-heuristic shape with no Enable* rules plays the same game as the
// registered production bot.
func TestForgeHeuristicZeroRulesMatchesBot(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	cfg := rules.Config{Seed: 11, Names: names, Decks: decks, Mulligans: 1}
	o := bots.Options{Seed: 11, AutoPayMana: true}
	play := func(first seat.Seat) string {
		other, err := bots.New(bots.Default, bots.Options{Seed: 12, AutoPayMana: true})
		if err != nil {
			t.Fatalf("bots.New: %v", err)
		}
		_, e, err := bench.PlayGame(cfg, []seat.Seat{first, other}, 200, 40000, bench.Hooks{})
		if err != nil {
			t.Fatalf("PlayGame: %v", err)
		}
		if !e.G.Over {
			t.Fatalf("game did not finish: turn %d", e.G.Turn)
		}
		return string(e.L.Head())
	}
	prod, err := bots.New(bots.Default, o)
	if err != nil {
		t.Fatalf("bots.New: %v", err)
	}
	if got, want := play(bot.Base(o)), play(prod); got != want {
		t.Errorf("zero-rule base head %s, production bot head %s", got, want)
	}
}

// TestForgeHeuristicCommonSenseWraps pins the common-sense wrapper.
func TestForgeHeuristicCommonSenseWraps(t *testing.T) {
	plain, err := New(bots.Options{Seed: 3, AutoPayMana: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := plain.(*heuristic.Bot); !ok {
		t.Fatalf("CommonSense off: seat is %T, want *heuristic.Bot", plain)
	}
	s, err := New(bots.Options{Seed: 3, AutoPayMana: true, CommonSense: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := s.(*heuristic.Bot); ok {
		t.Fatal("CommonSense on: seat is a bare *heuristic.Bot, want the sanity wrapper")
	}
	u, ok := s.(interface{ UnwrapSeat() seat.Seat })
	if !ok {
		t.Fatalf("CommonSense on: seat %T has no UnwrapSeat", s)
	}
	if _, ok := u.UnwrapSeat().(*heuristic.Bot); !ok {
		t.Errorf("wrapped seat is %T, want *heuristic.Bot", u.UnwrapSeat())
	}
}
