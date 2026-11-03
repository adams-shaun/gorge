package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/trigmatch"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestAttackTriggerReadsAttackerTypesPastColorTable pins Linden's "whenever a
// white creature you control attacks" on a board holding a live colour effect
// on another permanent. The attacker's derived type list is bound into the
// match from the layer scratch, and the spec's colour word builds the colour
// table on its first read at each event; that build derives the recoloured
// land, and it used to rewrite the list under the match, which then saw a land
// and no creature (CR 508.3a, CR 613.1).
func TestAttackTriggerReadsAttackerTypesPastColorTable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := combatTriggerBoard(t, reg,
		[]string{"Linden, the Steadfast Queen", "Herald of Faith"},
		[]string{"Name:TestIsle\nTypes:Basic Land Island\nOracle:x\n",
			"Name:TestPrison\nManaCost:2 U\nTypes:Enchantment Aura\nK:Enchant:Land\n" +
				"S:Mode$ Continuous | AffectedDefined$ Enchanted | SetColor$ Colorless | Description$ x\nOracle:x\n"}, nil, nil)
	linden := findBattlefield(t, e, 0, "Linden, the Steadfast Queen", 0)
	herald := findBattlefield(t, e, 0, "Herald of Faith", 0)
	aura := findBattlefield(t, e, 0, "TestPrison", 0)
	isle := findBattlefield(t, e, 0, "TestIsle", 0)
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{isle}})
	if !e.activeSummaryOf(e.active()).hasLColor {
		t.Fatal("the Aura's colour effect is not live; the board proves nothing")
	}
	// Any event moves the colour table's epoch, as the declaration does.
	ev := events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{herald}}
	e.emit(ev)
	lo := e.G.Obj(linden)
	tr := lo.Face().Triggers[0]
	e.emit(events.Event{Kind: events.Priority, Player: 0})
	if got := trigmatch.AttacksMatchedAttackers(boardOf(e), tr, linden, ev); len(got) != 1 || got[0] != herald {
		t.Fatalf("Linden matched %v on Herald's attack, want [%d]", got, herald)
	}
}
