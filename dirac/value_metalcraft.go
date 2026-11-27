package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Metalcraft (CR 207.2c: three or more artifacts) is one kernel condition
// shape: kgen lowers every Metalcraft spelling (Activation$, Metalcraft$ on a
// trigger, Condition$ Metalcraft) to Count$Valid Artifact.YouCtrl >= 3
// (kgen/lower_conditions.go's bareCondition). condEval reads it for the
// non-mana activation offer (legal.go's activateCondHolds), a trigger's
// intervening-if (triggers_gen.go's trigOK) and a gated mana ability's view
// (mana_gate.go / mana_dynpair.go's dynPairView).

// MetalcraftStats counts the metalcraft conditions that held in a lockstep
// game (TestDemoMetalcraftLockstep reads it so a green game is one where the
// new shapes fired; nothing in the engine reads it).
type MetalcraftStats struct {
	CondHeld int64 // metalcraft conditions evaluated true
}

// DebugMetalcraftStats, when set, receives the counts.
var DebugMetalcraftStats *MetalcraftStats

// DebugMetalcraftOff (TestDemoMetalcraftMutation) forces every metalcraft
// condition false: the gate-off view, the unfired trigger and the refused
// activation together are the new read disabled.
var DebugMetalcraftOff bool

// isMetalcraftCond reports whether c is kgen's metalcraft condition:
// Count$Valid Artifact.YouCtrl >= 3, not negated. A filter name cannot be
// compared by value here, so the check is the shape the lowering emits; every
// corpus line with it is a metalcraft line.
func isMetalcraftCond(c *kop.Cond) bool {
	return c != nil && c.Kind == kop.CondValue && !c.Neg && c.Op == kop.CmpGE &&
		c.B.K == kop.VConst && c.B.N == 3 && c.A.K == kop.VCountFilter && c.A.N != 0
}
