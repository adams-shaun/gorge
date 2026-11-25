package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 616.1 order ask of a life gain two replacements compete for: a
// doubler (ReplLifeDouble) and a plus-N (ReplLifePlus) of the gaining player
// do not commute (2(n+1) against 2n+1), so gorge parks the event on a
// KReplacement ask for that player (engine/rules/replacement_life.go
// continueLifeReplacements, poseLifeReplacementChoice), applies each pick as
// it is picked and re-collects the rest (CR 616.1e); the order is asked only
// while two or more of different kinds remain (lifeReplacementsCommute).
//
// Stage 1 poses the ask from the one site that resolves a lone gain on the
// stack, the resolving GainLife op of one player (lifePark): the op parks on
// s.g.nc / s.g.ro (the noncombat damage park's record, 0 B: ncDmg.ctrl's high
// bit marks a life park, ro.an carries the running amount), the answer applies
// the picks and re-executes the op (plusBatchAsked), whose gain reads the
// picked order's amount (lifeAsked). Every other gain site (lifelink, extort,
// trigger batches, redirects, SetLife) cannot suspend and stalls on the
// competition (StallLifeRepl, gainLife).

// ncLife marks an ncDmg record as a parked life gain (ncDmg.ctrl's high bit).
const ncLife uint8 = 0x80

// DebugLifeReplFixed (TestFourSeatLifeReplLockstep): the picked order is
// ignored, the doublers always apply first.
var DebugLifeReplFixed bool

// lifeReplLine is the life-gain replacement card carries: whether it doubles,
// its plus-N's addend, and how many such lines it has (more than one is not
// collected as one candidate each).
func lifeReplLine(k *prep, card CardID) (dbl bool, add, lines int) {
	cd := &k.t.Cards[card]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement {
			continue
		}
		switch a.Repl.Kind {
		case kop.ReplLifeDouble:
			dbl = true
			lines++
		case kop.ReplLifePlus:
			add = int(a.Repl.A)
			lines++
		}
	}
	return dbl, add, lines
}

// lifeCands collects p's life-gain replacement hosts in gorge's scan order
// (battlefield entry order): the host objects, the plus-N ones among them
// (bit per candidate). ok is false after a loud stall (more candidates than
// the record holds, or a host with two life lines).
func (s *State) lifeCands(k *prep, p uint8) (cand [4]Obj, c int, plus uint8, ok bool) {
	var bf [MaxPerm]Slot
	for _, b := range bf[:s.battlefieldOrder(p, s.statM.and(s.ctrlM[p]), bf[:], 0)] {
		card := s.cardAt(b)
		if k.gen.replBits[card]&(rbDblLife|rbPlusLife) == 0 || s.genLost(k, b) {
			continue
		}
		dbl, _, lines := lifeReplLine(k, card)
		if lines != 1 {
			s.stall(StallLifeRepl)
			return cand, 0, 0, false
		}
		if c == len(cand) {
			s.capFull(StallReplCands) // loud: never truncate the candidates
			return cand, 0, 0, false
		}
		cand[c] = s.perms[b].obj
		if !dbl {
			plus |= 1 << c
		}
		c++
	}
	return cand, c, plus, true
}

// lifeMixed reports that p's gain meets a doubler and a plus-N together.
func (s *State) lifeMixed(k *prep, p uint8) bool {
	_, c, plus, ok := s.lifeCands(k, p)
	return !ok || plus != 0 && bits.OnesCount8(plus) != c
}

// lifeAsked consumes the answered park's record for p's re-executed gain:
// the picked order's amount, which skips the fixed-order replLife.
func (s *State) lifeAsked(p uint8) (int, bool) {
	nc := &s.g.nc
	if !s.g.plusBatchAsked || nc.n <= 0 || nc.ctrl != ncLife|p || nc.code != PlayerCode(p) {
		return 0, false
	}
	n := int(nc.n)
	*nc = ncDmg{}
	s.g.plusBatchAsked = false
	return n, true
}

// lifeParkOK reports that the resolving op in may park a gain of n by the
// players q: one player, the op itself the one runProg stands on, no ask of
// another kind in progress.
func (s *State) lifeParkOK(k *prep, in *kop.Instr, q seatSeq, n int) bool {
	if q.n != 1 || n <= 0 || n > 0x3fff || s.g.ro.n != 0 || s.g.ro.on || s.g.nc.n != 0 || s.g.plusBatchAsked || s.pl[q.s[0]].lost {
		return false
	}
	prog := s.prog(k)
	return int(s.res.pc) < len(prog) && in == &prog[s.res.pc]
}

