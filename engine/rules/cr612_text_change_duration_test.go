package rules

// CR 612 text-change durations (cr-g36b). Crystal Spray, Trait Doctoring
// and Whim of Volrath change text "until end of turn" and carry no
// Duration$; only `Duration$ Permanent` ("This effect lasts indefinitely":
// Magical Hack, Sleight of Mind, New Blood) is indefinite. An
// until-end-of-turn change ends in the cleanup step (CR 514.2).

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// textChangeDur resolves `ChangeTypeWord$ <from> <to>` on id with the given
// Duration$ suffix ("" for none).
func textChangeDur(t *testing.T, e *Engine, id state.ObjID, from, to, dur string) {
	t.Helper()
	src := card(t, "Name:Spray\nManaCost:U\nTypes:Instant\nA:SP$ ChangeText | Defined$ Targeted | ChangeTypeWord$ "+
		from+" "+to+dur+"\nOracle:x\n")
	effects.Resolve(e, &effects.Ctx{Controller: 0, Targets: []state.Target{{Obj: id}}, TargetsOffered: true},
		src.Faces[0].Abilities[0])
}

// TestCR612UntilEndOfTurnTextChangeEndsAtCleanup: Swamp -> Island with no
// Duration$ on Bog Wraith is Islandwalk until the cleanup step, then
// Swampwalk again; a Duration$ Permanent change survives the same cleanup.
func TestCR612UntilEndOfTurnTextChangeEndsAtCleanup(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	wraith := onBoardCard(t, e, 0, corpusCard(t, "Bog Wraith"))
	other := onBoardCard(t, e, 0, corpusCard(t, "Bog Wraith"))
	textChangeDur(t, e, wraith, "Swamp", "Island", "")
	textChangeDur(t, e, other, "Swamp", "Forest", " | Duration$ Permanent")
	if kw := e.Derived(wraith).Keywords; !slices.Contains(kw, "Landwalk:Island") {
		t.Fatalf("precondition: keywords %v, want Landwalk:Island during the turn", kw)
	}
	e.EndOfTurnCleanup()
	if kw := e.Derived(wraith).Keywords; !slices.Contains(kw, "Landwalk:Swamp") || slices.Contains(kw, "Landwalk:Island") {
		t.Fatalf("CR 514.2: after cleanup an until-end-of-turn text change still applies: keywords %v, want Landwalk:Swamp", kw)
	}
	if kw := e.Derived(other).Keywords; !slices.Contains(kw, "Landwalk:Forest") {
		t.Fatalf("a Duration$ Permanent text change ended at cleanup: keywords %v, want Landwalk:Forest", kw)
	}
}
