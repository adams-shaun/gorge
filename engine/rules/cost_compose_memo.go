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
// run of bookkeeping markers stripped (bookkeepingKind, the priority memo's
// set: Priority, DecisionAsk and DecisionMade, whose fold writes only
// g.Priority/g.Passes -- which no cost-modifier gate, spec match, SVar or
// Amount$ program reads: the same audit the ability-loss and layer memos ran
// -- and nothing at all for the two markers), the continuous version (every
// registry write), the object-arena length, the kernel's restore epoch and
// the build count less the cross-walk retires. Step, Active and Turn ride
// along: a cost gate reads the phase and the active seat (Closing
// Statement's own-end-step clause, sorcerySpeed), and those move with the
// log everywhere the engine writes them. The build count moves on the
// no-event inputs the family retires for -- an invalidateScratchLayerLists,
// a static-derived grant, a cascade -- and a face probe's retires leave it
// alone, correctly: the probe is a pure scoped read whose outside-the-probe
// answers the recompute reproduces, and the gate below stands the memo down
// inside one.
type costComposeStamp struct {
	log      *events.Log
	nep      int
	ver      int
	objs     int
	tape     uint32
	rebuilds uint64
	step     state.Step
	active   state.PlayerID
	turn     int32
}

// costComposeMods is a stored composition: the CostMods fields the compose
// body can produce, minus the Extra (a full 832-byte Cost that is all zero in
// every composition without one -- storing it inline would copy it per
// store, and stores dominate this memo's cost). hasExtra and the extra
// pointer together carry it: a composition with extras stores the Extra
// behind the pointer, every other composition leaves both zero and serves
// the zero Extra the body would have produced.
type costComposeMods struct {
	raises         []int32
	raiseCol       state.Mana
	raiseGen       int32
	raiseLife      int32
	reduces        []costMod
	setFloor       int32
	hasExtra       bool
	waterbend      int32
	waterbendX     bool
	waterbendPartX int32
	raiseX         int32
}

// storeComposeMods splits one fresh composition for storage. The modifier
// slices are stored as computed -- the entry shares the fresh result's
// backings, so every caller that reshapes a composed result appends through
// a clone (morphTurnUpMods, mana.go's credit appends, defiler's pip append
// -- the audited rule), and a serve copies the entry's slices out
// (costComposeServeCopy) before any caller can see them.
func storeComposeMods(m costMods) (costComposeMods, *Cost) {
	s := costComposeMods{raises: m.Raises, raiseCol: m.RaiseCol, raiseGen: m.RaiseGen,
		raiseLife: m.RaiseLife, reduces: m.Reduces, setFloor: m.SetFloor,
		hasExtra: m.HasExtra, waterbend: m.Waterbend, waterbendX: m.WaterbendX,
		waterbendPartX: m.WaterbendPartX, raiseX: m.RaiseX}
	if !m.HasExtra {
		return s, nil
	}
	extra := m.Extra
	return s, &extra
}

// mods rebuilds the composition, with the stored (or zero) Extra. The
// modifier slices are shared with the entry: a serve copies them
// (costComposeServeCopy), and the compose body's callers only ever reshape a
// composition through a clone (the audited rule), so the entry's backings
// are never written through a returned composition.
func (s costComposeMods) mods(extra *Cost) costMods {
	out := costMods{Raises: s.raises, RaiseCol: s.raiseCol, RaiseGen: s.raiseGen,
		RaiseLife: s.raiseLife, Reduces: s.reduces, SetFloor: s.setFloor,
		HasExtra: s.hasExtra, Waterbend: s.waterbend, WaterbendX: s.waterbendX,
		WaterbendPartX: s.waterbendPartX, RaiseX: s.raiseX}
	if extra != nil {
		out.Extra = *extra
	}
	return out
}

