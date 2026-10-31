package rules

import (
	"math"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// SpellCopyAllowed reports whether id may be copied: false when the stack
// object's own ability carries CantCopy$ True ("This ability can't be
// copied"), or when a live CantBeCopied static names it.
// The source-zone walk follows staticSourceZones, so Stack-scoped spell
// statics and broader EffectZone statics share the normal static filter path.
func (e *Engine) SpellCopyAllowed(id state.ObjID) bool {
	candidate := e.G.Obj(id)
	if candidate == nil || candidate.Zone != state.ZStack {
		return false
	}
	if a := candidate.Ability; a != nil && (a.Params["CantCopy"] == "True" || a.Params["CantCopy"] == "true") {
		return false
	}
	for pi, p := range e.G.AliveFrom(0) {
		for _, z := range staticSourceZones {
			if z == state.ZStack && pi > 0 {
				continue
			}
			for _, source := range e.staticSourceIDs(p, z) {
				o := e.G.Obj(source)
				if o == nil || o.Face() == nil || offBattlefieldStaticsInert(z, o) || e.printedAbilitiesGone(o) {
					continue
				}
				for si, n := 0, o.PileStaticCount(); si < n; si++ {
					pst, ok := o.PileStaticAt(si)
					if !ok || pst.Static.Mode != "CantBeCopied" || !staticEffectZoneOK(pst.Static, o.Zone) {
						continue
					}
					spec := strings.TrimSpace(pst.Static.ParamStr(cards.PKValidCard))
					if spec == "" {
						continue
					}
					sv := staticView{Source: source, Controller: o.Controller, Params: pst.Static.Params, PS: pst.Static.ParamSetOf(), SVars: pst.Face.SVars}
					if e.matchesSpec(spec, id, e.staticSpecCtx(sv)) {
						return false
					}
				}
			}
		}
	}
	return true
}

// countersRemainApplies reports whether the departing permanent itself has
// an active CountersRemain static. Static lines are read through the canonical
// activeStatics walk so EffectZone, merged-face and face-down rules stay
// consistent with the other static consumers.
func (e *Engine) countersRemainApplies(id state.ObjID) bool {
	for _, sv := range e.activeStatics("CountersRemain") {
		spec := sv.ParamStr(cards.PKValidCard)
		if spec != "" && e.matchesSpec(spec, id, e.staticSpecCtx(sv)) {
			return true
		}
	}
	return false
}

// SurveilLookExtra reports the additional cards a surveil performed by player
// p looks at, from the battlefield statics with Mode$ SurveilNum whose
// ValidPlayer$ admits p (Enhanced Surveillance's "You may look at an
// additional two cards each time you surveil"). mandatory is added to the
// count unconditionally. optional holds ONE entry per OPTIONAL static -- that
// static's own Num$ -- in deterministic activeStatics order: each Optional$
// True static is an independent may effect (surveilnum-r2), so effSurveil
// poses one multi-select election over the entries and the controller can
// accept any subset, never an all-or-nothing sum of two "may"s. The walk is
// the canonical activeStatics collector, so a face-down, merged-pile or
// EffectZone-scoped static is read exactly as every other static mode is, and
// the order is deterministic. Num$ must be a literal or an SVar name the
// static's own face defines; anything else fails closed to no contribution,
// the same direction HandSizeValueOK takes.
func (e *Engine) SurveilLookExtra(p state.PlayerID) (mandatory int32, optional []int32) {
	for _, sv := range e.activeStatics("SurveilNum") {
		spec := strings.TrimSpace(sv.ParamStr(cards.PKValidPlayer))
		if spec == "" {
			spec = "You"
		}
		if !effects.MatchesPlayerSpec(e.G, spec, p, sv.Controller) {
			continue
		}
		n, ok := e.surveilNumValue(sv)
		if !ok || n <= 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(sv.ParamStr(cards.PKOptional)), "True") {
			optional = append(optional, n)
		} else {
			mandatory += n
		}
	}
	return mandatory, optional
}

// surveilNumValue prices one SurveilNum static's Num$: a plain decimal
// literal, else an SVar name resolved against the static's own face table.
// A missing/empty Num$, an unresolvable SVar and a negative value all report
// false.
func (e *Engine) surveilNumValue(sv staticView) (int32, bool) {
	raw := strings.TrimSpace(sv.ParamStr(cards.PKNum))
	if raw == "" {
		return 0, false
	}
	if v, err := strconv.Atoi(raw); err == nil {
		if v < 0 {
			return 0, false
		}
		return int32(v), true
	}
	body, ok := sv.SVars[raw]
	if !ok {
		return 0, false
	}
	v, ok := effects.EvalCountOK(e, effects.NewCtxPtr(sv.Source, sv.Controller, effects.CtxInit{SVars: sv.SVars}), body)
	if !ok || v < 0 {
		return 0, false
	}
	return v, true
}

// castRestricted reports whether p is forbidden from casting id (CantBeCast)
// as an ordinary cast at the current face's printed cost, the only cast whose
// CR 601.3a announcement is the printed {X}.
func (e *Engine) castRestricted(p state.PlayerID, id state.ObjID) bool {
	return e.castRestrictedPrinted(e.activeStatics("CantBeCast"), p, id, nil)
}

// castRestrictedFree is castRestricted for a cast that pays NO mana cost (a
// free Play, Suspend's exiled cast, cascade's election, a plotted card): such
// a cast announces no {X} (CR 107.3b fixes X at 0), so no choice can change
// its mana value and CR 601.3a's lookahead cannot apply.
func (e *Engine) castRestrictedFree(p state.PlayerID, id state.ObjID) bool {
	return e.castRestrictedCost(e.activeStatics("CantBeCast"), p, id, Cost{}, nil)
}

// castRestrictedPrinted gates a cast that will pay the current face's
// printed cost. pool is the mana-pricing pool CR 601.3a's X lookahead prices
// its candidate announcements against: nil is the seat's restriction-
// adjusted floating pool (the real walk and every direct offer), a
// potential walk's hypothetical bound prices the projection (see
// castRestrictedCost).
func (e *Engine) castRestrictedPrinted(statics []staticView, p state.PlayerID, id state.ObjID, pool *state.Mana) bool {
	return e.castRestrictedCostAs(statics, p, id, Cost{}, true, castPricingPool{pool: pool}, false)
}

