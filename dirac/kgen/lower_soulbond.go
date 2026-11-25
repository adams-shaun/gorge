package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Soulbond (CR 702.95): gorge cards/kw_soulbond.go expands the printed
// keyword into two ChangesZone triggers (this creature enters; another
// creature you control enters), each an intervening if over the "!Paired"
// word and each resolving api Pair (engine/effects/soulbond.go effPair). The
// kernel's half is dirac/kw_soulbond.go: the pair record, the Pair ask and
// the Paired / PairedWith filter words.

func init() {
	kwHandlers["Soulbond"] = (*lowerCtx).lowerSoulbondKw
	apiPS("Pair", (*lowerCtx).soulbondPair)
	instrOp("Pair", (*instrCtx).opPair)
	fwName[kop.FwPaired] = "FwPaired"
	fwName[kop.FwPairedWith] = "FwPairedWith"
	// Paired / !Paired: gorge predicates["Paired"] (o.Paired != 0).
	fwArms["Paired"] = func(c *lClause, _ string) error { return c.fwLit(lSpillLit{W: kop.FwPaired}) }
	fwArms["!Paired"] = func(c *lClause, _ string) error { return c.fwLit(lSpillLit{W: kop.FwPaired, Neg: true}) }
	// PairedWith: gorge predicates["PairedWith"] (the source's Paired is the
	// candidate). Gorge's '!' of it would read every other object; no corpus
	// line spells it, so only the positive word lowers.
	fwArms["PairedWith"] = func(c *lClause, _ string) error { return c.fwLit(lSpillLit{W: kop.FwPairedWith}) }
}

// lowerSoulbondKw lowers K:Soulbond: the printed keyword has no kernel
// meaning of its own beyond its expansion's two triggers, so the handler only
// checks the face carries both (a printed line whose expansion a guard
// suppressed would otherwise lose its meaning silently).
func (lc *lowerCtx) lowerSoulbondKw(node, _ string) {
	k := kwLineOf(node)
	n := 0
	for _, t := range lc.face.Triggers {
		if l := t.Params["KeywordLine"]; l == k+"#self" || l == k+"#other" {
			n++
		}
	}
	if n != 2 {
		lc.refuseNode(node, "keyword %q without its expansion", "Soulbond")
	}
}

// soulbondPair lowers api Pair: the source may pair with another unpaired
// creature its controller controls; RestrictToRemembered$ True (the
// "another creature enters" half) narrows the pick to the triggering
// entrant (C 1).
func (lc *lowerCtx) soulbondPair(ps *params) error {
	c := 0
	switch v := ps.str("RestrictToRemembered"); {
	case v == "":
	case strings.EqualFold(v, "True"):
		c = 1
	default:
		return refuse("Pair RestrictToRemembered %q", v)
	}
	lc.emit(lInstr{Op: "Pair", Obj: lRef{Kind: rSelf}, C: c, Note: "Soulbond's optional pairing (CR 702.95a; dirac/kw_soulbond.go)"})
	return nil
}

func (ic *instrCtx) opPair() error {
	ki := &ic.ki
	// OpExt ExtPair, Obj the soulbond creature, C 1 the triggering entrant only
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtPair)), int16(ic.in.C)
	return nil
}