// costComposeEnt is one remembered composition: the question (whose price,
// whose payer, whose scope, the announced X, the potential-pass flag and the
// target assignment -- the caller's scope.X canonicalised to zero, the body
// binds the announced X itself) and the answer (the modifiers, the mayApply
// flag the offer gate captures beside them and the provenance capture).
type costComposeEnt struct {
	next   *costComposeEnt
	id     state.ObjID
	p      state.PlayerID
	scope  costScope
	pot    bool
	x      int32
	tkey   []state.Target
	mods   costComposeMods
	extra  *Cost
	mayApp bool
	prov   bool
}

// entChunk is the entry arena's chunk size: entries are carved from stable
// chunk arrays (a slice of entries would copy its whole backing on every
// growth, and a fresh engine replays that ladder every run), so a chunk
// allocation is the only per-store cost and the chunks are reused across
// resets.
const entChunk = 32

// costComposeMemo is the per-state table: a dense ObjID -> chain-head index
// (the derivedMemoTable's shape -- a walk prices a fraction of the arena, so
// a fresh engine allocates an index word per object instead of a whole entry
// per object), the entries themselves in stable chunks, chained per object
// because one object composes under many questions (the nil-target base,
// each assignment, each announced X), and bump arenas for the target keys
// (so the caller's slice -- potentialCostModsUsing's per-assignment scratch
// -- may be reused) and the extras (rare: a composed Extra). Cleared,
// backings reused, whenever the stamp moves.
type costComposeMemo struct {
	idx    []*costComposeEnt
	chunks [][]costComposeEnt
	nchunk int
	free   int
	used   int
	keys   []state.Target
	extras []Cost
}

// reset clears the table for the next stamp; the backings are reused, so the
// steady state allocates nothing.
func (m *costComposeMemo) reset() {
	clear(m.idx)
	m.idx = m.idx[:0]
	m.nchunk, m.free, m.used = 0, 0, 0
	m.keys = m.keys[:0]
	m.extras = m.extras[:0]
}

// newEnt carves the next entry from the chunk arena.
func (m *costComposeMemo) newEnt() *costComposeEnt {
	if m.nchunk == 0 || m.free == entChunk {
		if m.nchunk == len(m.chunks) {
			m.chunks = append(m.chunks, make([]costComposeEnt, entChunk))
		}
		m.nchunk++
		m.free = 0
	}
	c := &m.chunks[m.nchunk-1][m.free]
	m.free++
	m.used++
	return c
}

// bucket returns id's chain-head slot, growing the index the derivedMemoTable
// way: only ever appended (a reset clears the whole backing first, so the
// words the growth exposes are zero), backing reused.
func (m *costComposeMemo) bucket(id state.ObjID, n int) **costComposeEnt {
	if len(m.idx) < n {
		if cap(m.idx) < n {
			grown := make([]*costComposeEnt, len(m.idx), max(n, 2*cap(m.idx)))
			copy(grown, m.idx)
			m.idx = grown
		}
		m.idx = m.idx[:n]
	}
	return &m.idx[id]
}

// at answers the table for the question, or nil. scope.X is canonicalised to
// zero: the body binds the announced X itself, so two calls differing only in
// the caller's scope.X are the same question. The statics collection is not
// keyed: every collection at a given state is served from the same fused
// board-statics cache (collectCostStatics slices.Clips one cached scan, and
// the walk's costStaticSource collects through it too), so equal stamps mean
// equal collections.
func (m *costComposeMemo) at(p state.PlayerID, id state.ObjID, scope costScope, potential bool, x int32, targets []state.Target) *costComposeEnt {
	scope.X = 0
	if int(id) >= len(m.idx) {
		return nil
	}
	for ent := m.idx[id]; ent != nil; ent = ent.next {
		if ent.p == p && ent.pot == potential && ent.x == x && ent.scope == scope &&
			len(ent.tkey) == len(targets) && slices.Equal(ent.tkey, targets) {
			return ent
		}
	}
	return nil
}

