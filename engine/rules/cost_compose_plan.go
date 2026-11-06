package rules

import (
	"github.com/adams-shaun/gorge/engine/state"
)

// The potential walk's per-call compose plan. potentialCostModsUsing prices
// one candidate target assignment at a time, and every assignment's compose
// re-runs the whole gate chain of every static it prices. The chain's HEAD
// (costStaticGatePrefixCore: class band, Type$, actor, OnlyFirstSpell$, the
// ValidCard$ branch, FirstForetell$) reads neither the pass's targets nor
// its potential/announced-X mode, so its verdict is the same for every
// assignment of one call: the first compose that reaches the chain records
// it per static, and the later assignments skip a recorded denial outright
// and run only the target-dependent tail (costStaticGateTail) for a recorded
// pass. The per-state compose-answer memo (cost_compose_memo.go) serves
// whole answers when a question repeats at one state; the plan serves the
// head's slice of the questions the memo misses, which are most of them
// (a per-assignment question is asked once per state).
//
// The plan is per CALL, never per state: it is created by
// potentialCostModsUsing, consulted only by that call's own composes (same
// payer, object, scope and announced X, potential pass), and dropped when
// the call returns. It lives in engineScratch because it is consulted
// through the engine's compose path, not threaded through five signatures.

// costComposePlan is one potentialCostModsUsing call's recorded head
// verdicts. skip holds a verdict per static VISITED, in the compose body's
// visit order (raise, reduce, set): the body visits every static of each
// collection on every compose (a denial continues, never breaks), so the
// order is the same for every assignment and the cursor indexes it.
type costComposePlan struct {
	p     state.PlayerID
	id    state.ObjID
	scope costScope
	x     int32
	// collect is true until one compose has reached the gate chain: that
	// compose records every static's head verdict (potentialCostModsUsing
	// then flips the plan to serve). A compose served from the per-state
	// compose memo never reaches the chain, so the first memo miss collects.
	collect bool
	// serve is set once a compose has collected; from then on the call's
	// composes consult skip.
	serve bool
	// touched says a compose actually reached the chain in collect mode (a
	// memo serve leaves the plan empty and unusable).
	touched bool
	// skip is the recorded head verdict per visited static. True = the head
	// denies the static for EVERY assignment of the call, so the compose
	// answers (false, true) without running any gate -- the verdict the full
	// chain would reach for this assignment, since the head is
	// target-independent and a head denial is target-independent too.
	skip []bool
	// i is the visit cursor of the compose in flight (reset at the eval's
	// entry, advanced once per visited static).
	i int
	// prov is the cast-provenance capture the collecting compose ended with
	// (e.CostProvenanceSeen): the capture is set by the head's ValidCard$
	// branch and is assignment-independent, so the serve passes -- which skip
	// the head -- restore it from here.
	prov bool
	// tmatch memoises the tail's per-(static, candidate) ValidTarget$
	// object-match verdicts: one call's assignments share the same candidates
	// and specs, so the matches the earlier assignments' tails computed serve
	// the later ones.
	tmatch []costTargetMatch
}

// costTargetMatch is one remembered ValidTarget$ verdict: the plan cursor of
// the static whose tail asked, the candidate it was asked about and the
// verdict.
type costTargetMatch struct {
	si int32
	t  state.Target
	m  bool
}

// targetMatch answers the memo, or reports a miss.
func (plan *costComposePlan) targetMatch(si int32, t state.Target) (bool, bool) {
	for i := range plan.tmatch {
		if plan.tmatch[i].si == si && plan.tmatch[i].t == t {
			return plan.tmatch[i].m, true
		}
	}
	return false, false
}

// rememberTargetMatch stores one verdict (append-only within the call).
func (plan *costComposePlan) rememberTargetMatch(si int32, t state.Target, m bool) {
	plan.tmatch = append(plan.tmatch, costTargetMatch{si: si, t: t, m: m})
}