// castPricingPool is the pricing pool a CantBeCast gate's CR 601.3a X
// lookahead prices against, read LAZILY: a legal walk's pool read is
// counted (legalWalk.pricing, walk_block_reuse.go), and the lookahead is
// the only consumer, so the read -- and its count -- happens only when a
// restriction matches and the cast announces an X. A gate whose answer
// never consulted the pool is pool-independent, which is exactly what the
// block-reuse recorder's count means. w nil reads pool directly.
type castPricingPool struct {
	w    *legalWalk
	pool *state.Mana
}

func (c castPricingPool) get() *state.Mana {
	if c.w != nil {
		return c.w.pricing()
	}
	return c.pool
}

// castRestrictedCost is the CantBeCast gate for a cast that will pay cost:
// CR 601.3a's X lookahead applies only when the cast will ANNOUNCE an X --
// i.e. only when cost itself carries {X} (CR 107.3b: a cast whose cost
// carries no {X}, including every free cast and every alternative cost
// without one, fixes X at 0) -- and the candidate X values it considers are
// only those cost with X folded in can pay from pool (CR 601.2b: the
// announcement must be one the payment can settle; xAnnouncementEscapes
// prices them through the offer's own affordability helper). pool nil is
// the seat's restriction-adjusted floating pool; a potential walk passes
// its hypothetical bound, so the projection sees an escape the caster can
// reach by floating untapped mana. CR 601.2e's recheck (recheckIllegal)
// then enforces the prohibition at the announced X, reversing a proposal
// whose X it still covers (CR 733.1). The restriction itself is still
// matched at the card's current mana value (X = 0 off the stack, CR
// 202.3e), as today. The gate also carries CR 205.4e's legendary lock
// (legendarySpellLocked) ahead of the restriction scan: no announcement can
// lift it, so it is checked before the loop, on every cast route that
// reaches here.
func (e *Engine) castRestrictedCost(statics []staticView, p state.PlayerID, id state.ObjID, cost Cost, pool *state.Mana) bool {
	return e.castRestrictedCostAs(statics, p, id, cost, false, castPricingPool{pool: pool}, false)
}

// castRestrictedCostPrototype is castRestrictedCost for the prototyped cast
// route: every restriction spec reads the card's K:Prototype inset mana
// value, colours and P/T (CR 718.3a) instead of its printed face.
func (e *Engine) castRestrictedCostPrototype(statics []staticView, p state.PlayerID, id state.ObjID, cost Cost, pool *state.Mana) bool {
	return e.castRestrictedCostAs(statics, p, id, cost, false, castPricingPool{pool: pool}, true)
}

// castRestrictedCostAs is the shared CantBeCast gate. printed true ignores
// cost and uses the current face's printed cost, compiled only when a
// restriction actually matches (the X lookahead is its only reader; almost
// every cast check sees no restriction at all), and pp is read only when
// the lookahead runs.
func (e *Engine) castRestrictedCostAs(statics []staticView, p state.PlayerID, id state.ObjID, cost Cost, printed bool, pp castPricingPool, asPrototype bool) bool {
	if e.legendarySpellLocked(p, id) {
		return true
	}
	for _, sv := range e.castRestrictionSources(statics, id) {
		if !e.actorMatches(sv, "Caster", p) {
			continue
		}
		// The shared continuous gate (engine/rules/layers.go) adds the IsPresent$/
		// IsPresent2$/PresentCompare$/PresentZone$/CheckSVar$/SVarCompare$
		// family for the CantBeCast consumer (Blizzard's "as long as the
		// defending player doesn't control a snow land"). It is wired HERE
		// and at recheckIllegal only: the other restrictionGateHolds callers
		// -- CantBeActivated, AssignCombatDamageAsUnblocked,
		// CombatDamageToughness -- keep their pre-existing gate set, so no
		// out-of-scope consumer's semantics move with this task. It subsumes
		// the checkSVarHolds the caller used to run separately, and the
		// duplicate ClassBand$/Condition$ reads inside restrictionGateHolds
		// below are pure state reads with identical semantics.
		if !e.continuousGateHolds(sv) || !e.restrictionGateHolds(sv, id) {
			continue
		}
		// Origin$ (Kunoros, Grafdigger's Cage, Memory Vessel's NoCast): the
		// zone the cast begins from, which at offer time is the restricted
		// object's CURRENT zone (every cast evaluation site's pending origin;
		// the CR 601.2e recheck reads pc.from instead, because the card is on
		// the stack by then). A zone the restriction does not name skips it.
		if o := e.G.Obj(id); o != nil && !castOriginGateHolds(sv, o.Zone) {
			continue
		}
		spec := castValidCardSpec(sv)
		countSpec := spec
		// The origin-zone cast-provenance split (task wascastfrom): a
		// CantBeCast restriction's ValidCard$ carrying a wasCastFromExile /
		// wasCastFromTheirHand-shaped token gates the cast IN PROGRESS -
		// which has no PutOnStack yet - so the origin is the restricted
		// object's CURRENT zone (every cast evaluation site's pending origin;
		// see castOriginAdmitsAtZone). Without the split the token is unknown
		// to the effects-side filter and the restriction silently never
		// applies (the permissive-wrong direction for a prohibition).
		if specCarriesCastOrigin(spec) {
			if o := e.G.Obj(id); o != nil {
				s, ok := e.castOriginAdmitsAtZone(spec, id, o.Zone)
				if !ok {
					continue
				}
				spec = s
			}
		}
		// Bind the delivering Effect's captured Remembered set (the
		// assignmentStaticSpecCtx pattern) for a Card.IsRemembered clause. The
		// loop reads nil for every printed static, so the allocation-free hot
		// path keeps its original shape -- routing this through a helper method
		// made the Resolve closure escape once per cast candidate
		// (TestLegalActionsReusesActionStaticMembership's pin).
		sc := e.spellSpecCtx(e.staticSpecCtx(sv), id)
		for _, rid := range sv.Remembered {
			sc.Remembered = append(sc.Remembered, state.Target{Obj: rid})
		}
		if asPrototype {
			sc = e.prototypeSpellSpecCtx(sc, id)
		}
		if e.matchesSpec(spec, id, sc) &&
			castLimitBinds(e.L.Events, sv, p, id, func(oid state.ObjID) bool {
				if len(sv.Remembered) == 0 {
					return e.matchesSpec(countSpec, oid, e.staticSpecCtx(sv))
				}
				sc := e.staticSpecCtx(sv)
				for _, rid := range sv.Remembered {
					sc.Remembered = append(sc.Remembered, state.Target{Obj: rid})
				}
				return e.matchesSpec(countSpec, oid, sc)
			}) {
			if printed {
				printed = false
				cost = Cost{}
				if o := e.G.Obj(id); o != nil && o.Face() != nil {
					cost = e.faceCompiledCost(o.Face()).Cost
				}
			}
			if cost.X > 0 && e.xAnnouncementEscapes(sv, spec, p, id, cost, pp.get(), asPrototype) {
				continue
			}
			return true
		}
	}
	return false
}

