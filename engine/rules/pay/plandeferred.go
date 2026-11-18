package pay

import (
	"math/bits"
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Taps-deferred census gap and its second pass.
//
// The census (appendUnitAlternatives) drops every alternative the tier
// classifier defers, so a source whose tap a watcher watches -- Brink of
// Disaster's "when enchanted permanent becomes tapped, destroy it" on the
// enchanted land -- contributes no unit, and a cast only that source can fund
// is never planned. The watcher is not a mana ability, so per CR 601.2g with
// 405.6c it is queued during the announcement and resolves only after the
// spell is on the stack: it cannot alter the payment, and the production it
// watches is ordinary. Only a ProduceMana replacement (priced beside the
// production) and a triggered mana ability's folded mana (added with the tap)
// act on the payment itself, and both are the tier classifier's separate
// verdicts.
//
// The kernel plans such a cast in the second pass its Taps-deferred gap route
// runs when the first pass proved no plan (dirac mana_tapsdefer.go planTiered,
// scriptTapsDefer): the deferred source is a census gap, activated like any
// other unit. planPaymentCostWithDemand's third phase is that pass, and
// PaymentPlanGapStep is how a gap step resolves against it.

// GapUnit is one second-pass unit: a source that contributed no census unit
// (every alternative deferred) and the gap alternatives that fund it.
type GapUnit struct {
	ID   state.ObjID
	Alts []Alt
}

// paymentPlanGapGate reports whether alt, a census alternative the tier
// classifier deferred, is a gap unit's alternative: the deferral is the
// interference class and every object deferring it only watches the tap
// (detail "source:interference...", Eval.SourceInterference's watcher verdict: a ProduceMana
// replacement -- Mana Reflection, Contamination -- or a mana-adding
// registration acts on the payment itself and stays deferred), the ability's cost is a bare tap and the
// production is exactly one unit of one symbol. That is the shape the kernel's
// gap route funds (dirac mana_tapsdefer.go: "Any other production stays
// unmodelled"); a shape deferral ("source:target", "source:rider", ...) and a
// multi-unit or choice production stay deferred, exactly as the kernel's
// first pass leaves them. A RestrictValid$ ability is no gap unit either: the
// kernel's plain gap floats plain mana and leaves a restricted source out
// (dirac scriptTapsDeferWalk: "the ability is restricted"), and the census
// never admits it without a payment to match (withhold, restrictedRefused), so
// Steelswarm Operator under a Cryoshatter funds no artifact spell.
func paymentPlanGapGate(e Engine, p state.PlayerID, id state.ObjID, alt Alt) bool {
	if alt.Tier != TierDeferred || alt.Ma == nil {
		return false
	}
	if _, _, detail := PaymentPlanAbilityTier(e, p, id, alt.Ma); !interferenceDetail(detail) {
		return false
	}
	if _, _, _, _, watcher := e.Eval().SourceInterference(id, alt.Ma, ""); !watcher {
		return false
	}
	if !PaymentPlanTapOnlyCost(AbilityCompiledCost(e, alt.Ma).Cost) || e.Eval().ManaStatic(alt.Ma).RestrictValid {
		return false
	}
	total := 0
	for _, n := range alt.Mana {
		total += int(n)
	}
	return total == 1
}

// detailInterference is the tier detail the classifier gives a source a
// watcher or replacement would act on ("source:interference", then the acting
// object's name); the check reads a label, not a game fact, so it is a byte
// compare rather than a strings call the ratchet counts.
const detailInterference = "source:interference"

func interferenceDetail(detail string) bool {
	return len(detail) >= len(detailInterference) && detail[:len(detailInterference)] == detailInterference
}

// paymentPlanGapRows builds u's gap alternatives: the census alternatives the
// tier classifier deferred that pass paymentPlanGapGate, built by the census's
// own row conversion (appendUnitAltRows) at TierDeferred with no consequence
// (an interference deferral discloses none). u must have contributed no census
// unit.
func paymentPlanGapRows(e Engine, p state.PlayerID, u WindowUnit) []Alt {
	payer := p
	if o := e.Game().Obj(u.ID); o != nil {
		payer = o.Controller
	}
	zoneSeq, creature := PaymentSourceZoneSeq(e, u.ID),
		slices.Contains(e.Chars().DerivedTypes(u.ID), "Creature")
	var rows []Alt
	for _, wa := range u.Alts {
		rows = appendUnitAltRows(e, rows, u, wa, TierDeferred, Consequence{}, zoneSeq, creature)
	}
	kept := rows[:0:0]
	for _, alt := range rows {
		if paymentPlanGapGate(e, payer, u.ID, alt) {
			kept = append(kept, alt)
		}
	}
	// The rows carry the source's flexibility as the census's last-resort
	// rows do (appendUnitAlternatives): FlexAll is the number of distinct mana
	// types over every kept row (key 5 in the relaxed passes, which phase 3
	// reads), and Colours stays 0 -- a source with no normal alternative has
	// none. Without it every gap source ranks as inflexible, so a source that
	// makes two types (Turbulent Fen) is spent before a plain land, where the
	// kernel's second pass taps in battlefield order.
	var all uint8
	for _, alt := range kept {
		for i, n := range alt.Mana {
			if n > 0 {
				all |= 1 << i
			}
		}
	}
	for i := range kept {
		kept[i].FlexAll = bits.OnesCount8(all)
	}
	return kept
}

// gapUnitOf is p's census unit for id and whether it carries gap rows.
func gapUnitOf(e Engine, p state.PlayerID, id state.ObjID) (GapUnit, bool) {
	for _, u := range PaymentPlanQueryUnits(e, p) {
		if u.ID != id {
			continue
		}
		if alts := PaymentPlanQueryAlternatives(e, u); len(alts) > 0 {
			return GapUnit{}, false // already a census unit of the first pass
		}
		rows := paymentPlanGapRows(e, p, u)
		if len(rows) == 0 {
			return GapUnit{}, false
		}
		return GapUnit{ID: id, Alts: rows}, true
	}
	return GapUnit{}, false
}

// PaymentPlanGapUnits is the second pass's census over p's sources: every
// census unit that contributed no census alternative and carries gap rows.
// Computed once per query scope and player, like the census itself; the rows
// park in the scope's alternative arena and verify mode compares the cached
// census against a fresh build.
func PaymentPlanGapUnits(e Engine, p state.PlayerID) []GapUnit {
	q := e.Session().PlanQuery
	if !q.Valid(e.Log()) {
		return paymentPlanGapUnitsFresh(e, p)
	}
	if units, ok := q.Gaps[p]; ok {
		return units
	}
	var out []GapUnit
	for _, u := range PaymentPlanQueryUnits(e, p) {
		if alts := PaymentPlanQueryAlternatives(e, u); len(alts) > 0 {
			continue
		}
		rows := paymentPlanGapRows(e, p, u)
		if len(rows) > 0 {
			out = append(out, GapUnit{ID: u.ID, Alts: gapStore(q, u.ID, rows)})
		}
	}
	if q.Gaps == nil {
		q.Gaps = map[state.PlayerID][]GapUnit{}
	}
	q.Gaps[p] = out
	if e.Verify() {
		if fresh := paymentPlanGapUnitsFresh(e, p); !sameGapUnits(out, fresh) {
			panic("payment plan gap census: cached gap units are stale")
		}
	}
	return out
}

// paymentPlanGapUnitsFresh is PaymentPlanGapUnits without the caches, for
// verify mode.
func paymentPlanGapUnitsFresh(e Engine, p state.PlayerID) []GapUnit {
	var out []GapUnit
	for _, u := range PaymentPlanQueryUnits(e, p) {
		if alts := PaymentPlanQueryAlternatives(e, u); len(alts) > 0 {
			continue
		}
		rows := paymentPlanGapRows(e, p, u)
		if len(rows) > 0 {
			out = append(out, GapUnit{ID: u.ID, Alts: rows})
		}
	}
	return out
}

// sameGapUnits is the verify-mode comparison of two gap censuses: the same
// sources in order, each with equal alternatives (PlanSameAlternative).
func sameGapUnits(a, b []GapUnit) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || !slices.EqualFunc(a[i].Alts, b[i].Alts, PlanSameAlternative) {
			return false
		}
	}
	return true
}

