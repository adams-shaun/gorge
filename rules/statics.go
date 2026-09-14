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
	return e.costModifiers(p, id, spellScope("")).apply(ParseCost(o.Face().ManaCost))
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

// costScope names WHAT is being priced when cost modifiers are collected:
// a spell cast (kind "Spell", with the cast variant mode naming the
// flashback/surge/kicked/miracle shape a ValidSpell$ may gate on) or an
// activated ability (kind "Ability", with ab the exact SA being activated —
// the thing a ValidSpell$ Activated.<keyword> static scopes to). The zero
// kind is "Spell"; mode "" is the plain cast.
type costScope struct {
	kind string
	mode string
	ab   *cards.SA
}

func spellScope(mode string) costScope    { return costScope{kind: "Spell", mode: mode} }
func abilityScope(ab *cards.SA) costScope { return costScope{kind: "Ability", ab: ab} }

// costMod is ONE evaluated ReduceCost static's contribution to a total cost.
// generic is the literal/SVar-evaluated Amount$; colored carries a Color$
// reduction (per colour slot, set only when the static names one);
// ignoreGeneric records the IgnoreGeneric$ flag (a colour reduction whose
// pip the cost lacks spills to generic only when this is false); floor is
// the MinMana$ lower bound ("can't reduce the mana in that cost to less than
// N mana") evaluated AFTER this reduction alone.
type costMod struct {
	generic       int32
	colored       state.Mana
	hasColor      bool
	ignoreGeneric bool
	floor         int32
}

// costMods is the evaluated composition for one cast/activation, in the CR
// 601.2f order: every increase first, then every reduction in static order
// (each with its own MinMana floor), then a SetCost floor last.
type costMods struct {
	raises   []int32
	reduces  []costMod
	setFloor int32
}

// empty reports whether the composition would change nothing, so a caller can
// keep its old zero-value shorthand.
func (m costMods) empty() bool {
	return len(m.raises) == 0 && len(m.reduces) == 0 && m.setFloor == 0
}

// apply composes c with the modifiers, CR 601.2f: increases before
// reductions, each reduction applied whole (Color$ reductions take the
// matching pips first, spilling the shortfall to generic unless
// IgnoreGeneric$), each reduction's MinMana$ floor restored immediately, and
// the SetCost floor raised to last (Trinisphere: total mana below 3 becomes
// 3). Generic never dips below zero at any point.
func (m costMods) apply(c Cost) Cost {
	for _, r := range m.raises {
		c.Generic = addClampedGeneric(c.Generic, int64(r))
	}
	for _, red := range m.reduces {
		leftover := red.generic
		if red.hasColor {
			for i := range red.colored {
				take := red.colored[i]
				if c.Colored[i] < take {
					take = c.Colored[i]
				}
				c.Colored[i] -= take
				leftover += red.colored[i] - take
			}
		}
		if leftover > 0 && !(red.hasColor && red.ignoreGeneric) {
			c.Generic -= leftover
			if c.Generic < 0 {
				c.Generic = 0
			}
		}
		if red.floor > 0 {
			if total := totalMana(c); total < red.floor {
				c.Generic = addClampedGeneric(c.Generic, int64(red.floor-total))
			}
		}
	}
	if m.setFloor > 0 {
		if total := totalMana(c); total < m.setFloor {
			c.Generic = addClampedGeneric(c.Generic, int64(m.setFloor-total))
		}
	}
	return c
}

// totalMana counts the mana a cost demands, pip by pip — every symbol CR
// 107.4 knows counts one, and numeric tokens count their value. This is the
// total the MinMana$ and SetCost floors are measured against.
func totalMana(c Cost) int32 {
	return c.CMC()
}

// effectZoneOK reports whether a static whose EffectZone$ reads v applies
// while its source sits in zone z. Forge's default is the battlefield, and
// the corpus names zones with Forge's comma-separated All/Battlefield/Stack/
// Graveyard/Hand/Library/Exile/Command words; anything unrecognised denies
// (the same fail-closed direction every unparseable static qualifier takes),
// because wrongly applying a hand-or-library static is exactly the class of
// over-reach the zone gate exists to prevent.
func effectZoneOK(v string, z state.Zone) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return z == state.ZBattlefield
	}
	for _, name := range strings.Split(v, ",") {
		switch strings.TrimSpace(name) {
		case "All":
			return true
		case "Battlefield":
			if z == state.ZBattlefield {
				return true
			}
		case "Stack":
			if z == state.ZStack {
				return true
			}
		case "Graveyard":
			if z == state.ZGraveyard {
				return true
			}
		case "Hand":
			if z == state.ZHand {
				return true
			}
		case "Library":
			if z == state.ZLibrary {
				return true
			}
		case "Exile":
			if z == state.ZExile {
				return true
			}
		case "Command":
			if z == state.ZCommand {
				return true
			}
		}
	}
	return false
}

