// Static play restrictions and cost modifiers: the six S: modes besides
// stat:Continuous (layers.go's own concern). These change what is legal and
// what things cost rather than what a permanent's characteristics are, so
// they hook into legalActions and ParseCost rather than the layer system.
package rules

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
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

// costStaticViews is one ordered snapshot of cost-modifier membership. The
// three slices preserve each mode's independent application order while the
// collector walks the game's zones only once. It contains no evaluated
// applicability, amount, target, X, condition, or final cost.
type costStaticViews struct {
	raise  []staticView
	reduce []staticView
	set    []staticView
}

// costStaticSource lazily owns one call-scoped membership snapshot. It is
// deliberately not stored on Engine: a legal-actions pass may reuse it, but
// a later pass or payment-side recomputation must observe the current board,
// including test fixtures that mutate setup without emitting events.
type costStaticSource struct {
	e     *Engine
	views costStaticViews
	ready bool
}

func (s *costStaticSource) get() costStaticViews {
	if !s.ready {
		s.views = s.e.collectCostStatics()
		s.ready = true
	}
	return s.views
}

// actionStaticViews contains ordered membership only, never evaluated
// restrictions, grants or affordability. A legalActions pass owns its source
// locally; later offers and payment/activation callers collect afresh.
type actionStaticViews struct {
	cantCast     []staticView
	cantActivate []staticView
	continuous   []staticView
}

type actionStaticSource struct {
	e     *Engine
	views actionStaticViews
	ready bool
}

func (s *actionStaticSource) get() actionStaticViews {
	if !s.ready {
		s.views = s.e.collectActionStatics()
		s.ready = true
	}
	return s.views
}

// collectActionStatics mirrors activeStatics' active-face-only battlefield
// walk. In particular it must not inherit staticEffects' alternate Room face
// expansion or collectCostStatics' other zones. Each mode keeps its original
// seat, zone and parsed-static order while sharing a single membership walk.
func (e *Engine) collectActionStatics() actionStaticViews {
	var out actionStaticViews
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			for _, st := range o.Face().Statics {
				var dst *[]staticView
				switch st.Mode {
				case "CantBeCast":
					dst = &out.cantCast
				case "CantBeActivated":
					dst = &out.cantActivate
				case "Continuous":
					dst = &out.continuous
				default:
					continue
				}
				*dst = append(*dst, staticView{Source: id, Controller: o.Controller, Params: st.Params})
			}
		}
	}
	return out
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
			if e.faceDownPrintedHides(o) {
				// CR 708.8: a face-down permanent's printed statics do not
				// exist while it is face down (the shared gate in layers.go).
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
	var predicates *effects.PredicatePrograms
	if e.compiledText != nil {
		predicates = e.compiledText.predicates
	}
	return effects.SpecContext{
		You:               you,
		Source:            source,
		PredicatePrograms: predicates,
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
	return e.castRestrictedUsing(e.activeStatics("CantBeCast"), p, id)
}

func (e *Engine) castRestrictedUsing(statics []staticView, p state.PlayerID, id state.ObjID) bool {
	for _, sv := range e.castRestrictionSources(statics, id) {
		if !e.actorMatches(sv, "Caster", p) {
			continue
		}
		if !e.restrictionGateHolds(sv, id) || !e.checkSVarHolds(sv) {
			continue
		}
		if effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], id, e.specCtx(sv.Source, sv.Controller)) {
			return true
		}
	}
	return false
}

// castRestrictionSources merges the battlefield restriction statics with the
// target card's OWN CantBeCast statics -- a self-restriction (Rakdos, Lord of
// Riots' "You can't cast this spell unless an opponent lost life this turn")
// is carried on the restricted card itself, so it must be live wherever that
// card sits, gated by each static's EffectZone$ (Forge's default is the
// battlefield, so the same gate collectCostStatics uses decides whether a
// hand/library/stack source is live). Without the self-merge a hand-zone
// lockout is invisible: activeStatics walks the battlefield only, and a
// battlefield-only restriction can never reach the card it restricts.
func (e *Engine) castRestrictionSources(statics []staticView, id state.ObjID) []staticView {
	out := statics
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return out
	}
	for _, st := range o.Face().Statics {
		if st.Mode != "CantBeCast" || !effectZoneOK(st.Params["EffectZone"], o.Zone) {
			continue
		}
		out = append(out, staticView{Source: id, Controller: o.Controller, Params: st.Params})
	}
	return out
}