// xAnnouncementEscapes reports whether some payable announced X gives id a
// mana value at which the restriction's spec no longer matches (CR 601.3a).
// Only a current face whose printed mana cost carries {X} can change its
// mana value by announcement, and only a cost that itself announces X can
// announce one, so either condition failing answers false at once. The
// candidate X values are priced with the SAME helper the offer's
// affordability gate runs -- castable against the seat's restriction-
// adjusted floating pool (pool nil), castablePriced against the potential
// walk's hypothetical bound -- so EVERY mana part (hybrid, Phyrexian, snow,
// the coloured pips, Delve's generic credit) is settled the way the payment
// will settle it, and an X the payment cannot afford never enables the
// escape. The Convoke/Harmonize tap credit and CR 601.2f cost modifiers are
// NOT priced, so an X affordable only through them does not enable the
// escape (the offer's own affordability gate still binds every cast the
// lookahead would begin). The mana value is read off the PRINTED cost with
// X folded in (CR 202.3b), never off cost -- an alternative cost's
// non-mana parts add no mana.
//
// The walk stops as soon as no larger X can change the answer, because the
// bound is not a count of real mana: one open source (Produced$ Any, an
// indeterminate Amount$) prices the potential pool at potentialUnbounded, so
// an unrestricted count from minX to the bound would never return. Two facts
// make stopping exact, and verifyXEscape holds both against the full walk in
// the rules test binary:
//
//   - Payability only falls as X grows. These two helpers apply no cost
//     modifier (the reason xAsk's own loop cannot break: a ReduceCost reading
//     the paid X), so X only adds generic mana, and Delve's credit never takes
//     back more than it adds. The first unpayable X ends the walk.
//   - The spec reads the announced X only through cmcValue, and only as a
//     comparison with a number or as parity (effects.ManaValueReads records
//     each). Past every number the spec compared, a larger mana value answers
//     every comparison the same way, so the match can only differ by parity:
//     two consecutive X past every number read leave nothing new to see, and
//     a spec that read no mana value at all ("Card", Voice of Victory's) has
//     its answer at the first payable X.
func (e *Engine) xAnnouncementEscapes(sv staticView, spec string, p state.PlayerID, id state.ObjID, cost Cost, pool *state.Mana, asPrototype bool) bool {
	if cost.X <= 0 {
		return false
	}
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false
	}
	printed := e.faceCompiledCost(o.Face()).Cost
	if asPrototype {
		raw, _, _, ok := state.PrototypeParams(o.Face())
		if !ok {
			return false
		}
		printed = ParseCost(raw)
	}
	if printed.X <= 0 {
		return false
	}
	// Each announced X unit is one more generic to pay, so the pricing
	// pool's raw tally plus Delve's graveyard credit bounds every payable X
	// from above; the priced loop below refuses inside it the announcements
	// the payment cannot settle. The raw tally is only a ceiling, never the
	// priced pool: RestrictedMana provenance is honoured by castable's own
	// AvailableFor read, and the bound is a pure mana ceiling (castablePriced).
	// An announcement is an int32, so the ceiling is too: Total saturates at
	// potentialUnbounded and the credit can only pass it.
	bound := e.G.Players[p].Pool
	if pool != nil {
		bound = *pool
	}
	ceiling := min(int64(bound.Total())+int64(e.delveCredit(p, id, math.MaxInt32)), math.MaxInt32)
	minX := int64(cost.XMin)
	if minX < 1 {
		minX = 1
	}
	escapes, witness := false, int64(0)
	var reads effects.ManaValueReads
	settled := 0
	for x := minX; x <= ceiling; x++ {
		if !e.xAnnouncementPayable(p, id, cost, int32(x), pool) {
			break
		}
		mv := printed.WithX(int32(x)).CMC()
		if !e.xAnnouncementMatches(sv, spec, id, mv, asPrototype, &reads) {
			escapes, witness = true, x
			break
		}
		if !reads.Any {
			break
		}
		if int64(mv) <= reads.Highest {
			settled = 0
			continue
		}
		if settled++; settled == 2 {
			break
		}
	}
	if xEscapeVerify {
		e.verifyXEscape(sv, spec, p, id, cost, printed, pool, asPrototype, minX, ceiling, escapes, witness)
	}
	return escapes
}

// xAnnouncementPayable prices cost at announced x: castable against the
// seat's restriction-adjusted floating pool (pool nil), castablePriced against
// the potential walk's hypothetical bound -- the same split
// legalWalk.affordable runs, so a potential action and the offer it mirrors
// cannot disagree.
func (e *Engine) xAnnouncementPayable(p state.PlayerID, id state.ObjID, cost Cost, x int32, pool *state.Mana) bool {
	paid := cost.WithX(x)
	if pool == nil {
		return e.castable(p, id, paid, false)
	}
	return e.castablePriced(p, id, paid, false, *pool)
}

// xAnnouncementMatches reports whether the restriction's spec matches id at
// mana value mv, recording in reads what the match read of it (nil records
// nothing).
func (e *Engine) xAnnouncementMatches(sv staticView, spec string, id state.ObjID, mv int32, asPrototype bool, reads *effects.ManaValueReads) bool {
	sc := e.spellSpecCtx(e.staticSpecCtx(sv), id)
	if asPrototype {
		sc = e.prototypeSpellSpecCtx(sc, id)
	}
	sc.HasManaValue = true
	sc.ManaValue = mv
	sc.ManaValueReads = reads
	return e.matchesSpec(spec, id, sc)
}

// xEscapeVerify makes xAnnouncementEscapes check its early stops against the
// unrestricted walk and panic on a difference. Set by the rules test binary
// (derivedmemo_verify_test.go), or at link time with derivedMemoVerifyFlag.
var xEscapeVerify = derivedMemoVerifyFlag != ""

// xEscapeVerifyLimit caps how far verifyXEscape walks a bound that is not a
// real count of mana. No corpus restriction compares a number near it.
const xEscapeVerifyLimit = 64