// costStatics collects the cost-modifier statics of `mode` from every zone a
// static can be live in, gated by each static's own EffectZone$ (Forge's
// default is the battlefield, so a static with no EffectZone$ behaves exactly
// as activeStatics did — a battlefield-only collector). This is what lets a
// card's own reduction apply while it is still in hand: Ghalta, Primal
// Hunger's `EffectZone$ All | ValidCard$ Card.Self` static is live from the
// hand, the library, the command zone and the stack alike. The walk is
// deterministic: AliveFrom(0) seats, a fixed zone order, slice order inside
// each zone, and each face's own Statics order.
func (e *Engine) costStatics(mode string) []staticView {
	var out []staticView
	add := func(o *state.Object, id state.ObjID) {
		f := o.Face()
		if f == nil {
			return
		}
		for _, st := range f.Statics {
			if st.Mode != mode || !effectZoneOK(st.Params["EffectZone"], o.Zone) {
				continue
			}
			out = append(out, staticView{Source: id, Controller: o.Controller, Params: st.Params})
		}
	}
	for _, p := range e.G.AliveFrom(0) {
		for _, z := range []state.Zone{state.ZBattlefield, state.ZStack, state.ZGraveyard,
			state.ZHand, state.ZLibrary, state.ZExile, state.ZCommand} {
			for _, id := range e.G.Zone(z, p) {
				if o := e.G.Obj(id); o != nil {
					add(o, id)
				}
			}
		}
	}
	return out
}

// modAmount evaluates one cost-modifier static's Amount$: a plain literal
// stands as itself; anything else is an SVar name on the source's face or an
// inline Count$ expression, resolved through effects.EvalCount against the
// source and its SVar table. An unresolvable value degrades to ZERO, never
// to 1: the old fallback made Rakdos, Lord of Riots and Herald of War reduce
// by exactly 1 whenever their SVar amount was genuinely 0, and a wrong
// reduction is a wrong cost — 0 ("no reduction") is the honest read of an
// amount the engine cannot evaluate.
func (e *Engine) modAmount(sv staticView) int32 {
	raw := strings.TrimSpace(sv.Params["Amount"])
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if n < 0 {
			return 0
		}
		if n > int64(math.MaxInt32) {
			return math.MaxInt32
		}
		return int32(n)
	}
	o := e.G.Obj(sv.Source)
	if o == nil || o.Face() == nil {
		return 0
	}
	f := o.Face()
	ctx := &effects.Ctx{Source: sv.Source, Controller: sv.Controller, SVars: f.SVars}
	// An SVar NAME resolves through its body on the source's face; anything
	// else is an inline Count$-class expression evaluated as written.
	if body, ok := f.SVars[raw]; ok {
		return effects.EvalCount(e, ctx, body)
	}
	return effects.EvalCount(e, ctx, raw)
}

// costActorMatches is the cost-modifier actor gate: a RaiseCost/ReduceCost
// static with an Activator$ or Caster$ parameter scopes to whose cost it
// modifies. With neither it applies regardless of actor.
func (e *Engine) costActorMatches(sv staticView, actor state.PlayerID) bool {
	if _, ok := sv.Params["Activator"]; ok {
		return e.actorMatches(sv, "Activator", actor)
	}
	if _, ok := sv.Params["Caster"]; ok {
		return e.actorMatches(sv, "Caster", actor)
	}
	return true
}

