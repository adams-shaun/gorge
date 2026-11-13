package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Phthisis: "Destroy target creature. Its controller loses life equal to its
// power plus its toughness." (Targeted$CardSumPT, read from the destroyed
// creature's last known information.)
func TestPhthisisChargesThePowerPlusToughnessOfTheDestroyedCreature(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Phthisis"))
	victim := e.G.AddObject(card(t, "Name:Target\nTypes:Creature Human\nPT:3/2\nOracle:x\n"), 1)
	victim.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{victim.ID})
	spell := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MB], e.G.Players[0].Pool[state.MC] = 4, 3
	castMode(t, e, spell, "")
	if d := e.Pending(); d != nil && len(d.Options) > 0 && d.Options[0].Obj == victim.ID {
		submitChoices(t, e, 0)
	}
	before := e.G.Players[1].Life
	e.resolveTop()
	if e.G.Obj(victim.ID).Zone != state.ZGraveyard {
		t.Fatalf("precondition: the target was not destroyed: %s", e.G.Obj(victim.ID).Zone)
	}
	if got := before - e.G.Players[1].Life; got != 5 {
		t.Fatalf("its controller lost %d life, want 5 (power 3 + toughness 2)", got)
	}
}
