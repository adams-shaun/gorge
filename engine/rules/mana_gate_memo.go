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
	return manaGateMemo{gen: m.gen, seq: ^uint64(0), ents: m.ents, arena: m.arena[:0]}
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
