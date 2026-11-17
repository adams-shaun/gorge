package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCDAColorTypeLockstep: a colour or type characteristic-defining static
// on the card itself (dirac/kgen/lower_cdacolortype.go) is part of its printed
// record, read in every zone. Transguild Courier ("is all colors") is black to
// Doom Blade and Terror and an artifact to Naturalize; Ghostfire is colourless
// in hand and on the stack; Stonework Packbeast is a Wizard to Wizard's
// Lightning's cost reduction (an artifact creature to Naturalize too).
func TestCDAColorTypeLockstep(t *testing.T) {
	t.Parallel()
	decks := cdaColorTypeDecks()
	lockstepRunPolicy(t, "cdacolortype", decks, 80, true, nil, nil, castFirstPolicy)
}

func cdaColorTypeDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Transguild Courier", Count: 4},
		{Name: "Ghostfire", Count: 4},
		{Name: "Stonework Packbeast", Count: 4},
		{Name: "Doom Blade", Count: 4},
		{Name: "Terror", Count: 4},
		{Name: "Naturalize", Count: 3},
		{Name: "Wizard's Lightning", Count: 4},
		{Name: "Mountain", Count: 7},
		{Name: "Swamp", Count: 6},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// TestCDAColorTypeLockstepMutation proves the decks exercise the folded
// colours: with every card's printed colours cleared, some seed diverges
// (Doom Blade and Terror then target the all-colour Courier).
func TestCDAColorTypeLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "cdacolortype", cdaColorTypeDecks(), 80, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			cards[i].Colors = 0
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the printed colours cleared")
	}
}
