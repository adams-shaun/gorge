package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestKannushiDropsAnOpposingRedAuraWhenARedPermanentArrives pins CR 704.5m with
// protection from the colors of permanents you control (Empty-Shrine
// Kannushi): an opponent's red Aura on it is legal while its controller
// controls no red permanent and leaves the battlefield once one arrives.
func TestKannushiDropsAnOpposingRedAuraWhenARedPermanentArrives(t *testing.T) {
	t.Parallel()
	const (
		kannushiSrc = "Name:Test Kannushi\nManaCost:W\nTypes:Creature Human Cleric\nPT:1/1\n" +
			"S:Mode$ Continuous | AffectedDefined$ Self | AddKeyword$ Protection from colorsYouCtrl | Description$ x\nOracle:x\n"
		auraSrc = "Name:Test Rage\nManaCost:1 R\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
			"S:Mode$ Continuous | AffectedDefined$ Enchanted | Affected$ Creature | AddPower$ 2 | AddToughness$ 1 | AddKeyword$ Menace | Description$ x\nOracle:x\n"
		redSrc = "Name:Own Red Ox\nManaCost:3 R R\nTypes:Creature Ox\nPT:4/2\nOracle:x\n"
	)
	e, _, _ := newFixtureDeck(t, 74, kannushiSrc, redSrc)
	k := putCreature(t, e, 0, kannushiSrc)
	aura := putToken(t, e, 1, auraSrc, state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{k}})
	e.checkStateBased()
	if got := e.Colors(aura); got != "R" {
		t.Fatalf("precondition: aura colours = %q, want R", got)
	}
	if e.G.Obj(aura).Zone != state.ZBattlefield || e.G.Obj(aura).AttachedTo != k {
		t.Fatalf("precondition: aura zone %s attached %d, want attached to %d on the battlefield", e.G.Obj(aura).Zone, e.G.Obj(aura).AttachedTo, k)
	}
	ox := putCreature(t, e, 0, redSrc)
	if got := e.Colors(ox); got != "R" {
		t.Fatalf("precondition: ox colours = %q, want R", got)
	}
	e.checkStateBased()
	// A token Aura ceases to exist after the graveyard move (CR 111.7).
	if z := e.G.Obj(aura).Zone; z == state.ZBattlefield {
		t.Fatalf("red Aura still on the battlefield after its bearer's controller got a red permanent")
	}
}
