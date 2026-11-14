package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// modeCost reads a Spree or Tiered mode's ModeCost$ (gorge kw_spree.go
// modeCost: the additional cost a chosen mode charges, CR 702.172a /
// 702.183a; one mechanism in gorge). Mana only, every mode of a card
// carrying the keyword, none elsewhere; a mode printing no ModeCost$ is
// refused rather than read as free. "0" parses as the empty cost.
//
// It returns the mode head's params with ModeCost$ consumed (nil: the
// chain builds them).
func (lc *lowerCtx) modeCost(sa *cards.SA, i int) (*params, error) {
	raw, has := sa.Params["ModeCost"]
	if lc.out.Escalate != nil {
		if has {
			return nil, refuse("ModeCost$ beside Escalate")
		}
		return nil, nil // the modes carry no cost of their own: Escalate prices the count (lower_escalate.go)
	}
	if !lc.out.Spree {
		if has {
			return nil, refuse("ModeCost$ without Spree or Tiered")
		}
		return nil, nil
	}
	if !has {
		return nil, refuse("Spree/Tiered mode without ModeCost$")
	}
	if i >= len(lc.out.ModeCost) {
		return nil, refuse("Spree/Tiered with more than %d modes", len(lc.out.ModeCost))
	}
	mc, err := parseManaCost(raw)
	if err != nil {
		return nil, refuse("ModeCost$ %q", raw)
	}
	if mc.X {
		return nil, refuse("ModeCost$ with {X}")
	}
	lc.out.ModeCost[i] = mc
	ps := newParams(sa.Params)
	ps.consume("ModeCost")
	return ps, nil
}
