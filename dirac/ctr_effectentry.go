package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// a SpellCast trigger's Effect that makes the cast spell
// enter with N additional +1/+1 counters (Torgal, A Fine Hound; Wildgrowth
// Archaic). gorge registers a live Event$ Moved replacement (effects/effect.go)
// carrying the spell, ExileOnMoved$ Stack and the frozen ChosenNumber;
// rules/replacement.go applyReplacement applies the move, ends the effect (the
// ExileOnMoved sweep) and only then matches the entry's triggers. Since
// gorge the entry plan (entry_counters.go
// effectEntryBodies, absorbForeign, mergeEntryGrants) folds every such effect
// body into the entrant's own enters-with grant of the kind: all counters of
// one kind the permanent enters with are ONE put, so a "that many plus one"
// replacement (Hardened Scales) applies once to the sum.
//
// The kernel keeps ONE sum per entry here (the pending count below; several
// effects add up) and hands it to the entry's window (ctr_entryperkind.go
// entryAcc), which merges it with the printed, requested and foreign counters
// into that one put.
//
// gorge stages the entry (and so folds the effects' bodies) only when the
// entrant has another source of entry counters (entryBodyCandidates reads the
// entrant's own body, any other permanent's lent body, a requested count; it
// does not read the Effect-borne bodies). A plain creature under two effects
// gets one put per effect, so the pending count keeps two effects apart
// (etbCodeAdd); a third effect, or a larger pair, stalls.
//
// The kernel keeps the pending count in the spell's stack entry: spc bits
// 11-15 (spcEtbShift; bit 10 is spcEvoked, bits 0-9 the colours
// spent). ExtEffEtbCtr adds to it while the spell is still on the stack;
// enterGen puts the counters on after the printed ones and before the
// entry's triggers are matched. A spell gone from the stack (countered in
// response) gets nothing: the effect ends with the spell's move. A stack copy
// is not the remembered card (copy_gen zeroes spc).

const (
	spcEtbShift = 11
	spcEtbMask  = 0x1f << spcEtbShift
)

// StallEffEtbCtr: more than 21 extra counters pending on one spell, a second
// effect beside more than 4 (or beside one of more than 4), or a third effect.
const StallEffEtbCtr = 117

func init() { stallNames[StallEffEtbCtr] = "effetbctr" }

// DebugEffEtbCtr is TestEffetbctrMutation's switch: the permanent enters
// with one counter fewer than the effect gave. It must make the lockstep
// diverge.
var DebugEffEtbCtr bool

// EffEtbStats counts the lockstep test's coverage: effect applications that
// added counters and applications that added two or more.
var EffEtbStats struct{ Applied, Big atomic.Int64 }

// prepEffEtbCtr sets genPrep.anyEffEtbCtr when some instruction is
// ExtEffEtbCtr.
func prepEffEtbCtr(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.A.K == kop.VConst && kop.ExtOp(in.A.N) == kop.ExtEffEtbCtr {
					g.anyEffEtbCtr = true
				}
			}
		}
	}
}

// effEtbCtr is ExtEffEtbCtr: the resolving SpellCast trigger's spell (still on
// the stack) will enter with max(B, 0) more P1P1 counters.
func (s *State) effEtbCtr(k *prep, in *kop.Instr) {
	o := s.res.e.trigObj
	if o == NoObj {
		return
	}
	n := s.value(k, in.B)
	if n <= 0 {
		return
	}
	for i := int(s.sp) - 1; i >= 0; i-- {
		e := &s.stack[i]
		if e.kind != seSpell || e.obj != o {
			continue
		}
		code, ok := etbCodeAdd(int(e.spc&spcEtbMask)>>spcEtbShift, n)
		if !ok {
			s.capFull(StallEffEtbCtr)
			return
		}
		e.spc = e.spc&^spcEtbMask | uint16(code)<<spcEtbShift
		EffEtbStats.Applied.Add(1)
		if n >= 2 {
			EffEtbStats.Big.Add(1)
		}
		return
	}
}

// effEtbEnter puts the resolving spell's pending extra counters on the
// permanent at sl as it enters (after the printed enters-with counters,
// before its triggers: gorge applyReplacement runs the body, then matches the
// entry's triggers).
func (s *State) effEtbEnter(k *prep, sl Slot, p *perm) {
	a, b := etbCodeParts(int(s.res.e.spc&spcEtbMask) >> spcEtbShift)
	if DebugEffEtbCtr && a+b > 0 {
		if b > 0 {
			b--
		} else {
			a--
		}
	}
	if a+b == 0 {
		return
	}
	if b > 0 && !s.entryPlan(k, sl, p) {
		// two effects on an entrant with no other source of entry counters:
		// gorge does not stage the entry (entryBodyCandidates), so each
		// effect's body runs as its own put (ctr_entryperkind.go)
		for _, n := range [2]int{a, b} {
			if n > 0 {
				s.placeCounters(k, sl, int(kop.CtrP1P1), n, p.ctrl, ctrPutBy)
			}
		}
		return
	}
	n := a + b
	if k.gen.anyEntryMerge && s.entryAcc(int(kop.CtrP1P1), n) {
		return // joins the entry's one put (ctr_entryperkind.go)
	}
	s.placeCounters(k, sl, int(kop.CtrP1P1), n, p.ctrl, ctrPutBy)
}
