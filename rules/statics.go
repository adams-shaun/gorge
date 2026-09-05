// Static play restrictions and cost modifiers: the six S: modes besides
// stat:Continuous (layers.go's own concern). These change what is legal and
// what things cost rather than what a permanent's characteristics are, so
// they hook into legalActions and ParseCost rather than the layer system.
package rules

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// staticView is one S: line together with where it came from, so the filter
// predicates that are relative to a source (Self, Other) and the ones
// relative to a controller (YouCtrl, OppCtrl) resolve correctly.
type staticView struct {
	Source     state.ObjID
	Controller state.PlayerID
	Params     map[string]string
}

// activeStatics collects every S:Mode$ <mode> line from a permanent on the
// battlefield. The order is deterministic: AliveFrom(0) walks seats in fixed
// APNAP order, each seat's battlefield zone is a slice built by ordinary
// append (never a map), and each object's Statics is the slice order its
// card script parsed in — nothing here ever ranges a map, so cost adjustment
// and the resulting option list are stable run to run, which is what
// TestActiveStaticsIsDeterministicallyOrdered checks for.
func (e *Engine) activeStatics(mode string) []staticView {
	var out []staticView
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			f := o.Face()
			if f == nil {
				continue
			}
			for _, st := range f.Statics {
				if st.Mode == mode {
					out = append(out, staticView{Source: id, Controller: o.Controller, Params: st.Params})
				}
			}
		}
	}
	return out
}

// actorMatches implements the Caster$/Activator$ parameter, which scopes a
// restriction to whose action it is. A restriction with no such parameter
// applies regardless of actor.
func (e *Engine) actorMatches(sv staticView, key string, actor state.PlayerID) bool {
	spec, ok := sv.Params[key]
	if !ok {
		return true
	}
	return effects.MatchesPlayerSpec(e.G, spec, actor, sv.Controller)
}

// specCtx builds the SpecContext a per-source "ValidCard$"/spec match is
// resolved against, with a resolver that answers "Chosen" (the source object's
// ChosenNumber) and any SVar name on the source's face via EvalCount. This is
// what a numeric-RHS restriction actually needs: Sanctum Prelate's
// "cmcEQChosen" (read the number chosen as it entered) and Chalice of the
// Void's "cmcEQY" (Y an SVar over the source's charge counters) both resolve
// through here. Without the resolver those terms would silently never match
// (numericPred's "recognised shape, unresolvable RHS never matches") and the
// restriction would be dead. The resolver closes over source/you -- both
// plain scalars -- so it is deterministic and Clone-safe.
func (e *Engine) specCtx(source state.ObjID, you state.PlayerID) effects.SpecContext {
	return effects.SpecContext{
		You:    you,
		Source: source,
		Resolve: func(name string) (int32, bool) {
			o := e.G.Obj(source)
			if o == nil {
				return 0, false
			}
			if name == "Chosen" {
				return o.ChosenNumber, true
			}
			f := o.Face()
			if f == nil {
				return 0, false
			}
			if body, ok := f.SVars[name]; ok {
				return effects.EvalCount(e, &effects.Ctx{Source: source, Controller: you, SVars: f.SVars}, body), true
			}
			return 0, false
		},
	}
}

// castRestricted reports whether p is forbidden from casting id (CantBeCast).
func (e *Engine) castRestricted(p state.PlayerID, id state.ObjID) bool {
	for _, sv := range e.activeStatics("CantBeCast") {
		if !e.actorMatches(sv, "Caster", p) {
			continue
		}
		if effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], id, e.specCtx(sv.Source, sv.Controller)) {
			return true
		}
	}
	return false
}

// abilityRestricted reports whether id's specific ability ab is forbidden
// from being activated (CantBeActivated), scoped by the restriction's
// ValidSA$ to the ability being considered (Task 10: p, the would-be
// activator, scopes Activator$; ab, the exact activated ability, scopes
// ValidSA$). A nonexistent object has no ability to restrict, so it degrades
// to false rather than dereferencing a nil Object.
func (e *Engine) abilityRestricted(p state.PlayerID, id state.ObjID, ab *cards.SA) bool {
	o := e.G.Obj(id)
	if o == nil {
		return false
	}
	for _, sv := range e.activeStatics("CantBeActivated") {
		if !e.actorMatches(sv, "Activator", p) {
			continue
		}
		if !effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], id, e.specCtx(sv.Source, sv.Controller)) {
			continue
		}
		if activatedMatchesValidSA(ab, sv.Params["ValidSA"]) {
			return true
		}
	}
	return false
}

