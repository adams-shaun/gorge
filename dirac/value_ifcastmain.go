package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Count$IfCastInOwnMainPhase.<yes>.<no> (ExIfCastOwnMain), gorge
// effects/count_body_heads.go: three conjuncts read live, with no stamp and
// no State. The step now is a main phase, the spell's controller is the
// active player, and the source spell was cast (a stack copy never was,
// CR 707.10). A spell cast in a main phase that resolves in another step
// reads the no arm, as gorge's does.

// DebugCastInOwnMainAny (mutation test only): the read drops the active
// player and the cast conjuncts.
var DebugCastInOwnMainAny bool

// castInOwnMain evaluates the three conjuncts for the spell entry in ctx.
func (s *State) castInOwnMain(ctx evalCtx) bool {
	if s.step != kop.StepMain1 && s.step != kop.StepMain2 {
		return false
	}
	if DebugCastInOwnMainAny {
		return true
	}
	e := ctx.e
	return s.active == ctx.ctrl && e != nil && e.kind == seSpell && e.obj != NoObj && s.objs[e.obj].of&ofCopy == 0
}