// costComposePlanFor returns the active plan for a compose, or nil. Only a
// potential pass for the plan's own question consults the plan: the recorded
// verdicts are answers about that question's statics, and the accept gate's
// own reads (a base compose, a pricing read) must not serve from them. The
// plan is returned in both of its phases -- the collecting compose itself
// must see it to record the verdicts.
func (e *Engine) costComposePlanFor(p state.PlayerID, id state.ObjID, scope costScope, x int32, potential bool) *costComposePlan {
	plan := e.costComposePlan
	if plan == nil || !potential ||
		plan.p != p || plan.id != id || plan.scope != scope || plan.x != x {
		return nil
	}
	return plan
}

// costStaticGatePlanned is costStaticGate under a compose plan. Collecting
// (the first compose that reaches the chain), it runs the chain exactly as
// costStaticGate does -- the head through costStaticGatePrefix, the tail
// through costStaticGateTail -- and records the head's verdict. Serving, a
// recorded denial answers (false, true) with no gate run at all and a
// recorded pass runs the tail alone; a static beyond the recorded prefix
// (unreachable: the body visits every static of each collection on every
// compose) falls back to the full chain. Verify mode recomputes every
// planned gate through the full chain (costComposePlanVerifyGate).
func (e *Engine) costStaticGatePlanned(sv staticView, mode string, p state.PlayerID, id state.ObjID, scope costScope, targets []state.Target, xBound bool, plan *costComposePlan) (ok, indepFail bool) {
	if plan == nil {
		return e.costStaticGate(sv, mode, p, id, scope, targets, xBound)
	}
	if plan.collect {
		plan.touched = true
		if !e.costStaticGatePrefix(sv, mode, p, id, scope, targets, xBound) {
			plan.skip = append(plan.skip, true)
			return false, true
		}
		si := int32(len(plan.skip))
		plan.skip = append(plan.skip, false)
		return e.costStaticGateTail(sv, mode, p, id, scope, targets, xBound, plan, si)
	}
	ok, indepFail = false, true
	if plan.i < len(plan.skip) {
		if !plan.skip[plan.i] {
			ok, indepFail = e.costStaticGateTail(sv, mode, p, id, scope, targets, xBound, plan, int32(plan.i))
		}
		plan.i++
	} else {
		ok, indepFail = e.costStaticGate(sv, mode, p, id, scope, targets, xBound)
	}
	if walkCacheVerify && !e.costComposeVerifying {
		// The skipped-gate branch is the one the plan asserts most: verify it
		// like every other.
		e.costComposePlanVerifyGate(sv, mode, p, id, scope, targets, xBound, ok, indepFail)
	}
	return ok, indepFail
}

// costComposePlanVerifyGate is the verify-mode check of one planned gate:
// the same static's full chain (the plan stood down) must reach the verdict
// the plan served. The plan's recorded verdicts are answers about every
// assignment of the call, so a stale entry is a wrong composition -- the
// walkCacheVerify builds (the rules test binary and botbench) pay one full
// chain per planned gate to catch it. The recompute may set the cast
// provenance capture the planned path did not; the compose body restores the
// plan's recorded capture after its last gate, so the caller's read is the
// same either way.
func (e *Engine) costComposePlanVerifyGate(sv staticView, mode string, p state.PlayerID, id state.ObjID, scope costScope, targets []state.Target, xBound bool, ok, indepFail bool) {
	prev := e.costComposePlan
	e.costComposePlan = nil
	full, fullIndep := e.costStaticGate(sv, mode, p, id, scope, targets, xBound)
	e.costComposePlan = prev
	if full != ok || fullIndep != indepFail {
		// The plan's recorded verdicts are answers about every assignment of
		// the call, so a stale entry is a wrong composition. (The ids and the
		// verdicts are on the plan and the engine; the strings ratchet keeps
		// this message constant.)
		panic("rules: potential-walk compose plan is stale")
	}
}
