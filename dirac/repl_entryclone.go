package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 616.1 order between an as-enters copy and the other replacements over
// the same entry (Waxen Shapethief copying a permanent while Thunderous
// Velocipede gives the entrant a +1/+1 counter). gorge asks the order with
// the permanent spell still on the stack (engine/rules/replacement_choice.go,
// replChoiceUpdated; the affected player is the entrant's controller), so
// the kernel poses the same KReplacement ask before the move and applies the
// chosen source first.
//
// gorge re-poses the ask for the sources not yet applied while two or more
// remain and they do not commute (replacement_updated.go,
// updatedReplacementsCommute: the copy never commutes, counter-only bodies
// and same-direction taps do), so a copy beside two counter sources asks
// "which of three" and then "which of the two left". The answers accumulate
// in s.g.pre.ordPick; a remainder that commutes applies in scan order.
//
// Not modelled: a source whose two abilities both match (the same candidate
// twice) keeps the own-line-first order, as does a competition gorge would
// ask but the kernel cannot class (a body that is neither a tap, an untap
// nor counters only commutes with nothing here, so it asks, as gorge's
// "other" class does).

// cloneEntryComp lists the replacements competing over the entry of the
// resolving permanent spell o, in gorge's scan order: the other permanents'
// matching enter replacements, then the entrant's own line (which follows its
// owner's battlefield when it enters from the stack). Only entryComp's
// other-source scan is reused: it does not add an entrant whose own body is
// neither a tap nor an untap, so the clone is appended here. over: more than
// eight candidates.
func (s *State) cloneEntryComp(k *prep, o Obj, ctrl uint8) entComp {
	skip := NoSlot
	if s.g.pre.ordMid {
		skip = s.objs[o].slot // the permanent is already on the battlefield
	}
	c, _ := s.entryComp(k, o, ctrl, skip, false)
	for i := uint8(0); i < c.n; i++ {
		if c.cand[i] == o {
			return c
		}
	}
	c.add(o, entClsOther)
	return c
}

// unpicked is the candidates the answers so far have not named, a bit per
// scan index.
func (c *entComp) unpicked(p *preAsk) (rest uint8) {
	for i := uint8(0); i < c.n; i++ {
		rest |= 1 << i
		for _, o := range p.ordPick[:p.ordN] {
			if o == c.cand[i] {
				rest &^= 1 << i
			}
		}
	}
	return rest
}

// restCommutes mirrors gorge's updatedReplacementsCommute over the candidates
// in rest: counter-only bodies, taps and untaps commute unless a tap meets an
// untap, and any other body (the copy) commutes with nothing.
func (c *entComp) restCommutes(rest uint8) bool {
	var tap, untap bool
	for i := uint8(0); i < c.n; i++ {
		b := uint8(1) << i
		switch {
		case rest&b == 0 || c.inert&b != 0:
		case c.tap&b != 0:
			tap = true
		case c.untap&b != 0:
			untap = true
		default:
			return false
		}
	}
	return !(tap && untap)
}

// inertBeside reports whether every candidate but o only adds counters.
func (c *entComp) inertBeside(o Obj) bool {
	for i := uint8(0); i < c.n; i++ {
		if c.cand[i] != o && c.inert&(1<<i) == 0 {
			return false
		}
	}
	return true
}

// distinct reports whether no source is a candidate twice.
func (c *entComp) distinct() bool {
	for i := uint8(0); i < c.n; i++ {
		for j := i + 1; j < c.n; j++ {
			if c.cand[i] == c.cand[j] {
				return false
			}
		}
	}
	return true
}

// cloneEntryOrderPark poses the order ask for the permanent spell o, which
// ctrl is about to put onto the battlefield as a copy, when another source
// also matches its entry. It reports whether the resolution suspended on the
// ask (or stalled at the candidate capacity).
func (s *State) cloneEntryOrderPark(k *prep, o Obj, ctrl uint8) bool {
	if s.g.pre.state != 2 || s.g.pre.kind != akClone || s.g.pre.obj != o || s.g.pre.ordSet {
		return false // no copy election was asked, or the order is already answered
	}
	if k.gen == nil || s.cloneOp(k, s.objs[o].card) == nil {
		return false
	}
	c := s.cloneEntryComp(k, o, ctrl)
	if c.over {
		s.stall(StallReplCands) // gorge asks whatever the count: never truncate the candidates
		return true
	}
	if c.n < 2 || c.n > 2 && !c.distinct() {
		return false
	}
	s.g.pre.kind, s.g.pre.state = akEntryOrder, 1
	s.g.pre.land = false
	s.g.pre.ordSet = true
	s.g.pre.ordN = 0
	s.res.active = false
	s.g.ctlRet = ctlPriority
	s.ctl = ctlGenAsk
	return true
}

// cloneEntryOrderOptions fills the ask: one option per competing source not
// yet named, in gorge's scan order (the source the option applies next),
// re-derived from the state that asking does not change.
func (s *State) cloneEntryOrderOptions(k *prep, d *Decision) {
	c := s.cloneEntryComp(k, s.g.pre.obj, s.cloneEntryCtrl())
	rest := c.unpicked(&s.g.pre)
	for i := uint8(0); i < c.n; i++ {
		if rest&(1<<i) != 0 {
			d.add(MakeAction(AkChoose, TCode(c.cand[i]), 0, i)) // mode: the scan index
		}
	}
}

