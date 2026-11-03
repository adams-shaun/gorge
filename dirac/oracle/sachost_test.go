package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ksachostDecks: Equipment whose granted trigger's optional cost, or granted
// activated ability's cost, sacrifices the Equipment itself, beside creatures
// to carry it.
var ksachostDecks = [2][]cardset.Entry{
	{
		{Name: "Sunfire Torch", Count: 4}, {Name: "Spare Dagger", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Raging Goblin", Count: 6}, {Name: "Mountain", Count: 20},
	},
	{
		{Name: "Trickster's Talisman", Count: 4}, {Name: "Spare Dagger", Count: 2}, {Name: "Deconstruction Hammer", Count: 2},
		{Name: "Citizen's Crowbar", Count: 2}, {Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Plains", Count: 10},
	},
}

// An Equipment that grants "whenever this creature attacks (deals combat
// damage to a player), you may sacrifice [the Equipment]; when you do ..."
// has its optional cost paid with that Equipment, not the creature carrying
// the trigger, and with the one that granted this very trigger when several
// are attached (CR 701.21a, 603.5, 613.1f). Both engines offer the sacrifice
// only while the Equipment is the payer's own.
func TestGrantedSacrificeHostLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "sachost", ksachostDecks, 64, "KSACHOST_SEEDS")
	lockstepSet(t, "sachost", [2][]cardset.Entry{ksachostDecks[1], ksachostDecks[0]}, 64, "KSACHOST_SEEDS")
}

// TestGrantedSacrificeHostMutation: with the cost sacrificing the creature
// that carries the trigger, some seed diverges, so the pool reaches a paid
// host sacrifice.
func TestGrantedSacrificeHostMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "sachost", ksachostDecks, 64, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				for p := range prog {
					if prog[p].Op == kop.OpIfPaid && int(prog[p].C&0xF) == kop.PaySacSelf && prog[p].B.N == kop.PayBSubSacHost {
						prog[p].B = kop.Const(0)
					}
				}
				abs[j].Prog = prog
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the host sacrifice turned into a sacrifice of the carrier")
	}
}
