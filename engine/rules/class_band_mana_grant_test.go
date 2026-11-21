package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 716.2: Alchemist's Talent's "Treasures you control have '{T}, Sacrifice
// this artifact: Add two mana of any one color'" is its level-2 ability. The
// granted-mana scan once read no ClassBand$, so a level-1 Talent gave its
// Treasure the second ability and the planner priced {1}{B} on one Treasure.
func TestClassBandGatesGrantedTreasureMana(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Alchemist's Talent"))
	talent := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, talent)
	var treasure state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken {
			treasure = id
		}
	}
	if treasure == 0 {
		t.Fatal("precondition: the Talent's enter trigger makes a Treasure")
	}
	e.emit(events.Event{Kind: events.Untap, Obj: treasure}) // it enters tapped
	if got := e.G.Obj(talent).ClassLevel(); got != 1 {
		t.Fatalf("precondition: entry level = %d, want 1", got)
	}
	if got := len(e.availableManaAbilities(0, treasure)); got != 1 {
		t.Fatalf("level 1: Treasure has %d mana abilities, want its own 1", got)
	}
	e.emit(events.Event{Kind: events.ClassLevelChange, Obj: talent, Amount: 1})
	if got := e.G.Obj(talent).ClassLevel(); got != 2 {
		t.Fatalf("precondition: level after the level-up = %d, want 2", got)
	}
	if got := len(e.availableManaAbilities(0, treasure)); got != 2 {
		t.Fatalf("level 2: Treasure has %d mana abilities, want 2", got)
	}
}
