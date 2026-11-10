package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ExtTurnRepl (kop/turnrepl.go): two until-end-of-turn replacements an Effect
// makes over a player's side of the board. Neither costs State bytes: the
// exile one is a scope record in the cold exMark table, the counter one a
// region of the cold buffer (coldLayout.ctrP) that exists only for a table
// with such a program.

// DebugTurnReplOff is the mutation switch: both replacements are set but
// ignored, so a lockstep that reaches either diverges.
var DebugTurnReplOff bool

// TurnReplStats counts the lockstep tests' coverage: opposing creatures sent
// to exile by the scope record, placements the creature counter-plus-one made
// larger, and CR 616.1 order asks that offered a scope record as a candidate.
var TurnReplStats struct{ Exiled, Plus, Asked atomic.Int64 }

// StallTurnRepl is a second scope record or creature counter record past what
// the cold table holds: the exile-instead table is full.
const StallTurnRepl = 159

func init() { stallNames[StallTurnRepl] = "turnrepl" }

// slotExScope is the exMarkRec.sl of a scope record: "a creature an opponent
// of pl controls would die, exile it instead". No battlefield slot is one.
const slotExScope Slot = NoSlot - 2

// prepTurnRepl sets what the table's ExtTurnRepl programs need: the counter
// kind reads genPrep.anyRepl's rbPlusCtrEOT gates beside Boimler's, and the cold
// buffer region.
func prepTurnRepl(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.A.K == kop.VConst && kop.ExtOp(in.A.N) == kop.ExtTurnRepl && in.C == kop.TurnReplPlusCreatureCtr {
					g.anyRepl |= rbPlusCtrEOT
					g.anyPlusCre = true
				}
			}
		}
	}
}

// turnRepl is ExtTurnRepl.
func (s *State) turnRepl(k *prep, in *kop.Instr) {
	switch in.C {
	case kop.TurnReplExileOpp:
		for i := range s.cold.exMark {
			if r := &s.cold.exMark[i]; !r.live {
				*r = exMarkRec{sl: slotExScope, src: s.res.e.obj, live: true, pl: s.res.e.ctrl}
				return
			}
		}
		s.capFull(StallTurnRepl)
	case kop.TurnReplPlusCreatureCtr:
		// each activation is its own replacement effect, so the count
		// stacks; two sources make the order ask stall (ctrGather)
		c := s.plusCreCold()
		if p := s.res.e.ctrl; c != nil && c[p] < 255 {
			if c[p] == 0 {
				c[MaxSeats+int(p)] = uint8(s.res.e.obj)
			} else if Obj(c[MaxSeats+int(p)]) != s.res.e.obj {
				c[MaxSeats+int(p)] = uint8(NoObj)
			}
			c[p]++
		}
	}
}

// exScopeHit reports whether scope record r makes creature sl's move to the
// graveyard an exile: its controller is an opponent of the effect's.
func (s *State) exScopeHit(k *prep, r *exMarkRec, sl Slot) bool {
	return r.live && r.sl == slotExScope && r.pl != s.perms[sl].ctrl && s.isCreature(k, sl)
}

// exScopeN is how many live scope records make creature sl's move to the
// graveyard an exile. Each is its own CR 616.1 replacement: dieZone counts
// them with the marks and the leave replacements, and a competition is asked
// (graveMoveCompetes, exileOrderOptions) like two marks on one creature.
func (s *State) exScopeN(k *prep, sl Slot) int {
	if DebugTurnReplOff || len(s.cold.exMark) == 0 {
		return 0
	}
	n := 0
	for i := range s.cold.exMark {
		if s.exScopeHit(k, &s.cold.exMark[i], sl) {
			n++
		}
	}
	return n
}

// plusCreCold is the creature counter-plus-one record: per player the count of
// activations, then per player the source of them (NoObj: more than one).
func (s *State) plusCreCold() []uint8 {
	l := s.cold.layout
	if l == nil || l.ctrP == 0 {
		return nil
	}
	at := 0
	_ = coldView[mayPlay](nil, &at, l.may)
	_ = coldView[embRec](nil, &at, l.emb)
	return coldView[uint8](s.cold.buf, &at, l.ctrP)
}

// plusCreN is the number of Prairie Dog activations whose replacement puts one
// more of kind counters on sl, and their source (NoObj when two differ).
func (s *State) plusCreN(k *prep, sl Slot, kind int) (uint8, Obj) {
	if DebugTurnReplOff || kind != int(kop.CtrP1P1) {
		return 0, NoObj
	}
	c := s.plusCreCold()
	if c == nil {
		return 0, NoObj
	}
	p := s.perms[sl].ctrl
	if c[p] == 0 || !s.isCreature(k, sl) {
		return 0, NoObj
	}
	return c[p], Obj(c[MaxSeats+int(p)])
}

// turnReplEnd ends the creature record (the cleanup and the new turn's reset
// both call it) and, with scope, the scope records (the cleanup).
func (s *State) turnReplEnd(scope bool) {
	if c := s.plusCreCold(); c != nil {
		clear(c)
	}
	if scope {
		for i := range s.cold.exMark {
			if r := &s.cold.exMark[i]; r.sl == slotExScope {
				r.live = false
			}
		}
	}
}
