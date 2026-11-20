package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 508.1c / 509.1b companion restrictions ("can only attack alone", "can't
// attack unless a creature with greater power also attacks"): a property of the
// whole declaration, decided by combat.CompanionViolated. The ask publishes it
// per option (decision.Option.AloneOnly / CompanionNeed / CompanionFrom, which
// Decision.Validate and the bot repair read through AloneLegal) and the
// engine's declaration validators recompute it from the same oracle. The offer
// never enumerates subsets: every creature stays offered, the restriction is
// declare-then-validate.

// publishCompanions stamps the companion facts on the final (re-indexed)
// option list of a KAttackers (attack) or KBlockers decision. An option's
// creature is its Obj on both sides (the attacker / the blocker).
func publishCompanions(b combat.Board, opts []decision.Option, attack bool) {
	for i := range opts {
		o := &opts[i]
		c := combat.CompanionOf(b, o.Obj, attack)
		switch c {
		case combat.CompanionNone:
		case combat.CompanionOnlyAlone:
			o.AloneOnly = true
		default:
			o.CompanionNeed = c.Need()
			for j := range opts {
				if opts[j].Obj != o.Obj && combat.CompanionCounts(b, c, o.Obj, opts[j].Obj) {
					o.CompanionFrom = append(o.CompanionFrom, opts[j].Index)
				}
			}
		}
	}
}

// validateCompanionDeclaration enforces the companion restrictions on ONE whole
// declaration: no declared creature may break its own rule against the set.
func validateCompanionDeclaration(b combat.Board, chosen []decision.Option, attack bool) error {
	ids := make([]state.ObjID, 0, len(chosen))
	for _, o := range chosen {
		ids = append(ids, o.Obj)
	}
	for _, id := range ids {
		if combat.CompanionViolated(b, id, attack, ids) {
			if attack {
				return fmt.Errorf("attacker %d can't attack with this declaration (its attack restriction is not met)", id)
			}
			return fmt.Errorf("blocker %d can't block with this declaration (its block restriction is not met)", id)
		}
	}
	return nil
}
