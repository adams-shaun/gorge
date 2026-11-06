package rules

import (
	"fmt"
	"slices"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The cost-modifier compose memo serves costModifiersCompose0's answer for a
// pure-read pricing from a per-state table, the payment-plan memo family's
// newest member: the potential walk's per-candidate target-assignment
// enumeration (potentialCostModsUsing), the offer gate's base compose and the
// builder plans' shape detail all ask the same question several times while
// the engine is at rest, and each ask re-walks every cost static's gate chain
// and re-prices every Amount$. An answer is a pure function of the board, the
// collected statics (every caller passes a fresh or same-state collection)
// and the question, so it is served whenever the per-state stamp below is
// unchanged -- per state, never per session: any real event, registry write,
// object append, restore or active() rebuild moves the stamp and the table is
// cleared. Verify mode (walkCacheVerify, the family's switch the rules test
// binary and the botbench build set) recomputes every serve and panics on a
// difference.

// costComposeStamp is the memo's per-state key, read AFTER the active()
// refresh every board reader performs anyway (the derived memo's own
// discipline): the engine's log as its lineage, its length with the trailing
// run of ask's DecisionAsk markers stripped (their Apply writes nothing, so a
// compose read between an ask and the next real event is the same question
// the walk answered -- the derivedMemoTailLive argument), the continuous
// version (every registry write), the object-arena length, the kernel's
// restore epoch and the build count less the cross-walk retires. The build
// count moves on the no-event inputs the family retires for -- an
// invalidateScratchLayerLists, a static-derived grant, a cascade -- and a
// face probe's retires leave it alone, correctly: the probe is a pure scoped
// read whose outside-the-probe answers the recompute reproduces, and the
// gate below stands the memo down inside one.
type costComposeStamp struct {
	log      *events.Log
	nep      int
	ver      int
	objs     int
	tape     uint32
	rebuilds uint64
}

// costComposeEnt is one remembered composition: the question (whose price,
// whose payer, whose scope, the announced X, the potential-pass flag and the
// target assignment -- the caller's scope.X canonicalised to zero, the body
// binds the announced X itself) and the answer (the modifiers, the mayApply
// flag the offer gate captures beside them and the provenance capture).
type costComposeEnt struct {
	next   int32
	id     state.ObjID
	p      state.PlayerID
	scope  costScope
	pot    bool
	x      int32
	tkey   []state.Target
	mods   costMods
	mayApp bool
	prov   bool
}

// costComposeMemo is the per-state table: a dense ObjID -> chain-head index
// (the derivedMemoTable's shape -- a walk prices a fraction of the arena, so
// a fresh engine allocates an index word per object instead of a whole entry
// per object) and the entries themselves, chained per object because one
// object composes under many questions (the nil-target base, each
// assignment, each announced X). The target keys live in a bump arena so the
// caller's slice (potentialCostModsUsing's per-assignment scratch) may be
// reused. Cleared, backing reused, whenever the stamp moves.
type costComposeMemo struct {
	idx     []int32
	entries []costComposeEnt
	keys    []state.Target
}

// reset clears the table for the next stamp; the backing is reused, so the
// steady state allocates nothing.
func (m *costComposeMemo) reset() {
	clear(m.idx)
	m.idx = m.idx[:0]
	m.entries = m.entries[:0]
	m.keys = m.keys[:0]
}

// bucket returns id's chain-head slot, growing the index the derivedMemoTable
// way: only ever appended (a reset clears the whole backing first, so the
// words the growth exposes are zero), backing reused.
func (m *costComposeMemo) bucket(id state.ObjID, n int) *int32 {
	if len(m.idx) < n {
		if cap(m.idx) < n {
			grown := make([]int32, len(m.idx), max(n, 2*cap(m.idx)))
			copy(grown, m.idx)
			m.idx = grown
		}
		m.idx = m.idx[:n]
	}
	return &m.idx[id]
}

// at answers the table for the question, or nil. scope.X is canonicalised to
// zero: the body binds the announced X itself, so two calls differing only in
// the caller's scope.X are the same question.
func (m *costComposeMemo) at(p state.PlayerID, id state.ObjID, scope costScope, potential bool, x int32, targets []state.Target) *costComposeEnt {
	scope.X = 0
	if int(id) >= len(m.idx) {
		return nil
	}
	for k := m.idx[id]; k != 0; {
		ent := &m.entries[k-1]
		if ent.p == p && ent.pot == potential && ent.x == x && ent.scope == scope &&
			len(ent.tkey) == len(targets) && slices.Equal(ent.tkey, targets) {
			return ent
		}
		k = ent.next
	}
	return nil
}

// remember stores one answer under its question, chained on id's bucket. The
// modifiers are stored as computed; a serve copies the slices (serve), so a
// caller's append can never reach the entry's backing.
func (m *costComposeMemo) remember(p state.PlayerID, id state.ObjID, scope costScope, potential bool, x int32, targets []state.Target, mods costMods, mayApp, prov bool) {
	scope.X = 0
	start, end := len(m.keys), len(m.keys)+len(targets)
	m.keys = append(m.keys, targets...)
	m.entries = append(m.entries, costComposeEnt{
		p: p, id: id, scope: scope, pot: potential, x: x,
		tkey: m.keys[start:end:end],
		mods: mods, mayApp: mayApp, prov: prov,
	})
	head := m.bucket(id, int(id)+1)
	ent := &m.entries[len(m.entries)-1]
	ent.next = *head
	*head = int32(len(m.entries))
}

// costComposeServeCopy is a served answer: the slices are copied so a
// caller's append (offerCastable's extras) can never reach the entry's
// backing.
func costComposeServeCopy(ent *costComposeEnt) costMods {
	out := ent.mods
	out.Raises = slices.Clone(ent.mods.Raises)
	out.Reduces = slices.Clone(ent.mods.Reduces)
	return out
}

// costModsSame is the verify comparison: every scalar and array by value, the
// slices by value, and the Extra through Cost.Equal (its DeepEqual strictness
// is exact -- the body is deterministic, so the same question builds the same
// nil-or-not shape).
func costModsSame(a, b costMods) bool {
	return slices.Equal(a.Raises, b.Raises) &&
		a.Extra.Equal(&b.Extra) &&
		a.RaiseCol == b.RaiseCol &&
		a.RaiseGen == b.RaiseGen && a.RaiseLife == b.RaiseLife &&
		slices.Equal(a.Reduces, b.Reduces) &&
		a.SetFloor == b.SetFloor && a.HasExtra == b.HasExtra &&
		a.Waterbend == b.Waterbend && a.WaterbendX == b.WaterbendX &&
		a.WaterbendPartX == b.WaterbendPartX && a.RaiseX == b.RaiseX
}

// costComposeUsable reports whether this compose is a pure read the memo may
// serve: the same at-rest contexts the payment-plan memo family sanctions
// (the posed decision's ask walk, its potential walk, the builder plans, the
// bot planner, a ValidateCastPayment re-derivation, a view projection) and
// none of the state-bearing flows it excludes -- a real cast's pricing
// windows (e.cast), an engaged cost-composition exclusion, a graveyard-priced
// composition, a face probe, a caller whose statics view the stamp cannot see
// (offerNamedMods' per-n SVar binding, costComposeSuspend), or a
// derivation-guard context (derivedMemoUsable) whose non-event inputs the
// stamp cannot see.
func (e *Engine) costComposeUsable() bool {
	return e.L != nil && e.G != nil && e.cast == nil && e.costCompositionEvent == 0 &&
		e.pricedFromGraveyard == 0 && e.offerProbeDepth == 0 && e.costComposeSuspend == 0 &&
		e.derivedMemoUsable()
}

// costComposeStampNow reads the memo's per-state key; active() must be
// current (costModifiersComposeMemoized calls it first), so the build count
// is exact.
func (e *Engine) costComposeStampNow() costComposeStamp {
	evs := e.L.Events
	nep := len(evs)
	for nep > 0 && evs[nep-1].Kind == events.DecisionAsk {
		nep--
	}
	return costComposeStamp{log: e.L, nep: nep, ver: e.continuousVersion, objs: len(e.G.Objs),
		tape: e.tapeEpoch, rebuilds: e.activeBuildSeq - e.crossWalkRetires}
}

// costModifiersComposeMemoized is the memo path of costModifiersCompose0
// (called only when costComposeUsable): serve a stored answer for the
// question, recomputing it under verify mode first, or compute, remember and
// return the fresh one.
func (e *Engine) costModifiersComposeMemoized(statics costStaticViews, p state.PlayerID, id state.ObjID, scope costScope, targets []state.Target, potential bool, x int32, mayApply *bool) costMods {
	// The stamp's build count is exact only past the refresh every board
	// reader performs (the derived memo's discipline: read it after active()).
	e.active()
	st := e.costComposeStampNow()
	if e.costComposeStamp != st {
		e.costComposeStamp = st
		e.costCompose.reset()
	}
	// The body always writes a mayApply answer for the store; a caller that
	// passed nil keeps its nil (the write lands here).
	local := false
	mp := mayApply
	if mp == nil {
		mp = &local
	}
	ent := e.costCompose.at(p, id, scope, potential, x, targets)
	if ent != nil && walkCacheVerify && !e.costComposeVerifying {
		prev := e.costComposeVerifying
		e.costComposeVerifying = true
		fresh := e.costModifiersComposeEval(statics, p, id, scope, targets, potential, x, mp)
		e.costComposeVerifying = prev
		if !costModsSame(ent.mods, fresh) || ent.mayApp != *mp || ent.prov != e.CostProvenanceSeen {
			panic(fmt.Sprintf("rules: cost-modifier compose memo is stale for obj %d (player %d, x %d, potential %v, %d targets): want %+v, got %+v",
				id, p, x, potential, len(targets), ent.mods, fresh))
		}
	}
	if ent != nil {
		e.costComposeServed++
		*mp = ent.mayApp
		e.CostProvenanceSeen = ent.prov
		return costComposeServeCopy(ent)
	}
	mods := e.costModifiersComposeEval(statics, p, id, scope, targets, potential, x, mp)
	e.costCompose.remember(p, id, scope, potential, x, targets, mods, *mp, e.CostProvenanceSeen)
	return mods
}
