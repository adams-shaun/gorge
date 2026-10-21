package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFightCorpusShapeCensus pins the Fight class by shape (cr-g18). fightPairs
// models exactly three shapes -- Defined$+ValidTgts$, ValidTgts$ only, and
// Defined$ only -- and a Fight with neither is a loud no-op. The targetless
// Defined$-only shape was a SILENT no-op before cr-g18 (it read its opponent
// from chosen targets it never asked for), so its members are pinned by name
// in both directions: a new targetless carrier fails until it is checked
// against fightPairs' first-two-Defined$ pairing and listed here.
func TestFightCorpusShapeCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	counts := map[string]int{}
	gotTargetless := map[string]string{}
	for _, card := range reg.AllCards() {
		for _, face := range card.Faces {
			eachFaceSA(face, func(key string, sa *cards.SA) {
				if sa.API != "Fight" {
					return
				}
				targeted := TargetsOf(sa).Has(TgtValidPresent)
				defined := DefinedRefOf(sa).Set()
				switch {
				case targeted && defined:
					counts["defined+targets"]++
				case targeted:
					counts["targets"]++
				case defined:
					counts["defined"]++
					gotTargetless[face.Name+"/"+key] = DefinedRefOf(sa).Text
				default:
					t.Errorf("%s/%s: Fight with neither Defined$ nor ValidTgts$ -- fightPairs has no pairing for it", face.Name, key)
				}
			})
		}
	}
	t.Logf("Fight shapes: %v", counts)
	pinMap(t, "targetless Defined$ Fight", gotTargetless, fightTargetlessCarriers)
}

// fightTargetlessCarriers is every Defined$-only Fight with its Defined$ text.
// Ezuri's Predation (RepeatEach UseImprinted$ also binds the subject as
// Remembered) and Grothama (OriginalHost of a static-granted trigger resolves
// to the recipient) do not yet resolve two distinct creatures; fightPairs
// notes them loudly and deals nothing.
var fightTargetlessCarriers = map[string]string{
	"Arena/DBFight":                         "Targeted",
	"Ezuri's Predation/DBFight":             "Imprinted & Remembered",
	"Grothama, All-Devouring/GrothamaFight": "TriggeredAttackerLKICopy & OriginalHost",
	"Guild Feud/PicksFight":                 "Remembered",
	"Hans Eriksson/DBFight":                 "Remembered & Self",
	"Magus of the Arena/DBFight":            "Targeted",
	"Skophos Maze-Warden/TrigFight":         "Self & TriggeredTarget",
	"Tunnel of Love/DBFight":                "Remembered",
}