// costModifiers evaluates and orders the RaiseCost/ReduceCost/SetCost statics
// that apply to a cast/activation of id by p, per CR 601.2f and Forge's
// CostAdjustment. A static applies only when every gate it carries holds:
// Type$ (the other kind is skipped, neither means both), Activator$/Caster$
// (whose action), ValidCard$ (what is being paid for), ValidSpell$ (which
// spell or ability — Auriok Steelshaper's Activated.Equip), AffectedZone$
// for an ability modifier (which zone its source sits in) and IsPresent$
// (an intervening-if, e.g. Trinisphere's untapped self). Amount$ is
// evaluated through the SVar/Count$ machinery, and the modifiers are
// returned in Forge's application order (increases, reductions in static
// order, SetCost floor).
func (e *Engine) costModifiers(p state.PlayerID, id state.ObjID, scope costScope) costMods {
	var mods costMods
	for _, mode := range []string{"RaiseCost", "ReduceCost"} {
		for _, sv := range e.costStatics(mode) {
			if !e.costStaticApplies(sv, mode, p, id, scope) {
				continue
			}
			if mode == "RaiseCost" {
				mods.raises = append(mods.raises, e.modAmount(sv))
				continue
			}
			red := costMod{
				ignoreGeneric: sv.Params["IgnoreGeneric"] == "True",
				floor:         parseAmount(sv.Params["MinMana"], 0),
			}
			if col, ok := sv.Params["Color"]; ok && strings.TrimSpace(col) != "" {
				// Each listed token is reduced by the Amount$: colour letters
				// take their pip from the cost's coloured part, and Forge's
				// "1" token is the generic slot (even_the_score's
				// "Color$ U U U" takes three blue pips; brush_off's
				// "Color$ 1 U" takes one generic and one blue).
				red.hasColor = true
				for _, tok := range strings.Fields(col) {
					if tok == "1" {
						red.generic = addClampedGeneric(red.generic, int64(e.modAmount(sv)))
						continue
					}
					red.colored[state.ManaIndex(tok[0])] = addClampedGeneric(
						red.colored[state.ManaIndex(tok[0])], int64(e.modAmount(sv)))
				}
			} else {
				red.generic = e.modAmount(sv)
			}
			mods.reduces = append(mods.reduces, red)
		}
	}
	for _, sv := range e.costStatics("SetCost") {
		if !e.costStaticApplies(sv, "SetCost", p, id, scope) {
			continue
		}
		if n := e.modAmount(sv); n > mods.setFloor {
			mods.setFloor = n
		}
	}
	return mods
}

// costStaticApplies runs the gate chain one cost-modifier static must pass
// before its Amount$ is evaluated and applied. Every unimplementable
// qualifier denies (never silently over-applies): ValidTarget$ needs the
// chosen targets no offer-time composition has, so a target-conditional
// modifier is skipped; ValidSpell$ shapes this build cannot evaluate fail
// closed; a SetCost without RaiseTo$ True is not the shape this build
// implements.
func (e *Engine) costStaticApplies(sv staticView, mode string, p state.PlayerID, id state.ObjID, scope costScope) bool {
	if ty, ok := sv.Params["Type"]; ok && ty != "" && ty != scope.kind {
		return false
	}
	if !e.costActorMatches(sv, p) {
		return false
	}
	if !effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], id, e.specCtx(sv.Source, p)) {
		return false
	}
	if vs, ok := sv.Params["ValidSpell"]; ok && !e.validSpellMatches(scope, p, id, vs) {
		return false
	}
	if az, ok := sv.Params["AffectedZone"]; ok && scope.kind == "Ability" {
		o := e.G.Obj(id)
		if o == nil {
			return false
		}
		if !affectedZoneOK(az, o.Zone) {
			return false
		}
	}
	if spec, ok := sv.Params["IsPresent"]; ok && !e.isPresent(spec, sv) {
		return false
	}
	if !e.costConditionHolds(sv, p) {
		return false
	}
	if !e.checkSVarHolds(sv) {
		return false
	}
	if _, ok := sv.Params["ValidTarget"]; ok {
		// Target-conditional cost modifiers ("spells that target a creature
		// cost {2} more") need the chosen targets, which no offer-time
		// composition has. Skipping is the conservative direction for a
		// raise (never overcharge) and keeps the reduction twin honest for
		// the same reason. Measured population reported in the audit.
		return false
	}
	if mode == "SetCost" && sv.Params["RaiseTo"] != "True" {
		// Forge's SetCost carries RaiseTo$ True (Trinisphere) for the
		// raise-to-N shape; any other SetCost shape is unimplemented and
		// must not silently floor the cost.
		return false
	}
	if sv.Params["Secondary"] == "True" {
		// Forge's Secondary$ marks a static that duplicates another one's
		// effect under a different wording; applying both would double the
		// modifier (the paired pair of "spells that target ... cost {2} more"
		// lines). Skipping the secondary applies the primary only.
		return false
	}
	if sv.Params["Relative"] == "True" {
		// Relative$ Amount$ scales with the spell's target count (IncreaseCost
		// per target beyond the first) — a per-target shape no offer-time
		// composition knows. Skipping, like ValidTarget$.
		return false
	}
	return true
}