// verifyXEscape runs xAnnouncementEscapes' walk without its early stops, over
// the whole bound or the first xEscapeVerifyLimit announcements of it, and
// panics unless it finds the same answer. An escape the capped walk cannot
// reach is the one answer it cannot judge.
func (e *Engine) verifyXEscape(sv staticView, spec string, p state.PlayerID, id state.ObjID, cost, printed Cost, pool *state.Mana, asPrototype bool, minX, ceiling int64, escapes bool, witness int64) {
	last := min(ceiling, minX+xEscapeVerifyLimit)
	want := false
	for x := minX; x <= last && !want; x++ {
		want = e.xAnnouncementPayable(p, id, cost, int32(x), pool) &&
			!e.xAnnouncementMatches(sv, spec, id, printed.WithX(int32(x)).CMC(), asPrototype, nil)
	}
	if want != escapes && !(escapes && witness > last) {
		panic("rules: xAnnouncementEscapes stopped early on a different answer than the full walk for " + spec)
	}
}

// spellSpecCtx binds the mana value of the SPELL id is about to become
// (state.SpellManaValue: its current face, X as 0) and the AsSpellHalf
// colour/type override while id is still off the stack. CR 601.3a/709.3a: a
// cast restriction or cost modifier evaluates only the half being cast, so a
// split card in hand is not read with its combined CR 709.4b mana value,
// colours or types here. A context that already carries a mana value (the CR
// 601.2e recheck) and an object already on the stack are left alone.
func (e *Engine) spellSpecCtx(sc effects.SpecContext, id state.ObjID) effects.SpecContext {
	if o := e.G.Obj(id); o != nil && o.Zone != state.ZStack && !sc.HasManaValue {
		sc.HasManaValue, sc.ManaValue = true, state.SpellManaValue(e.G, o)
		sc.AsSpellHalf = true
	}
	return sc
}

// prototypeSpellSpecCtx binds the K:Prototype inset cost's mana value and the
// AsPrototype characteristic override for a cast restriction or cost modifier
// reading the prototyped cast route (CR 718.3a). A face without the keyword is
// left exactly as spellSpecCtx returned it.
func (e *Engine) prototypeSpellSpecCtx(sc effects.SpecContext, id state.ObjID) effects.SpecContext {
	o := e.G.Obj(id)
	if o == nil {
		return sc
	}
	cost, _, _, ok := state.PrototypeParams(o.Face())
	if !ok {
		return sc
	}
	sc.AsPrototype = true
	sc.HasManaValue = true
	sc.ManaValue = cards.ManaValueOfCost(cost)
	return sc
}

// legendarySpellLocked reports CR 205.4e: "A player can't cast a legendary
// instant or sorcery spell unless that player controls a legendary creature or
// a legendary planeswalker." It reads the CURRENT face (o.Face()), so an
// Adventure/split/MDFC half is judged as the half being cast, matching the
// alternate-face probes, which pass the probed face through offerAsFace. The
// battlefield walk reads layer-derived types (typeCharacteristics), so a
// permanent made legendary by a layer-4 effect counts and a phased-out one
// does not. An opponent's legend is never on p's battlefield list.
func (e *Engine) legendarySpellLocked(p state.PlayerID, id state.ObjID) bool {
	o := e.G.Obj(id)
	if o == nil {
		return false
	}
	f := o.Face()
	if f == nil || !f.IsLegendary() || !(f.IsInstant() || f.IsSorcery()) {
		return false
	}
	for _, pid := range e.G.Zone(state.ZBattlefield, p) {
		perm := e.G.Obj(pid)
		if perm == nil || perm.PhasedOut {
			continue
		}
		legend, legendKind := false, false
		for _, t := range e.typeCharacteristics(pid, 0) {
			switch t {
			case "Legendary":
				legend = true
			case "Creature", "Planeswalker":
				legendKind = true
			}
		}
		if legend && legendKind {
			return false
		}
	}
	return true
}

// castValidCardSpec is a CantBeCast static's ValidCard$ with an absent one
// read as "Card" (Fires of Invention carries none: every spell). An Origin$
// static without a ValidCard$ (Kunoros, Grafdigger's Cage's "from graveyards")
// takes the same default now that castOriginGateHolds enforces the origin
// against the cast's actual start zone -- the default can no longer turn an
// origin restriction into a blanket one.
func castValidCardSpec(sv staticView) string {
	spec := strings.TrimSpace(sv.ParamStr(cards.PKValidCard))
	if spec == "" {
		return "Card"
	}
	return spec
}

// numLimitEachTurn parses NumLimitEachTurn$ N once: a positive decimal
// literal, else ok=false (absent, empty, non-numeric or non-positive).
func (sv staticView) numLimitEachTurn() (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(sv.ParamStr(cards.PKNumLimitEachTurn)))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// castLimitBinds is the ONE per-turn cast-limit rule CantBeCast's
// NumLimitEachTurn$ names ("can't cast more than N spells each turn"),
// shared by the offer (castRestrictedCost) and the CR 601.2e recheck
// (recheckIllegal). A static without the parameter always binds; with it the
// static binds only once p has already cast N matching spells this turn,
// the candidate's own in-flight PutOnStack excluded.
func castLimitBinds(log []events.Event, sv staticView, p state.PlayerID, candidate state.ObjID, match func(state.ObjID) bool) bool {
	n, ok := sv.numLimitEachTurn()
	if !ok {
		return true
	}
	return castsThisTurnMatchingBy(log, p, candidate, match) >= n
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
//
// It also appends one view per LIVE registered continuous effect whose
// Restriction is CantBeCast and whose parameters pass
// effects.CantBeCastParamsReadable (the playLandForbidden precedent: one
// whitelist shared by effEffect's registration gate and this consultation, so
// the two paths cannot disagree about what is readable). This is the route
// Bilbo's Gambit's `DB$ Effect | StaticAbilities$ STCantBeCast` reaches the
// cast gate through; a printed-static-only board appends nothing and pays
// only the active() walk.
func (e *Engine) castRestrictionSources(statics []staticView, id state.ObjID) []staticView {
	out := statics
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return out
	}
	for _, st := range o.Face().Statics {
		if st.Mode != "CantBeCast" || !staticEffectZoneOK(st, o.Zone) {
			continue
		}
		out = append(out, staticView{Source: id, Controller: o.Controller, Params: st.Params, PS: st.ParamSetOf()})
	}
	// Index, not range-copy: the views below borrow the effect's slices, so
	// the loop must not dance over a range copy that escapes. A board with no
	// registered CantBeCast effect (every ordinary cast) appends nothing.
	active := e.active()
	for i := range active {
		ce := &active[i]
		if cards.StaticModeOf(ce.Restriction) != cards.StaticCantBeCast || !effects.CantBeCastParamsReadable(ce.RestrictParams) {
			continue
		}
		out = append(out, staticView{
			Source: ce.Source, Controller: ce.Controller,
			Params: ce.RestrictParams, SVars: ce.RestrictSVars,
			Remembered: ce.Remembered, rememberedPlayers: ce.RememberedPlayers,
			effectDelivered: true,
		})
	}
	return out
}