// activatedMatchesValidSA reports whether a CantBeActivated restriction whose
// ValidSA$ reads validSA applies to the specific activated ability ab. The
// grammar is Forge's comma-separated OR list of "<kind>.<constraint>" values.
//
// An absent ValidSA$ applies to every activated ability (including mana). A
// value whose kind is not "Activated" (a Spell / Instant / Sorcery shape)
// describes a cast, never an activation, so it does not match an ability.
// Within the Activated kind: no constraint matches everything; "!ManaAbility"
// matches everything but a mana ability (ab.API == "Mana"); "ManaAbility"
// matches only a mana ability, and "ManaAbility<Produce:C>" the subset that
// produces colour C (test-only grammar, fix round 1 -- see the case below);
// and a constraint this build cannot evaluate --
// Loyalty, Equip, Crew+Vehicle, hasTapCost, ... -- DENIES, per the "a
// restriction that cannot be evaluated must deny, not silently allow" rule:
// erring toward applying a CantBeActivated is the safe direction, because the
// consequence of wrongly allowing an activation a static forbids is an illegal
// game action, while wrongly blocking one merely withholds an option the
// activator could have taken.
func activatedMatchesValidSA(ab *cards.SA, validSA string) bool {
	v := strings.TrimSpace(validSA)
	if v == "" {
		return true // no ValidSA$: applies to every activated ability
	}
	for _, alt := range strings.Split(v, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		kind, constraint := alt, ""
		if i := strings.IndexByte(alt, '.'); i >= 0 {
			kind, constraint = alt[:i], alt[i+1:]
		}
		if kind != "Activated" {
			continue
		}
		if constraint == "" {
			return true
		}
		switch {
		case constraint == "!ManaAbility":
			if ab.API != "Mana" {
				return true
			}
			// else: mana abilities are expressly spared; try the next alt
		case constraint == "ManaAbility" || strings.HasPrefix(constraint, "ManaAbility<"):
			if ab.API != "Mana" {
				break // not a mana ability; try the next alt
			}
			// Bare ManaAbility matches every mana ability. A
			// ManaAbility<Produce:C> scopes to a mana ability that produces
			// colour C (fix round 1, reviewer Important 2): a permanent with
			// several mana abilities can then have one singled out by a
			// CantBeActivated while the others stay activatable -- the shape
			// that exposed the gate/activation disagreement Test
			// TestActivateSkipsRestrictedManaAbility exercises. The corpus has
			// no such value (grep shows CantBeActivated ValidSA$ is empty,
			// "Activated", or "Activated.!ManaAbility"), so this extension is
			// test-only grammar, but it makes the per-ability agreement
			// reachable instead of hypothetical.
			if constraint != "ManaAbility" && strings.HasSuffix(constraint, ">") {
				inner := constraint[len("ManaAbility<") : len(constraint)-1]
				color := inner
				if j := strings.IndexByte(inner, ':'); j >= 0 {
					color = inner[j+1:]
				}
				produced := strings.TrimSpace(ab.Params["Produced"])
				if produced == "" {
					produced = "C"
				}
				target := strings.TrimSpace(color)
				if target != "" && (produced == target || (len(target) == 1 && strings.Contains(produced, target))) {
					return true
				}
				break // this mana ability produces a different colour; next alt
			}
			return true // bare ManaAbility
		default:
			// Unevaluable constraint under the Activated kind: deny (see doc).
			return true
		}
	}
	return false
}

// adjustedCost applies RaiseCost and ReduceCost to id's printed mana cost.
// Both modes only ever touch the Generic component (never Colored), so
// clamping Generic at zero is already CR 601.2f-safe on its own: a reduction
// can consume the generic requirement down to nothing but can never reach
// into the coloured pips to reduce those, because this function never
// writes to c.Colored at all.
//
// A missing object or a Face()-less one (an ability object or a token
// mid-resolution) degrades to the zero Cost rather than panicking; nothing
// in this build calls adjustedCost with such an id today; the guard exists
// because a caller might one day compute a would-be cost speculatively.
func (e *Engine) adjustedCost(p state.PlayerID, id state.ObjID) Cost {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return Cost{}
	}
	c := ParseCost(o.Face().ManaCost)
	apply := func(mode string, sign int32) {
		for _, sv := range e.activeStatics(mode) {
			if !e.actorMatches(sv, "Activator", p) {
				continue
			}
			if !effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], id, e.specCtx(sv.Source, p)) {
				continue
			}
			c.Generic += sign * parseAmount(sv.Params["Amount"], 1)
		}
	}
	apply("RaiseCost", +1)
	apply("ReduceCost", -1)
	if c.Generic < 0 {
		c.Generic = 0
	}
	return c
}

