package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Count$AltCost.<paid>.<unpaid> (the Mastery cycle's AltCostPaid): <paid>
// when the resolving spell was cast for its own S:Mode$ AlternativeCost
// static, else <unpaid>. gorge stamps FlagAltCostPaid on such a cast of a
// face that reads Count$AltCost (rules cast_begin.go, faceWantsAltCost) and
// reads it, or the pending cast's PendingAltCost before payment, in
// effects count_body_heads.go. The kernel marks the stack entry
// trigPlAltCost as the alternative cast begins (takeAltCast), only on a table
// with an ExAltCost read, so no other cast's entry changes. A copy is not
// cast and drops the mark (copy_gen.go), as a gorge copy carries no
// CastFlags. kgen lowers the head only in a spell ability of a card whose
// own alternative cost the kernel casts through castAltTgt.

// prepAltCostRead sets genPrep.anyAltCostRead for a table with an ExAltCost
// expression.
func prepAltCostRead(t *kop.Table, g *genPrep) {
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExAltCost {
			g.anyAltCostRead = true
			return
		}
	}
}

// DebugAltCostNeverPaid makes ExAltCost read the unpaid arm always (the
// mutation test: the lockstep must catch it).
var DebugAltCostNeverPaid bool

// altCostValue is ExAltCost: Sub[0] when the entry being resolved or built
// is a spell cast for its own alternative cost, else Sub[1].
func (s *State) altCostValue(k *prep, x *kop.Expr, ctx evalCtx) int {
	if e := ctx.e; e != nil && e.kind == seSpell && e.trigPl == trigPlAltCost && !DebugAltCostNeverPaid {
		return s.evalV(k, x.Sub[0], ctx)
	}
	return s.evalV(k, x.Sub[1], ctx)
}
