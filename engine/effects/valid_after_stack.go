package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// validAfterStackPlayerPrefix and validAfterStackGraveyardProp are the one
// ValidAfterStack$ spelling the corpus writes on a player target (Keeper of
// the Dead): `Ability.IsTargeting Player.HasCardsInGraveyard_<spec>_<cmp>`.
const (
	validAfterStackTargetingPrefix = "Ability.IsTargeting "
	validAfterStackPlayerPrefix    = "Player."
	validAfterStackGraveyardProp   = "HasCardsInGraveyard_"
)

// PlayerAdmitsValidAfterStack reports whether player q, chosen as the target
// of an ability, satisfies the ability's ValidAfterStack$ spec (Forge's
// SpellAbility.isLegalAfterStack, checked once the targets are chosen and
// before costs are paid, CR 602.2b). The count comparison is evaluated from
// the activating Ctx's perspective -- its SVars (Keeper of the Dead's
// SVar Y = the controller's creature cards minus two) and controller -- over
// q's graveyard, through the shared HasProperty zone-count evaluator
// (hasPropertyStateBacked), so the grammar has one home. A spec this build
// does not evaluate (the corpus also writes Player.hasMoreLife,
// withMoreCreaturesThanYou and HasCardsInHand_ on other Keepers) admits
// everyone, as before ValidAfterStack$ was read (fail open); so does an empty
// spec.
func PlayerAdmitsValidAfterStack(h Host, c *Ctx, spec string, q state.PlayerID) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return true
	}
	rest, ok := strings.CutPrefix(spec, validAfterStackTargetingPrefix)
	if !ok {
		return true
	}
	rest, ok = strings.CutPrefix(strings.TrimSpace(rest), validAfterStackPlayerPrefix)
	if !ok || !strings.HasPrefix(rest, validAfterStackGraveyardProp) {
		return true
	}
	n, ok := hasPropertyStateBacked(h, h.Game(), c, []state.PlayerID{q}, "HasProperty"+rest, "")
	return ok && n == 1
}
