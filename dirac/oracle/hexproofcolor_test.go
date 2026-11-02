package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The printed hexproof-from-a-colour lockstep (CR 702.11d):
// Knight of Grace (hexproof from black) and Knight of Malice (hexproof from
// white) against black and white removal (which cannot target them), red and
// blue sources (which can) and creatures of their own. The expected answer at
// every decision is gorge's.
var kkwhexcolorDecks = [2][]cardset.Entry{
	{
		{Name: "Knight of Grace", Count: 8}, {Name: "Knight of Malice", Count: 8},
		{Name: "Plains", Count: 12}, {Name: "Swamp", Count: 12},
	},
	{
		{Name: "Doom Blade", Count: 2}, {Name: "Terror", Count: 2}, {Name: "Disfigure", Count: 2}, {Name: "Go for the Throat", Count: 2},
		{Name: "Swords to Plowshares", Count: 2}, {Name: "Pacifism", Count: 2}, {Name: "Journey to Nowhere", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Lightning Bolt", Count: 2}, {Name: "Prodigal Sorcerer", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 3}, {Name: "Island", Count: 3},
	},
}

func TestHexproofFromColorLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "hexproofcolor", kkwhexcolorDecks, 600, "KKWHEXCOLOR_SEEDS")
}

// TestHexproofFromColorLockstepMutation: with every printed HexColors zeroed, a
// black / white source can target a Knight and some seed diverges.
func TestHexproofFromColorLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "hexproofcolor", kkwhexcolorDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			cards[i].HexColors = 0
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the printed hexproof dropped")
	}
}