// alternativeCosts lists extra ways to cast id, each becoming its own
// "cast" option in legalActions so the client can present the choice
// without knowing any rules. Two sources: another permanent's static
// granting the alternative (activeStatics, battlefield-only), and a static
// the card carries on itself, which activeStatics alone would never see
// while the card is still in hand.
//
// p is unused today: the table this task implements gives AlternativeCost
// only ValidCard$/Cost$, no Activator$-style actor scoping. The parameter is
// kept for symmetry with adjustedCost and because Forge does have
// AlternativeCost lines gated by who is casting; adding that scoping later
// is then a one-line change here rather than a signature change at every
// call site.
func (e *Engine) alternativeCosts(p state.PlayerID, id state.ObjID) []Cost {
	var out []Cost
	for _, sv := range e.activeStatics("AlternativeCost") {
		if !effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], id, e.specCtx(sv.Source, sv.Controller)) {
			continue
		}
		out = append(out, ParseCost(sv.Params["Cost"]))
	}
	if o := e.G.Obj(id); o != nil {
		if f := o.Face(); f != nil {
			for _, st := range f.Statics {
				if st.Mode == "AlternativeCost" {
					out = append(out, ParseCost(st.Params["Cost"]))
				}
			}
		}
	}
	return out
}

// blockRestricted reports whether blocker is forbidden from blocking
// attacker (CantBlock, CantBlockBy). Called from rules/combat.go's canBlock,
// which askBlockers and handleBlockers both use for real declare-blockers
// option generation and validation.
func (e *Engine) blockRestricted(blocker, attacker state.ObjID) bool {
	for _, sv := range e.activeStatics("CantBlock") {
		if effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], blocker, e.specCtx(sv.Source, sv.Controller)) {
			return true
		}
	}
	for _, sv := range e.activeStatics("CantBlockBy") {
		if !effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], attacker, e.specCtx(sv.Source, sv.Controller)) {
			continue
		}
		spec, ok := sv.Params["ValidBlocker"]
		if !ok {
			return true
		}
		if effects.MatchesSpecCtx(e.G, spec, blocker, e.specCtx(sv.Source, sv.Controller)) {
			return true
		}
	}
	return false
}

// parseAmount reads an Amount$ parameter, falling back to def for anything
// that is not a plain non-negative int32 (missing, empty, negative, out of
// int32 range, or a malformed value from a card script this build cannot
// otherwise validate).
//
// Ruling T19b-c: this used to cast straight to int32 with no range check at
// all, unlike mana.go's ParseCost (which explicitly checks 0 <= n <=
// math.MaxInt32 before ever converting). An out-of-range Amount$ silently
// wrapped into a negative int32 -- inverting a RaiseCost into a discount and
// a ReduceCost into a tax -- and a plain negative Amount$ was accepted as-is,
// turning a ReduceCost into a raise (adjustedCost's sign is fixed by the
// mode, so a negative n flips the intended direction rather than reducing
// the magnitude). Both are reachable from card data with no need for the
// value to be anywhere near a real int32 overflow at the mana-cost level:
// the bug is entirely in this parse, not in anything cost-shaped.
func parseAmount(s string, def int32) int32 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || v < 0 || v > int64(math.MaxInt32) {
		return def
	}
	return int32(v)
}

func init() {
	effects.RegisterNonAPI("stat:CantBeCast", "stat:CantBeActivated", "stat:RaiseCost",
		"stat:ReduceCost", "stat:AlternativeCost", "stat:CantBlock", "stat:CantBlockBy",
		"stat:Continuous")
}

// altCostLabel names the nth (0-indexed) alternative-cost option for a
// spell, distinct from the base "Cast <name>" label and from each other when
// a card somehow offers more than one alternative.
func altCostLabel(name string, i int) string {
	if i == 0 {
		return "Cast " + name + " (alternative cost)"
	}
	return fmt.Sprintf("Cast %s (alternative cost %d)", name, i+1)
}
