package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestMBCZoneMoves6Lowers plays the MBC zone-moves-6 spelling family's cards
// in a lockstep fixture built on their dr-mbc pool halves, one game per
// spelling:
//
//   - kw:LimitSearchLibrary: Aven Mindcensor. gorge playerkeywords.go reads
//     only the Shroud and Hexproof player grants, so the static's grant is
//     inert and kgen lowers the static to nothing; the card still plays.
//   - other:library position N of Self from Graveyard: Bookwurm. Its
//     {2}{G} ability (ActivationZone$ Graveyard) puts CARDNAME from its own
//     graveyard into its library third from the top (OpToLibrary
//     kop.LibThird); the Breaker of Armies deck trades it into the
//     graveyard so the activation is offered.
//   - param:ChangeZone.AttachedTo: Boonweaver Giant. Its enters trigger
//     searches the library, graveyard and hand for an Aura and puts it onto
//     the battlefield attached to the Giant (AttachedTo$ Self, gorge
//     markChangeZoneAttach and the entry gate).
//
// The cleave spelling param:ChangeZone.NonBasicSpell has no playable card of
// its own: Alchemist's Retrieval's cleave body lowers its NonBasicSpell$
// marker but the card stays refused by the spell-cost family (its cleave
// Cost$ differs from the printed mana cost, a separate family), so the
// fixture cannot deal it.
//
// Each deck's cards must lower: cardset's matchup refuses a deck with a
// refused card, so reverting the lowering fails these games at the deal.
func TestMBCZoneMoves6Lowers(t *testing.T) {
	aven := []cardset.Entry{
		{Name: "Aven Mindcensor", Count: 4}, {Name: "Plains", Count: 18}, {Name: "Island", Count: 18},
	}
	avenB := []cardset.Entry{
		{Name: "Aven Mindcensor", Count: 2}, {Name: "Plains", Count: 19}, {Name: "Forest", Count: 19},
	}
	bookwurm := []cardset.Entry{
		{Name: "Bookwurm", Count: 4}, {Name: "Forest", Count: 36},
	}
	bookwurmB := []cardset.Entry{
		{Name: "Breaker of Armies", Count: 3}, {Name: "Swamp", Count: 14}, {Name: "Plains", Count: 23},
	}
	boonweaver := []cardset.Entry{
		{Name: "Boonweaver Giant", Count: 4}, {Name: "Armored Ascension", Count: 4}, {Name: "Plains", Count: 32},
	}
	boonweaverB := []cardset.Entry{
		{Name: "Canopy Cover", Count: 2}, {Name: "Plains", Count: 38},
	}
	t.Run("LimitSearchLibrary", func(t *testing.T) {
		fixtureLockstep(t, "dr-mbc-ba", [2][]cardset.Entry{aven, avenB}, 16)
	})
	t.Run("LibraryPosition2Graveyard", func(t *testing.T) {
		fixtureLockstep(t, "dr-mbc-ba", [2][]cardset.Entry{bookwurm, bookwurmB}, 16)
	})
	t.Run("SearchAttachedTo", func(t *testing.T) {
		fixtureLockstep(t, "dr-mbc-ab", [2][]cardset.Entry{boonweaver, boonweaverB}, 16)
	})
}
