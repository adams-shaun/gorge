package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The mid-resolution ChooseColor locksteps (gorge effChooseColor records the
// answer on the source; dirac ExtChooseColorRes keeps it in res.chosen and
// res.col). The expected answer at every decision is gorge's.

// kchosenProtDecks: Akroma's Blessing and Brave the Elements (protection
// from the chosen colour until end of turn, CR 702.16: all creatures you
// control, or only the white ones -- the green Grizzly Bears tell them
// apart) against red burn, red and green attackers and blockers, and a blue
// Prodigal Sorcerer's ping (targeting).
var kchosenProtDecks = [2][]cardset.Entry{
	{
		{Name: "Akroma's Blessing", Count: 5}, {Name: "Brave the Elements", Count: 5}, {Name: "Savannah Lions", Count: 8},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Plains", Count: 12}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Prodigal Sorcerer", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 4}, {Name: "Island", Count: 4},
	},
}

// kchosenSweepDecks: Wash Out (every permanent of the chosen colour to its
// owner's hand), Addle (you pick a card of the chosen colour from the
// target player's revealed hand) and Persecute (that player discards every
// card of the chosen colour), against a three-colour creature deck.
var kchosenSweepDecks = [2][]cardset.Entry{
	{
		{Name: "Wash Out", Count: 4}, {Name: "Addle", Count: 5}, {Name: "Persecute", Count: 5},
		{Name: "Coral Merfolk", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Island", Count: 9}, {Name: "Swamp", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 7}, {Name: "Plains", Count: 6},
	},
}

// kgreevilDecks: Root Greevil (sacrificed as the cost, so the filter's
// source has left: destroy all enchantments of the chosen colour) against
// white Auras and anthems.
var kgreevilDecks = [2][]cardset.Entry{
	{
		{Name: "Root Greevil", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Forest", Count: 20},
	},
	{
		{Name: "Pacifism", Count: 6}, {Name: "Glorious Anthem", Count: 4}, {Name: "Savannah Lions", Count: 10},
		{Name: "Suntail Hawk", Count: 6}, {Name: "Plains", Count: 14},
	},
}

// kchosenDragonDecks: Crosis, the Purger and Dromar, the Banisher (a
// combat-damage trigger's optional {2}{B} / {2}{U}: the damaged player
// discards every card of the chosen colour; every creature of the chosen
// colour returns to its owner's hand; the source is still on the
// battlefield).
var kchosenDragonDecks = [2][]cardset.Entry{
	{
		{Name: "Crosis, the Purger", Count: 6}, {Name: "Dromar, the Banisher", Count: 6},
		{Name: "Island", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Savannah Lions", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 6},
	},
}

func TestChosenProtLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "chosenprot", kchosenProtDecks, 600, "KCHOSENPROT_SEEDS")
}

func TestChosenSweepLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "chosensweep", kchosenSweepDecks, 600, "KCHOSENSWEEP_SEEDS")
}

func TestGreevilLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "greevil", kgreevilDecks, 600, "KGREEVIL_SEEDS")
}

func TestChosenDragonsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "chosendragons", kchosenDragonDecks, 600, "KCHOSENDRAGONS_SEEDS")
}

// dropExt replaces every OpExt instruction of sub-op x (and C bits c, when
// c != 0) with OpNop in the table.
func dropExt(x kop.ExtOp, c int16) func(*kop.Table) {
	return func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for n := range p {
					if p[n].Ext() == x && (c == 0 || p[n].C&c != 0) {
						p[n] = kop.Instr{Op: kop.OpNop}
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
}

// TestChosenProtLockstepMutation: with the protection grant dropped (the
// colour still asked), some seed diverges.
func TestChosenProtLockstepMutation(t *testing.T) {
	t.Parallel()
	if lockstepRun(t, "chosenprot", kchosenProtDecks, 300, false, dropExt(kop.ExtColorGrant, 4)) == 0 {
		t.Fatal("no seed diverged with the chosen-colour protection dropped")
	}
}

// chosenFilterMutation: with the ChosenColor flag dropped from every filter
// (each sweep then reads every colour), some seed diverges.
func chosenFilterMutation(tbl *kop.Table) {
	fs := append([]kop.Filter(nil), tbl.Filters...)
	for i := range fs {
		fs[i].Flags2 &^= kop.F2ChosenColor
	}
	tbl.Filters = fs
}

func TestChosenSweepLockstepMutation(t *testing.T) {
	t.Parallel()
	if lockstepRun(t, "chosensweep", kchosenSweepDecks, 300, false, chosenFilterMutation) == 0 {
		t.Fatal("no seed diverged with the ChosenColor filters reading every colour")
	}
}

func TestGreevilLockstepMutation(t *testing.T) {
	t.Parallel()
	if lockstepRun(t, "greevil", kgreevilDecks, 300, false, chosenFilterMutation) == 0 {
		t.Fatal("no seed diverged with Root Greevil's filter reading every colour")
	}
}

func TestChosenDragonsLockstepMutation(t *testing.T) {
	t.Parallel()
	if lockstepRun(t, "chosendragons", kchosenDragonDecks, 600, false, chosenFilterMutation) == 0 {
		t.Fatal("no seed diverged with Crosis's and Dromar's filters reading every colour")
	}
}