// castOriginGateHolds evaluates a CantBeCast restriction's Origin$: the zone
// the cast may BEGIN from (Forge reads card.getCastFrom(); CR 601.2e's
// recheck reads the pending cast's origin, because the card is on the stack
// by then). No Origin$ always holds; a list naming no zone the cast could
// start from -- or an unparseable one -- withholds the restriction, the
// fail-closed direction for an unevaluable term. Grafdigger's Cage's
// "from graveyards" and Memory Vessel's Origin$ Hand are the corpus shapes.
func castOriginGateHolds(sv staticView, from state.Zone) bool {
	origin, ok := sv.Param(cards.PKOrigin)
	if !ok {
		return true
	}
	zones := effects.ZoneListOf(origin)
	return zones.OK() && zones.Admits(from)
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
	if !e.classBandGateHolds(sv.ParamStr(cards.PKClassBand), sv.Source) {
		return false
	}
	if az, ok := sv.Param(cards.PKAffectedZone); ok {
		o := e.G.Obj(target)
		if o == nil || !affectedZoneOK(az, o.Zone) {
			return false
		}
	}
	// Phases$ (CR 506.1, 604.1): "during combat" restrictions (Yuriko, Hand
	// to Hand) hold only inside the window. An absent window always holds; an
	// unparseable one withholds the restriction, like every other
	// unevaluable term here.
	if ph := sv.ParamStr(cards.PKPhases); ph != "" {
		if pp := e.parsedPhaseSpec(ph); !pp.valid || !pp.set.Has(e.G.Step) {
			return false
		}
	}
	switch staticConditionOf(sv) {
	case condBlank:
		return true
	case condPlayerTurn:
		return e.G.Active == sv.Controller
	case condNotPlayerTurn:
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
		if e.abilityRestrictionApplies(sv, p, id, ab) {
			return true
		}
	}
	// An Effect-delivered CantBeActivated body lives in the continuous
	// registry, which the printed walk above never visits. Walk it in
	// active() order; the view borrows the registration's own slices, so a
	// board with no live CantBeActivated effect allocates nothing.
	active := e.active()
	for i := range active {
		ce := &active[i]
		if cards.StaticModeOf(ce.Restriction) != cards.StaticCantBeActivated ||
			!effects.CantBeActivatedParamsReadable(ce.RestrictParams) {
			continue
		}
		sv := staticView{
			Source: ce.Source, Controller: ce.Controller,
			Params: ce.RestrictParams, SVars: ce.RestrictSVars,
			Remembered: ce.Remembered, rememberedPlayers: ce.RememberedPlayers,
			effectDelivered: true,
		}
		if e.abilityRestrictionApplies(sv, p, id, ab) {
			return true
		}
	}
	return false
}