// costConditionHolds evaluates Condition$ on a cost-modifier static. The
// implementable conditions: PlayerTurn / NotPlayerTurn (the caster is or is
// not the active player — discontinuity's "During your turn") and Metalcraft
// (three artifacts on the battlefield, evaluated through the Count$ machinery
// so a replay derives it). An unimplementable condition (Delirium, Night)
// DENIES: a conditional discount that silently always applies is a wrong
// cost, the same fail-closed direction the ValidSpell$ shapes take.
func (e *Engine) costConditionHolds(sv staticView, p state.PlayerID) bool {
	cond, ok := sv.Params["Condition"]
	if !ok {
		return true
	}
	switch strings.TrimSpace(cond) {
	case "PlayerTurn":
		return e.G.Active == p
	case "NotPlayerTurn":
		return e.G.Active != p
	case "Metalcraft":
		o := e.G.Obj(sv.Source)
		if o == nil {
			return false
		}
		ctx := &effects.Ctx{Source: sv.Source, Controller: o.Controller, SVars: o.Face().SVars}
		return effects.EvalCount(e, ctx, "Count$Valid Artifact.YouCtrl") >= 3
	}
	return false
}

// checkSVarHolds evaluates the CheckSVar$/SVarCompare$ intervening-if: the
// named SVar (or inline Count$ expression) is evaluated with the same
// machinery modAmount uses, and compared under SVarCompare$ ("<op><number>",
// e.g. GE4 — the threshold may also be an SVar name, resolved the same way).
// No SVarCompare$ means "nonzero" (Forge's default truthiness read).
func (e *Engine) checkSVarHolds(sv staticView) bool {
	raw, ok := sv.Params["CheckSVar"]
	if !ok {
		return true
	}
	o := e.G.Obj(sv.Source)
	if o == nil || o.Face() == nil {
		return false
	}
	f := o.Face()
	ctx := &effects.Ctx{Source: sv.Source, Controller: sv.Controller, SVars: f.SVars}
	val := int32(0)
	if body, ok := f.SVars[strings.TrimSpace(raw)]; ok {
		val = effects.EvalCount(e, ctx, body)
	} else {
		val = effects.EvalCount(e, ctx, raw)
	}
	cmp := strings.TrimSpace(sv.Params["SVarCompare"])
	if cmp == "" {
		return val != 0
	}
	if len(cmp) < 3 {
		return false
	}
	op, rhs := cmp[:2], cmp[2:]
	threshold := int32(0)
	if n, err := strconv.ParseInt(rhs, 10, 64); err == nil {
		threshold = int32(n)
	} else if body, ok := f.SVars[rhs]; ok {
		threshold = effects.EvalCount(e, ctx, body)
	} else {
		threshold = effects.EvalCount(e, ctx, rhs)
	}
	switch op {
	case "EQ":
		return val == threshold
	case "GE":
		return val >= threshold
	case "GT":
		return val > threshold
	case "LE":
		return val <= threshold
	case "LT":
		return val < threshold
	}
	return false
}

// isPresent evaluates the IsPresent$ intervening-if: an object matching the
// spec exists on any battlefield (Trinisphere's `Card.Self+untapped` matches
// the trinisphere itself while untapped). Resolved against the static's
// source so Self-class predicates bind.
func (e *Engine) isPresent(spec string, sv staticView) bool {
	ctx := e.specCtx(sv.Source, sv.Controller)
	for _, p := range e.G.AliveFrom(0) {
		for _, oid := range e.G.Zone(state.ZBattlefield, p) {
			if effects.MatchesSpecCtx(e.G, spec, oid, ctx) {
				return true
			}
		}
	}
	return false
}

