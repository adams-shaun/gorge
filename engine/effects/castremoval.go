package effects

import (
	"strconv"
	"strings"

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

// CastRemovalTargets is the coarse target scope of the card's primary
// removal ability, read off its compiled ValidTgts$ alternatives' base
// tokens: "creature" when they name creatures and no players, "player" when
// they name players and no creatures, "any" when they name both, and "" when
// they name neither (Shatter's artifact filter, Stone Rain's land filter, a
// graveyard move). It is meant to be read together with CastRemovalKind:
// an ability whose kind is empty -- not a held removal shape at all, like a
// reanimation spell's ChangeZone that names creatures -- reports "", so the
// scope is only ever carried next to a kind and can never make an unheld
// spell look held-eligible. The classifier reads only the base tokens; a
// qualifier rider (.YouOwn, .ControlledBy$) narrows the legal set and never
// widens it, so a scope is an upper bound on what the spell may legally
// target -- exactly what a hold rule needs, never a legality guarantee.
func CastRemovalTargets(sa *cards.SA) string {
	if CastRemovalKind(sa) == "" {
		return ""
	}
	spec := TargetsOf(sa).ValidTgts
	if spec == "" {
		return ""
	}
	creatures, players := false, false
	for alt := range strings.SplitSeq(spec, ",") {
		base, _, _ := strings.Cut(strings.TrimSpace(alt), ".")
		switch strings.ToLower(base) {
		case "creature":
			creatures = true
		case "any":
			creatures, players = true, true
		case "player", "opponent":
			players = true
		}
	}
	switch {
	case creatures && players:
		return "any"
	case creatures:
		return "creature"
	case players:
		return "player"
	}
	return ""
}

// CastRemovalDamage is the literal NumDmg$ of the card's primary removal
// ability when it is a plain positive integer, 0 when it is absent, dynamic
// (NumDmg$ X, an SVar reference) or not positive, and 0 for an ability whose
// CastRemovalKind is empty -- the scope's own pairing rule, so the damage is
// only ever carried next to a held shape. A 0 answers "unknown", never "no
// damage": the hold rule treats an unknown figure as no face candidate at
// all (the ft1 contract's standing rule, an unknown must never be treated as
// lethal). The negative clamp mirrors DealDamage's own
// (engine/effects/damage_deal.go): a negative NumDmg deals 0, so it is no
// face candidate either. The read goes through DamageAmount -- NumDmg$'s one
// reader (params.DamageAmount, DealDamage's compiled one) -- so this view
// fact reads exactly what the resolution deals.
func CastRemovalDamage(sa *cards.SA) int32 {
	if CastRemovalKind(sa) == "" {
		return 0
	}
	amt := DamageAmount(sa)
	if !amt.Present {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(amt.Text))
	if err != nil || n <= 0 {
		return 0
	}
	return int32(n)
}
