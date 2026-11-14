package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestBlockPairsPastMaxOptionsStall: the blockers declaration offers one
// option per (blocker, attacker) pair, as gorge does; 40 untapped blockers
// against 14 attackers are 560 pairs, past MaxOptions (512). The kernel
// stalls (StallOptions) instead of panicking "decision option overflow"
// in blockerOptions (random-policy FDN games 359575 and 370308 of a random-deck
// hunt: 32 x 14 and 37 x 12).
func TestBlockPairsPastMaxOptionsStall(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	var atk []Obj
	for i := 0; i < 14; i++ {
		atk = append(atk, sc.perm(v.HillGiant, 0))
	}
	for i := 0; i < 40; i++ {
		sc.perm(v.HillGiant, 1)
	}
	sc.passUntil(t, kop.StepAttackers)
	for _, a := range atk {
		sc.do(t, AkAttack, TCode(a), scAny)
	}
	sc.do(t, AkDone, scAny, scAny)
	for i := 0; i < 20 && !sc.s.over; i++ {
		d := sc.s.Step()
		if d.Kind == DkNone {
			break
		}
		if d.Kind == DkBlockers {
			t.Fatalf("blockers decision posed with %d options", d.N)
		}
		sc.s.Apply(d.Options[0])
	}
	if got := sc.s.StallReason(); got != StallOptions || !sc.s.over {
		t.Fatalf("stall reason %q over %v, want %q and over", StallName(got), sc.s.over, StallName(StallOptions))
	}
}

// TestAttackPairsPastMaxOptionsStall: the attackers declaration offers each
// creature the player and each planeswalker; 90 attackers against FDN's
// five walkers are 540 pairs. The kernel stalls (StallOptions) instead of
// panicking in attackerOptions.
func TestAttackPairsPastMaxOptionsStall(t *testing.T) {
	walkers := []string{"Ajani, Caller of the Pride", "Chandra, Flameshaper", "Kaito, Cunning Infiltrator", "Liliana, Dreadhorde General", "Vivien Reid"}
	deck := []cardset.Entry{{Name: "Llanowar Elves", Count: 1}, {Name: "Swamp", Count: 8}}
	for _, w := range walkers {
		deck = append(deck, cardset.Entry{Name: w, Count: 1})
	}
	sc, id := newObjScen(t, deck)
	for i := 0; i < 90; i++ {
		sc.perm(id["Llanowar Elves"], 0)
	}
	for _, w := range walkers {
		o := sc.perm(id[w], 1)
		sc.s.perms[sc.slot(o)].setCtrN(sc.s, int(kop.CtrLoyalty), 6)
	}
	for i := 0; i < 20 && !sc.s.over; i++ {
		d := sc.s.Step()
		if d.Kind == DkNone {
			break
		}
		if d.Kind == DkAttackers {
			t.Fatalf("attackers decision posed with %d options", d.N)
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if got := sc.s.StallReason(); got != StallOptions || !sc.s.over {
		t.Fatalf("stall reason %q over %v, want %q and over", StallName(got), sc.s.over, StallName(StallOptions))
	}
}