// remember stores one answer under its question, chained on id's bucket. The
// target keys are copied into the table's bump arena so the caller's slice
// (potentialCostModsUsing's per-assignment scratch) may be reused; the
// modifier slices are stored as computed (storeComposeMods) -- a serve
// copies them (costComposeServeCopy), so a caller's append can never reach
// the entry's backing.
func (m *costComposeMemo) remember(p state.PlayerID, id state.ObjID, scope costScope, potential bool, x int32, targets []state.Target, mods costMods, mayApp, prov bool) {
	scope.X = 0
	start, end := len(m.keys), len(m.keys)+len(targets)
	m.keys = append(m.keys, targets...)
	stored, extra := storeComposeMods(mods)
	if extra != nil {
		m.extras = append(m.extras, *extra)
		extra = &m.extras[len(m.extras)-1]
	}
	ent := m.newEnt()
	*ent = costComposeEnt{
		p: p, id: id, scope: scope, pot: potential, x: x,
		tkey: m.keys[start:end:end],
		mods: stored, extra: extra, mayApp: mayApp, prov: prov,
	}
	head := m.bucket(id, int(id)+1)
	ent.next = *head
	*head = ent
}

// costComposeServeCopy is a served answer: the slices are copied so a
// caller's append (offerCastable's extras, morphTurnUpMods' rider) can never
// reach the entry's backing.
func costComposeServeCopy(ent *costComposeEnt) costMods {
	out := ent.mods.mods(ent.extra)
	out.Raises = slices.Clone(out.Raises)
	out.Reduces = slices.Clone(out.Reduces)
	return out
}

// costModsSameStored is the verify comparison: every scalar and array by
// value, the slices by value, and the Extra through Cost.Equal (its
// DeepEqual strictness is exact -- the body is deterministic, so the same
// question builds the same nil-or-not shape). Every CostMods field the body
// can write is named here and in costComposeMods; a new one must reach both
// or verify mode compares an incomplete answer.
func costModsSameStored(s costComposeMods, extra *Cost, b costMods) bool {
	if !slices.Equal(s.raises, b.Raises) ||
		s.raiseCol != b.RaiseCol ||
		s.raiseGen != b.RaiseGen || s.raiseLife != b.RaiseLife ||
		!slices.Equal(s.reduces, b.Reduces) ||
		s.setFloor != b.SetFloor || s.hasExtra != b.HasExtra ||
		s.waterbend != b.Waterbend || s.waterbendX != b.WaterbendX ||
		s.waterbendPartX != b.WaterbendPartX || s.raiseX != b.RaiseX {
		return false
	}
	if extra == nil {
		var zero Cost
		return (&zero).Equal(&b.Extra)
	}
	return (*extra).Equal(&b.Extra)
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
	for nep > 0 && bookkeepingKind(evs[nep-1].Kind) {
		nep--
	}
	return costComposeStamp{log: e.L, nep: nep, ver: e.continuousVersion, objs: len(e.G.Objs),
		tape: e.tapeEpoch, rebuilds: e.activeBuildSeq - e.crossWalkRetires,
		step: e.G.Step, active: e.G.Active, turn: e.G.Turn}
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
		// The recompute is plan-free: the potential walk's compose plan is
		// per call (cost_compose_plan.go) and bound to one candidate
		// assignment's question, so its recorded head verdicts are not
		// answers about this recompute's own full-chain walk.
		prevPlan := e.costComposePlan
		e.costComposePlan = nil
		fresh := e.costModifiersComposeEval(statics, p, id, scope, targets, potential, x, mp)
		e.costComposePlan = prevPlan
		e.costComposeVerifying = prev
		if !costModsSameStored(ent.mods, ent.extra, fresh) || ent.mayApp != *mp || ent.prov != e.CostProvenanceSeen {
			panic(fmt.Sprintf("rules: cost-modifier compose memo is stale for obj %d (player %d, x %d, potential %v, %d targets): want %+v, got %+v",
				id, p, x, potential, len(targets), ent.mods.mods(ent.extra), fresh))
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