// abilityRestrictionApplies is the ONE per-view match both CantBeActivated
// delivery routes run: the printed S: walk and the Effect-delivered
// continuous-registry walk above. An Effect view's absent ValidCard$ means
// every card (Forge's default); a printed static keeps its existing read.
func (e *Engine) abilityRestrictionApplies(sv staticView, p state.PlayerID, id state.ObjID, ab *cards.SA) bool {
	if !e.actorMatches(sv, "Activator", p) {
		return false
	}
	if !e.restrictionGateHolds(sv, id) || !e.checkSVarHolds(sv) {
		return false
	}
	if spec := sv.ParamStr(cards.PKValidCard); spec != "" {
		sc := e.staticSpecCtx(sv)
		if sv.effectDelivered {
			sc = e.assignmentStaticSpecCtx(sv)
		}
		if !e.matchesSpec(spec, id, sc) {
			return false
		}
	} else if !sv.effectDelivered {
		return false
	}
	return activatedMatchesValidSA(ab, sv.ParamStr(cards.PKValidSA))
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
	for alt := range strings.SplitSeq(v, ",") {
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
			if !cards.IsManaAbilitySA(ab) {
				return true
			}
			// else: mana abilities are expressly spared; try the next alt
		case activatedValidSACodes.Code(constraint) == activatedValidSAPowerUp:
			// Forge's Activated.PowerUp (SpellAbility.isPowerUp): scopes the
			// restriction to an ability carrying PowerUp$ True. A non-power-up
			// ability is spared and the next alternative is tried.
			if powerUpTrueCodes.Code(ab.ParamStr(cards.PKPowerUp)) == powerUpTrue {
				return true
			}
		case constraint == "ManaAbility" || strings.HasPrefix(constraint, "ManaAbility<"):
			if !cards.IsManaAbilitySA(ab) {
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
				produced := effects.ManaOf(ab).Produced
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

// activatedValidSACode indexes the Activated-kind constraints
// activatedMatchesValidSA evaluates. A value absent from this table keeps the
// fail-closed deny below.
type activatedValidSACode uint16

const (
	activatedValidSAPowerUp activatedValidSACode = iota + 1
)

var activatedValidSACodes = state.NewStrCodes(
	state.StrEntry[activatedValidSACode]{Key: "PowerUp", Val: activatedValidSAPowerUp},
)

// powerUpTrueCode indexes the PowerUp$ parameter's one corpus value.
type powerUpTrueCode uint16

const (
	powerUpTrue powerUpTrueCode = iota + 1
)

var powerUpTrueCodes = state.NewStrCodes(
	state.StrEntry[powerUpTrueCode]{Key: "True", Val: powerUpTrue},
)

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
	return e.costModifiers(p, id, spellScope("")).Apply(e.faceCost(o.Face()))
}

// castWithFlash reports whether an active CastWithFlash static gives p
// permission to cast id at instant speed. It is intentionally shared by every
// zone that can cast a spell; a Vedalken Orrery must not stop working when a
// later alternative permits casting from another zone.
//
// The offer walk passes id's POTENTIAL legal targets: a target-conditional
// grant such as Flash Photography's
// `ValidSA$ Spell.IsTargeting Valid Permanent.YouCtrl` is permission only when
// some legal target can satisfy the restriction. CR 601.2e's recheckIllegal
// re-runs castWithFlashTargets with the ANNOUNCED targets, so a cast that took
// the permission on a non-qualifying target is reversed rather than completed.
func (e *Engine) castWithFlash(p state.PlayerID, id state.ObjID) bool {
	// Fast path: with no CastWithFlash static anywhere (the overwhelming
	// majority of offers), do not pay for a potential-target census.
	if len(e.withSelfStatics(e.activeStatics("CastWithFlash"), id, "CastWithFlash")) == 0 {
		return false
	}
	return e.castWithFlashTargets(p, id, e.costPotentialTargets(p, id, spellScope("")), false)
}

// castWithFlashTargets is castWithFlash against an explicit target list. The
// whole CastWithFlash collection is read through ONE home -- the battlefield
// walk every static consumer uses plus the card's OWN face statics, which
// activeStatics never sees while the card is still in hand (the same two
// sources alternativeCosts reads for a self-carried AlternativeCost) -- so the
// offer and the recheck cannot disagree about which statics grant the
// permission.
func (e *Engine) castWithFlashTargets(p state.PlayerID, id state.ObjID, targets []state.Target, teamworkOffer bool) bool {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false
	}
	for _, sv := range e.withSelfStatics(e.activeStatics("CastWithFlash"), id, "CastWithFlash") {
		if !e.actorMatches(sv, "Caster", p) || !e.staticTimingGate(sv) {
			continue
		}
		// EffectZone$ (Skittering Cicada's EffectZone$ Battlefield): the
		// static functions only while its source sits in the named zone --
		// a battlefield static's permission ends with the source's presence,
		// exactly like every other zone-scoped static.
		if az, ok := sv.Param(cards.PKEffectZone); ok {
			src := e.G.Obj(sv.Source)
			if src == nil || !affectedZoneOK(az, src.Zone) {
				continue
			}
		}
		if !e.spellMatchesValidSA(o.Face(), sv.ParamStr(cards.PKValidSA), id, sv.Source, p, targets, teamworkOffer) {
			continue
		}
		if e.matchesSpec(sv.ParamStr(cards.PKValidCard), id, e.staticSpecCtx(sv)) {
			return true
		}
	}
	return false
}

// castWithFlashAsFace is castWithFlash priced AS face f of id: the
// potential-target census and the self statics read f, not the face the
// object currently displays. The split_alt offer uses it so a target-
// conditional grant is judged against the half actually being cast (CR
// 709.4: each half is cast as its own spell, with its own targets and
// face-local statics), and the fuse offer's per-half timing reads it for
// each half -- so neither half borrows the other's permission. f must be one
// of the object's own faces; offerAsFace prices the live face unchanged
// otherwise, and is a no-op when f IS the live face.
func (e *Engine) castWithFlashAsFace(p state.PlayerID, id state.ObjID, f *cards.Face) bool {
	return e.offerAsFace(id, f, func() bool { return e.castWithFlash(p, id) })
}

// withSelfStatics appends the named statics the card carries on its OWN face
// to a collected list: activeStatics never sees a self-carried static while
// the card is still in hand (the same second source alternativeCosts reads
// for a self-carried AlternativeCost). A self static's source and controller
// are the card itself, so its Caster$/You context resolves relative to the
// caster's own card. base is copied before any append because activeStatics
// returns a cached slice during a legal-actions walk and must never be
// mutated.
//
// Only a self static that names its own EffectZone$ is appended (CR 113.6:
// an ability of a permanent card functions only on the battlefield unless it
// says otherwise, and a battlefield static is already in activeStatics while
// its card is there). Radagast's "you may cast spells as though they had
// flash" therefore does not grant its own card flash from hand, while
// Illusion Spinners' `EffectZone$ All` "this spell" grant does (CR 113.6e);
// the caller still checks the named zone against the source. The same
// default applies to any mode a later caller passes.
func (e *Engine) withSelfStatics(base []staticView, id state.ObjID, mode string) []staticView {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return base
	}
	var count int
	for _, st := range o.Face().Statics {
		if st.Mode == mode && st.HasParam(cards.PKEffectZone) {
			count++
		}
	}
	if count == 0 {
		return base
	}
	out := append([]staticView(nil), base...)
	for _, st := range o.Face().Statics {
		if st.Mode == mode && st.HasParam(cards.PKEffectZone) {
			out = append(out, staticView{Source: id, Controller: o.Controller, Params: st.Params, PS: st.ParamSetOf()})
		}
	}
	return out
}

// hasTargetConditionalFlash reports whether id carries or is granted a
// CastWithFlash static whose ValidSA$ names an `IsTargeting` alternative. It
// is the narrow predicate recheckIllegal uses to decide whether an off-sorcery
// cast's timing rested on a target-conditional grant (a card with no such
// static keeps its existing CR 601.2e behaviour).
func (e *Engine) hasTargetConditionalFlash(p state.PlayerID, id state.ObjID) bool {
	for _, sv := range e.withSelfStatics(e.activeStatics("CastWithFlash"), id, "CastWithFlash") {
		if !e.actorMatches(sv, "Caster", p) {
			continue
		}
		if validSpellHasTargeting(sv.ParamStr(cards.PKValidSA)) || validSpellHasTeamwork(sv.ParamStr(cards.PKValidSA)) {
			return true
		}
	}
	return false
}

// flashGrantCoversTargets is the CR 601.2e enforcement half of a target-
// conditional CastWithFlash grant, read for ONE face of a split card: it
// answers false when face f's off-sorcery timing could have rested on an
// IsTargeting or Teamwork condition AND the announced target or paid choice
// does not satisfy that grant. A face with its own unconditional timing (instant, Flash,
// MayFlashSac's rider) never rested on the grant; a grant without an
// IsTargeting alternative imposes no target requirement. The reads are
// face-scoped (offerAsFace), so a fused cast's alternate half is judged
// against ITS face and ITS stage's targets -- the same read the offer ran
// through castWithFlashAsFace, keeping offer and enforcement on one
// interpretation.
func (e *Engine) flashGrantCoversTargets(p state.PlayerID, id state.ObjID, f *cards.Face, targets []state.Target) bool {
	if f == nil || f.IsInstant() || e.hasKeywordH(id, kwhFlash) || mayFlashSacFace(f) {
		return true
	}
	return e.offerAsFace(id, f, func() bool {
		return !e.hasTargetConditionalFlash(p, id) || e.castWithFlashTargets(p, id, targets, false)
	})
}

