package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestEndTheTurnRemovesFromCombat: Time Stop cast during combat ends the
// turn, which removes every creature from combat (CR 723.1b); before, the
// attacker kept its attacking flag into the cleanup step and the next turn
// (pairs 3336 5080 5624 9072).
func TestEndTheTurnRemovesFromCombat(t *testing.T) {
	deck := []cardset.Entry{{Name: "Time Stop", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
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
	s := sc.s
	for i := 0; i < 6; i++ {
		sc.perm(id["Island"], 0)
	}
	atk := sc.perm(id["Eager Trufflesnout"], 0)
	stop := sc.hand(id["Time Stop"], 0)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(atk), scAny)
	sc.do(t, AkDone, scAny, scAny)
	sc.do(t, AkCast, TCode(stop), scAny)
	for i := 0; i < 8 && (s.objs[stop].zone == kop.ZoneStack || s.objs[stop].zone == kop.ZoneHand); i++ {
		d := s.Step()
		s.Apply(d.Options[0])
	}
	if s.objs[stop].zone != kop.ZoneExile {
		t.Fatalf("Time Stop zone %d, want exile", s.objs[stop].zone)
	}
	s.Step()
	if s.perms[sc.slot(atk)].flags&pfAttacking != 0 {
		t.Fatalf("the attacker is still attacking after the turn ended (step %d turn %d)", s.step, s.turn)
	}
}

// TestEndTheTurnChecksBeforeCleanup: a state-based action the resolution left
// due is performed before the cleanup step, so it removes no damage first and
// opens no CR 514.3a priority (Ultima's destroyed hosts leave their Auras to the
// check).
func TestEndTheTurnChecksBeforeCleanup(t *testing.T) {
	deck := []cardset.Entry{{Name: "Time Stop", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "fdn")
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
	s := sc.s
	for i := 0; i < 6; i++ {
		sc.perm(id["Island"], 0)
	}
	doomed := sc.perm(id["Eager Trufflesnout"], 1)
	stop := sc.hand(id["Time Stop"], 0)
	turn := s.turn
	sc.do(t, AkCast, TCode(stop), scAny)
	for i := 0; i < 8 && s.objs[stop].zone != kop.ZoneExile; i++ {
		d := s.Step()
		if d.Kind == DkPriority && d.Player == 1 {
			s.perms[sc.slot(doomed)].damage = 99
		}
		s.Apply(d.Options[0])
	}
	if s.objs[stop].zone != kop.ZoneExile {
		t.Fatalf("Time Stop zone %d, want exile", s.objs[stop].zone)
	}
	s.Step()
	if s.turn != turn+1 {
		t.Errorf("turn %d step %d, want turn %d", s.turn, s.step, turn+1)
	}
	if z := s.objs[doomed].zone; z != kop.ZoneGraveyard {
		t.Errorf("the damaged creature is in zone %d, want the graveyard", z)
	}
}

// TestEndTheTurnDropsOnlyEarlierTriggers: end-the-turn drops the triggers
// waiting before it begins (CR 724.1a) and exiles the other stack objects
// (724.1b); the pending queue is cleared BEFORE the exile, so a trigger the
// exile causes survives into cleanup (724.1f; gorge). FDN and BLB
// have no stack-to-exile listener, so the survivor is enqueued by hand
// after the exile, where the old code's trailing clear dropped it.
func TestEndTheTurnDropsOnlyEarlierTriggers(t *testing.T) {
	deck := []cardset.Entry{{Name: "Time Stop", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
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
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	src := sc.perm(id["Eager Trufflesnout"], 0)
	other := sc.hand(id["Time Stop"], 1)
	stop := sc.hand(id["Time Stop"], 0)
	s.detach(other)
	s.objs[other].zone = kop.ZoneStack
	s.detach(stop)
	s.objs[stop].zone = kop.ZoneStack
	s.stack[0] = stackEntry{kind: seSpell, obj: other, card: id["Time Stop"], ctrl: 1, trigObj: NoObj}
	s.stack[1] = stackEntry{kind: seSpell, obj: stop, card: id["Time Stop"], ctrl: 0, trigObj: NoObj}
	s.sp = 2
	s.enqueue(src, s.objs[src].ver, id["Eager Trufflesnout"], 0, 0, NoObj, 0, 0)
	if s.npend != 1 {
		t.Fatalf("setup: npend %d, want 1", s.npend)
	}
	s.endTurnExile()
	if s.npend != 0 {
		t.Fatalf("npend %d after end-the-turn, want the earlier trigger dropped (CR 724.1a)", s.npend)
	}
	if s.sp != 1 || s.objs[other].zone != kop.ZoneExile {
		t.Fatalf("sp %d, other spell zone %d; want 1 and exile (CR 724.1b)", s.sp, s.objs[other].zone)
	}
	s.enqueue(src, s.objs[src].ver, id["Eager Trufflesnout"], 0, 0, other, 0, 0)
	if s.npend != 1 {
		t.Fatalf("npend %d, want the trigger the exile caused to survive (CR 724.1f)", s.npend)
	}
}
