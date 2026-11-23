package rules

import (
	"fmt"
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Per-walk memo of appendAvailableManaAbilitiesGate.
//
// One walk asks the gate about the same object many times -- the priority
// offer, PotentialMana, every candidate cast's payment-window census and
// source-unit reads -- and a search world re-walks the same board for every
// pass: measured on a bench, 60% of gate calls repeated an (object,
// walk) already answered. The answer is a pure function of the walk's fixed
// state, exactly the inputs the Derived memo and boardStaticsWalk key on: the
// board moves only through events (walkKey's log head, continuousVersion and
// object count), and the no-event inputs are the derivation guards
// (derivedMemoUsable) and retireCrossWalkMemo (derivedSeq). The gate reads
// the board (tapped and summoning-sick flags, the mana pool, statics,
// activation counts), never the caller's out, so its answer for one (object,
// player, ignorePayable, statics-or-fresh membership) is cached for the
// walk when it holds only configured abilities (manaGateStable); a hit
// appends a copy, so no caller ever holds the memo's storage.
type manaGateMemo struct {
	key  walkKey
	seq  uint64 // derivedSeq the current gen was opened at
	gen  uint64
	ents []manaGateEnt
	// arena holds every cached answer of the current gen back to back.
	arena []*cards.SA
	// grants caches manaGrantGatesHold per printed static for the walk
	// grantKey/grantSeq name (grantGateMemo): the verdict reads only the
	// board and the static, never the object the grant is offered to, so one
	// walk asks it once per static instead of once per (object, static).
	grants   []grantGateEnt
	grantKey walkKey
	grantSeq uint64
	grantOK  bool
}

type grantGateEnt struct {
	src  state.ObjID
	ctl  state.PlayerID
	ps   *cards.ParamSet
	hold bool
}

type manaGateEnt struct {
	gen      uint64
	start, n int32
}

// manaGateMemoVerify: see derivedMemoVerify. Set by the rules test binary.
var manaGateMemoVerify = derivedMemoVerifyFlag != ""

// recycled is m's storage for the next engine: the gen counter carries on,
// so no recycled entry reads as current.
func (m *manaGateMemo) recycled() manaGateMemo {
	return manaGateMemo{gen: m.gen, seq: ^uint64(0), ents: m.ents, arena: m.arena[:0], grants: m.grants[:0]}
}

// manaGateSlot is the memo slot for one gate question, or -1 when the
// question is not memoised (a seat past the second, an unknown object).
func manaGateSlot(p state.PlayerID, id state.ObjID, ignorePayable, fresh bool) int {
	if p < 0 || p > 1 || id <= 0 {
		return -1
	}
	i := int(id)<<3 | int(p)<<2
	if ignorePayable {
		i |= 2
	}
	if fresh {
		i |= 1
	}
	return i
}

func (e *Engine) appendAvailableManaAbilitiesGate(out []*cards.SA, statics *actionStaticSource, p state.PlayerID, id state.ObjID, ignorePayable bool) []*cards.SA {
	slot := manaGateSlot(p, id, ignorePayable, statics == nil)
	now, ok := e.walkKeyNow()
	if slot < 0 || !ok || !e.derivedMemoUsable() {
		return e.appendAvailableManaAbilitiesGateCompute(out, statics, p, id, ignorePayable)
	}
	m := &e.manaGate
	if m.gen == 0 || m.seq != e.derivedSeq || !e.walkKeyHit(m.key, now) {
		m.gen++
		m.key, m.seq = now, e.derivedSeq
		m.arena = m.arena[:0]
	}
	if slot < len(m.ents) && m.ents[slot].gen == m.gen {
		ent := m.ents[slot]
		hit := m.arena[ent.start : ent.start+ent.n]
		if manaGateMemoVerify {
			if fresh := e.appendAvailableManaAbilitiesGateCompute(nil, statics, p, id, ignorePayable); !slices.Equal(fresh, hit) {
				panic(fmt.Sprintf("rules: mana gate memo for object %d seat %d is stale (%d cached vs %d fresh)", id, p, len(hit), len(fresh)))
			}
		}
		return append(out, hit...)
	}
	base := len(out)
	out = e.appendAvailableManaAbilitiesGateCompute(out, statics, p, id, ignorePayable)
	// The compute may have nested into the memo (a reflected source reads
	// others); the entry is recorded only while this gen is still current.
	// An answer holding an ability without configured facts is not cached:
	// a granted or AddAbility$ ability is a fresh cards.ResolveSVar copy per
	// call, which its caller owns and may rebind, so two callers must never
	// share one; a configured ability is the compiled text's own pointer.
	if m.gen != 0 && m.seq == e.derivedSeq && e.walkKeyHit(m.key, now) && e.manaGateStable(out[base:]) {
		if slot >= len(m.ents) {
			need := max(slot+1, 8*len(e.G.Objs)+8)
			m.ents = slices.Grow(m.ents, need-len(m.ents))[:need]
		}
		m.ents[slot] = manaGateEnt{gen: m.gen, start: int32(len(m.arena)), n: int32(len(out) - base)}
		m.arena = append(m.arena, out[base:]...)
	}
	return out
}

// manaGateStable reports whether every ability in mas is a configured
// ability (manaFactsOf non-nil): the shared compiled pointer, the same on
// every call.
func (e *Engine) manaGateStable(mas []*cards.SA) bool {
	for _, ma := range mas {
		if e.manaFactsOf(ma) == nil {
			return false
		}
	}
	return true
}

// manaGrantGatesHoldMemo is manaGrantGatesHold (fresh) for one printed AddAbility$ carrier,
// answered once per walk. The verdict is a pure function of the board (the
// same walk-fixed inputs the gate memo above keys on: walkKey and derivedSeq,
// under derivedMemoUsable; scope, usable and seq are the caller's reads of them) and
// the static itself, so a hit is the answer a fresh read gives. Only a
// printed view is cached (compiled ParamSet, no Effect-delivered binding),
// keyed by its source, controller and ParamSet; anything else, or any read
// outside a memo scope, recomputes. The slice is scanned in insertion order
// and never ranged for output.
func manaGrantGatesHoldMemo(m *manaGateMemo, sv staticView, scope func() (walkKey, bool), usable func() bool, seq uint64, keyHit func(k, now walkKey) bool,
	g *state.Game, classBand func(string, state.ObjID) bool, present func(staticView, string) bool, matches func(string, state.ObjID, state.PlayerID, state.ObjID) bool) bool {
	fresh := func() bool { return manaGrantGatesHold(g, sv, classBand, present, matches) }
	now, ok := scope()
	if !ok || !usable() || sv.PS == nil || sv.effectStamp != 0 || sv.Remembered != nil ||
		sv.castByGrantor || sv.chosenNumberBound || sv.ChosenNumber != 0 || sv.chosenName != "" || sv.chosenNameFrozen ||
		sv.rememberedPlayers != nil {
		return fresh()
	}
	if !m.grantOK || m.grantSeq != seq || !keyHit(m.grantKey, now) {
		m.grants = m.grants[:0]
		m.grantKey, m.grantSeq, m.grantOK = now, seq, true
	}
	for i := range m.grants {
		if g := &m.grants[i]; g.src == sv.Source && g.ctl == sv.Controller && g.ps == sv.PS {
			if manaGateMemoVerify && g.hold != fresh() {
				panic("rules: grant gate memo is stale")
			}
			return g.hold
		}
	}
	hold := fresh()
	// fresh may have nested into the memo scope; record only while current.
	if m.grantOK && m.grantSeq == seq && keyHit(m.grantKey, now) {
		m.grants = append(m.grants, grantGateEnt{src: sv.Source, ctl: sv.Controller, ps: sv.PS, hold: hold})
	}
	return hold
}