// cloneEntryCtrl is the player the ask is posed to: the resolving spell's
// controller, or the entered permanent's once gorge has moved it in.
func (s *State) cloneEntryCtrl() uint8 {
	if s.res.ask == raGenRetChoice {
		return s.res.askP // a returning clone's controller (copy_returnorder.go)
	}
	if s.g.pre.ordMid {
		return s.perms[s.objs[s.g.pre.obj].slot].ctrl
	}
	return s.stack[s.sp-1].ctrl
}

// cloneEntryOrderAnswer takes an answer to the order ask: the named source
// applies next. While two or more sources remain and they do not commute the
// ask is posed again for them (CR 616.1f, gorge re-poses a live
// non-commuting remainder); otherwise the resolution goes on and the
// permanent enters with the picks in order, then the rest.
//
// gorge does not re-pose with the spell on the stack: the first answer moves
// the permanent in with every counter body folded into the move, and the
// re-ask comes with it on the battlefield, the copy still to apply
// (replacement_updated.go, resumeUpdatedComposition). The kernel does the
// same when every other source only adds counters; any other body in that
// competition stalls (StallEntryOrder) rather than guess.
func (s *State) cloneEntryOrderAnswer(k *prep, a Action) {
	p := &s.g.pre
	p.ordPick[p.ordN] = Obj(a.Source())
	p.ordN++
	c := s.cloneEntryComp(k, p.obj, s.cloneEntryCtrl())
	if rest := c.unpicked(p); bits.OnesCount8(rest) > 1 && !c.restCommutes(rest) {
		if p.ordMid {
			return // still akEntryOrder, state 1: the next Step poses it again
		}
		if !c.inertBeside(p.obj) {
			s.stall(StallEntryOrder)
			return
		}
		p.ordMid = true // the permanent enters now, the copy waits for the rest of the answers
	} else if p.ordMid {
		s.cloneEntryLateFinish(k)
		return
	}
	p.kind = akClone // the copy stays answered; ordPick names the order
	p.state = 2
	s.ctl = ctlPriority
	s.resolveTop(k)
	if s.g.pre.state != 1 {
		s.g.pre = preAsk{}
	}
}

// cloneEntryLatePark suspends the resolution of the permanent spell that just
// entered at sl on gorge's second ask, when the first answer left a
// non-commuting remainder (ordMid).
func (s *State) cloneEntryLatePark(sl Slot) bool {
	p := &s.g.pre
	if !p.ordMid || p.obj != s.perms[sl].obj {
		return false
	}
	p.kind, p.state = akEntryOrder, 1
	s.res.active = false
	s.g.ctlRet = ctlPriority
	s.ctl = ctlGenAsk
	return true
}

// cloneEntryLateFinish applies the entrant's own line, the copy, to the
// permanent that entered under the first answer, and goes on with the
// resolution it suspended: its enters triggers fire now, with the copy's
// abilities.
func (s *State) cloneEntryLateFinish(k *prep) {
	p := &s.g.pre
	o := p.obj
	sl := s.objs[o].slot
	ctrl := s.perms[sl].ctrl
	p.kind, p.state, p.ordMid = akClone, 2, false
	s.occ.clr(sl) // as-enters replacements see the board without the entrant (enterGen)
	s.ctrlM[ctrl].clr(sl)
	saveRep := s.g.replaced
	s.g.replaced = o
	s.runOwnEnterPrograms(k, sl, o, s.objs[o].card, ctrl, 0)
	s.g.replaced = saveRep
	s.occ.set(sl)
	s.ctrlM[ctrl].set(sl)
	s.refreshTy(k)
	if s.anyETB(k) {
		s.fireObj(k, kop.TrigETB, sl, o, ctrl, 0)
	}
	s.g.pre = preAsk{}
	s.afterResolve()
}

// runEntryOrdered applies the entering permanent's own enters programs and the
// other permanents' enter replacements in the order the answered ask named:
// the picks in turn, then every other source not named, and the entrant's own
// line last when it was not named (a named copy applies where it was named).
// The entrant's own line waits when gorge asks again with the permanent in
// (ordMid): cloneEntryLateFinish applies it.
//
// When every other source only adds counters the order named changes nothing
// the permanent shows: gorge folds those bodies into the move, matched against
// the entrant as it was cast (replacement_updated.go, bodyAbsorbed), whichever
// is named first, so a copy of a noncreature artifact still gets them. They
// apply before the copy whatever the answers were.
func (s *State) runEntryOrdered(k *prep, sl Slot, o Obj, cid CardID, ctrl, face uint8, g *genPrep) {
	if c := s.cloneEntryComp(k, o, ctrl); c.inertBeside(o) {
		s.runOthersEnterPrograms(k, sl, g, s.statM)
		if !s.g.pre.ordMid {
			s.runOwnEnterPrograms(k, sl, o, cid, ctrl, face)
		}
		return
	}
	rest := s.statM
	own := false
	for _, src := range s.g.pre.ordPick[:s.g.pre.ordN] {
		if src == o {
			s.runOwnEnterPrograms(k, sl, o, cid, ctrl, face)
			own = true
			continue
		}
		at := s.objs[src].slot
		s.runOthersEnterPrograms(k, sl, g, slotBit(at))
		rest = rest.without(at)
	}
	s.runOthersEnterPrograms(k, sl, g, rest)
	if !own {
		s.runOwnEnterPrograms(k, sl, o, cid, ctrl, face)
	}
}