// affectedZoneOK reports whether zone z is named in an AffectedZone$ list
// (Forge's comma-separated zone words; an unrecognised word denies, the same
// direction effectZoneOK takes).
func affectedZoneOK(v string, z state.Zone) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return true
	}
	for _, name := range strings.Split(v, ",") {
		switch strings.TrimSpace(name) {
		case "All":
			return true
		case "Battlefield":
			if z == state.ZBattlefield {
				return true
			}
		case "Graveyard":
			if z == state.ZGraveyard {
				return true
			}
		case "Hand":
			if z == state.ZHand {
				return true
			}
		case "Exile":
			if z == state.ZExile {
				return true
			}
		case "Stack":
			if z == state.ZStack {
				return true
			}
		case "Library":
			if z == state.ZLibrary {
				return true
			}
		case "Command":
			if z == state.ZCommand {
				return true
			}
		}
	}
	return false
}

// validSpellMatches implements the ValidSpell$ parameter: a comma-separated
// OR list of "Kind.Constraint" shapes scoping a cost modifier to WHICH spell
// or ability is being paid for. Kind Spell matches a cast (constraint
// checked against the cast variant and the face); Kind Activated matches an
// activated ability (constraint checked against the ability's own keyword
// tag, its API, or the loyalty-ability classifier); Kind Static (morph-up,
// foretell, unlock — casting options this build does not model) never
// matches, so a static naming one is inert rather than over-applied. A
// constraint this build cannot evaluate denies — a discount that wrongly
// applies is a wrong cost, the same fail-closed direction the ValidSA$
// grammar takes.
func (e *Engine) validSpellMatches(scope costScope, p state.PlayerID, id state.ObjID, spec string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return true
	}
	for _, alt := range strings.Split(spec, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		kind, constraint := alt, ""
		if i := strings.IndexByte(alt, '.'); i >= 0 {
			kind, constraint = alt[:i], alt[i+1:]
		}
		switch kind {
		case "Spell":
			if scope.kind != "Spell" {
				continue
			}
			if e.spellConstraintMatches(scope, id, constraint) {
				return true
			}
		case "Activated":
			if scope.kind != "Ability" || scope.ab == nil {
				continue
			}
			if e.abilityConstraintMatches(scope, p, id, constraint) {
				return true
			}
		}
		// Kind "Static" (and anything else): never matches.
	}
	return false
}

// spellConstraintMatches checks one ValidSpell$ Spell.* constraint against a
// cast. The constraints the engine can evaluate: bare (any spell), the cast
// variant modes the cast flow names, and the card types Instant/Sorcery.
// Everything else — Bargain, Buyback, Blitz, Dash, isCastFaceDown,
// IsTargeting, MayPlaySource — is a casting option or target shape this
// build does not model, and denies.
func (e *Engine) spellConstraintMatches(scope costScope, id state.ObjID, constraint string) bool {
	switch strings.TrimSpace(constraint) {
	case "":
		return true
	case "Flashback":
		return scope.mode == "flashback"
	case "Kicked":
		return scope.mode == "kicked"
	case "Surged":
		return scope.mode == "surged"
	case "Miracle":
		return scope.mode == "miracle"
	case "Instant":
		if o := e.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().IsInstant()
		}
		return false
	case "Sorcery":
		if o := e.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().IsSorcery()
		}
		return false
	}
	return false
}

// abilityConstraintMatches checks one ValidSpell$ Activated.* constraint
// against an activated ability. Keyword-derived constraints (Equip, Ninjutsu,
// Cycling, Boast, Exhaust, ...) match the Keyword$ tag every keyword
// expansion carries (cards/keywords.go); ManaAbility/!ManaAbility read the
// SA API; Loyalty reuses the loyalty-ability classifier; YouCtrl reads the
// ability source's controller. An unevaluable constraint denies.
func (e *Engine) abilityConstraintMatches(scope costScope, p state.PlayerID, id state.ObjID, constraint string) bool {
	ab := scope.ab
	constraint = strings.TrimSpace(constraint)
	switch constraint {
	case "":
		return true
	case "ManaAbility":
		return ab.API == "Mana"
	case "!ManaAbility":
		return ab.API != "Mana"
	case "Loyalty":
		return isLoyaltyAbility(ab)
	case "YouCtrl":
		o := e.G.Obj(id)
		return o != nil && o.Controller == p
	case "YouDontCtrl":
		o := e.G.Obj(id)
		return o != nil && o.Controller != p
	}
	// Keyword-derived: the expansion's Keyword$ tag (comma list).
	for _, kw := range strings.Split(ab.Params["Keyword"], ",") {
		if strings.EqualFold(strings.TrimSpace(kw), constraint) {
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
