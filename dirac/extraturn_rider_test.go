package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The ExtraTurnDelayedTrigger rider (Final Fortune, Last Chance, Warrior's
// Oath): the granted turn's end step fires the loss. gorge registers the
// delayed trigger at the -1 ExtraTurn consumption with the granted turn as
// MinTurn (engine/events/apply_turn.go foldExtraTurn), so the granting
// turn's own end step never fires it and the loss is exactly once, in the
// granted turn.

// riderScenario builds the table the rider tests play on and puts the lands
// seat 0 needs on the battlefield.
func riderScenario(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "extraturn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the extraturn table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	for i := 0; i < 2; i++ {
		sc.perm(id["Mountain"], 0)
	}
	return sc, id
}

// resolveOffStack pays the cast begun on o and passes it through to
// resolution (the card stays in its hand until the payment completes).
func resolveOffStack(t *testing.T, s *State, o Obj) {
	t.Helper()
	for i := 0; i < 40; i++ {
		z := s.objs[o].zone
		if z != kop.ZoneHand && z != kop.ZoneStack {
			return
		}
		d := s.Step()
		if d.Kind == DkNone {
			return
		}
		s.Apply(d.Options[0])
	}
	t.Fatalf("the spell never left the hand or the stack (zone %d)", s.objs[o].zone)
}

// TestFinalFortuneRider: the cast grants the extra turn, the granted turn
// begins at the granting turn's cleanup, and its end step fires the loss.
func TestFinalFortuneRider(t *testing.T) {
	sc, id := riderScenario(t, []cardset.Entry{{Name: "Final Fortune", Count: 1}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 4}})
	s := sc.s
	ff := sc.hand(id["Final Fortune"], 0)
	granted := s.turn
	sc.do(t, AkCast, TCode(ff), scAny)
	resolveOffStack(t, s, ff)
	if s.g.exN != 1 || s.g.exQ[0] != 0 {
		t.Fatalf("pending %d (%d), want one grant to the caster", s.g.exN, s.g.exQ[0])
	}
	if s.g.exRide&1 == 0 {
		t.Fatal("the grant carries no rider")
	}
	if s.pl[0].lost || s.g.exRideN != 0 {
		t.Fatalf("caster lost %v, armed riders %d: the granting turn must not fire the rider", s.pl[0].lost, s.g.exRideN)
	}
	sc.passUntil(t, kop.StepEnd)
	s.exTake() // the granting turn's cleanup: the granted turn begins
	if s.active != 0 || s.turn != granted+1 {
		t.Fatalf("active %d turn %d, want the caster's granted turn %d", s.active, s.turn, granted+1)
	}
	if s.g.exN != 0 || s.g.exRideN != 1 || s.g.exRideS[0] != 0 {
		t.Fatalf("pending %d armed %d (%d): the rider is armed in the granted turn", s.g.exN, s.g.exRideN, s.g.exRideS[0])
	}
	for i := 0; i < 40 && !s.pl[0].lost; i++ {
		d := s.Step()
		if d.Kind == DkNone {
			break
		}
		s.Apply(d.Options[0])
	}
	if !s.pl[0].lost {
		t.Fatalf("the granted turn's end step never fired the loss: turn %d riders %d", s.turn, s.g.exRideN)
	}
	if s.g.exRideN != 0 {
		t.Fatalf("%d riders still armed: the record fired once", s.g.exRideN)
	}
}

// TestFinalFortuneRiderMutation: with DebugExRideAtGrant the rider arms at
// the grant, so the granting turn's own end step fires the loss and the
// granted turn never begins.
func TestFinalFortuneRiderMutation(t *testing.T) {
	sc, id := riderScenario(t, []cardset.Entry{{Name: "Final Fortune", Count: 1}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 4}})
	s := sc.s
	ff := sc.hand(id["Final Fortune"], 0)
	granted := s.turn
	DebugExRideAtGrant = true
	defer func() { DebugExRideAtGrant = false }()
	sc.do(t, AkCast, TCode(ff), scAny)
	resolveOffStack(t, s, ff)
	sc.passUntil(t, kop.StepEnd)
	for i := 0; i < 40 && !s.pl[0].lost; i++ {
		d := s.Step()
		if d.Kind == DkNone {
			break
		}
		s.Apply(d.Options[0])
	}
	if !s.pl[0].lost {
		t.Fatal("the mutation never fired the loss")
	}
	if s.turn != granted {
		t.Fatalf("turn %d, want %d: the mutation's loss fired before the granted turn", s.turn, granted)
	}
	if s.g.exN == 0 {
		t.Fatal("the mutation consumed the grant before the cleanup reached it")
	}
}

// TestFinalFortuneRiderTimeStop: end-the-turn in the granted turn skips its
// end step, so the rider stays armed into the following turn and its end
// step fires the loss (gorge's MinTurn gate: the first end step of a turn
// from the granted one on).
func TestFinalFortuneRiderTimeStop(t *testing.T) {
	sc, id := riderScenario(t, []cardset.Entry{{Name: "Final Fortune", Count: 1}, {Name: "Time Stop", Count: 1}, {Name: "Mountain", Count: 2}, {Name: "Island", Count: 4}})
	s := sc.s
	for i := 0; i < 4; i++ {
		sc.perm(id["Island"], 0)
	}
	ff := sc.hand(id["Final Fortune"], 0)
	stop := sc.hand(id["Time Stop"], 0)
	granted := s.turn
	sc.do(t, AkCast, TCode(ff), scAny)
	resolveOffStack(t, s, ff)
	sc.passUntil(t, kop.StepEnd)
	s.exTake()
	if s.active != 0 || s.g.exRideN != 1 {
		t.Fatalf("active %d armed %d: the granted turn began with its rider armed", s.active, s.g.exRideN)
	}
	sc.do(t, AkCast, TCode(stop), scAny)
	resolveOffStack(t, s, stop)
	for i := 0; i < 20 && s.turn == granted+1 && !s.pl[0].lost; i++ {
		d := s.Step()
		if d.Kind == DkNone {
			break
		}
		s.Apply(d.Options[0])
	}
	if s.active == 0 || s.turn != granted+2 {
		t.Fatalf("active %d turn %d: the granted turn ended at the Time Stop, no end step came", s.active, s.turn)
	}
	if s.pl[0].lost {
		t.Fatal("the rider fired in a turn that never had an end step")
	}
	if s.g.exRideN != 1 {
		t.Fatalf("%d riders armed: the skipped end step left the record", s.g.exRideN)
	}
	for i := 0; i < 40 && !s.pl[0].lost; i++ {
		d := s.Step()
		if d.Kind == DkNone {
			break
		}
		s.Apply(d.Options[0])
	}
	if !s.pl[0].lost || s.g.exRideN != 0 {
		t.Fatalf("lost %v armed %d: the following turn's end step fired the loss", s.pl[0].lost, s.g.exRideN)
	}
}
