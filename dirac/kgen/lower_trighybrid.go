package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A printed trigger's own optional cost with hybrid mana ("you may pay
// {G/W}", Emiel the Blessed; Kels, Fight Fixer's {U/B}; Alesha's
// {W/B}{W/B}). gorge's triggered-cost window (rules respay.go) offers the
// pay when the pool or the potential bound covers the cost, funds it
// through the payment planner (the hybrid pip resolved as planHybridCost
// does: the first resolution the sources afford) and charges it from the
// pool; no pip is announced. The kernel's PayMana ask already pays a
// hybrid cost that way (mana_resolutionpay.go payReplanned's hybResolve):
// the lowering carries the pips in the instruction's F (kop.PayManaF).

// trigHybridOK reports whether c's hybrid mana is a trigger's own cost the
// kernel pays: mana (and life) alone, at most kop.PayHybMax pips. An unless
// or ward cost (gorge's mana window), a cost inside an effect and the
// NumTimes, {X}, exile, draw and twobrid shapes keep refusing.
func (lc *lowerCtx) trigHybridOK(c lCost) bool {
	m := c.Mana
	return lc.trigOwnCost && !lc.unlessCost && !lc.wardCost && lc.trigXMax == nil &&
		len(m.Hybrid) <= kop.PayHybMax && len(m.Twobrid)+len(m.Phyrexian) == 0 &&
		c.ExileG == nil && !c.ExileStackSelf && c.DrawN == 0
}

// payManaF packs a PayMana instruction's F: the life paid beside the mana
// and the hybrid pips in printed order. ok is false past the encoding.
func payManaF(life int, hyb []string) (kop.FilterID, bool) {
	var pips [kop.PayHybMax][2]uint8
	if len(hyb) > len(pips) {
		return 0, false
	}
	for i, h := range hyb {
		if len(h) != 2 {
			return 0, false
		}
		a, b := strings.IndexByte("WUBRG", h[0]), strings.IndexByte("WUBRG", h[1])
		if a < 0 || b < 0 || a == b {
			return 0, false
		}
		pips[i] = [2]uint8{uint8(a), uint8(b)}
	}
	return kop.PayManaF(life, pips[:len(hyb)])
}
