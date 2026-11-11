package kgen

import (
	"strings"
)

// animateAll lowers api:AnimateAll (gorge effAnimateAll): Animate's grant
// registered on every battlefield permanent matching ValidCards$ (default
// Creature) at resolution (CR 611.2c), seat 0 first, each seat in entry
// order. OpOverrideAll applies the override to each. RemoveKeywords$ and
// RemoveAllAbilities$ ride the override record (gorge parseAnimateGrant,
// layer 6, CR 613.1f). A player-kind ValidTgts$ ("creatures target player
// controls": gorge targetPlayerKindScope) rides Obj and limits the sweep to
// that player's permanents. Refused: Zone$ (a non-battlefield sweep),
// Defined$, host-bound durations (AsLongAsInPlay, UntilHostLeavesPlay,
// UntilUntaps: OverrideAll has no host anchor), and HiddenKeywords$, Replacements$, CantHaveKeyword$, or
// parameters gorge does not read here (Colors$, Abilities$, ...: unread).
func (lc *lowerCtx) animateAll(ps *params) error {
	for _, k := range []string{"Zone", "Defined", "HiddenKeywords", "Replacements", "CantHaveKeyword"} {
		if _, ok := ps.get(k); ok {
			return refuse("AnimateAll %s$", k)
		}
	}
	// OverrideAll has no host anchor (dirac/static_animateallscope.go overrideAll
	// passes SetP/SetT where hostLeft reads the host Obj), so a host-bound
	// duration cannot ride it.
	switch d := ps.str("Duration"); d {
	case "AsLongAsInPlay", "UntilHostLeavesPlay", "UntilUntaps":
		return refuse("AnimateAll Duration$ %s: host-bound, OverrideAll has no host anchor", d)
	}
	var scope lRef
	if vt, ok := ps.get("ValidTgts"); ok {
		pf, err := lc.filter(vt)
		if err != nil {
			return err
		}
		if len(pf.Clauses) != 0 || pf.Players == 0 {
			return refuse("AnimateAll ValidTgts$ %q: not a player target", vt)
		}
		scope = lRef{Kind: rTarget, N: lc.addTarget(lTarget{F: pf})}
	}
	if _, hasP := ps.get("Power"); hasP {
		if _, hasT := ps.get("Toughness"); !hasT {
			return refuse("AnimateAll Power$ without Toughness$")
		}
	}
	spec := ps.str("ValidCards")
	if spec == "" {
		spec = "Creature"
	}
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	ps.consume("IsCurse")
	lc.animAll = true
	o, dur, chosen, err := lc.animateOverride(ps, lRef{})
	lc.animAll = false
	if err != nil {
		return err
	}
	if o.WhileHost {
		return refuse("AnimateAll host-bound duration")
	}
	if chosen {
		// gorge's AnimateAll grant reads ChosenColor on the SOURCE, which
		// an AnimateAll chain never asks
		return refuse("AnimateAll Colors$ ChosenColor")
	}
	in := lInstr{Op: "OverrideAll", F: f, Obj: scope, CS: dur, Note: overrideString(o)}
	lc.out.overrides = append(lc.out.overrides, o)
	in.C = len(lc.out.overrides) - 1
	lc.emit(in)
	return nil
}

// animateHiddenKeywords reads an Animate's HiddenKeywords$ (gorge
// animateGrant.hiddenKws: layer-6 keyword text). Two sentences are read:
// "CARDNAME can't attack or block." (the kernel's KwCantBlock plus the
// override's CantAttack, gorge cards.HiddenKeywordCantAttackOrBlock) and
// "CARDNAME must be blocked if able." (CR 509.1c; the mark ExtMustBeBlocked,
// which emitOverride follows the record with). The mark
// lasts until end of turn and names one object, so it needs that object
// (obj, not the zero ref of an AnimateAll or a guarded sweep) and the
// end-of-turn duration.
func (lc *lowerCtx) animateHiddenKeywords(ps *params, o *lOverride, obj lRef, dur string) error {
	v, ok := ps.get("HiddenKeywords")
	if !ok {
		return nil
	}
	if strings.TrimSpace(v) == "CARDNAME must be blocked if able." {
		if obj.Kind == rNone || dur != "EOT" {
			return refuse("Animate HiddenKeywords$ %q without one object until end of turn", v)
		}
		o.MustBeBlocked = true
		return nil
	}
	if strings.TrimSpace(v) != "CARDNAME can't attack or block." {
		return refuse("Animate HiddenKeywords$ %q", v)
	}
	o.AddKw = append(o.AddKw, "CantBlock")
	o.CantAttack = true
	return nil
}
