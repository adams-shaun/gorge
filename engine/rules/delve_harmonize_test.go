package rules

// A granted delve (Teval, Arbiter of Virtue's "Spells you cast have delve")
// is announced as part of the total cost (CR 702.66a), so its shortfall is
// taken after every other announced contribution has reduced the generic
// part. A Harmonize creature whose power already covers the whole generic
// leaves nothing for delve to pay: the cast must not offer graveyard cards.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestGrantedDelveSkipsHarmonizeCoveredGeneric(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Teval, Arbiter of Virtue"))
	teval := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, teval)
	rebuff := e.G.AddObject(corpusAlternativeCard(t, "Ureni's Rebuff"), 0)
	rebuff.Zone = state.ZGraveyard
	// A graveyard card the granted delve could exile if it had a shortfall.
	husk := e.G.AddObject(card(t, "Name:Husk\nManaCost:G\nTypes:Creature Husk\nPT:1/1\nOracle:x\n"), 0)
	husk.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{rebuff.ID, husk.ID})
	// Ureni's Rebuff's harmonize cost is {5}{U}; Teval's six power reduces
	// the generic part to nothing, leaving exactly the {U}.
	e.G.Players[0].Pool[state.MU] = 1
	castMode(t, e, rebuff.ID, "harmonize")
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("harmonize announcement not asked: %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Kind == "harmonize" && o.Obj == teval {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("Teval was not offered as the harmonize payment: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	d = e.Pending()
	if d != nil && d.Kind == decision.KChoose {
		for _, o := range d.Options {
			if o.Kind == "exile" {
				t.Fatalf("delve offered %q after harmonize covered the generic: %+v",
					e.G.Obj(o.Obj).Face().Name, d.Options)
			}
		}
	}
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("next ask after the harmonize announcement = %+v, want the target ask", d)
	}
}
