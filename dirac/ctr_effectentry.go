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

// StallEffEtbCtr: more than 21 extra counters pending on one spell, a later effect of more than 4 beside another, more than 15 effects of one size, a second
// (the third and later effects ride stackEntry.etbX: nibbles counting effects of 1..4 counters).
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
// ExtEffEtbCtr, and genPrep.anyEtbOwnOther with the per-card gfEtbOrder flag
// for the cards whose own enter body is one the CR 616.1 entry-order ask
// orders (repl_etborder.go).
func prepEffEtbCtr(t *kop.Table, p *prep, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.A.K == kop.VConst && kop.ExtOp(in.A.N) == kop.ExtEffEtbCtr {
					g.anyEffEtbCtr = true
				}
			}
		}
		if etbOwnPrepares(t, &p.cards[i], CardID(i)) {
			g.setFlag(CardID(i), gfEtbOrder)
			g.anyEtbOwnOther = true
		}
	}
}

// etbOwnPrepared reports whether an own enter body is exactly one prepared
// grant of the replaced object (Strife Scholar's "enters prepared",
// AlterAttribute Prepared on the ETBReplacement keyword's line; gorge's
// expansion carries ReplacementResult$ Updated, so the composition
// composes it over the entry whatever Repl.C the lowerer chose).
func etbOwnPrepared(a *kop.Ability) bool {
	return a.Repl.Cond == 0 &&
		len(a.Prog) == 1 && a.Prog[0].Op == kop.OpPrepare && a.Prog[0].Obj == kop.RefReplaced
}

// etbOwnPrepares reports whether card pc's own enter body is exactly one
// prepared Updated line: gorge composes the entry's Updated replacements
// (replacement_updated.go) into [the effect's PutCounter rider, this body],
// two candidates that do not commute, and asks the order (CR 616.1c). False
// when the card has none, another own enter line beside it (a longer
// candidate list gorge would ask over), or a printed enters-tapped or
// enters-with rider that joins the composition.
func etbOwnPrepares(t *kop.Table, pc *pcard, c CardID) bool {
	if pc.enterTap || pc.enterCtr != 0 {
		return false
	}
	var found bool
	for i := range t.Cards[c].Abilities {
		a := &t.Cards[c].Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram {
			continue
		}
		if !etbOwnPrepared(a) || found {
			return false // a second own line: gorge asks over three or more candidates
		}
		found = true
	}
	return found
}

// etbXGet and etbXSet read and write the number of effects of n (1..4)
// counters beyond the pair code, one nibble each in stackEntry.etbX. Several
// effects of one spell on an entrant with no other source of entry counters
// are separate puts and commute, so the multiset is exact.
func etbXGet(e *stackEntry, n int) int {
	return int(e.etbX[(n-1)>>1]>>(uint(n-1)&1*4)) & 15
}

func etbXSet(e *stackEntry, n, v int) {
	sh := uint(n-1) & 1 * 4
	e.etbX[(n-1)>>1] = e.etbX[(n-1)>>1]&^(15<<sh) | uint8(v)<<sh
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
	if k.gen != nil && k.gen.has(s.objs[o].card, gfEtbOrder) {
		// the entrant's own body is one the CR 616.1 entry-order ask orders:
		// record the rider's source (the resolving trigger's, the source of
		// the Effect's registered replacement) so the ask can be posed
		// before the move (repl_etborder.go). A second effect with another
		// source is a third candidate gorge asks over: stall.
		src := s.res.e.obj
		for i := int(s.sp) - 1; i >= 0; i-- {
			e := &s.stack[i]
			if e.kind != seSpell || e.obj != o {
				continue
			}
			if e.etbSrc != 0 && e.etbSrc != src {
				s.capFull(StallEffEtbCtr)
				return
			}
			e.etbSrc = src
			break
		}
	}
	for i := int(s.sp) - 1; i >= 0; i-- {
		e := &s.stack[i]
		if e.kind != seSpell || e.obj != o {
			continue
		}
		code, ok := etbCodeAdd(int(e.spc&spcEtbMask)>>spcEtbShift, n)
		if ok {
			e.spc = e.spc&^spcEtbMask | uint16(code)<<spcEtbShift
		} else if e.spc&spcEtbMask != 0 && n <= 4 && etbXGet(e, n) < 15 {
			etbXSet(e, n, etbXGet(e, n)+1) // a later effect the pair code cannot hold
		} else {
			s.capFull(StallEffEtbCtr)
			return
		}
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
	var xs [4]int // effects beyond the pair, as counts of effects of 1..4 counters
	nx, xsum := 0, 0
	for n := 1; n <= 4; n++ {
		xs[n-1] = etbXGet(&s.res.e, n)
		nx += xs[n-1]
		xsum += xs[n-1] * n
	}
	if DebugEffEtbCtr && a+b+xsum > 0 {
		switch {
		case nx > 0:
			for n := 1; n <= 4; n++ { // one counter fewer on the first extra effect
				if xs[n-1] > 0 {
					xs[n-1]--
					if n > 1 {
						xs[n-2]++
					} else {
						nx--
					}
					xsum--
					break
				}
			}
		case b > 0:
			b--
		default:
			a--
		}
	}
	if a+b+xsum == 0 {
		return
	}
	if (b > 0 || nx > 0) && !s.entryPlan(k, sl, p) {
		// two or more effects on an entrant with no other source of entry
		// counters: gorge does not stage the entry (entryBodyCandidates), so
		// each effect's body runs as its own put (ctr_entryperkind.go)
		for _, n := range [2]int{a, b} {
			if n > 0 {
				s.placeCounters(k, sl, int(kop.CtrP1P1), n, p.ctrl, ctrPutBy)
			}
		}
		for n := 1; n <= 4; n++ {
			for i := 0; i < xs[n-1]; i++ {
				s.placeCounters(k, sl, int(kop.CtrP1P1), n, p.ctrl, ctrPutBy)
			}
		}
		return
	}
	n := a + b + xsum
	if k.gen.anyEntryMerge && s.entryAcc(int(kop.CtrP1P1), n) {
		return // joins the entry's one put (ctr_entryperkind.go)
	}
	s.placeCounters(k, sl, int(kop.CtrP1P1), n, p.ctrl, ctrPutBy)
}
