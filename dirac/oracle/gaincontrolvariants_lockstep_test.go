package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// GainControl variants' lockstep (dirac/kgen/lower_zone.go gainControl;
// kernel static_control.go opGainControl): NewController$ names a triggered
// player (Clambassadors gives its permanent to the player it damaged,
// Contested War Zone goes to the controller of a creature that hit its
// controller, Drooling Ogre to whoever cast an artifact: Mind Stone), and
// Slave of Bolas steals permanently with haste for as long as the steal
// lasts (AddKWs$ Haste with no LoseControl$: the kernel's GcHasteEOT record). Both engines decide every cast,
// attack and choice together.
func kgcvDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Clambassadors", Count: 4}, {Name: "Contested War Zone", Count: 4},
		{Name: "Drooling Ogre", Count: 4}, {Name: "Slave of Bolas", Count: 4},
		{Name: "Mind Stone", Count: 3}, {Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 4}, {Name: "Island", Count: 5},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

func TestGainControlVariantsLockstep(t *testing.T) {
	t.Parallel()
	if n := lockstepRun(t, "gaincontrolvariants", kgcvDecks(), 80, true, nil); n != 0 {
		t.Fatalf("%d seeds diverged", n)
	}
}

// kgcvMutate rewrites every OpGainControl of the table with f.
func kgcvMutate(f func(in *kop.Instr)) func(*kop.Table) {
	return func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				for k := range prog {
					if prog[k].Op == kop.OpGainControl {
						f(&prog[k])
					}
				}
				abs[j].Prog = prog
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
}

// TestGainControlVariantsLockstepMutationHaste: with the haste-for-the-
// steal rider dropped, some seed diverges (gorge's stolen creature attacks
// the turn it is taken; the kernel's is summoning sick).
func TestGainControlVariantsLockstepMutationHaste(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "gaincontrolvariants", kgcvDecks(), 80, false, kgcvMutate(func(in *kop.Instr) { in.B.N &^= kop.GcHasteEOT }))
	if n == 0 {
		t.Fatal("no seed diverged with the control record's haste dropped")
	}
}

// TestGainControlVariantsLockstepMutationController: with the new controller
// forced back to the resolving controller, some seed diverges (a triggered
// player no longer receives the permanent).
func TestGainControlVariantsLockstepMutationController(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "gaincontrolvariants", kgcvDecks(), 80, false, kgcvMutate(func(in *kop.Instr) {
		if in.C == int16(kop.RefTriggerPlayer) || in.C == int16(kop.RefTriggerTgtCtrl) {
			in.C = int16(kop.RefNone)
		}
	}))
	if n == 0 {
		t.Fatal("no seed diverged with the triggered new controller dropped")
	}
}