// lifePark parks the resolving GainLife op's gain when its player's doubler
// and plus-N compete for it, true when it parked (runProg then poses the ask
// on nc.n). A gain a CantGainLife static swallows, or one without the
// competition, is the plain gainLife's.
func (s *State) lifePark(k *prep, in *kop.Instr, q seatSeq, n int) bool {
	g := k.gen
	if g == nil || g.anyRepl&(rbDblLife|rbPlusLife) != rbDblLife|rbPlusLife || !s.lifeParkOK(k, in, q, n) {
		return false
	}
	p := q.s[0]
	if g.anyCantGain && s.playerBanned(k, kop.StatCantGainLife, p) {
		return false
	}
	cand, c, plus, ok := s.lifeCands(k, p)
	if !ok || plus == 0 || bits.OnesCount8(plus) == c {
		return false
	}
	s.g.ro.replHdr = replHdr{n: uint8(c), plus: plus, p: p, cand: cand, an: int16(n)}
	s.g.nc = ncDmg{code: PlayerCode(p), ctrl: ncLife | p, n: ncPend}
	return true
}

// lifeOrderOptions poses the ask: the replacements not yet applied, one per
// candidate, by gorge's scan index.
func (s *State) lifeOrderOptions(d *Decision) {
	ro := &s.g.ro
	d.Kind, d.Sub, d.Player = DkChoose, ChGenReplOrder, ro.p
	for i := 0; i < int(ro.n); i++ {
		if ro.used&(1<<i) == 0 {
			d.add(MakeAction(AkChoose, TCode(ro.cand[i]), 0, uint8(i)))
		}
	}
}

// lifeApply applies candidate i to the running amount, as gorge applies each
// pick as it is picked (CR 616.1e).
func (s *State) lifeApply(k *prep, i int) {
	ro := &s.g.ro
	if ro.plus&(1<<i) != 0 {
		sl := s.objs[ro.cand[i]].slot
		_, add, _ := lifeReplLine(k, s.cardAt(sl))
		ro.an = s.sat16(int(ro.an) + add)
	} else if ro.an <= 0x3fff {
		ro.an *= 2
	}
}

// lifeOrderAnswer applies the pick and asks again while two or more
// replacements of different kinds remain (gorge re-collects after each pick);
// else the rest apply in scan order, the amount lands in the record and the
// op re-executes.
func (s *State) lifeOrderAnswer(k *prep, a Action) {
	ro := &s.g.ro
	i := int(a.Mode())
	if DebugLifeReplFixed {
		i = s.lifeFixedPick()
	}
	if i >= int(ro.n) || ro.used&(1<<i) != 0 {
		return
	}
	ro.used |= 1 << i
	s.lifeApply(k, i)
	for {
		rest, plus := 0, uint8(0)
		for j := 0; j < int(ro.n); j++ {
			if ro.used&(1<<j) == 0 {
				rest++
				plus |= ro.plus & (1 << j)
			}
		}
		if rest >= 2 && plus != 0 && bits.OnesCount8(plus) != rest {
			return // asked again
		}
		for j := 0; j < int(ro.n); j++ {
			if ro.used&(1<<j) == 0 {
				s.lifeApply(k, j)
			}
		}
		break
	}
	s.g.nc.n = ro.an
	s.g.ro = replOrd{}
	s.g.plusBatchAsked = true
	s.res.ask = raNone
	s.runProg(k)
}

// lifeFixedPick is the mutation's pick: the first unused doubler.
func (s *State) lifeFixedPick() int {
	ro := &s.g.ro
	for j := 0; j < int(ro.n); j++ {
		if ro.used&(1<<j) == 0 && ro.plus&(1<<j) == 0 {
			return j
		}
	}
	return 0
}

// lifeGainN is p's gain of n after the life-gain replacements, for a site
// that cannot suspend: a doubler and a plus-N competing for it is gorge's
// CR 616.1 order ask, which only the staged site poses, so the game stalls.
func (s *State) lifeGainN(k *prep, p uint8, n int) int {
	if k.gen.anyRepl&(rbDblLife|rbPlusLife) == rbDblLife|rbPlusLife && s.lifeMixed(k, p) {
		s.stall(StallLifeRepl)
		return n
	}
	return s.replLife(k, p, n)
}

// lifeParked reports that the order ask in progress is a parked GainLife op's.
func (s *State) lifeParked() bool { return s.g.nc.n == ncPend && s.g.nc.ctrl&ncLife != 0 }

// orderOptions is the replacement-order ask's options: a parked ask body's
// (repl_dmbody.go), a parked life gain's, else the damage walk's.
func (s *State) orderOptions(d *Decision) {
	if s.g.nc.body != 0 {
		s.replBodyOptions(d)
		return
	}
	if s.lifeParked() {
		s.lifeOrderOptions(d)
		return
	}
	s.replOrderOptions(d)
}

// orderAnswered answers the ask of a parked ask body (repl_dmbody.go), a
// contested token creation or a parked life gain, true when it was one of
// them; the damage walk's is the caller's.
func (s *State) orderAnswered(k *prep, a Action) bool {
	switch {
	case s.g.nc.body != 0:
		return s.replBodyNCAnswer(k, a) // an ask body's park (repl_dmbody.go)
	case s.g.tok.on:
		s.tokAnswer(k, a) // a contested token creation (repl_tokorder.go)
	case s.lifeParked():
		s.lifeOrderAnswer(k, a)
	default:
		return false
	}
	return true
}
