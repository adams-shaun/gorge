package rules

import (
	"os"
	"strings"
	"sync/atomic"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// autoPayFastVerify is the fast path's proof switch: when it is set
// (GORGE_AUTOPAY_FAST_VERIFY=1, read once at package init, or toggled
// directly by a sequential test), every fast-path ACCEPT also runs the
// ordinary planner path and panics on disagreement. Under it the fast path's
// verdicts are identical to the planner's by construction, so a digest taken
// with it on proves the offer surface is unchanged.
var autoPayFastVerify = os.Getenv("GORGE_AUTOPAY_FAST_VERIFY") != ""

// autoPayFastDisabled disables the fast path entirely
// (GORGE_AUTOPAY_FAST_DISABLE=1, read once at package init). It exists so the
// offer-equivalence method is reproducible without editing the tree:
// a digest taken with it set is the planner-only baseline the fast path's
// digest must equal byte-for-byte.
var autoPayFastDisabled = os.Getenv("GORGE_AUTOPAY_FAST_DISABLE") != ""

// autoPayFastAccepts counts fast-path accepts, for measurement (a zero
// count after a benchmark run means the fast path is dead on that board).
var autoPayFastAccepts atomic.Int64

// autoPayFast is the exact fast path for autoPayOffer (legal_walk_autopay.go):
// it accepts ONLY when the printed ability's mana cost is a plain
// coloured/generic cost (plus the ability's own self-tap) and a greedy
// assignment of the payer's ordinary free-tap mana sources provably covers it.
//
// The ordinary path runs the payment planner (planAbilityPayment) and then the
// engine's own offer gate. This path proves the same verdict more cheaply by
// exploiting a plain cost's structure:
//
//   - the planner's only cost-shape rejections for a plain cost are the global
//     mana effect and a nil plan (planComposedCost: PlanCostDetail is empty,
//     there are no hybrid/phyrexian/snow/X parts, no TapPermanent/Sac
//     reservations, and planLeavesCostPayable is trivially true);
//   - the planner admits a source's alternative exactly when its tier is
//     TierNormal and its cost is TapOnly (appendUnitAlternatives);
//   - a covering assignment over that admitted subset is a witness the
//     planner's complete phase-1 search finds, so the planner returns the same
//     accept.
//
// Every non-mana part of the cost is still read for real by the confirming
// gate below, so an ability whose cost carries a part this path does not model
// falls through to the planner. ab may be nil; base is the RAW (pre-compose)
// cost the walk gated on.
func (w *legalWalk) autoPayFast(p state.PlayerID, id state.ObjID, base Cost, ab *cards.SA, statics costStaticViews) bool {
	if autoPayFastDisabled {
		return false
	}
	// PowerUp's ReduceCost is applied by the walk but not by the planner, so
	// the walk's base would differ from the planner's cost. Exclude it; the
	// planner owns those abilities.
	if ab != nil && strings.EqualFold(strings.TrimSpace(ab.ParamStr(cards.PKPowerUp)), "True") {
		return false
	}
	scope := abilityScope(ab)
	composed := w.e.offerCostForUsing(statics, p, id, base, scope)
	// Only a plain mana cost, plus the ability's own tap and a sacrifice or
	// exile of the source itself (each withholds id from the census below).
	// Anything else (life, X, hybrid, phyrexian, snow, sacrificing another
	// permanent, tap-a-permanent, ...) is the planner's.
	if !autoPayFastPlain(&composed, id) {
		return false
	}
	if global, _ := w.e.paymentPlanGlobalManaEffect(p, id); global {
		return false
	}
	units := w.autoPlainUnits(p)
	// witness is the pool plus every chosen source's production; have is the
	// same sum with the coloured requirement consumed as it is covered, so the
	// leftover at the end can absorb the generic part.
	witness := w.e.G.Players[p].Pool
	have := witness
	needCol := composed.Colored
	needGen := composed.Generic
	for i := range units {
		u := &units[i]
		if u.ID == id {
			continue // the ability's own tap or self-sacrifice withholds its mana ability
		}
		best := -1
		var bestMana state.Mana
		bestScore := int64(-1)
		for ai := range u.Alts {
			a := u.Alts[ai]
			if !w.autoPlainAltOK(p, u.ID, a) {
				continue
			}
			m := a.Mana()
			if score := autoPayCoverScore(m, needCol); score > bestScore {
				bestScore, best, bestMana = score, ai, m
			}
		}
		if best < 0 {
			continue
		}
		witness = addManaVec(witness, bestMana)
		have = addManaVec(have, bestMana)
		autoPayConsumeColored(&have, &needCol)
		if needCol == (state.Mana{}) && have.Total() >= needGen {
			break
		}
	}
	if needCol != (state.Mana{}) || have.Total() < needGen {
		return false
	}
	// The engine's own offer gate confirms the witness exactly as the ordinary
	// path does, so the fast path and the planner cannot disagree about the
	// non-mana parts or the pool it produces.
	probe := base
	if !w.e.offerCastableUsing(statics, p, id, &probe, scope, true, &witness) {
		return false
	}
	autoPayFastAccepts.Add(1)
	return true
}

// autoPayFastPlain reports whether composed's only non-mana parts are the
// ability's own tap and a sacrifice or exile of the source id itself. Every
// other field must be zero (Cost.Equal compares every field, so a new field
// cannot silently slip past).
//
// A self-sacrifice is exact here: planComposedCost keeps id out of every
// consuming alternative (kept) and adds no reservation part, and
// planLeavesCostPayable then rejects only a plan that consumes id, which the
// fast path's witness never touches (the cover loop skips id). planComposedCost
// does not model an Exile part at all, so the confirming offer gate is the only
// reader of a self-exile on both paths.
func autoPayFastPlain(composed *Cost, id state.ObjID) bool {
	plain := Cost{Colored: composed.Colored, Generic: composed.Generic, Tap: composed.Tap}
	if autoPaySelfParts(composed.Sac, id) {
		plain.Sac = composed.Sac
	}
	if autoPaySelfParts(composed.Exile, id) {
		plain.Exile = composed.Exile
	}
	return composed.Equal(&plain)
}

// autoPaySelfParts reports whether every part names the source itself once
// (pay.PlanSelfCost) with a fixed amount.
func autoPaySelfParts(parts []CostPart, id state.ObjID) bool {
	for _, part := range parts {
		if part.Announced || part.Dyn != "" || !pay.PlanSelfCost(part, id) {
			return false
		}
	}
	return true
}

// autoPlainUnits is the walk's lazily built payment-window census
// (windowManaUnits), shared by every ability the walk offers. It is the same
// census the planner's phase 1 reads, so the two cannot drift.
func (w *legalWalk) autoPlainUnits(p state.PlayerID) []windowManaUnit {
	if !w.autoPlainSet || w.autoPlainP != p {
		w.autoPlain = w.e.windowManaUnitsOnly(p, w.e.G.Zone(state.ZBattlefield, p))
		w.autoPlainP = p
		w.autoPlainSet = true
	}
	return w.autoPlain
}

// autoPlainAltOK reports whether alt is one of the ordinary free-tap mana
// alternatives the planner's phase 1 admits as TierNormal: a concrete
// (non-Any) production with a positive amount, a TapOnly activation cost and a
// normal source shape (no interference). The planner may ADD mana to such an
// alternative (addTapExtra), so a witness built only from these is always
// reachable by the planner's search.
func (w *legalWalk) autoPlainAltOK(p state.PlayerID, id state.ObjID, a windowManaAlt) bool {
	if !pay.PlanAltOK(a) {
		return false
	}
	if !w.e.manaStaticOf(a.Ma).TapOnly {
		return false
	}
	if _, ok := pay.PaymentAbility(w.e.G, id, a.Ma); !ok {
		return false
	}
	tier, _, _ := pay.PaymentPlanAbilityTier(asPayer(w.e), p, id, a.Ma)
	return tier == pay.TierNormal
}

// autoPayCoverScore ranks one alternative by how much of the remaining
// coloured requirement it covers, breaking ties toward more total production
// (which can absorb generic). It is a heuristic: soundness never depends on the
// choice, only on whether the final witness covers.
func autoPayCoverScore(m, needCol state.Mana) int64 {
	var score int64
	for i := range m {
		n := m[i]
		if n > needCol[i] {
			n = needCol[i]
		}
		score += int64(n)*1000 + int64(m[i])
	}
	return score
}

// autoPayConsumeColored pays as much of needCol as have can, slot by slot.
func autoPayConsumeColored(have, needCol *state.Mana) {
	for i := range have {
		if needCol[i] <= 0 || have[i] <= 0 {
			continue
		}
		n := have[i]
		if needCol[i] < n {
			n = needCol[i]
		}
		have[i] -= n
		needCol[i] -= n
	}
}

// addManaVec returns a+b.
func addManaVec(a, b state.Mana) state.Mana {
	for i := range a {
		a[i] += b[i]
	}
	return a
}
