package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// reflexiveCaptured is the one seam every resolution Ctx builder shares
// (resolveTop, resumeResolution): it seeds a reflexive ability's Snap.TargetPT
// with the queue-time P/T of its remembered objects, as a private copy.
func TestReflexiveCapturedSeedsRememberedPT(t *testing.T) {
	t.Parallel()
	pt := map[state.ObjID]effects.TargetPT{22: {Power: 5, Toughness: 1}}

	ctx := &effects.Ctx{}
	ctx.TriggerContext.Reflexive = true
	reflexiveCaptured(ctx, pt)
	if got, ok := ctx.Snap.TargetPT[22]; !ok || got.Power != 5 {
		t.Fatalf("reflexive Ctx TargetPT[22] = %+v (present %v), want power 5", got, ok)
	}
	ctx.Snap.TargetPT[22] = effects.TargetPT{Power: 9}
	if pt[22].Power != 5 {
		t.Fatalf("seed aliases the stack object's map: its entry is now %+v", pt[22])
	}

	plain := &effects.Ctx{}
	reflexiveCaptured(plain, pt)
	if plain.Snap.TargetPT != nil {
		t.Fatalf("non-reflexive Ctx was seeded: %+v", plain.Snap.TargetPT)
	}

	bound := &effects.Ctx{}
	bound.TriggerContext.Reflexive = true
	reflexiveCaptured(bound, nil)
	if bound.Snap.TargetPT != nil {
		t.Fatalf("nil seed left TargetPT = %+v, want nil", bound.Snap.TargetPT)
	}
}