// restrictionGateHolds evaluates the non-matching gates one CantBeCast /
// CantBeActivated restriction must pass before its ValidCard$/ValidSA$ match
// is even consulted: AffectedZone$ (the zone the restricted card must sit
// in -- Linvala's and Karn's Battlefield, Ashes of the Abhorrent's Graveyard)
// and Condition$ (PlayerTurn / NotPlayerTurn, resolved against the SOURCE's
// controller: Grand Abolisher's "During your turn" is the abolisher
// controller's turn, never the restricted caster's, which is why the shared
// costConditionHolds -- keyed to the payer -- must not be reused here). A
// condition this build cannot evaluate fails closed: a lockout that silently
// always applies over-restricts, but one that silently never applies lets an
// illegal action through, and the static family's whole point is the
// prohibition.
func (e *Engine) restrictionGateHolds(sv staticView, target state.ObjID) bool {
	if az, ok := sv.Params["AffectedZone"]; ok {
		o := e.G.Obj(target)
		if o == nil || !affectedZoneOK(az, o.Zone) {
			return false
		}
	}
	switch strings.TrimSpace(sv.Params["Condition"]) {
	case "":
		return true
	case "PlayerTurn":
		return e.G.Active == sv.Controller
	case "NotPlayerTurn":
		return e.G.Active != sv.Controller
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
	return e.abilityRestrictedUsing(e.activeStatics("CantBeActivated"), p, id, ab)
}

func (e *Engine) abilityRestrictedUsing(statics []staticView, p state.PlayerID, id state.ObjID, ab *cards.SA) bool {
	o := e.G.Obj(id)
	if o == nil {
		return false
	}
	for _, sv := range statics {
		if !e.actorMatches(sv, "Activator", p) {
			continue
		}
		if !e.restrictionGateHolds(sv, id) || !e.checkSVarHolds(sv) {
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
			if !isManaAbilityAPI(ab.API) {
				return true
			}
			// else: mana abilities are expressly spared; try the next alt
		case constraint == "ManaAbility" || strings.HasPrefix(constraint, "ManaAbility<"):
			if !isManaAbilityAPI(ab.API) {
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

// adjustedCost applies the full RaiseCost/ReduceCost/SetCost composition to
// id's printed mana cost. RaiseCost Cost$ may add coloured pips (and fixed
// life), while ReduceCost Color$ may remove matching coloured pips before its
// remaining generic reduction and MinMana$ floor. costMods.apply owns that
// CR 601.2f order and clamps only the generic component at zero; it never
// lets a generic reduction consume a coloured pip.
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
	return e.costModifiers(p, id, spellScope("")).apply(e.parseCost(o.Face().ManaCost))
}

// castWithFlash reports whether an active CastWithFlash static gives p
// permission to cast id at instant speed. It is intentionally shared by every
// zone that can cast a spell; a Vedalken Orrery must not stop working when a
// later alternative permits casting from another zone.
func (e *Engine) castWithFlash(p state.PlayerID, id state.ObjID) bool {
	for _, sv := range e.activeStatics("CastWithFlash") {
		if !e.actorMatches(sv, "Caster", p) || !e.staticTimingGate(sv) {
			continue
		}
		// EffectZone$ (Skittering Cicada's EffectZone$ Battlefield): the
		// static functions only while its source sits in the named zone --
		// a battlefield static's permission ends with the source's presence,
		// exactly like every other zone-scoped static.
		if az, ok := sv.Params["EffectZone"]; ok {
			src := e.G.Obj(sv.Source)
			if src == nil || !affectedZoneOK(az, src.Zone) {
				continue
			}
		}
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || !spellMatchesValidSA(o.Face(), sv.Params["ValidSA"], id, sv.Source) {
			continue
		}
		if effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], id, e.specCtx(sv.Source, sv.Controller)) {
			return true
		}
	}
	return false
}

// presentGate evaluates one IsPresent spec against PresentCompare (default
// GE1); staticTimingGate fails closed when either present gate does not hold.
func (e *Engine) presentGate(sv staticView, spec string) bool {
	n := e.countStaticPresent(sv, spec)
	cmp := sv.Params["PresentCompare"]
	if cmp == "" {
		cmp = "GE1"
	}
	return comparePresent(n, cmp)
}

// staticTimingGate evaluates the static conditions that can decide whether a
// CastWithFlash permission exists before a spell is announced. An unknown
// gate fails closed: granting instant timing without proving the script's
// condition would permit an illegal cast.
func (e *Engine) staticTimingGate(sv staticView) bool {
	if spec, ok := sv.Params["IsPresent"]; ok && !e.presentGate(sv, spec) {
		return false
	}
	if spec, ok := sv.Params["IsPresent2"]; ok && !e.presentGate(sv, spec) {
		return false
	}
	if name, ok := sv.Params["CheckSVar"]; ok {
		o := e.G.Obj(sv.Source)
		if o == nil || o.Face() == nil {
			return false
		}
		body, ok := o.Face().SVars[name]
		if !ok {
			return false
		}
		cmp := sv.Params["SVarCompare"]
		if cmp == "" || !comparePresent(int(effects.EvalCount(e, &effects.Ctx{Source: sv.Source, Controller: sv.Controller, SVars: o.Face().SVars}, body)), cmp) {
			return false
		}
	}
	switch strings.TrimSpace(sv.Params["Condition"]) {
	case "", "PlayerTurn":
		if sv.Params["Condition"] == "PlayerTurn" && e.G.Active != sv.Controller {
			return false
		}
	case "Ferocious":
		found := false
		for _, id := range e.G.Zone(state.ZBattlefield, sv.Controller) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().IsCreature() && !o.BestowedAttached() && e.Derived(id).Power >= 4 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	default:
		return false
	}
	if phase := strings.TrimSpace(sv.Params["Phases"]); phase != "" && !(strings.Contains(phase, "End of Turn") && e.G.Step == state.StepEnd) {
		return false
	}
	if turn := strings.TrimSpace(sv.Params["PlayerTurn"]); turn != "" {
		switch turn {
		case "Opponent":
			if e.G.Active == sv.Controller {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (e *Engine) countStaticPresent(sv staticView, spec string) int {
	zone := strings.TrimSpace(sv.Params["PresentZone"])
	if zone == "" || zone == "Battlefield" {
		return e.countPresent(spec, sv.Source, sv.Controller)
	}
	var want state.Zone
	switch zone {
	case "Graveyard":
		want = state.ZGraveyard
	default:
		return 0
	}
	n := 0
	e.forEachObject(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o != nil && o.Zone == want && effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(sv.Source, sv.Controller)) {
			n++
		}
	})
	return n
}

// spellMatchesValidSA checks the spell-side subset of Forge's ValidSA grammar.
// Activated-only or target/X-dependent constraints are not knowable before
// announcing a spell and therefore do not accidentally grant flash timing.
// id is the card the cast offers and staticSource the static's source: the
// "Spell.Self" form (115 corpus lines, all on self-granting AlternativeCost
// statics, Daze the most-played) means the affected card itself is the spell
// -- true exactly when the cast card IS the static's source (the card's own
// S: line, where alternativeCosts builds the view with source == id), false
// for a grant from another permanent. Constraint values beyond Self
// (XCostLE3, Teamwork, IsTargeting...) are unimplemented shapes and fail
// closed.
func spellMatchesValidSA(f *cards.Face, raw string, id, staticSource state.ObjID) bool {
	if strings.TrimSpace(raw) == "" {
		return true
	}
	for _, alt := range strings.Split(raw, ",") {
		kind, constraint, _ := strings.Cut(strings.TrimSpace(alt), ".")
		switch kind {
		case "Spell":
			if constraint == "" {
				return true
			}
			if constraint == "Self" && id == staticSource {
				return true
			}
		case "Instant":
			if constraint == "" && f.IsInstant() {
				return true
			}
		case "Sorcery":
			if constraint == "" && f.IsSorcery() {
				return true
			}
		}
	}
	return false
}

// spellTimingOK is the one timing predicate for every zone which offers a
// spell cast. CastWithFlash is a permission, not a hand-only property: it
// also applies to Flashback, Harmonize, and command-zone casts.
func (e *Engine) spellTimingOK(p state.PlayerID, id state.ObjID, f *cards.Face, sorcery bool) bool {
	return sorcery || (f != nil && (f.IsInstant() || e.HasKeyword(id, "Flash") || e.castWithFlash(p, id)))
}

// altCostView is one alternative-cost entry: the parsed cost plus the
// granting static's own riders the cast flow needs (the Announce$ X value —
// an alternative cost whose X the caster announces at CR 601.2b before the
// exile filter's cmcEQX resolves, the Shoal cycle) and the static's source,
// whose face SVar table resolves the announced value's SVar.
type altCostView struct {
	cost     Cost
	announce string
	src      state.ObjID
}

// alternativeCosts lists extra ways to cast id, each becoming its own
// "cast" option in legalActions so the client can present the choice
// without knowing any rules. Two sources: another permanent's static
// granting the alternative (activeStatics, battlefield-only), and a static
// the card carries on itself, which activeStatics alone would never see
// while the card is still in hand.
//
// p is the casting player: the ValidPlayer$ rider on an AlternativeCost
// static scopes WHO may take the alternative (Deadly Rollick/Deflecting
// Swat's "ValidPlayer$ You"), evaluated against the static's own controller
// so a grant from another permanent's static resolves You/Opponent relative
// to the granter, exactly like every other static filter predicate.
func (e *Engine) alternativeCosts(p state.PlayerID, id state.ObjID) []altCostView {
	var out []altCostView
	for _, sv := range e.activeStatics("AlternativeCost") {
		if !effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], id, e.specCtx(sv.Source, sv.Controller)) {
			continue
		}
		if !e.alternativeCostScopeOK(sv.Params, id, sv.Source, p, sv.Controller) {
			continue
		}
		out = append(out, altCostView{cost: e.parseCost(sv.Params["Cost"]),
			announce: strings.TrimSpace(sv.Params["Announce"]), src: sv.Source})
	}
	if o := e.G.Obj(id); o != nil {
		if f := o.Face(); f != nil {
			for _, st := range f.Statics {
				if st.Mode != "AlternativeCost" {
					continue
				}
				if !e.alternativeCostScopeOK(st.Params, id, id, p, o.Controller) {
					continue
				}
				out = append(out, altCostView{cost: e.parseCost(st.Params["Cost"]),
					announce: strings.TrimSpace(st.Params["Announce"]), src: id})
			}
		}
	}
	// A MayPlay static's MayPlayAltManaCost$ (Darksteel Monolith) is the same
	// "pay THIS instead of the mana cost" shape delivered by the may-play
	// family; the family root carries its own gates and limit.
	for _, c := range e.mayPlayAltCosts(p, id) {
		out = append(out, altCostView{cost: c})
	}
	return out
}

// altCostXCandidates returns the ASCENDING distinct mana values at which at
// least one exilable card in the caster's hand still matches the view's
// Exile parts — the candidate set the Announce$ X ask offers (Blazing/Disrupting
// Shoal's "exile a red/blue card with mana value X"): the spec's cmcEQX
// predicate is bound to each candidate value through SpecContext.Resolve,
// the same closure mechanism a Chosen* predicate resolves through, so the
// filter never sees an unannounced X. An empty hand or no matching card at
// any value yields an empty set (the offer gate and the xAsk arm both
// withhold on that).
func (e *Engine) altCostXCandidates(p state.PlayerID, id state.ObjID, alt altCostView) []int32 {
	vals := []int32{}
	seen := map[int32]bool{}
	for _, part := range alt.cost.Exile {
		zone := part.Zone
		if zone == 0 {
			zone = state.ZHand
		}
		for _, oid := range e.G.Zone(zone, p) {
			o := e.G.Obj(oid)
			if o == nil || o.Face() == nil {
				continue
			}
			v := o.Face().ManaValue()
			if seen[v] {
				continue
			}
			sc := effects.SpecContext{You: p, Source: alt.src, Resolve: func(name string) (int32, bool) {
				if name == alt.announce {
					return v, true
				}
				return 0, false
			}}
			if effects.MatchesSpecCtx(e.G, part.Spec, oid, sc) {
				seen[v] = true
				vals = append(vals, v)
			}
		}
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	return vals
}

// alternativeCostScopeOK reads an AlternativeCost static's scope riders:
// ValidSA$ (which cast the alternative prices — Daze's, the Force cycle's and
// the Flare cycle's "Spell.Self", the commander free-cast's bare "Spell") and
// EffectZone$ (the zone the static's source must sit in — the self-carried
// free-cast statics name "All" so the grant reaches the hand), ValidPlayer$
// (the casting player, relative to the static's controller — Deadly
// Rollick/Deflecting Swat) and IsPresent$ (an existence precondition over
// the battlefield, the shared presentGate with PresentCompare$ defaulting to
// GE1 — the commander-protection cycle). An absent rider is vacuously true;
// a ValidSA$ value whose Spell constraint this build cannot evaluate denies,
// the same fail-closed direction ValidSpell$ takes — a wrongly-granted free
// cast is an illegal game action, a wrongly-withheld one merely an option
// lost.
func (e *Engine) alternativeCostScopeOK(params map[string]string, id, srcID state.ObjID, caster, controller state.PlayerID) bool {
	if vp := strings.TrimSpace(params["ValidPlayer"]); vp != "" && !effects.MatchesPlayerSpec(e.G, vp, caster, controller) {
		return false
	}
	if ip := strings.TrimSpace(params["IsPresent"]); ip != "" {
		view := staticView{Source: srcID, Controller: controller, Params: params}
		if !e.presentGate(view, ip) {
			return false
		}
	}
	if vs := strings.TrimSpace(params["ValidSA"]); vs != "" {
		ok := false
		for _, alt := range strings.Split(vs, ",") {
			alt = strings.TrimSpace(alt)
			kind, constraint := alt, ""
			if i := strings.IndexByte(alt, '.'); i >= 0 {
				kind, constraint = alt[:i], alt[i+1:]
			}
			if kind != "Spell" {
				// An Activated/Static kind scopes an ability or an unmodelled
				// casting option; this list prices a spell cast only.
				continue
			}
			switch strings.TrimSpace(constraint) {
			case "":
				ok = true // bare Spell: any cast
			case "Self":
				if id == srcID {
					ok = true // the card's own cast (Daze, the Flares)
				}
			}
			// An unevaluable Spell constraint (Spell.Samurai, ...): deny — a
			// free cast wrongly granted is an illegal action.
		}
		if !ok {
			return false
		}
	}
	if ez := strings.TrimSpace(params["EffectZone"]); ez != "" {
		if src := e.G.Obj(srcID); src != nil && !effectZoneOK(ez, src.Zone) {
			return false
		}
	}
	// CheckSVar$ / CheckSecondSVar$ (Mogg Salvage's two-condition
	// alternative cost): Forge's StaticAbility.checkConditions — the value
	// of each named SVar must satisfy its compare, the DEFAULT being GE1 for
	// both (X = Count$Valid Island.OppCtrl, Y = Count$Valid
	// Mountain.YouCtrl: opponent controls an Island AND you control a
	// Mountain). The gate evaluates against the static's SOURCE face's SVar
	// table, the same precedence sVarGateOK applies to an ability's own
	// CheckSVar$. An unresolvable body fails OPEN — the documented
	// conditionMet convention — so a gate this build cannot evaluate never
	// withholds the alternative by itself.
	ctx := &effects.Ctx{Source: srcID, Controller: controller}
	if o := e.G.Obj(srcID); o != nil && o.Face() != nil {
		ctx.SVars = o.Face().SVars
	}
	if ck := strings.TrimSpace(params["CheckSVar"]); ck != "" {
		if holds, evaluated := effects.CheckSVarHolds(e, ctx, ck, params["SVarCompare"]); evaluated && !holds {
			return false
		}
	}
	if ck := strings.TrimSpace(params["CheckSecondSVar"]); ck != "" {
		if holds, evaluated := effects.CheckSVarHolds(e, ctx, ck, params["SecondSVarCompare"]); evaluated && !holds {
			return false
		}
	}
	return true
}

// onlyFirstSpellUsed reports whether a ReduceCost static carrying
// OnlyFirstSpell$ (Conduit of Ruin: "The first creature spell you cast each
// turn costs {2} less") has already spent its this-turn application: a covered
// spell was cast by the payer earlier in the turn. The answer is a log walk
// over PutOnStack events back to the last TurnChange (cast.go's
// spellsCastThisTurn derivation), so a replay agrees by construction, and it
// counts the ValidCard$-covered casts -- the oracle's "first creature spell"
// is first among the covered kind, not first among all spells.
//
// Every cost-static evaluation site runs BEFORE the cast's own PutOnStack
// exists (the offer walk, beginCast's modifier snapshot, and the
// target-announcement recompute all precede pushCast -- CR 601.2f's
// modifiers are computed into pc.mods and manaToPay only reads them), so the
// in-flight cast is never in the log while it is being priced. The ev.Obj ==
// id exclusion is defensive against a future evaluation site past the push:
// the cast being priced must not count as its own "previous cast".
//
// A cast whose object no longer carries a face (or whose face the spec cannot
// re-evaluate) is not counted -- the missing-match direction for a USED
// tracking, which widens the discount by at most one cast on a board this
// build cannot reconstruct, never withholds it.
func (e *Engine) onlyFirstSpellUsed(sv staticView, p state.PlayerID, id state.ObjID) bool {
	spec := strings.TrimSpace(sv.Params["ValidCard"])
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind != events.PutOnStack || ev.Player != p || ev.Obj == id {
			continue
		}
		if spec == "" {
			return true
		}
		if o := e.G.Obj(ev.Obj); o != nil && o.Face() != nil &&
			effects.MatchesSpecCtx(e.G, spec, ev.Obj, e.specCtx(sv.Source, sv.Controller)) {
			return true
		}
	}
	return false
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
		// ValidAttacker$ is Forge's own spelling for the attacker side of a
		// CantBlockBy static (Steel Leaf Champion's "Creature.Self", the
		// Unblockable pump templates' "Card.IsRemembered", the blocker-side
		// "CARDNAME can block only creatures with flying" shape) — 594 corpus
		// files carry it and NONE of them spell the attacker with ValidCard$;
		// that spelling is the hand-authored test fixture's. The historical
		// ValidCard$ read stays as the fallback so both grammars work, and an
		// SA carrying neither fails closed exactly as before (the empty spec
		// matches nothing).
		attackerSpec := sv.Params["ValidAttacker"]
		if attackerSpec == "" {
			attackerSpec = sv.Params["ValidCard"]
		}
		if !effects.MatchesSpecCtx(e.G, attackerSpec, attacker, e.specCtx(sv.Source, sv.Controller)) {
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
	raises []int32
	// raiseCol / raiseLife carry the aggregated RaiseCost Cost$ raises: the
	// coloured pips, generic and life a "Black spells you cast cost {B} more"
	// static adds to the total (an additional cost — never reduced by the
	// reductions that follow, per CR 601.2f's increase-then-reduce order).
	raiseCol  state.Mana
	raiseGen  int32
	raiseLife int32
	reduces   []costMod
	setFloor  int32
}

// empty reports whether the composition would change nothing, so a caller can
// keep its old zero-value shorthand.
func (m costMods) empty() bool {
	return len(m.raises) == 0 && m.raiseGen == 0 && m.raiseLife == 0 &&
		m.raiseCol.Total() == 0 && len(m.reduces) == 0 && m.setFloor == 0
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
	for i := range m.raiseCol {
		c.Colored[i] = addClampedGeneric(c.Colored[i], int64(m.raiseCol[i]))
	}
	c.Generic = addClampedGeneric(c.Generic, int64(m.raiseGen))
	c.Life = addClampedGeneric(c.Life, int64(m.raiseLife))
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

// hasFloor reports whether the composition carries any MinMana$ or SetCost
// floor. A floor raises the cost's total mana (Cost.CMC) to a minimum, and
// CR 202.4b prices a monocolour-hybrid pip at its generic face — the highest
// of its faces — so an unresolved pip overprices every cheaper face the
// announcement may resolve it to. That is the one part of the composition
// whose result depends on the not-yet-announced pip faces; raises and
// reductions are face-independent, so only a composition with a floor needs
// the per-face enumeration feasibleAny performs.
func (m costMods) hasFloor() bool {
	if m.setFloor > 0 {
		return true
	}
	for _, red := range m.reduces {
		if red.floor > 0 || red.hasColor {
			return true
		}
	}
	return false
}

// feasibleAny is THE one shared mana-feasibility primitive of the cast flow.
// It answers the CR 601.2b/601.2f question for a cost whose flexible pips may
// still be unresolved — at the offer gate (offerCastable), at each CR 601.2b
// announcement menu (announceFeasible, with the pips already announced folded
// in as their final resolved faces) and at the target-repricing gates (a cost
// with no live pip, where it degenerates to the composed payable check): is
// there SOME legal face assignment of the remaining announcement pips
// (two-colour hybrid, monocolour hybrid, Phyrexian, hybrid-Phyrexian) that,
// with m composed onto the RESOLVED faces at the leaf of the walk and
// taxGeneric added after (an additional cost is never reduced), is payable
// from pool/snow/life? Because the offer gate, every announcement menu and
// the charge (manaToPay/payMana) compose the same modifiers through this one
// primitive, an offered cast, an offered announcement face and the charged
// total can never disagree.
// delve is the payer's Delve graveyard credit pool (0 without Delve): the
// credit is taken off each assignment's generic after the composition and
// tax — the same place the payment (payCast) takes it off manaToPay.
//
// A cost with no announcement pip, or a composition with no face-sensitive
// modifier, composes identically for every face and degrades to the single
// composed payable check. Without the enumeration an offer priced under a
// floor with an unresolved twobrid pip could be offered although NEITHER of
// its announced faces completes the raised cost — Trinisphere over an
// unresolved {2/W} composes {2/W}+{1} (payable from {W}{W} by its white face
// plus one generic), while the white face announces to {W}+{2} and the
// generic face to {3}, neither payable from {W}{W}. Color$ needs the same
// ordering: a W reduction must see an announced W half of {W/U}. The walk
// resolves one pip per level in announcePip order and stops at the first
// payable assignment, so a payable cost is found without visiting the whole
// tree.
func (m costMods) feasibleAny(c Cost, pool, snow state.Mana, life, taxGeneric, delve int32, bLifeOK bool, rider pipRider, conv *manaConv) bool {
	composed := func(c Cost) bool {
		cc := m.apply(c)
		cc.Generic = addClampedGeneric(cc.Generic, int64(taxGeneric))
		if cc.Generic > delve {
			cc.Generic -= delve
		} else {
			cc.Generic = 0
		}
		_, ok := cc.resolveManaWith(pool, snow, life, bLifeOK, rider, conv)
		return ok
	}
	if !m.hasFloor() || c.annPipCount() == 0 {
		return composed(c)
	}
	var walk func(c Cost) bool
	walk = func(c Cost) bool {
		if c.annPipCount() == 0 {
			return composed(c)
		}
		for _, alt := range c.announcePip(0) {
			r := c
			switch {
			case alt.color != 0:
				r.Colored[state.ManaIndex(alt.color)]++
			case alt.generic > 0:
				r.Generic = addClampedGeneric(r.Generic, int64(alt.generic))
			case alt.life > 0:
				r.Life = addClampedGeneric(r.Life, int64(alt.life))
			}
			if walk(r.dropAnnouncePrefix(1)) {
				return true
			}
		}
		return false
	}
	return walk(c)
}

// manaFeasible is the engine-facing form of the shared primitive: it reads
// the payer's restriction-aware pool (manaAvailableFor: RestrictValid$ mana
// is invisible to a payment its restriction does not admit), snow tally and
// life, the payer's stat:ManaConvert conversion set (paymentConv), and hands
// them to costMods.feasibleAny. Every mana-feasibility gate of the cast flow
// goes through it — the offer gate (offerCastable), each CR 601.2b
// announcement menu (announceFeasible, over the partially announced cost) and
// the target-repricing gates — so no site re-derives its own "payable so far"
// answer. With no RestrictValid$ batch and no ManaConvert static on the
// battlefield this is exactly the plain-pool payable check it was before the
// mana-shaping primitives landed (paymentConv returns nil, manaAvailableFor
// returns Pool verbatim), so every pre-existing game resolves byte-identically.
func (e *Engine) manaFeasible(p state.PlayerID, id state.ObjID, ability bool, c Cost, mods costMods, taxGeneric, delve int32) bool {
	return e.manaFeasibleGrant(p, id, ability, c, mods, taxGeneric, delve,
		pipRider{anyColor: e.payerGrantsIgnoreColor(p, id), anyType: e.payerGrantsIgnoreType(p, id)})
}

// manaFeasibleGrant is manaFeasible with the may-play ignore-colour rider
// passed explicitly (a pendingCast's pc.mayPlayIgnore, or the offer-side
// payerGrantsIgnoreColor derivation), and the payer's PayLifeInsteadOf:B
// grant derived here. Both widen the leaf payable check the same way the
// payment (resolveManaWith) widens it, so an offered cast, an offered
// announcement face and the charged total can never disagree on a
// K'rrik-shaped or MayPlayIgnoreColor$-shaped cost either.
func (e *Engine) manaFeasibleGrant(p state.PlayerID, id state.ObjID, ability bool, c Cost, mods costMods, taxGeneric, delve int32, rider pipRider) bool {
	pl := e.G.Players[p]
	return mods.feasibleAny(c, e.manaAvailableFor(p, id, ability), pl.Snow, pl.Life, taxGeneric, delve,
		e.payerGrantsPayLifeInsteadOfB(p), rider, e.paymentConv(p, id, ability))
}

// manaFeasiblePool is manaFeasible priced against an EXPLICIT pool instead of
// the seat's restriction-adjusted floating one. The ordinary gate
// (manaFeasible above) is exactly this with the real manaAvailableFor pool;
// the potential-action walk (rules/legal.go legalActionsPriced) passes the
// hypothetical bound the seat would hold after floating every untapped
// source. The payer grants and conversion shaping are the same reads in both
// modes, so a potential action and the offer the walk mirrors can never
// disagree about what the pool may satisfy.
func (e *Engine) manaFeasiblePool(p state.PlayerID, id state.ObjID, ability bool, c Cost, mods costMods, taxGeneric, delve int32, pool state.Mana) bool {
	pl := e.G.Players[p]
	return mods.feasibleAny(c, pool, pl.Snow, pl.Life, taxGeneric, delve,
		e.payerGrantsPayLifeInsteadOfB(p),
		pipRider{anyColor: e.payerGrantsIgnoreColor(p, id), anyType: e.payerGrantsIgnoreType(p, id)},
		e.paymentConv(p, id, ability))
}

// manaFeasiblePriced is manaFeasible's priced-mode entry: hyp nil keeps the
// ordinary real-pool gate, hyp non-nil prices the feasibility against the
// potential walk's hypothetical bound (rules/legal.go legalActionsPriced).
func (e *Engine) manaFeasiblePriced(p state.PlayerID, id state.ObjID, ability bool, c Cost, mods costMods, taxGeneric, delve int32, hyp *state.Mana) bool {
	pool := e.manaAvailableFor(p, id, ability)
	if hyp != nil {
		pool = *hyp
	}
	return e.manaFeasiblePool(p, id, ability, c, mods, taxGeneric, delve, pool)
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

// collectCostStatics collects cost-modifier statics from every zone a static
// can be live in, gated by each static's own EffectZone$ (Forge's
// default is the battlefield, so a static with no EffectZone$ behaves exactly
// as activeStatics did — a battlefield-only collector). This is what lets a
// card's own reduction apply while it is still in hand: Ghalta, Primal
// Hunger's `EffectZone$ All | ValidCard$ Card.Self` static is live from the
// hand, the library, the command zone and the stack alike. The walk is
// deterministic: AliveFrom(0) seats, a fixed zone order, slice order inside
// each zone, and each face's own Statics order.
func (e *Engine) collectCostStatics() costStaticViews {
	var out costStaticViews
	add := func(o *state.Object, id state.ObjID) {
		f := o.Face()
		if f == nil {
			return
		}
		for _, st := range f.Statics {
			var dst *[]staticView
			switch st.Mode {
			case "RaiseCost":
				dst = &out.raise
			case "ReduceCost":
				dst = &out.reduce
			case "SetCost":
				dst = &out.set
			default:
				continue
			}
			if !effectZoneOK(st.Params["EffectZone"], o.Zone) {
				continue
			}
			*dst = append(*dst, staticView{Source: id, Controller: o.Controller, Params: st.Params})
		}
	}
	for pi, p := range e.G.AliveFrom(0) {
		for _, z := range []state.Zone{state.ZBattlefield, state.ZStack, state.ZGraveyard,
			state.ZHand, state.ZLibrary, state.ZExile, state.ZCommand} {
			// The stack is a SHARED zone (state.Game.Zone returns g.Stack for
			// every player), so walking it under every alive seat would
			// collect each stack card's statics once per seat -- a spell's own
			// reduction (Dargo's EffectZone$ All statics while it sits on the
			// stack) would apply twice. Walk the shared stack exactly once,
			// under the first alive seat, keeping the original zone order.
			if z == state.ZStack && pi > 0 {
				continue
			}
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
	return e.modAmountX(sv, 0)
}

// modAmountX is modAmount with the cast's announced {X} bound into the
// evaluation context, so an Amount$ chain that reads Count$xPaid (Dargo's
// SVar:X:Count$xPaid over SVar:Y:SVar$X/Times.2) sees the announced value
// during the in-cast recomputation manaToPay/manaToPayX run. x=0 is the
// offer-time read (an unbound {X} prices as 0), identical to modAmount.
func (e *Engine) modAmountX(sv staticView, x int32) int32 {
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
	ctx := &effects.Ctx{Source: sv.Source, Controller: sv.Controller, SVars: f.SVars, X: x}
	// An SVar NAME resolves through its body on the source's face; anything
	// else is an inline Count$-class expression evaluated as written.
	if body, ok := f.SVars[raw]; ok {
		return effects.EvalCount(e, ctx, body)
	}
	return effects.EvalCount(e, ctx, raw)
}

// raiseFromCost parses a RaiseCost Cost$ into its mana and life raise. Only
// the plain shapes apply: single colour letters, numeric tokens, and the
// fixed PayLife<N> token. Anything else — hybrid pips (none in the corpus's
// cost raises), X/T, or a <...> component this build does not model as an
// additional raise (Waterbend, ExileFromHand, BeholdExile, Sac<...>,
// AddCounter, tapXType) — reports false, so the static degrades to the
// Amount$ reading (absent → the zero raise) rather than silently pricing an
// unmodelled cost as one generic mana.
func raiseFromCost(s string) (col state.Mana, gen, life int32, ok bool) {
	for toks := (costTokenIter{s: s}); ; {
		sym, more := toks.next()
		if !more {
			break
		}
		switch {
		case len(sym) == 1 && strings.ContainsRune("WUBRGC", rune(sym[0])):
			col[state.ManaIndex(sym[0])]++
		case isDigitRun(sym):
			n, err := strconv.ParseInt(sym, 10, 64)
			if err != nil || n < 0 || n > int64(math.MaxInt32) {
				return col, 0, 0, false
			}
			gen = addClampedGeneric(gen, n)
		default:
			if m := lifeCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[1], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					return col, 0, 0, false
				}
				life = addClampedGeneric(life, n)
				continue
			}
			return col, 0, 0, false
		}
	}
	return col, gen, life, true
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
// spell or ability — Auriok Steelshaper's Activated.Equip), ValidTarget$
// (the announced target, repriced before payment), AffectedZone$ for an
// ability modifier (which zone its source sits in) and IsPresent$
// (an intervening-if, e.g. Trinisphere's untapped self). Amount$ is
// evaluated through the SVar/Count$ machinery, and the modifiers are
// returned in Forge's application order (increases, reductions in static
// order, SetCost floor).
func (e *Engine) costModifiers(p state.PlayerID, id state.ObjID, scope costScope) costMods {
	return e.costModifiersWithTargets(p, id, scope, nil, false)
}

// costModifiersForTargets is costModifiers with the chosen cast-time targets
// supplied. A nil target slice is the pre-announcement offer phase, where a
// ValidTarget$ static cannot yet apply; target choice re-enters this helper
// before payment with the actual targets.
func (e *Engine) costModifiersForTargets(p state.PlayerID, id state.ObjID, scope costScope, targets []state.Target) costMods {
	return e.costModifiersWithTargets(p, id, scope, targets, false)
}

// costModifiersForPotentialTargets is the offer-side counterpart for a
// target-conditional REDUCTION. It admits a spell whose base cost is
// unaffordable only when at least one legal target can make the reduction
// apply. Target-conditional raises and SetCost floors are intentionally not
// assumed: they can only make an otherwise legal offer more expensive, so
// charging them speculatively would incorrectly withhold a nonmatching
// target choice. The selected target is always repriced by
// costModifiersForTargets before payment.
func (e *Engine) costModifiersForPotentialTargets(p state.PlayerID, id state.ObjID, scope costScope, targets []state.Target) costMods {
	return e.costModifiersWithTargets(p, id, scope, targets, true)
}

// costModifiersForTargetsX is costModifiersForTargets with the cast's
// announced {X} bound: the payment-side recomputation (manaToPay/manaToPayX)
// uses it when the cost announces a variable sacrifice count (Sac<X/Spec>),
// because the offer-time snapshot priced every Amount$ with X=0 and a
// reduction reading Count$xPaid would otherwise never apply (Dargo's
// "{2} less for each permanent sacrificed this way").
func (e *Engine) costModifiersForTargetsX(p state.PlayerID, id state.ObjID, scope costScope, targets []state.Target, x int32) costMods {
	return e.costModifiersWithTargetsX(p, id, scope, targets, false, x)
}

func (e *Engine) costModifiersWithTargetsX(p state.PlayerID, id state.ObjID, scope costScope, targets []state.Target, potential bool, x int32) costMods {
	return e.costModifiersWithTargetsXUsing(e.collectCostStatics(), p, id, scope, targets, potential, x)
}

func (e *Engine) costModifiersWithTargetsXUsing(statics costStaticViews, p state.PlayerID, id state.ObjID, scope costScope, targets []state.Target, potential bool, x int32) costMods {
	var mods costMods
	xBound := x != 0
	for _, group := range []struct {
		mode  string
		views []staticView
	}{
		{"RaiseCost", statics.raise},
		{"ReduceCost", statics.reduce},
	} {
		mode := group.mode
		for _, sv := range group.views {
			if potential && mode == "RaiseCost" {
				if _, targetConditional := sv.Params["ValidTarget"]; targetConditional {
					continue
				}
			}
			if !e.costStaticApplies(sv, mode, p, id, scope, targets, xBound) {
				continue
			}
			if mode == "RaiseCost" {
				// A RaiseCost Cost$ names the whole additional cost (Forge
				// CostAdjustment's RaiseCost branch): a plain mana/life cost
				// is raised as-is, pips and life included. A Cost$ paired
				// with an Amount$ ("you may pay {1}{G} any number of times")
				// is an OPTIONAL additional-cost shape this build does not
				// model -- the exotic Amount$ skips the static below, so only
				// the plain raise applies. Cost$ shapes that are not plain
				// mana/life (Waterbend, ExileFromHand, Sac<...>) parse
				// nowhere and are skipped by raiseFromCost.
				rc, rg, rl, costOK := raiseFromCost(sv.Params["Cost"])
				if costOK {
					if _, hasAmt := sv.Params["Amount"]; !hasAmt {
						for i := range rc {
							mods.raiseCol[i] = addClampedGeneric(mods.raiseCol[i], int64(rc[i]))
						}
						mods.raiseGen = addClampedGeneric(mods.raiseGen, int64(rg))
						mods.raiseLife = addClampedGeneric(mods.raiseLife, int64(rl))
						continue
					}
				}
				mods.raises = append(mods.raises, e.modAmountX(sv, x))
				continue
			}
			red := costMod{
				ignoreGeneric: sv.Params["IgnoreGeneric"] == "True",
				floor:         parseAmount(sv.Params["MinMana"], 0),
			}
			if col, ok := sv.Params["Color"]; ok && strings.TrimSpace(col) != "" {
				// Each listed token is reduced by the Amount$: colour letters
				// take their pip from the cost's coloured part, and a numeric
				// token names that many generic pips.  Numeric is deliberately
				// not limited to "1": Discontinuity's real `Color$ 2 U U`
				// removes two generic and two blue pips.  Treating `2` as a
				// colour letter would route it through ManaIndex and remove one
				// colourless pip instead.  Amount$ applies to every token, so
				// `Color$ 2 U | Amount$ X` means 2*X generic plus X blue.
				red.hasColor = true
				amount := e.modAmountX(sv, x)
				for _, tok := range strings.Fields(col) {
					if isDigitRun(tok) {
						n, err := strconv.ParseInt(tok, 10, 64)
						if err != nil || n < 0 || n > int64(math.MaxInt32) {
							continue // malformed Color$ token fails closed
						}
						red.generic = addClampedGeneric(red.generic, n*int64(amount))
						continue
					}
					if len(tok) == 1 && strings.ContainsRune("WUBRGC", rune(tok[0])) {
						red.colored[state.ManaIndex(tok[0])] = addClampedGeneric(
							red.colored[state.ManaIndex(tok[0])], int64(amount))
					}
				}
			} else {
				red.generic = e.modAmountX(sv, x)
			}
			mods.reduces = append(mods.reduces, red)
		}
	}
	for _, sv := range statics.set {
		if potential {
			if _, targetConditional := sv.Params["ValidTarget"]; targetConditional {
				continue
			}
		}
		if !e.costStaticApplies(sv, "SetCost", p, id, scope, targets, xBound) {
			continue
		}
		if n := e.modAmountX(sv, x); n > mods.setFloor {
			mods.setFloor = n
		}
	}
	return mods
}

func (e *Engine) costModifiersWithTargets(p state.PlayerID, id state.ObjID, scope costScope, targets []state.Target, potential bool) costMods {
	return e.costModifiersWithTargetsUsing(e.collectCostStatics(), p, id, scope, targets, potential)
}

func (e *Engine) costModifiersWithTargetsUsing(statics costStaticViews, p state.PlayerID, id state.ObjID, scope costScope, targets []state.Target, potential bool) costMods {
	var mods costMods
	for _, group := range []struct {
		mode  string
		views []staticView
	}{
		{"RaiseCost", statics.raise},
		{"ReduceCost", statics.reduce},
	} {
		mode := group.mode
		for _, sv := range group.views {
			if potential && mode == "RaiseCost" {
				if _, targetConditional := sv.Params["ValidTarget"]; targetConditional {
					continue
				}
			}
			if !e.costStaticApplies(sv, mode, p, id, scope, targets, false) {
				continue
			}
			if mode == "RaiseCost" {
				// A RaiseCost Cost$ names the whole additional cost (Forge
				// CostAdjustment's RaiseCost branch): a plain mana/life cost
				// is raised as-is, pips and life included. A Cost$ paired
				// with an Amount$ ("you may pay {1}{G} any number of times")
				// is an OPTIONAL additional-cost shape this build does not
				// model — the exotic Amount$ skips the static below, so only
				// the plain raise applies. Cost$ shapes that are not plain
				// mana/life (Waterbend, ExileFromHand, Sac<...>) parse
				// nowhere and are skipped by raiseFromCost.
				rc, rg, rl, costOK := raiseFromCost(sv.Params["Cost"])
				if costOK {
					if _, hasAmt := sv.Params["Amount"]; !hasAmt {
						for i := range rc {
							mods.raiseCol[i] = addClampedGeneric(mods.raiseCol[i], int64(rc[i]))
						}
						mods.raiseGen = addClampedGeneric(mods.raiseGen, int64(rg))
						mods.raiseLife = addClampedGeneric(mods.raiseLife, int64(rl))
						continue
					}
				}
				mods.raises = append(mods.raises, e.modAmount(sv))
				continue
			}
			red := costMod{
				ignoreGeneric: sv.Params["IgnoreGeneric"] == "True",
				floor:         parseAmount(sv.Params["MinMana"], 0),
			}
			if col, ok := sv.Params["Color"]; ok && strings.TrimSpace(col) != "" {
				// Each listed token is reduced by the Amount$: colour letters
				// take their pip from the cost's coloured part, and a numeric
				// token names that many generic pips.  Numeric is deliberately
				// not limited to "1": Discontinuity's real `Color$ 2 U U`
				// removes two generic and two blue pips.  Treating `2` as a
				// colour letter would route it through ManaIndex and remove one
				// colourless pip instead.  Amount$ applies to every token, so
				// `Color$ 2 U | Amount$ X` means 2*X generic plus X blue.
				red.hasColor = true
				amount := e.modAmount(sv)
				for _, tok := range strings.Fields(col) {
					if isDigitRun(tok) {
						n, err := strconv.ParseInt(tok, 10, 64)
						if err != nil || n < 0 || n > int64(math.MaxInt32) {
							continue // malformed Color$ token fails closed
						}
						red.generic = addClampedGeneric(red.generic, n*int64(amount))
						continue
					}
					if len(tok) == 1 && strings.ContainsRune("WUBRGC", rune(tok[0])) {
						red.colored[state.ManaIndex(tok[0])] = addClampedGeneric(
							red.colored[state.ManaIndex(tok[0])], int64(amount))
					}
				}
			} else {
				red.generic = e.modAmount(sv)
			}
			mods.reduces = append(mods.reduces, red)
		}
	}
	for _, sv := range statics.set {
		if potential {
			if _, targetConditional := sv.Params["ValidTarget"]; targetConditional {
				continue
			}
		}
		if !e.costStaticApplies(sv, "SetCost", p, id, scope, targets, false) {
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
func (e *Engine) costStaticApplies(sv staticView, mode string, p state.PlayerID, id state.ObjID, scope costScope, targets []state.Target, xBound bool) bool {
	if ty, ok := sv.Params["Type"]; ok && ty != "" && ty != scope.kind {
		return false
	}
	if !e.costActorMatches(sv, p) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(sv.Params["OnlyFirstSpell"]), "True") &&
		e.onlyFirstSpellUsed(sv, p, id) {
		// OnlyFirstSpell$ (Conduit of Ruin: "The first creature spell you cast
		// each turn costs {2} less"): the reduction is spent once the
		// turn's first covered spell has been announced. See
		// onlyFirstSpellUsed for the tracking.
		return false
	}
	if spec, ok := sv.Params["ValidCard"]; ok && !effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(sv.Source, sv.Controller)) {
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
	if spec, ok := sv.Params["ValidTarget"]; ok && !e.costTargetsMatch(sv, spec, targets) {
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
	if sv.Params["Relative"] == "True" && !(mode == "ReduceCost" && xBound) {
		// Relative$ Amount$ scales with something the composition point does
		// not yet know (IncreaseCost per target beyond the first, or a game
		// state the offer-time read cannot price) — a per-target shape no
		// offer-time composition knows. Skipping, like ValidTarget$.
		// EXCEPTION: the announced-X recomputation (costModifiersForTargetsX,
		// manaToPay/manaToPayX) is exactly the caller whose composition point
		// DOES know the variable a "costs {2} less for each permanent
		// sacrificed this way" amount scales with — Dargo's Relative$ True
		// static reads Amount$ Y over SVar:Y:SVar$X/Times.2 with X the
		// announced sacrifice count, and at that point the amount is
		// evaluated with X bound. A Relative$ ReduceCost whose Amount$ is
		// unresolvable still degrades to zero (the honest no-reduction),
		// never to an invented discount.
		return false
	}
	return true
}

// costTargetsMatch reports whether at least one chosen target satisfies a
// ValidTarget$ cost-modifier requirement. Forge's "spells that target a
// creature" grammar is an any-target condition: selecting one matching
// target is sufficient, including in a multi-target spell. Object targets use
// the static source context so Card.Self/NICKNAME bind to the permanent that
// supplied the modifier; player targets use the same controller-relative
// player-spec matcher as the other static gates. A nil target slice is the
// pre-announcement offer phase and deliberately cannot satisfy the condition.
func (e *Engine) costTargetsMatch(sv staticView, spec string, targets []state.Target) bool {
	if len(targets) == 0 {
		return false
	}
	ctx := e.specCtx(sv.Source, sv.Controller)
	for _, target := range targets {
		if target.IsPlayer {
			if effects.MatchesPlayerSpec(e.G, spec, target.Player, sv.Controller) {
				return true
			}
			continue
		}
		if effects.MatchesSpecCtx(e.G, spec, target.Obj, ctx) {
			return true
		}
	}
	return false
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
//
// The grammar lives in effects.CheckSVarHolds, the ONE SVar-compare evaluator
// this build ships: conditionMet's ConditionCheckSVar$ branch
// (effects/conditions.go) and rules/legal.go's ability-offer gate delegate to
// the same function. This wrapper only maps the staticView onto a Ctx — the
// source face's SVar table, the static's controller as You.
func (e *Engine) checkSVarHolds(sv staticView) bool {
	raw, ok := sv.Params["CheckSVar"]
	if !ok {
		return true
	}
	if strings.TrimSpace(raw) == "" {
		// Present-but-empty: the empty expression, EvalCount reads it 0 and
		// the no-compare nonzero read fails it -- what the pre-delegation
		// code did too (and no corpus static carries).
		return false
	}
	o := e.G.Obj(sv.Source)
	if o == nil || o.Face() == nil {
		return false
	}
	f := o.Face()
	ctx := &effects.Ctx{Source: sv.Source, Controller: sv.Controller, SVars: f.SVars}
	holds, evaluated := effects.CheckSVarHolds(e, ctx, raw, sv.Params["SVarCompare"])
	if !evaluated {
		// The statics' shipped convention: an unreadable gate body (an
		// unmodelled count head, an unparseable compare) degrades to zero and
		// the static's gate fails — a continuous "as long as X" must not
		// silently always-apply on a gate this build cannot read. The SA-level
		// callers (conditionMet, the ability-offer gate) fail OPEN instead;
		// each call site documents its own direction.
		return false
	}
	return holds
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
		// The bare form is the single-cost Kicker's mode; the and/or
		// two-part Kicker's per-part modes (kicked1/kicked2/kickedboth) are
		// kicked casts too -- a cost static gated on "was this kicked" must
		// not depend on WHICH part was paid. A multikicked cast (CR 702.43's
		// kicker variant) is a kicked cast the same way.
		return scope.mode == "kicked" || scope.mode == "kicked1" ||
			scope.mode == "kicked2" || scope.mode == "kickedboth" ||
			scope.mode == "multikicked"
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
		return e.isLoyaltyAbility(ab)
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
	effects.RegisterNonAPI("stat:CantBeCast", "stat:CantBeActivated", "stat:RaiseCost", "stat:CastWithFlash",
		"stat:ReduceCost", "stat:AlternativeCost", "stat:CantBlock", "stat:CantBlockBy",
		"stat:CantGainLife", "stat:Continuous", "stat:ManaConvert", "stat:NumLoyaltyAct",
		// combatrestriction1: the three combat/sacrifice restriction statics.
		// CantAttack is enforced per (attacker, defender) pair
		// (rules/layers.go attackBlocked, consulted by askAttackers /
		// validateAttackers / mustAttackRequired's pair gate), CantSacrifice at
		// every sacrifice candidate choke point (rules.Engine.SacrificeBlocked,
		// the effects.Host method), and MustAttack by the board-wide
		// activeStatics walk in mustAttackRequired. Only the whitelisted
		// parameter shapes are enforced (cantRestrictionParamsReadable for the
		// two Cant* statics; the Mode$/ValidCreature$/Description$ whitelist the
		// requirement solver already carried for MustAttack) — the conditional
		// shapes stay unregistered behaviour-wise and are ledgered in AGENTS.md.
		"stat:CantAttack", "stat:CantSacrifice", "stat:MustAttack")
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

// panharmoniconEchoes reports how many ADDITIONAL trigger placements the
// battlefield's stat:Panharmonicon statics demand for the trigger the EVENT
// ev just caused on source object src (CR 702.109: "that ability triggers an
// additional time"). One echo per matching static, in activeStatics'
// deterministic scan order (which never matters here -- the echoes are summed
// -- but keeps the call off any map range).
//
// A static must match the triggering CAUSE, not just the triggering
// permanent, before it doubles anything. The gate reads the corpus's own
// parameter set, each optional, every one present needing to match:
//
//   - ValidMode$: the comma-separated Forge event-mode family the causing
//     event belongs to (panharmoniconModes); a static naming modes the event
//     is not in contributes nothing.
//   - ValidCause$: the event's causing OBJECT -- the moved/cast card for a
//     zone change or cast, the single attacker of a one-attacker declare
//     (a multi-attacker event has no singular cause and fails closed, the
//     same convention triggerReferents' Attacks case applies) -- matched as
//     a filter spec. Origin$/Destination$ scope a ChangesZone cause to the
//     move's own zones exactly as the trigger grammar does.
//   - ValidSource$: the in-flight damage source (e.damaging -- combat
//     damage's dealing creature; absent for any other event), a filter spec.
//   - ValidTarget$: the event's target -- the damage recipient (object or
//     player, whichever the event carries) or the BecomesTarget-ed object
//     (the trigger's own source), a filter spec or player spec.
//   - ValidActivator$: the cast's player, a player spec.
//   - ValidPlayer$: the event's player (the life gainer, the drawing
//     player), a player spec.
//   - CombatDamage$ True: the in-flight damage must be the combat damage
//     step's own assignment (the combatDamaging flag); False is the default
//     and matches either.
//
// ValidZone$ (Echoes of Eternity's "Battlefield,Stack") is honoured as a
// filter on the triggering object's own zone; a static naming zones the
// object is not in contributes nothing. A static with no ValidCard$
// contributes nothing rather than matching everything -- a Panharmonicon
// static that does not say WHAT it doubles is a script defect this build
// will not paper over by doubling every trigger on the board. Params this
// build cannot evaluate (IsPresent/PresentCompare, Condition, ValidTurned --
// there is no TurnFaceUp event) fail closed: the static does not double,
// never over-applies.
func (e *Engine) panharmoniconEchoes(g *state.Game, src state.ObjID, ev events.Event) int {
	o := g.Obj(src)
	if o == nil {
		return 0
	}
	modes := panharmoniconModes(ev)
	n := 0
	for _, sv := range e.activeStatics("Panharmonicon") {
		if vm := sv.Params["ValidMode"]; vm != "" {
			ok := false
			for _, want := range strings.Split(vm, ",") {
				want = strings.TrimSpace(want)
				for _, have := range modes {
					if want == have {
						ok = true
						break
					}
				}
				if ok {
					break
				}
			}
			if !ok {
				continue
			}
		}
		if vz := sv.Params["ValidZone"]; vz != "" {
			zones, all, valid := effects.ParseZones(vz)
			ok := false
			if valid || all {
				for _, z := range zones {
					if z == o.Zone {
						ok = true
						break
					}
				}
			}
			if !ok {
				continue
			}
		}
		// ValidCause$: the causing object per event family. A zone change or
		// cast's cause is the moved/cast card; an attack's is the single
		// declared attacker (a multi-attacker event has no singular cause);
		// a damage event's cause is not modelled -- fail closed.
		if spec := sv.Params["ValidCause"]; spec != "" {
			cause := state.ObjID(0)
			switch ev.Kind {
			case events.MoveZone, events.Draw, events.PutOnStack:
				cause = ev.Obj
			case events.DeclareAttackers:
				if len(ev.IDs) == 1 {
					cause = ev.IDs[0]
				}
			}
			if cause == 0 || !effects.MatchesSpecFrom(g, spec, cause, sv.Controller, sv.Source) {
				continue
			}
		}
		if sv.Params["Origin"] != "" && effects.ParseZone(sv.Params["Origin"]) != ev.From {
			continue
		}
		if sv.Params["Destination"] != "" && effects.ParseZone(sv.Params["Destination"]) != ev.To {
			continue
		}
		if spec := sv.Params["ValidSource"]; spec != "" {
			if ev.Kind != events.Damage || e.damaging == 0 ||
				!effects.MatchesSpecFrom(g, spec, e.damaging, sv.Controller, sv.Source) {
				continue
			}
		}
		if spec := sv.Params["ValidTarget"]; spec != "" {
			switch {
			case ev.Kind == events.Damage && ev.Obj != 0:
				if !effects.MatchesSpecFrom(g, spec, ev.Obj, sv.Controller, sv.Source) {
					continue
				}
			case ev.Kind == events.Damage:
				if !effects.MatchesPlayerSpec(g, spec, ev.Player, sv.Controller) {
					continue
				}
			case ev.Kind == events.TargetsChosen:
				// The BecomesTarget-ed object is the trigger's own source.
				if !effects.MatchesSpecFrom(g, spec, src, sv.Controller, sv.Source) {
					continue
				}
			default:
				continue
			}
		}
		if spec := sv.Params["ValidActivator"]; spec != "" {
			if ev.Kind != events.PutOnStack || !effects.MatchesPlayerSpec(g, spec, ev.Player, sv.Controller) {
				continue
			}
		}
		if spec := sv.Params["ValidPlayer"]; spec != "" {
			if (ev.Kind != events.LifeChange && ev.Kind != events.Draw) ||
				!effects.MatchesPlayerSpec(g, spec, ev.Player, sv.Controller) {
				continue
			}
		}
		if sv.Params["CombatDamage"] == "True" && !(ev.Kind == events.Damage && e.combatDamaging) {
			continue
		}
		spec := sv.Params["ValidCard"]
		if spec == "" {
			continue
		}
		if effects.MatchesSpecFrom(g, spec, src, sv.Controller, sv.Source) {
			n++
		}
	}
	return n
}

// panharmoniconModes maps the folded causing event to the Forge trigger-mode
// family it can serve as a triggering event for -- the same event-to-mode
// correspondence trigger_match.go's per-mode matchers test one mode at a
// time, stated once here for a ValidMode$ list to check against. A MoveZone
// with the card played from hand also serves LandPlayed (landPlayedMatches
// matches the same event), the way the corpus spells multi-family statics.
func panharmoniconModes(ev events.Event) []string {
	switch ev.Kind {
	case events.MoveZone:
		out := []string{"ChangesZone", "ChangesZoneAll"}
		if ev.From == state.ZHand && ev.To == state.ZBattlefield {
			out = append(out, "LandPlayed")
		}
		return out
	case events.Draw:
		return []string{"Drawn"}
	case events.PutOnStack:
		return []string{"SpellCast", "SpellCastOrCopy"}
	case events.StackCopy:
		return []string{"SpellCopy", "SpellCastOrCopy"}
	case events.DeclareAttackers:
		return []string{"Attacks", "AttackersDeclared", "AttackersDeclaredOneTarget"}
	case events.Damage:
		return []string{"DamageDone", "DamageDealtOnce", "DamageDoneOnce", "DamageAll"}
	case events.TargetsChosen:
		return []string{"BecomesTarget", "BecomesTargetOnce"}
	case events.LifeChange:
		if ev.Amount > 0 {
			return []string{"LifeGained"}
		}
		return []string{"LifeLost"}
	default:
		return nil
	}
}
