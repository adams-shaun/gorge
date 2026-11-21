package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 716.2a: Alchemist's Talent's "Treasures you control have '{T},
// Sacrifice: add two mana of any one color'" is a level-2 grant. At level 1 a
// Treasure carries only its printed add-one ability, so the mana scan must not
// admit the granted member (it once did, letting three mana pay a {3}{R} spell).
func TestClassLevelGatesGrantedManaAbility(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	c := mustCorpusCard(t, reg, "Alchemist's Talent")
	e := printedAddabilityEngine(t, c)
	talent := moveCardToBattlefield(t, e, c)
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "c_a_treasure_sac"})
	var tok state.ObjID
	for _, x := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(x); o != nil && o.IsToken {
			tok = x
		}
	}
	if tok == 0 {
		t.Fatal("precondition: the Treasure token was not minted")
	}
	if got := e.G.Obj(talent).ClassLevel(); got != 1 {
		t.Fatalf("precondition: the Class is at level %d, want 1", got)
	}
	if n := len(e.availableManaAbilities(0, tok)); n != 1 {
		t.Fatalf("level 1: Treasure has %d mana abilities, want only the printed one", n)
	}
	e.emit(events.Event{Kind: events.ClassLevelChange, Obj: talent, Amount: 1})
	if got := e.G.Obj(talent).ClassLevel(); got != 2 {
		t.Fatalf("precondition: the Class is at level %d after a level-up, want 2", got)
	}
	if n := len(e.availableManaAbilities(0, tok)); n != 2 {
		t.Fatalf("level 2: Treasure has %d mana abilities, want the printed one plus the granted add-two", n)
	}
}
