package kgen

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// K:Strive:<cost> (CR 702.52a): "this spell costs <cost> more to cast for
// each target beyond the first". gorge captures the keyword's raw parameter
// at the cast (beginCast) and folds len(targets)-1 copies of it into the cost
// once CR 601.2c's targets are in, before the modifiers and 601.2h's payment
// (repriceForTargets' foldStriveCost); its target ask carries the
// AffordableTargets hint (striveAffordableTargets).
//
// That is exactly the kernel's self RaisePerTarget raise (Officious
// Interrogation's "costs {W}{U} more for each target beyond the first",
// lower_raisecost.go raisePerTarget): the pips charged once per target less
// one, read from the cast being built, added before the reductions take
// theirs. kgen therefore lowers the keyword as that printed static. A
// parameter that is not plain mana (digits and WUBRGC letters) is refused by
// raisePerTarget's own parse.
func (lc *lowerCtx) lowerStrive(node, param string) {
	if strings.TrimSpace(param) == "" {
		lc.refuseNode(node, "strive cost %q", param)
		return
	}
	s := cards.Static{Mode: "RaiseCost", Params: map[string]string{
		"Mode":       "RaiseCost",
		"ValidCard":  "Card.Self",
		"Type":       "Spell",
		"EffectZone": "All",
		"Relative":   "True",
		"Cost":       param,
		"Amount":     "TargetedObjects$Amount/Minus.1",
	}}
	lc.guard(node, func() error {
		n := len(lc.out.Statics)
		if err := lc.lowerStatic(s); err != nil {
			return err
		}
		for _, st := range lc.out.Statics[n:] {
			if !slices.Contains(st.Flags, "RaisePerTarget") {
				lc.out.Statics = lc.out.Statics[:n]
				return refuse("strive cost %q outside the per-target raise", param)
			}
		}
		return nil
	})
}
