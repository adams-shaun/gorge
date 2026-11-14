package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The granted protection from the host's chosen color (Ward Sliver's
// "All Slivers have protection from the chosen color", Pentarch Ward's and
// Benevolent Blessing's "Enchanted creature has protection from the chosen
// color" with their "doesn't remove" exceptions: AddKeyword$
// Protection:Card.ChosenColor:chosenColor, the kernel's Static.ProtChosen and
// ProtExc), beside the opposing seat's creatures, burn and removal that the
// protection blocks, prevents or targets around. The expected answer at every
// decision is gorge's.
var kmbcchosenprotDecks = [2][]cardset.Entry{
	{
		{Name: "Ward Sliver", Count: 4}, {Name: "Pentarch Ward", Count: 4},
		{Name: "Benevolent Blessing", Count: 4}, {Name: "Winged Sliver", Count: 3},
		{Name: "Savannah Lions", Count: 4}, {Name: "Serra Angel", Count: 2},
		{Name: "Plains", Count: 19},
	},
	{
		{Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 3},
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Terror", Count: 3},
		{Name: "Mountain", Count: 13}, {Name: "Swamp", Count: 10},
	},
}

func TestMBCProtectionChosenColorLowers(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "mbcchosenprot", kmbcchosenprotDecks, 400, "KMBCCHOSENPROT_SEEDS")
}

// TestMBCProtectionChosenColorLowersMutation: with the chosen-colour quality
// dropped from every grant, some seed diverges, so the pool plays the
// protection and the kernel prices it (the lowering reverted fails the
// lockstep).
func TestMBCProtectionChosenColorLowersMutation(t *testing.T) {
	t.Parallel()
	hit := 0
	n := lockstepRun(t, "mbcchosenprot", kmbcchosenprotDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbStatic && abs[j].Static.ProtChosen {
					abs[j].Static.ProtChosen = false
					hit++
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if hit == 0 {
		t.Fatal("no chosen-colour grant in the table")
	}
	if n == 0 {
		t.Errorf("no seed diverged: the pool never plays a granted chosen-colour protection")
	}
}

// TestMBCProtectionChosenColorExceptionMutation: with the exception dropped,
// some seed diverges, so the pool reaches the Aura state check the exception
// spares (a white Pentarch Ward or Benevolent Blessing on a creature of the
// chosen color).
func TestMBCProtectionChosenColorExceptionMutation(t *testing.T) {
	t.Parallel()
	hit := 0
	n := lockstepRun(t, "mbcchosenprot", kmbcchosenprotDecks, 400, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbStatic && abs[j].Static.ProtExc != 0 {
					abs[j].Static.ProtExc = 0
					hit++
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if hit == 0 {
		t.Fatal("no exception in the table")
	}
	if n == 0 {
		t.Errorf("no seed diverged: the pool never reaches an excepted attachment")
	}
}
