package kgen

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// lockedCantBlock lowers a no-target Effect whose one StaticAbilities$ body
// is "Mode$ Continuous | Affected$ <spec> | AddHiddenKeyword$ CARDNAME can't
// block." with an Affected$ spec that reads power or toughness (Temur Charm's
// "Creatures with power 3 or less can't block this turn") to OpOverrideAll
// carrying a can't-block override, not to an emblem static. An emblem's
// Affected$ filter is evaluated when its consumers ask (CR 611.2c says that
// is how a STATIC ability works), but the continuous effect an Effect
// registers is a resolution-generated one: its set is determined when the
// effect begins and never changes (gorge effects.lockEffectGrant freezes it
// the same way). OverrideAll evaluates its filter once at resolution (seat 0's
// battlefield in entry order, then seat 1's), so a creature the charm found at
// power 5 keeps blocking after a Disfigure shrinks it, and one that was power
// 2 stays unable to block after a pump. It lasts until end of turn; a
// UntilYourNextTurn spelling and a body with any other parameter refuses.
// A spec that reads no P/T stays the emblem (its set was never moved by a
// characteristic change, and gorge keeps it dynamic).
func (lc *lowerCtx) lockedCantBlock(ps *params, ss []cards.Static) (bool, error) {
	if len(ss) != 1 || ss[0].Mode != "Continuous" {
		return false, nil
	}
	sp := newParams(ss[0].Params)
	if strings.TrimSpace(sp.str("AddHiddenKeyword")) != "CARDNAME can't block." || !effects.SpecReadsPT(sp.str("Affected")) {
		return false, nil
	}
	// AffectedZone$ Battlefield (Mournwillow) is the default scope spelled out
	keys := []string{"Mode", "Affected", "AddHiddenKeyword", "Description", "AffectedZone=Battlefield"}
	for k, v := range ss[0].Params {
		if !slices.Contains(keys, k) && !slices.Contains(keys, k+"="+v) {
			return true, refuse("Effect locked can't-block static with %s$", k)
		}
	}
	eot, _, _, err := effectEmblemDuration(ps)
	if err != nil {
		return true, err
	}
	if !eot {
		return true, refuse("Effect locked can't-block static Duration$ %q", ps.str("Duration"))
	}
	f, err := lc.filter(sp.str("Affected"))
	if err != nil {
		return true, err
	}
	ps.consume("Name", "Image", "Stackable")
	left := slices.DeleteFunc(ps.unread(), func(k string) bool { return strings.HasPrefix(k, "Condition") })
	if len(left) > 0 {
		return true, refuse("Effect static unread %v", left)
	}
	o := &lOverride{AddKw: []string{"CantBlock"}}
	lc.out.overrides = append(lc.out.overrides, o)
	lc.emit(lInstr{Op: "OverrideAll", F: f, CS: "EOT", Note: overrideString(o), C: len(lc.out.overrides) - 1})
	return true, nil
}
