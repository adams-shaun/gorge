package rules

// Mode$ BecomesTarget with FirstTime$ True, the CR 400.7 half: a creature that
// left and came back is a NEW object, so "the first time each turn" re-arms for
// it mid-turn. Kira, Great Glass-Spinner's blinked shape (XMage
// KiraGreatGlassSpinnerTest#counterFirstBlinkCounterNext). The gate is
// trigmatch.firstTimeTargetedThisTurn.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestBecomesTargetFirstTimeCountsANewObjectAfterAZoneChange(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	trig := corpusTriggerMode(t, mustCorpusCard(t, reg, "Glyph Keeper"), "BecomesTarget")
	if trig.Params["FirstTime"] != "True" {
		t.Fatalf("corpus fixture drift: Glyph Keeper FirstTime$ = %q, want True", trig.Params["FirstTime"])
	}
	e, ids := becomesTargetOnceBoard(t, reg,
		[]string{"Glyph Keeper", "Prodigal Pyromancer", "Prodigal Pyromancer"},
		nil)
	keeper := ids["Glyph Keeper"]
	var pyros []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Prodigal Pyromancer" {
			pyros = append(pyros, id)
		}
	}
	if len(pyros) != 2 {
		t.Fatalf("precondition: %d Pyromancers, want 2", len(pyros))
	}
	if o := e.G.Obj(keeper); o == nil || o.Zone != state.ZBattlefield || o.Damage != 0 {
		t.Fatalf("precondition: Glyph Keeper not an undamaged battlefield permanent: %+v", o)
	}
	ping := func(p state.ObjID) {
		t.Helper()
		if o := e.G.Obj(p); o == nil || o.Tapped || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: source %d not an untapped battlefield permanent: %+v", p, o)
		}
		e.askPriority(0)
		submitChoices(t, e, abilityOption(t, e, p, 0).Index)
		submitChoices(t, e, targetOptionIdx(t, e, keeper))
		passUntilStackEmpty(t, e, 40)
	}
	ping(pyros[0])
	if got := e.G.Obj(keeper).Damage; got != 0 {
		t.Fatalf("first targeting this turn was not countered: Keeper damage = %d, want 0", got)
	}
	// Battlefield -> exile -> battlefield: the returned permanent is a new
	// object (CR 400.7), so the next targeting is its first this turn.
	e.emit(events.Event{Kind: events.MoveZone, Obj: keeper, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: keeper, From: state.ZExile, To: state.ZBattlefield})
	if o := e.G.Obj(keeper); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Glyph Keeper not back on the battlefield after the blink: %+v", o)
	}
	ping(pyros[1])
	if got := e.G.Obj(keeper).Damage; got != 0 {
		t.Fatalf("the returned object's first targeting was not countered: Keeper damage = %d, want 0", got)
	}
}
