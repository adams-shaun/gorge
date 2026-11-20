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

// cloneETBSVarBody reads one SVar name's raw body (CloneStaticGrantReadable's
// dynamic-key shape: the key is the function's string parameter).
func cloneETBSVarBody(svars map[string]string, name string) (string, bool) {
	body, ok := svars[name]
	return strings.TrimSpace(body), ok
}

// DerivedPTSource reads a candidate's layer-derived power and toughness: the
// Host's own Power/Toughness (rules' Engine). A nil source leaves the printed
// read.
type DerivedPTSource interface {
	Power(state.ObjID) int32
	Toughness(state.ObjID) int32
}

// CloneETBSelectorMatches is the common announcement/replacement matcher.
// Other selectors keep their original resolver-free context; Mockingbird's
// Y is the mana spent to cast the entering object, and 0 when it was not
// cast.
//
// pt binds the candidate's layer-derived P/T (CR 613.4) when the selector reads
// one, so "Creature.YouCtrl+powerGE4" sees an equipped or pumped creature's
// power rather than its printed face (Deceptive Frostkite).
func CloneETBSelectorMatches(g *state.Game, spec string, id state.ObjID, sc SpecContext, pt DerivedPTSource) bool {
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
	if pt != nil && SpecReadsPT(spec) {
		sc.DerivedPower, sc.DerivedToughness, sc.HasDerivedPT = pt.Power(id), pt.Toughness(id), true
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
	// The own-P/T exception's SVar-table half: a non-literal SetPower$ /
	// SetToughness$ name (the text half admitted the bare-name shape) is
	// offered only when the carrier's table resolves it to exactly
	// Count$CardPower (power) / Count$CardToughness (toughness) -- Hulking
	// Metamorph's "its power and toughness are equal to CARDNAME's power
	// and toughness", the entering object's own characteristic read before
	// the copy overwrites it (CR 707.9b). Any other body (Living Lore's
	// Remembered$CardManaCost, Thunderbond Vanguard's
	// Count$Valid Creature.YouCtrl) keeps the body withheld.
	for _, own := range [...]struct {
		pt   ParamText
		want string
	}{{cp.SetPower, "Count$CardPower"}, {cp.SetToughness, "Count$CardToughness"}} {
		if _, literal := literalInt(own.pt); !own.pt.Present || literal {
			continue
		}
		body, ok := cloneETBSVarBody(svars, strings.TrimSpace(own.pt.Text))
		if !ok || body != own.want {
			return false
		}
	}
	// A SubAbility$ chain is offered only as Altered Ego's "enters with X
	// additional +1/+1 counters" rider, Superior Spider-Man's reflexive
	// exile, or Undercover Operative's condition-gated shield counter
	// (CloneETBSubChainOK).
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

// CloneForeignEntryOffered reports whether a DB$ Clone ETB-replacement body
// is the implemented foreign-entry shape (CloneParams.ETBForeignShapeOK):
// the replacement sits on one permanent and makes ANOTHER entering object
// enter as a copy (Infinite Reflection, CR 614.1c/707.2). It is never an
// election: the entering object becomes a copy of the carrier's enchanted
// creature through the ordinary Clone resolution, the become operand being
// the replaced card.
func CloneForeignEntryOffered(sa *cards.SA) bool {
	return isCloneSA(sa) && CloneOf(sa).ETBForeignShapeOK
}
