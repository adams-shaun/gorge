package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestTapeMayAskSeesASelfExileReturn: an "exile until this leaves the
// battlefield" ability whose targets include its own host (Vault 13:
// Dweller's Journey's chapter I may pick the Saga itself) exiles the host in
// the same resolution, so every card it just exiled comes straight back
// (CR 610.3) -- and an Aura among them asks what it enchants as it returns
// (CR 303.4f). Nothing is in exile and no holder carries an ExileReturn
// entry when the predicate runs, so only the resolving object's own targets
// show the ask; a miss panicked a Commander pod mid-resolution.
func TestTapeMayAskSeesASelfExileReturn(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	onBattlefield := func(src string) state.ObjID {
		o := e.G.AddObject(card(t, src), 0)
		o.Zone = state.ZBattlefield
		e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), o.ID))
		return o.ID
	}
	host := onBattlefield("Name:Holding Saga\nManaCost:3 W\nTypes:Enchantment\nOracle:x\n")
	aura := onBattlefield(attachedAuraSrc)
	bear := onBattlefield("Name:Plain Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	ab := e.G.AddObject(card(t, "Name:Holding Saga Ability\nTypes:Enchantment\nOracle:x\n"), 0)
	ab.Zone = state.ZStack
	ab.Source = host
	e.G.Stack = append(e.G.Stack, ab.ID)

	ab.Targets = []state.Target{{Obj: host}, {Obj: aura}}
	if !tapeSelfExileMayAsk(e) || !tapeBoardCompetes(e) {
		t.Fatal("a self-exiling until-leaves ability taking an Aura with it was judged ask-free")
	}
	ab.Targets = []state.Target{{Obj: host}, {Obj: bear}}
	if tapeSelfExileMayAsk(e) {
		t.Fatal("a self-exile returning only a vanilla creature was judged to ask")
	}
	ab.Targets = []state.Target{{Obj: aura}}
	if tapeSelfExileMayAsk(e) {
		t.Fatal("an exile that leaves its host in play was judged to return in the same resolution")
	}
}
