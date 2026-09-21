package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestDelayTriggerRememberedPassesPlayersAndObjects pins definedSpec's
// DelayTriggerRemembered case: the delayed trigger's remembered set is
// handed back AS-IS, players included (Arcane Denial's RememberObjects$
// RememberedController names the countered spell's CONTROLLER, a player),
// and an object-remembered registration (Berserk's TrigDestroy, Lodestone
// Bauble's DrawSlowtrip, 29 corpus files) keeps resolving to that object.
//
// The regression this guards: an empty case body does NOT fall through in
// Go, so a commented-out-but-return-less case walks past the switch and
// definedSpec reports ok=false. Defined then falls back to the SA's chosen
// targets or to the resolving SOURCE, silently retargeting every carrier.
func TestDelayTriggerRememberedPassesPlayersAndObjects(t *testing.T) {
	h := newHost(t, 4)

	// A remembered PLAYER survives the read (the objectsOf sibling forms
	// would drop it and the whole Execute$ would no-op).
	ctx := &Ctx{Source: 7, Controller: 1,
		Remembered: []state.Target{{Player: 3, IsPlayer: true}}}
	got := Defined(h, ctx, sa(t, "SP$ X | Defined$ DelayTriggerRemembered"))
	if len(got) != 1 || !got[0].IsPlayer || got[0].Player != 3 {
		t.Fatalf("remembered player -> %+v, want the remembered seat 3", got)
	}

	// A remembered OBJECT resolves to that object, never to the source.
	ctx = &Ctx{Source: 7, Controller: 1, Remembered: []state.Target{{Obj: 9}}}
	got = Defined(h, ctx, sa(t, "SP$ X | Defined$ DelayTriggerRemembered"))
	if len(got) != 1 || got[0].IsPlayer || got[0].Obj != 9 {
		t.Fatalf("remembered object -> %+v, want object 9", got)
	}

	// The LKI spelling keeps its objects-only read (it names an LKI copy of
	// a captured OBJECT), so the two cases stay distinguishable.
	ctx = &Ctx{Source: 7, Controller: 1,
		Remembered: []state.Target{{Player: 3, IsPlayer: true}, {Obj: 9}}}
	got = Defined(h, ctx, sa(t, "SP$ X | Defined$ DelayTriggerRememberedLKI"))
	if len(got) != 1 || got[0].Obj != 9 {
		t.Fatalf("LKI spelling -> %+v, want the object entry only", got)
	}

	// The result must not alias Ctx.Remembered (Defined's copy contract).
	ctx = &Ctx{Source: 7, Remembered: []state.Target{{Obj: 5}}}
	got = Defined(h, ctx, sa(t, "SP$ X | Defined$ DelayTriggerRemembered"))
	got[0] = state.Target{Obj: 999}
	if ctx.Remembered[0].Obj != 5 {
		t.Fatalf("mutating the result changed Ctx.Remembered: %+v", ctx.Remembered)
	}
}
