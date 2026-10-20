package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// CastRemovalKind names the one-shot removal shapes the cast policy can
// recognise from a spell's primary ability: "destroy", "damage", or
// "bounce". It is deliberately narrower than the target-ask classifier
// (engine/rules/target_legal.go's targetRemoval): the cast policy only holds
// Destroy, DealDamage and ChangeZone-to-hand, and an unfamiliar destination
// is not inferred as bounce. Empty means "not a one-shot removal this policy
// reads".
func CastRemovalKind(sa *cards.SA) string {
	if sa == nil {
		return ""
	}
	switch sa.APIKind() {
	case cards.APIDestroy:
		return "destroy"
	case cards.APIDealDamage:
		return "damage"
	case cards.APIChangeZone:
		cz := ChangeZoneOf(sa)
		if cz.DestinationKnown && cz.Destination == state.ZHand && cz.OriginMask.Has(state.ZBattlefield) {
			return "bounce"
		}
	}
	return ""
}
