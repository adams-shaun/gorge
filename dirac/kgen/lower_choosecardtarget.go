package kgen

import "strings"

// chooseCardTargetChooser reads ChooseCard's ValidTgts$ when the targeted
// player is the one who chooses.
//
// gorge's effChooseCard (engine/effects/choose_control.go:584) asks each of
// chooseCardChoosers (:430), which is choiceChoosers (:399-425):
// DefinedRef(h, c, DefinedRefOf(sa), sa) mapped through PlayerOf. With no
// Defined$, DefinedRef (engine/effects/context.go:80) falls back to the
// ability's chosen targets whenever ValidTgts$ is present (:130-139), so the
// chooser is the TARGETED opponent, not the resolving controller (Forge's
// ChooseCardEffect getDefinedPlayersOrTargeted agrees). The pool is still
// matched from the chooser's perspective (choose_control.go:170, 393-397) and
// TargetedPlayerCtrl names the target either way; Mandatory$ True clamps
// Min=Max=1 to the pool, and an empty pool asks nothing.
//
// Only ValidTgts$ exactly Opponent is lowered. A target-count, zone or
// controller rider, a non-player filter, or a Defined$ beside ValidTgts$
// (which would name the chooser itself) all change who answers, so they
// refuse by name. The optional branch keeps its own chooser and stays
// refused, and the mandatory branch's Amount$ 1 / ChoiceTitle$ reads happen
// in chooseCard after the target slot exists.
func (lc *lowerCtx) chooseCardTargetChooser(ps *params) (lRef, bool, error) {
	vt, ok := ps.get("ValidTgts")
	if !ok {
		return lRef{}, false, nil
	}
	for _, k := range []string{"TargetMin", "TargetMax", "TgtZone", "TargetUnique", "TargetsWithSameController", "TargetsWithDifferentControllers"} {
		if _, ok := ps.get(k); ok {
			return lRef{}, false, refuse("ChooseCard ValidTgts$ %q with %s$", vt, k)
		}
	}
	if d, ok := ps.get("Defined"); ok {
		return lRef{}, false, refuse("ChooseCard ValidTgts$ %q with Defined$ %q", vt, d)
	}
	if !ps.is("Mandatory", "True") {
		return lRef{}, false, refuse("optional ChooseCard with ValidTgts$")
	}
	if strings.TrimSpace(vt) != "Opponent" {
		return lRef{}, false, refuse("ChooseCard ValidTgts$ %q", vt)
	}
	f, err := lc.filter(vt)
	if err != nil {
		return lRef{}, false, err
	}
	if f.Players != plOpp || len(f.Clauses) != 0 {
		return lRef{}, false, refuse("ChooseCard ValidTgts$ %q target filter", vt)
	}
	t := lc.addTarget(lTarget{F: f, Zone: zNone})
	ps.consume("ValidTgts")
	return lRef{Kind: rTarget, N: t}, true, nil
}
