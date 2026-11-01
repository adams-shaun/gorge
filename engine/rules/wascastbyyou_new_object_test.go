package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestWasCastByYouEndsOnAnyZoneChangeAfterTheCast pins CR 400.7 for the
// cast-provenance log reads: after its latest cast, any zone change of the
// object other than the resolution's stack -> battlefield entry makes a new
// object that was never cast. A Nine-Lives Familiar that is countered
// (stack -> graveyard) and reanimated by Zombify enters with no revival
// counters (dirac pairs 36065 62776); before the fix only a
// departure FROM the battlefield ended the provenance, so it entered with
// eight. The same read governs WasCastFromHandByYou ("if you cast it from
// your hand").
func TestWasCastByYouEndsOnAnyZoneChangeAfterTheCast(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Nine-Lives Familiar"), corpusAlternativeCard(t, "Zombify"))
	hand := e.G.Zone(state.ZHand, 0)
	familiar, zombify := hand[0], hand[1]

	e.emit(events.Event{Kind: events.PutOnStack, Obj: familiar, Player: 0, From: state.ZHand, To: state.ZStack})
	if !e.WasCastByYou(familiar, 0) || !e.WasCastFromHandByYou(familiar, 0) {
		t.Fatal("precondition: the Familiar on the stack was cast from hand by p0")
	}
	// Countered: the card goes to its owner's graveyard.
	e.emit(events.Event{Kind: events.MoveZone, Obj: familiar, From: state.ZStack, To: state.ZGraveyard})
	if e.WasCastByYou(familiar, 0) || e.WasCastFromHandByYou(familiar, 0) {
		t.Fatal("the countered card in the graveyard is a new object that was never cast (CR 400.7)")
	}
	zo := e.G.Obj(zombify)
	effects.Resolve(e, &effects.Ctx{Source: zombify, Controller: 0, SVars: zo.Face().SVars,
		Targets: []state.Target{{Obj: familiar}}}, zo.Face().SpellAbility())
	if o := e.G.Obj(familiar); o.Zone != state.ZBattlefield {
		t.Fatalf("Zombify did not return the Familiar: zone %v", o.Zone)
	}
	if got := e.G.Obj(familiar).Counter("REVIVAL"); got != 0 {
		t.Fatalf("reanimated Familiar entered with %d revival counters, want 0 (it was not cast)", got)
	}

	// Control: the resolution's own stack -> battlefield entry keeps the cast.
	e2 := handEngine(t, corpusAlternativeCard(t, "Nine-Lives Familiar"))
	id := e2.G.Zone(state.ZHand, 0)[0]
	e2.emit(events.Event{Kind: events.PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
	e2.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZBattlefield})
	if !e2.WasCastByYou(id, 0) || !e2.WasCastFromHandByYou(id, 0) {
		t.Fatal("a resolved permanent spell on the battlefield was cast")
	}
}
