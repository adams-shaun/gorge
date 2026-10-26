package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Only this ETB selector has a supported numeric RHS: Y is the total mana
// captured at payment, not the entering permanent's printed mana value.
const mockingbirdETBSelector = "Creature.Other+cmcLEY"
const mockingbirdSpendSVar = "Count$CastTotalManaSpent"
const mockingbirdSpendName = "Y"

// CloneETBSpendSelector reports whether the body's SVar table can supply the
// one resolver-dependent ETB Clone selector this build implements.
func CloneETBSpendSelector(spec string, svars map[string]string) bool {
	return spec == mockingbirdETBSelector && strings.TrimSpace(svars["Y"]) == mockingbirdSpendSVar
}

// CloneETBSelectorMatches is the common announcement/replacement matcher.
// Other selectors keep their original resolver-free context; Mockingbird's
// Y is the mana spent to cast the entering object, and 0 when it was not
// cast.
func CloneETBSelectorMatches(g *state.Game, spec string, id state.ObjID, sc SpecContext) bool {
	if spec == "" {
		spec = "Creature.Other"
	}
	if !strings.Contains(spec, ".") && !strings.HasPrefix(spec, "Card") {
		spec = "Card." + spec
	}
	if spec == mockingbirdETBSelector {
		source := g.Obj(sc.Source)
		if source == nil || source.Face() == nil || !CloneETBSpendSelector(spec, source.Face().SVars) {
			return false
		}
		// Count$CastTotalManaSpent is 0 when the permanent was not cast
		// (Forge AbilityUtils, XMage ManaSpentToCastCount): a reanimated or
		// blinked Mockingbird may still copy a mana-value-0 creature.
		spend := int32(0)
		if source.CastFlags&state.FlagManaSpent != 0 {
			spend = source.ManaSpent
		}
		sc.Resolve = func(name string) (int32, bool) {
			return spend, name == mockingbirdSpendName
		}
	}
	return MatchesSpecCtx(g, spec, id, sc)
}

// CloneETBOffered reports whether a DB$ Clone ETB-replacement body is inside
// the supported as-enters scope, so the cast offers the copy election instead
// of taking the loud "unimplemented API Clone" fallback. The key whitelist and
// the text-only value checks are compiled once (CloneParams.ETBShapeOK); a
// supported KEY is not a supported VALUE: only Mockingbird's cast-spend Y
// selector has a resolver on this route, multi-word AddKeywords$ heads and
// non-True IntoPlayTapped$ values stay withheld, and every named grant
// (AddStaticAbilities$, AddTriggers$, AddSVars$) must resolve in the carrier's
// SVar table so an unreadable grant fails closed before the election.
func CloneETBOffered(sa *cards.SA, svars map[string]string) bool {
	cp := CloneOf(sa)
	if !cp.ETBShapeOK || (SpecNeedsResolver(cp.Choices) && !CloneETBSpendSelector(cp.Choices, svars)) {
		return false
	}
	// A SubAbility$ chain is offered only as Altered Ego's "enters with X
	// additional +1/+1 counters" rider or Superior Spider-Man's reflexive
	// exile (CloneETBSubChainOK).
	if !CloneETBSubChainOK(sa, svars) {
		return false
	}
	for _, name := range cp.StaticNames {
		if !CloneStaticGrantReadable(svars, name) {
			return false
		}
	}
	return CloneNamedGrantsReadable(svars, cp)
}
