package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// mayPlayCardOwner reports whether a MayPlayPlayer$ value is CardOwner: the
// grant's beneficiary is each affected card's OWNER, per card, not the
// granting static's or effect's controller (Soul Partition: "For as long as
// that card remains exiled, its owner may play it").
func mayPlayCardOwner(v string) bool { return v == effects.MayPlayCardOwner }

// mayPlayPerCardBeneficiary reports whether a MayPlayPlayer$ value names a
// per-card beneficiary (Exiler or CardOwner) rather than the controller.
func mayPlayPerCardBeneficiary(v string) bool { return mayPlayExiler(v) || mayPlayCardOwner(v) }

// mayPlayEffectSeatOpen reports whether the effect-delivered may-play grant
// ce can benefit p for SOME card: its controller's, or any seat's for a
// CardOwner grant (mayPlayEffectBenefits is the per-card filter). It is the
// card-independent gate the effect walks run before enumerating cards.
func mayPlayEffectSeatOpen(ce *state.ContinuousEffect, p state.PlayerID) bool {
	return ce.Controller == p || mayPlayCardOwner(ce.MayPlayPlayer)
}

// mayPlayEffectBenefits reports whether the effect-delivered may-play grant
// ce gives p the play of card o: o's owner for a CardOwner grant, the
// effect's controller otherwise.
func mayPlayEffectBenefits(ce *state.ContinuousEffect, o *state.Object, p state.PlayerID) bool {
	if mayPlayCardOwner(ce.MayPlayPlayer) {
		return o.Owner == p
	}
	return ce.Controller == p
}

// mayPlayEffectKeep narrows an effect walk's card test keep to the cards ce
// grants p (mayPlayEffectBenefits), so a CardOwner grant enumerates only p's
// own cards.
func mayPlayEffectKeep(ce *state.ContinuousEffect, p state.PlayerID, keep func(*state.Object) bool) func(*state.Object) bool {
	if !mayPlayCardOwner(ce.MayPlayPlayer) {
		return keep
	}
	return func(o *state.Object) bool { return o.Owner == p && keep(o) }
}
