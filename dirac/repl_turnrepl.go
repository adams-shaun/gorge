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
// to exile by the scope record, and placements the creature counter-plus-one
// made larger.
var TurnReplStats struct{ Exiled, Plus atomic.Int64 }

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

// exScopeN is how many live scope records make creature sl's move to the
// graveyard an exile: its controller is an opponent of the effect's. Each is
// its own CR 616.1 replacement, counted with the marks and the leave
// replacements by dieZone (a competition stalls there: the scope has no
// candidate for gorge's order ask).
func (s *State) exScopeN(k *prep, sl Slot) int {
	if DebugTurnReplOff || len(s.cold.exMark) == 0 {
		return 0
	}
	n := 0
	for i := range s.cold.exMark {
		if r := &s.cold.exMark[i]; r.live && r.sl == slotExScope && r.pl != s.perms[sl].ctrl {
			n++
		}
	}
	if n != 0 && !s.isCreature(k, sl) {
		return 0
	}
	if n != 0 {
		TurnReplStats.Exiled.Add(1)
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
