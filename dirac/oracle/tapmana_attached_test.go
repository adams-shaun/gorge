package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// An Aura-on-land TapsForMana extra (Mode$ TapsForMana | Static$ True over
// Card.AttachedBy) adds the tapped land's controller mana beside the land's
// own production when the body is one literal colour: Wild Growth {G},
// Overgrowth {G}{G}, Wolfwillow Haven {G}, and the non-Aura literal bodies
// over every Forest (Vernal Bloom) and every Mountain (Gauntlet of Might).
// Gorge folds a literal body into the planned tap
// (pay.PaymentPlanFoldableTapMana); the kernel's StatTapExtra must price the
// same mana on every source the carrier reaches, and the cast a land only
// pays with the extra must be paid identically on both sides.
var ktapmanaAttachedDecks = [2][]cardset.Entry{
	{
		{Name: "Wild Growth", Count: 4}, {Name: "Overgrowth", Count: 4},
		{Name: "Wolfwillow Haven", Count: 4},
		{Name: "Vernal Bloom", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Llanowar Elves", Count: 2}, {Name: "Forest", Count: 20},
	},
	{
		{Name: "Shock", Count: 10}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Gauntlet of Might", Count: 2},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

func TestTapManaAttachedLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "tapmana_attached", ktapmanaAttachedDecks, 300, "TAPMANA_ATTACHED_SEEDS")
}

// TestTapManaAttachedLockstepMutation: with every StatTapExtra static dropped
// from the kernel's table, a seed diverges, so the pool exercises the extra
// rather than passing on the carriers' other text.
func TestTapManaAttachedLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "tapmana_attached", ktapmanaAttachedDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			var keep []kop.Ability
			for _, a := range cards[i].Abilities {
				if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatTapExtra {
					continue
				}
				keep = append(keep, a)
			}
			cards[i].Abilities = keep
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the tap-extra static dropped")
	}
}
