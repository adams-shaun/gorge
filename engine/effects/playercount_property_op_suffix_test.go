package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestPlayerCountPropertyOpSuffix pins Bonecache Overseer's SVar spelling:
// `PlayerCountPropertyYou$SacrificedThisTurn Food/Times.3`. Forge's
// AbilityUtils.playerXCount splits the property string on "/" and applies the
// extracted operator to the summed count (CardFactoryUtil.extractOperators ->
// doXMath), so the property reads "Food" and the count is multiplied by 3.
// Reading the whole argument as the type spec ("Food/Times.3") matches no
// Food and yields 0.
func TestPlayerCountPropertyOpSuffix(t *testing.T) {
	h, c := fixtureHost(t)
	c.Controller = 0
	g := h.g

	// Two Foods the resolving controller sacrificed this turn. They live in
	// the graveyard and are stamped into g.Entered the way events.Apply's
	// sacrifice fold records them, which is where sacrificedThisTurn reads.
	var foods []state.ObjID
	for range 2 {
		o := g.AddObject(mkCard(t, "Name:Food\nTypes:Artifact Food\nOracle:x\n"), 0)
		o.Zone = state.ZGraveyard
		g.SetZone(state.ZGraveyard, 0, append(g.Zone(state.ZGraveyard, 0), o.ID))
		foods = append(foods, o.ID)
	}
	for _, id := range foods {
		g.Entered = append(g.Entered, state.ZoneEntry{
			Obj:        id,
			From:       state.ZBattlefield,
			To:         state.ZGraveyard,
			Sacrificed: true,
			Controller: 0,
		})
	}

	// Precondition: the reader's candidate set matches "Food" in the zone it
	// moved to; otherwise both counts below would be zero for the wrong
	// reason and the assertions would be vacuous.
	if len(foods) != 2 {
		t.Fatalf("fixture has %d Foods, want 2", len(foods))
	}
	if !matchesZoneSpecCtx(g, "Food", foods[0], c.SpecContext(0), state.ZGraveyard) {
		t.Fatal("Food does not match the sacrificed object in the graveyard")
	}

	// The plain spec counts the two Foods.
	if got, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$SacrificedThisTurn Food"); !ok || got != 2 {
		t.Fatalf("SacrificedThisTurn Food = (%d, %v), want (2, true)", got, ok)
	}
	// Bonecache Overseer's exact body: the /Times.3 suffix multiplies the
	// Food count, it is not part of the type spec.
	if got, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$SacrificedThisTurn Food/Times.3"); !ok || got != 6 {
		t.Fatalf("SacrificedThisTurn Food/Times.3 = (%d, %v), want (6, true)", got, ok)
	}
}
