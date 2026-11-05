package rules

// A library search that puts its found card onto the battlefield
// WithCountersType$ CHARGE / WithCountersAmount$ 7 (Captain Jean-Luc
// Picard's ability) must have the counters on the permanent as it enters, so
// the real Sledge-Class Seedship's Station 7+ static sees them and makes it a
// creature (and grants flying). The effects-level entry-payload pin lives in
// engine/effects/search_counters_test.go; this leaf reads the derived
// characteristics the static produces.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestSearchToBattlefieldEntersWithStationCounters(t *testing.T) {
	reg := searchTestRegistry(t)
	ship := searchCorpusCard(t, reg, "Sledge-Class Seedship")
	// Freely authored fixture, never a corpus script: the same rider Picard's
	// activated ability carries, reduced to the search alone.
	search := card(t, "Name:Charge Search\nManaCost:U\nTypes:Sorcery\n"+
		"A:SP$ ChangeZone | Origin$ Library | Destination$ Battlefield | ChangeType$ Spacecraft | ChangeNum$ 1 | "+
		"WithCountersType$ CHARGE | WithCountersAmount$ 7 | Mandatory$ True | SpellDescription$ x\nOracle:x\n")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{search, ship}, nil)
	if o := e.G.Obj(moveByName(t, e, 0, "Charge Search", state.ZHand)); o == nil {
		t.Fatal("fixture spell not moved to hand")
	}
	shipID := findByName(e, "Sledge-Class Seedship", 0)
	if shipID == 0 {
		t.Fatal("corpus fixture: Sledge-Class Seedship not seeded")
	}
	if got := e.G.Obj(shipID).Zone; got != state.ZLibrary {
		moveByName(t, e, 0, "Sledge-Class Seedship", state.ZLibrary)
	}
	// Precondition: in the library, no counters, not yet a creature.
	if o := e.G.Obj(shipID); o.Zone != state.ZLibrary || o.Counter("CHARGE") != 0 || hasTypeWord(o.Face().Types, "Creature") {
		t.Fatalf("precondition: ship zone=%s counters=%d printed types=%v", o.Zone, o.Counter("CHARGE"), o.Face().Types)
	}

	e.priorityRound()
	addMana(t, e, 0, "U")
	castNamed(t, e, "Charge Search")
	picked := false
	for i := 0; i < 12; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while resolving the search")
		}
		if d.Kind == decision.KPriority {
			pass := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pass = o.Index
					break
				}
			}
			if pass < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d.Options)
			}
			submitChoices(t, e, pass)
			continue
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "search" {
			if len(d.Options) == 0 {
				t.Fatalf("search offered no card: %+v", d)
			}
			submitChoices(t, e, d.Options[0].Index)
			picked = true
			break
		}
		t.Fatalf("unexpected decision while resolving the search: %+v", d)
	}
	if !picked {
		t.Fatal("the search never posed its pick")
	}
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)

	o := e.G.Obj(shipID)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("searched Spacecraft zone = %v, want battlefield", o)
	}
	if got := o.Counter("CHARGE"); got != 7 {
		t.Fatalf("searched Spacecraft CHARGE counters = %d, want 7", got)
	}
	dv := e.Derived(shipID)
	if !hasTypeWord(dv.Types, "Creature") {
		t.Fatalf("searched Spacecraft types = %v, want Creature at seven CHARGE counters (Station 7+)", dv.Types)
	}
	if !slices.Contains(dv.Keywords, "Flying") {
		t.Fatalf("searched Spacecraft keywords = %v, want Flying from Station 7+", dv.Keywords)
	}
}