const validSATeamworkConstraint = "Teamwork"

func validSpellHasTeamwork(raw string) bool {
	for alt := range strings.SplitSeq(raw, ",") {
		if strings.TrimSpace(alt) == "Spell."+validSATeamworkConstraint {
			return true
		}
	}
	return false
}

// validSpellHasTargeting reports whether a ValidSA$/ValidSpell$ OR-list
// carries an `IsTargeting` alternative under the Spell base. Both parameters
// spell that shape identically (Flash Photography's
// `ValidSA$ Spell.IsTargeting Valid Permanent.YouCtrl`, Head of the Class's
// `ValidSpell$ Spell.IsTargeting Valid Creature`).
func validSpellHasTargeting(raw string) bool {
	for alt := range strings.SplitSeq(raw, ",") {
		kind, constraint, _ := strings.Cut(strings.TrimSpace(alt), ".")
		if kind == "Spell" && strings.HasPrefix(strings.TrimSpace(constraint), "IsTargeting") {
			return true
		}
	}
	return false
}

// presentGate evaluates one IsPresent spec, counted over PresentZone$
// (default battlefield, countStaticPresent), against PresentCompare$
// (default GE1). The ONE static IsPresent$ gate: staticTimingGate, the
// Continuous gate and costStaticApplies all read it, so an EQ0/GEn threshold
// or a hand/graveyard zone cannot mean different things on different paths.
func (e *Engine) presentGate(sv staticView, spec string) bool {
	return e.presentGateSkipping(sv, spec, 0)
}

// presentGateSkipping is presentGate without one card in the zones it counts
// outside the battlefield.
func (e *Engine) presentGateSkipping(sv staticView, spec string, skip state.ObjID) bool {
	n := e.countStaticPresent(sv, spec, skip)
	cmp := sv.ParamStr(cards.PKPresentCompare)
	if cmp == "" {
		cmp = "GE1"
	}
	return comparePresent(n, e.presentCompareFor(cmp, sv.Source, sv.Controller))
}

