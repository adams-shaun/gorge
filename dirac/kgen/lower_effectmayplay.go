package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// effectEmblemMayPlay lowers an Effect's no-target, no-RememberObjects$
// Continuous MayPlay$ static (Chainer, Nightmare Adept; Liliana, Untouched
// by Death; Chandra, Flame's Catalyst; Wish) to the MayPlayFrom static the
// printed-permission path already builds. The static is carried by the
// Effect's emblem, whose controller is the effect's controller, exactly as
// gorge's Effect-delivered continuous grant is the resolving player's.
//
// Only the zone permission shapes staticBody's mayPlayStatic reads lower:
// a graveyard, hand, exile or sideboard grant. Anything else keeps its named
// refusal from staticBody.
func (lc *lowerCtx) effectEmblemMayPlay(s cards.Static) (lStatic, error) {
	sts, err := lc.staticBody(s)
	if err != nil {
		return lStatic{}, err
	}
	if len(sts) != 1 || sts[0].Kind != "MayPlayFrom" {
		return lStatic{}, refuse("Effect static Continuous MayPlay lowered to %d statics", len(sts))
	}
	st := sts[0]
	if st.F != nil {
		for _, c := range st.F.Clauses {
			// The zone permission is a plain card set (type, subtype, colour,
			// owner, controller). A counter, P/T, name or other predicate
			// reads a characteristic the Effect's grant is not verified on
			// (Lara Croft's discovery-counter set): refuse rather than widen.
			if c.Flags != 0 || c.CmpStat != cmpNone || c.CmpCounter != "" || c.Name != "" ||
				c.CtrlTgt != 0 || c.Special != 0 || c.Unblocked || c.SpellBase || c.BasePermanent {
				return lStatic{}, refuse("Effect static Continuous MayPlay filter %s reads a predicate", st.F)
			}
		}
	}
	return st, nil
}
