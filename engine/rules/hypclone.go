package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// Hypothetical clones: the throwaway engines a pure read builds to try a
// line of play (potentialWitnessReaches, PotentialPlayScript's search). Each
// such clone dies before the read returns, so its storage -- the log, object
// arena and memo arrays CloneInto draws from a Spare -- is recycled through
// a small stack of Spares the reading engine owns instead of being allocated
// and collected per clone. CloneInto's contract (TestCloneIntoIsInvisible)
// makes a recycled clone identical to a fresh Clone in everything a caller
// can observe.

// hypSparePool is an engine's stack of spent hypothetical clones' Spares.
// owner guards it against a by-value Engine copy (entryPreview's preview),
// which must never pop or push the owner's stack.
type hypSparePool struct {
	owner  *Engine
	spares []Spare
	// opts is a stack of cleared option lists the temporary offer walks
	// (legalActionsWalkTemp) build into.
	opts [][]decision.Option
	// pm is PotentialMana's working storage (potential.go), taken for each
	// call (empty while one runs).
	pm potentialManaScratch
	// planSearch is the payment-plan search's working storage
	// (payment_plan_search.go).
	planSearch pay.SearchScratch
	// planTables is a stack of the payment planner's spent table scratch
	// (pay.PlanScratch), taken per plan call the way opts is.
	planTables []*pay.PlanScratch
	// zoneEntry backs the engine's zone-entry index (payment_zone_entry.go).
	zoneEntry []zoneEntryRec
	// ids is a stack of id lists read-only walks borrow (idsBorrow).
	ids [][]state.ObjID
	// takes counts the clones taken from the pool, the hypothetical
	// searches' cost: a bounded-search test reads it around one query.
	takes int
	// live lists this pool's clones still alive and the event array each
	// was cloned into (hypClone): a Spare whose provenance is one of those
	// arrays stays trusted until that clone is released (hypRelease).
	live []hypLive
}

// hypLive is one live hypothetical clone and its own event array.
type hypLive struct {
	eng *Engine
	arr *events.Event
}

// hypPool is e's own pool, created on first use.
func (e *Engine) hypPool() *hypSparePool {
	if e.hypSpares == nil || e.hypSpares.owner != e {
		e.hypSpares = &hypSparePool{owner: e}
	}
	return e.hypSpares
}

// hypClone clones c (e itself or one of e's hypothetical clones) from e's
// pool. The clone must be handed back with hypRelease once it, and every
// clone made from it, is dead.
func (e *Engine) hypClone(c *Engine) *Engine {
	sp := e.hypTakeFor(c)
	// A pooled log array sized for an earlier, shorter history would be
	// skipped by CloneInto (it needs the history plus its fork slack) and
	// the clone would regrow a fresh one at only an eighth of slack; the
	// live log keeps growing all game, so that regrow would recur every few
	// decisions. Replace a short array once with half the history again of
	// headroom. Capacity only: CloneInto overwrites every slot it hands out.
	if need := len(c.L.Events) + hypLogSlack; cap(sp.events) < need {
		sp.events = make([]events.Event, 0, need+len(c.L.Events)/2)
		sp.evFrom, sp.evN, sp.evDirty = nil, 0, 0
	}
	// Likewise the object arena (Game.CloneInto needs the objects plus its
	// small minting headroom; a fresh array is zero, as a released one is).
	if need := len(c.G.Objs) + hypObjSlack; cap(sp.objs) < need {
		sp.objs = make([]state.Object, 0, need+len(c.G.Objs)/2)
		sp.objDirty = 0
	}
	cl := c.CloneInto(&sp)
	// Only an array of the clone's own (spare capacity past its history; a
	// shared parent prefix has cap == len) is the clone's to vouch for.
	if n := len(cl.L.Events); n > 0 && cap(cl.L.Events) > n {
		pl := e.hypPool()
		pl.live = append(pl.live, hypLive{eng: cl, arr: &cl.L.Events[0]})
	}
	return cl
}

// hypLogSlack is the appends a hypothetical clone's log array holds past its
// parent's history; it is at least events' fork slack (256), so CloneInto
// always takes the array.
const hypLogSlack = 512

// hypObjSlack is the objects a hypothetical clone's arena holds past its
// parent's; at least state's clone headroom (8).
const hypObjSlack = 32

// hypTake pops a Spare from e's pool (the zero Spare when it is empty).
func (e *Engine) hypTake() Spare {
	pl := e.hypPool()
	pl.takes++
	n := len(pl.spares)
	if n == 0 {
		return Spare{}
	}
	sp := pl.spares[n-1]
	pl.spares[n-1] = Spare{}
	pl.spares = pl.spares[:n-1]
	return sp
}

