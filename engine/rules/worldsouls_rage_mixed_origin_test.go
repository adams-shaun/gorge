package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestWorldsoulsRagePicksFromHandAndGraveyardInOneAnswer pins the multi-pick
// mixed-Hand origin (Worldsoul's Rage's DBChangeZone with a literal count,
// CR 400.1 "from your hand and/or graveyard"): ONE search offers the land in
// hand and the land in the graveyard with Max 2, nothing moves before the
// answer, both answered lands enter tapped, and no mixed-origin Note fires.
func TestWorldsoulsRagePicksFromHandAndGraveyardInOneAnswer(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureGame(t, 314, sorceryFixtureSrc("WorldsoulFx",
		"A:SP$ ChangeZone | Origin$ Hand,Graveyard | Destination$ Battlefield | ChangeType$ Land | ChangeNum$ 2 | Tapped$ True"),
		[]string{isleCardSrc, isleCardSrc}, nil)
	addMana(t, e, 0, "B")
	clearHandExcept(t, e, 0, id)
	handLand := putNamed(t, e, 0, "Isle", state.ZHand)
	graveLand := putNamed(t, e, 0, "Isle", state.ZGraveyard)
	d := castSpellObj(t, e, id)
	if d == nil || d.ResumeKind != "search" || d.Max != 2 {
		t.Fatalf("mixed-origin search = %+v, want one search with Max 2", d)
	}
	hi := optionIdxOfObj(t, d, handLand)
	gi := optionIdxOfObj(t, d, graveLand)
	requireInZone(t, e, state.ZHand, handLand)
	requireInZone(t, e, state.ZGraveyard, graveLand)
	if d = pickChoices(t, e, hi, gi); d != nil && d.ResumeKind == "search" {
		t.Fatalf("a second search was posed: %+v", d)
	}
	for _, land := range []state.ObjID{handLand, graveLand} {
		requireInZone(t, e, state.ZBattlefield, land)
		if !e.G.Obj(land).Tapped {
			t.Fatalf("land %d entered untapped", land)
		}
	}
	if hasNote(e, "mixed ChangeZone Origin") {
		t.Fatal("a mixed-origin Note fired although both chosen lands moved")
	}
	replayCheck(t, e, cfg)
}
