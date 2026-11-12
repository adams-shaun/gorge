package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEscapePaysTheSpellsOwnAdditionalCostFirst: escape is an alternative
// cost, so the spell's own additional cost is still paid on top of it
// (CR 702.138a, 118.9d), and the sacrifice is asked before the escape
// fodder (CR 601.2h, gorge costObjectAsks: sacrifice ... then exAsk).
func TestEscapePaysTheSpellsOwnAdditionalCostFirst(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Underworld Breach"))
	breach := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: breach, From: state.ZHand, To: state.ZBattlefield})

	rites := graveyardCard(t, e, "Name:Grave Rites\nManaCost:B\nTypes:Instant\nA:SP$ Draw | Cost$ B Sac<1/Creature> | NumCards$ 2 | SpellDescription$ x\nOracle:x\n")
	fod1 := graveyardCard(t, e, "Name:Fodder One\nManaCost:1 R\nTypes:Creature Lizard\nPT:1/1\nOracle:x\n")
	fod2 := graveyardCard(t, e, "Name:Fodder Two\nManaCost:2 U\nTypes:Creature Fish\nPT:1/1\nOracle:x\n")
	fod3 := graveyardCard(t, e, "Name:Fodder Three\nManaCost:G\nTypes:Creature Insect\nPT:1/1\nOracle:x\n")
	body := e.G.AddObject(card(t, "Name:Sac Body\nManaCost:0\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: body.ID, From: state.ZLibrary, To: state.ZBattlefield})

	addMana(t, e, 0, "B")
	d := e.Pending()
	esc := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == rites && o.Mode == "escape" {
			esc = o.Index
		}
	}
	if esc < 0 {
		t.Fatalf("escape cast not offered: %+v", d.Options)
	}
	submitChoices(t, e, esc)
	// The spell's own sacrifice is the first ask, before the fodder.
	de := e.Pending()
	if de == nil || de.Kind != decision.KChoose || len(de.Options) != 1 || de.Options[0].Obj != body.ID {
		t.Fatalf("first cost ask = %+v, want the own sacrifice of %d", de, body.ID)
	}
	for _, o := range de.Options {
		if o.Obj == fod1 || o.Obj == fod2 || o.Obj == fod3 {
			t.Fatalf("the sacrifice ask offered escape fodder: %+v", de.Options)
		}
	}
	submitChoices(t, e, de.Options[0].Index)
	// Then the escape fodder: the three other cards, the cast card excluded.
	de = e.Pending()
	if de == nil || de.Kind != decision.KChoose || len(de.Options) != 3 || de.Min != 3 || de.Max != 3 {
		t.Fatalf("escape fodder ask missing or wrong: %+v", de)
	}
	seen := map[state.ObjID]bool{}
	for _, o := range de.Options {
		seen[o.Obj] = true
	}
	if !seen[fod1] || !seen[fod2] || !seen[fod3] || seen[rites] {
		t.Fatalf("fodder options %+v, want the three other cards", de.Options)
	}
	submitChoices(t, e, de.Options[0].Index, de.Options[1].Index, de.Options[2].Index)
	passUntilStackEmpty(t, e, 50)
	if o := e.G.Obj(rites); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("escaped Rites zone=%v, want graveyard", o)
	}
	if o := e.G.Obj(body.ID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("sacrificed body zone=%v, want graveyard", o)
	}
}