// hypTakeFor is hypTake preferring a Spare whose event-array provenance is
// c's own array (a spent sibling cloned from c), so the clone copies only
// the events c logged since; any Spare is otherwise as good as another
// (CloneInto's contract), and the top one is taken.
func (e *Engine) hypTakeFor(c *Engine) Spare {
	pl := e.hypPool()
	if len(c.L.Events) > 0 {
		arr := &c.L.Events[0]
		for i := len(pl.spares) - 1; i >= 0; i-- {
			if from, n := pl.spares[i].Provenance(); from == arr && n <= len(c.L.Events) {
				sp := pl.spares[i]
				pl.spares = slices.Delete(pl.spares, i, i+1)
				pl.takes++
				return sp
			}
		}
	}
	return e.hypTake()
}

// hypRelease releases a dead hypothetical clone into e's pool. Its event
// array is recycled from here on, so every pooled Spare whose provenance
// names it (a spent clone of c) is detached first (hypPut).
func (e *Engine) hypRelease(c *Engine) {
	pl := e.hypPool()
	var cur *events.Event
	if c.L != nil && len(c.L.Events) > 0 {
		cur = &c.L.Events[0]
	}
	for i := 0; i < len(pl.live); i++ {
		lv := pl.live[i]
		if lv.eng != c {
			continue
		}
		pl.live = slices.Delete(pl.live, i, i+1)
		i--
		for k := range pl.spares {
			if from, _ := pl.spares[k].Provenance(); from != nil && (from == lv.arr || from == cur) {
				pl.spares[k].Detach()
			}
		}
	}
	e.hypPut(c.Release())
}

// hypPut returns a Spare to e's pool.
//
// A Spare's event-array provenance (events[:evN] are the first evN events of
// the array at evFrom, so a clone of that same history copies only the rest)
// holds only while that array is never rewritten. The pool owner's own log
// is append-only, but a hypothetical clone's array is itself recycled
// through this pool and refilled with another history: a clone of a clone
// released here, then reused under the parent array's next tenant, kept
// the old tenant's appended events in the prefix it skipped (a witness
// search's grandchild saw its stale mana activations where the live log had
// the declare-attackers event, re-asked the declaration and refused a legal
// cast, census pair 1905). So only a Spare cloned straight from the owner,
// or from one of the pool's clones still alive (whose own array is
// append-only history until hypRelease detaches its dependents), keeps its
// provenance; every other one is detached.
func (e *Engine) hypPut(sp Spare) {
	if from, _ := sp.Provenance(); from != nil && (len(e.L.Events) == 0 || from != &e.L.Events[0]) && !e.hypLiveArray(from) {
		sp.Detach()
	}
	pl := e.hypPool()
	pl.spares = append(pl.spares, sp)
}

// hypLiveArray reports whether arr is the event array of one of e's pool's
// live clones.
func (e *Engine) hypLiveArray(arr *events.Event) bool {
	for _, lv := range e.hypPool().live {
		if lv.arr == arr {
			return true
		}
	}
	return false
}

// optBorrow pops a cleared option list from e's pool (nil when empty).
func (e *Engine) optBorrow() []decision.Option {
	pl := e.hypPool()
	n := len(pl.opts)
	if n == 0 {
		return nil
	}
	b := pl.opts[n-1]
	pl.opts[n-1] = nil
	pl.opts = pl.opts[:n-1]
	return b
}

// optRelease clears a list legalActionsWalkTemp returned (dropping its
// labels, slices and grants) and pushes it back onto e's pool.
func (e *Engine) optRelease(b []decision.Option) {
	if cap(b) == 0 {
		return
	}
	clear(b)
	pl := e.hypPool()
	pl.opts = append(pl.opts, b[:0])
}

// releaseHypPool detaches e's own pool for a Spare (nil when e has none).
// Its Spares and scratch hold only cleared or call-local storage.
func (e *Engine) releaseHypPool() *hypSparePool {
	pl := e.hypSpares
	e.hypSpares = nil
	if pl == nil || pl.owner != e {
		return nil
	}
	pl.owner = nil
	return pl
}

// adoptHypPool makes a Spare's pool e's own.
func (e *Engine) adoptHypPool(pl *hypSparePool) {
	if pl == nil {
		return
	}
	pl.owner = e
	e.hypSpares = pl
	// The pool's Spares were cloned under its previous owner, whose array
	// may be recycled now: none of their provenance can be trusted (hypPut).
	for i := range pl.spares {
		pl.spares[i].Detach()
	}
	clear(pl.live)
	pl.live = pl.live[:0]
}

// idsBorrow pops an empty id list from e's pool (nil when empty); the caller
// appends into it, only ranges the result, and hands it back with
// idsRelease. Nested borrowers get distinct lists.
func (e *Engine) idsBorrow() []state.ObjID {
	pl := e.hypPool()
	n := len(pl.ids)
	if n == 0 {
		return nil
	}
	b := pl.ids[n-1]
	pl.ids[n-1] = nil
	pl.ids = pl.ids[:n-1]
	return b[:0]
}

// idsRelease returns a borrowed id list to e's pool.
func (e *Engine) idsRelease(b []state.ObjID) {
	if cap(b) == 0 {
		return
	}
	pl := e.hypPool()
	pl.ids = append(pl.ids, b[:0])
}