// staticTimingGate evaluates the static conditions that can decide whether a
// CastWithFlash permission exists before a spell is announced. An unknown
// gate fails closed: granting instant timing without proving the script's
// condition would permit an illegal cast.
func (e *Engine) staticTimingGate(sv staticView) bool {
	if !e.classBandGateHolds(sv.ParamStr(cards.PKClassBand), sv.Source) {
		return false
	}
	if spec, ok := sv.Param(cards.PKIsPresent); ok && !e.presentGate(sv, spec) {
		return false
	}
	if spec, ok := sv.Param(cards.PKIsPresent2); ok && !e.presentGate(sv, spec) {
		return false
	}
	if !e.checkSVarHolds(sv) {
		return false
	}
	switch staticConditionOf(sv) {
	case condBlank, condPlayerTurn:
		if sv.ParamStr(cards.PKCondition) == "PlayerTurn" && e.G.Active != sv.Controller {
			return false
		}
	case condFerocious:
		found := false
		for _, id := range e.G.Zone(state.ZBattlefield, sv.Controller) {
			if e.IsCreature(id) && e.Derived(id).Power >= 4 {
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
	if phase := strings.TrimSpace(sv.ParamStr(cards.PKPhases)); phase != "" && !(strings.Contains(phase, "End of Turn") && e.G.Step == state.StepEnd) {
		return false
	}
	if turn := strings.TrimSpace(sv.ParamStr(cards.PKPlayerTurn)); turn != "" {
		if turn != "Opponent" || e.G.Active == sv.Controller {
			return false
		}
	}
	return true
}

func (e *Engine) countStaticPresent(sv staticView, spec string, skip state.ObjID) int {
	zones, ok := presentZoneFromParam(sv.ParamStr(cards.PKPresentZone))
	if !ok {
		return 0
	}
	sc := e.staticSpecCtx(sv)
	n := 0
	for _, zone := range zones {
		if zone == state.ZBattlefield {
			n += e.countPresentCtx(spec, sv.Source, sv.Controller, sc)
			continue
		}
		e.forEachObject(func(id state.ObjID) {
			o := e.G.Obj(id)
			if o != nil && o.Zone == zone && id != skip && e.matchesSpec(presentSpecForZone(spec, zone), id, sc) {
				n++
			}
		})
	}
	return n
}

// presentZoneFromParam maps a Forge PresentZone$ value onto the state.Zone set
// the IsPresent$ count family scans. An ABSENT value is the battlefield -- the
// default every card without PresentZone$ reads, so the empty string is a valid
// mapping, not an unknown one. Named zones are parsed by effects.ParseZones,
// shared by the static, delayed, trigger and activation gates, so each caller
// counts the same set. Wildcards are not meaningful for PresentZone$ and fail
// closed. forEachObject walks every seat's library, hand, battlefield,
// graveyard, exile, stack and command zone (Living Conundrum's Library, Kefnet
// the Mindful's hand, Ketramose's exile, Molten Disaster's stack).
func presentZoneFromParam(zone string) ([]state.Zone, bool) {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return []state.Zone{state.ZBattlefield}, true
	}
	zones, all, ok := effects.ParseZones(zone)
	return zones, ok && !all
}

// presentZoneCounter is the one engine capability presentCountInZones needs,
// so the free helper below does not take an *Engine.
type presentZoneCounter interface {
	presentZoneCount(zone state.Zone, spec string, source state.ObjID, you state.PlayerID) int
}

// presentCountInZones counts spec matches in every zone a PresentZone$ value
// names (the battlefield when absent), the single read the IsPresent$ gates
// that carry no static-specific context share. known is false for a
// PresentZone$ word presentZoneFromParam does not map; the caller must then
// fail closed (n is 0), including past a PresentCompare$ a zero would satisfy.
func presentCountInZones(c presentZoneCounter, zoneParam, spec string, source state.ObjID, you state.PlayerID) (n int, known bool) {
	zones, known := presentZoneFromParam(zoneParam)
	if !known {
		return 0, false
	}
	for _, zone := range zones {
		n += c.presentZoneCount(zone, spec, source, you)
	}
	return n, true
}

// spellMatchesValidSA checks the spell-side subset of Forge's ValidSA grammar.
// Activated-only constraints are not knowable before announcing a spell and
// therefore do not accidentally grant flash timing. id is the card the cast
// offers and staticSource the static's source: the "Spell.Self" form (115
// corpus lines, all on self-granting AlternativeCost statics, Daze the
// most-played) means the affected card itself is the spell -- true exactly
// when the cast card IS the static's source (the card's own S: line, where
// alternativeCosts builds the view with source == id), false for a grant from
// another permanent.
//
// The target-conditional form (`Spell.IsTargeting <spec>`, Flash Photography
// and Timely Ward) is evaluated through effects' ONE `Spell.IsTargeting`
// grammar against the targets argument: the offer passes the potential legal
// targets and CR 601.2e's recheck passes the announced ones. A nil/empty
// target list matches nothing, so an IsTargeting alternative never grants
// unconditional timing. you is the caster the target spec's You clause binds
// (never the granting static's controller). Constraint values beyond Self and
// IsTargeting and Teamwork (e.g. XCostLE3) remain unimplemented and fail
// closed.
func (e *Engine) spellMatchesValidSA(f *cards.Face, raw string, id, staticSource state.ObjID, you state.PlayerID, targets []state.Target, teamworkOffer bool) bool {
	if strings.TrimSpace(raw) == "" {
		return true
	}
	for alt := range strings.SplitSeq(raw, ",") {
		kind, constraint, _ := strings.Cut(strings.TrimSpace(alt), ".")
		switch spellMatchesValidSACodes.Code(string(kind)) {
		case spellMatchesValidSASpell:
			if constraint == "" {
				return true
			}
			if constraint == "Self" && id == staticSource {
				return true
			}
			if constraint == validSATeamworkConstraint {
				// During announcement the selected tap cost is known, but its
				// CastInfo is not emitted until payment. The target census and
				// CR 601.2e recheck must use that committed choice, not the
				// prospective offer or a printed keyword.
				if teamworkOffer || e.cast != nil && e.cast.card == id && castAnswerCodes.Code(e.cast.mode) == castAnswerTeamworkMode && e.cast.teamworkPaid {
					return true
				}
				o := e.G.Obj(id)
				if e.cast == nil && o != nil && o.Zone == state.ZStack && o.TeamworkPaid {
					return true
				}
			}
			if strings.HasPrefix(constraint, "IsTargeting") {
				sc := e.specCtx(staticSource, you)
				sc.AsStack = true
				sc.ProposedTargets = targets
				if e.matchesSpec("Spell."+constraint, id, sc) {
					return true
				}
			}
		case spellMatchesValidSAInstant:
			if constraint == "" && f.IsInstant() {
				return true
			}
		case spellMatchesValidSASorcery:
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
//
// ActivationPhases$ and its rider qualifiers are consulted here too, so the
// cast window binds to EVERY way the card is cast (hand, may-play, command
// zone, flashback/harmonize/escape, adventure, foretell) rather than to the
// hand walk alone. This is a pure read; the helper never emits.
func (e *Engine) spellTimingOK(p state.PlayerID, id state.ObjID, f *cards.Face, sorcery bool) bool {
	ok, needFlash := e.spellTimingPre(p, id, f, sorcery)
	if !ok {
		return false
	}
	return !needFlash || e.castWithFlashAsFace(p, id, f) || mayPlayFlashGranted(e.activeStatics, e.mayPlayGrantedBy, e.G.Obj(id), p)
}

// spellTimingPre is spellTimingOK's decision up to the CastWithFlash lookup:
// ok reports the timing is satisfiable so far, and needFlash reports that it
// is satisfiable EXACTLY when castWithFlash grants it (the other arms did not
// already allow the cast). It is the one home for the predicate so the walk's
// bit-fast entry (legal_walk_flash.go, w.spellTimingOK) cannot drift from the
// engine's. It is a pure read; the helper never emits.
func (e *Engine) spellTimingPre(p state.PlayerID, id state.ObjID, f *cards.Face, sorcery bool) (ok, needFlash bool) {
	if f == nil {
		return sorcery, false
	}
	sa := f.SpellAbility()
	// IsPresent$ / PresentCompare$ / PresentZone$ on the spell's own SA is a
	// cast restriction ("Cast this spell only if a creature is attacking you",
	// Confront the Assault; "...only if you control two or more Vampires",
	// Feast of Blood). An unmet condition withholds the offer (CR 601.3)
	// rather than fizzling at resolution. Only the spell's own SA is read: a
	// SubAbility's IsPresent$ is a resolution condition, not a restriction.
	if !e.activationPhasesOK(p, sa) || (sa != nil && sa.MayHaveAnyParam(spellPresentKeys) && !e.abilityPresentHolds(p, id, sa)) {
		return false, false
	}
	if sorcery || f.IsInstant() || e.spellFaceHasFlash(id, f) || mayFlashSacFace(f) {
		return true, false
	}
	return true, true
}

// spellFaceHasFlash reads the Flash keyword off the face being cast (CR 715.3a,
// CR 720.3a). An alternate face needs the probe only when its own printing or
// the live face's precheck, which covers every grant on the board, leaves Flash
// possible.
func (e *Engine) spellFaceHasFlash(id state.ObjID, f *cards.Face) bool {
	if o := e.G.Obj(id); o == nil || o.Face() == f {
		return e.hasKeywordH(id, kwhFlash)
	}
	if !f.HasKeywordRef(kwRefFlash) && !e.mayHaveDerivedKeywordH(id, kwhFlash) {
		return false
	}
	return e.offerAsFace(id, f, func() bool { return e.hasKeywordH(id, kwhFlash) })
}

// spellPresentKeys are the keys the spell cast gate reads for IsPresent$.
var spellPresentKeys = cards.ParamMaskOf(cards.PKIsPresent)

type spellMatchesValidSACode uint16

const (
	spellMatchesValidSASpell spellMatchesValidSACode = iota + 1
	spellMatchesValidSAInstant
	spellMatchesValidSASorcery
)

var spellMatchesValidSACodes = state.NewStrCodes(
	state.StrEntry[spellMatchesValidSACode]{Key: "Spell", Val: spellMatchesValidSASpell},
	state.StrEntry[spellMatchesValidSACode]{Key: "Instant", Val: spellMatchesValidSAInstant},
	state.StrEntry[spellMatchesValidSACode]{Key: "Sorcery", Val: spellMatchesValidSASorcery},
)