// gapStore parks rows in the query scope's alternative arena, the storage the
// plan call's search reads them from and a later step resolution re-reads
// (no alternative outlives its scope).
func gapStore(q *PlanQuery, id state.ObjID, rows []Alt) []Alt {
	at := len(q.AltArena)
	q.AltArena = append(q.AltArena, rows...)
	stored := q.AltArena[at:len(q.AltArena):len(q.AltArena)]
	if q.GapAlts == nil {
		q.GapAlts = map[state.ObjID][]Alt{}
	}
	q.GapAlts[id] = stored
	return stored
}

// PaymentPlanGapStep resolves pa against its source's gap alternatives, the
// lookup a gap plan's validation and execution make for a step the census's
// own rows never listed. The source must still have no census alternative:
// a source whose deferral left (the watcher died) is an ordinary census unit,
// and the primary lookup answered it first.
func PaymentPlanGapStep(e Engine, pa decision.PaymentActivation) (Alt, bool) {
	q := e.Session().PlanQuery
	if q.Valid(e.Log()) {
		if alts, ok := q.GapAlts[pa.Source]; ok {
			return gapStepMatch(alts, pa)
		}
	}
	o := e.Game().Obj(pa.Source)
	if o == nil {
		return Alt{}, false
	}
	unit, ok := gapUnitOf(e, o.Controller, pa.Source)
	if !ok {
		return Alt{}, false
	}
	rows := unit.Alts
	if q.Valid(e.Log()) {
		rows = gapStore(q, pa.Source, rows)
		if e.Verify() {
			if fresh, ok := gapUnitOf(e, o.Controller, pa.Source); !ok || !slices.EqualFunc(rows, fresh.Alts, PlanSameAlternative) {
				panic("payment plan gap step: cached gap rows are stale")
			}
		}
	}
	return gapStepMatch(rows, pa)
}

// gapStepMatch is PaymentPlanStepAlternative's identity-and-production pair
// over one row list.
func gapStepMatch(alts []Alt, pa decision.PaymentActivation) (Alt, bool) {
	for _, candidate := range alts {
		if candidate.Ma != nil && candidate.Activation.Ability == pa.Ability && candidate.Activation.Produces == pa.Produces &&
			ConsequenceEqual(candidate.Consequence, pa.Consequence) {
			return candidate, true
		}
	}
	return Alt{}, false
}

// PlanUsesGapStep reports whether plan's steps include a gap step: any step
// that resolves to a TierDeferred alternative. ValidateCastPayment's phase
// rule prices the submitted plan's tiers against the planner's own best plan.
func PlanUsesGapStep(e Engine, units []WindowUnit, plan decision.PaymentPlan) bool {
	for _, pa := range plan.Activations {
		if step, ok := PaymentPlanStepAlternative(e, units, pa); ok && step.Tier == TierDeferred {
			return true
		}
	}
	return false
}
