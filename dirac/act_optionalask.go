package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Optional asks on the discard/sacrifice ops (kop.OptAsk): gorge poses a
// may-election before an Optional$ pick (effects/cardflow.go effDiscard,
// zone_bulk.go effSacrifice), and the kernel mirrors it as its own
// suspended decisions.

// optDiscard poses an Optional$ discard (the core: an unfiltered one). Mirrors
// gorge effDiscard's TgtChoose arm (discardBounds: min 0, max min(NumCards,
// eligible)) and its Mode$ Hand arm (effDiscardHand: a whole-hand
// may-election, asked with no empty-hand guard -- an empty-hand "yes"
// discards nothing either way). Hand position order keeps ChDiscard's
// canonical keys stable across the picks.
func (s *State) optDiscard(k *prep, in *kop.Instr, p uint8, n int) bool {
	pl := &s.pl[p]
	if in.C&kop.OptAll != 0 { // Mode$ Hand: the whole-hand may-election
		s.res.ask, s.res.askP, s.res.chosen = raOptDiscMay, p, p
		s.ctl = ctlResolve
		return false
	}
	if int(pl.handN) == 0 {
		return true // gorge: eligible == 0 -> nothing
	}
	if in.C&kop.OptAny != 0 { // AnyNumber$: no election, Min 0, Max the eligible count
		s.optDiscardAsk(p, p, int(pl.handN))
		return false
	}
	if n < 1 {
		return true // discardBounds keeps a resolved zero at zero: askMax == 0 skips the ask
	}
	s.res.ask, s.res.askP, s.res.chosen = raOptDiscMay, p, p // the may-election
	s.ctl = ctlResolve
	return false
}

// optSac poses an Optional$ sacrifice (the core). Mirrors gorge effSacrifice's
// player-target arm: a non-strict optional asks 0..min(Amount, eligible)
// whenever one eligible permanent exists (no strict-supersets gate); the
// Optional$+StrictAmount$ disjoint {0, Amount} shape is refused at lower
// time, so the kernel never sees it.
func (s *State) optSac(k *prep, in *kop.Instr, p uint8, n int) bool {
	if n <= 0 {
		return true // gorge: Amount <= 0 has no meaningful answer
	}
	if s.countSac(k, p, in.F, s.resSrcSlot(k)) == 0 {
		return true // gorge: eligible == 0 -> nothing
	}
	s.res.ask = raOptSac
	s.res.askP = p
	s.res.peekI, s.res.peekN = 0, 0
	s.res.left = uint8(min(n, 255)) // Max = min(Amount, eligible); countSac already > 0
	s.ctl = ctlResolve
	return false
}

// startOptSac poses a generated table's Optional$ sacrifice player by
// player (gorge effSacrifice asks each player in turn; a player with no
// eligible permanent is skipped) -- the generated-table twin of startSac,
// with the Min 0 ask.
func (s *State) startOptSac(k *prep, in *kop.Instr, p uint8) bool {
	for p != 255 {
		n := s.value(k, in.A)
		if n > 0 && s.countSac(k, p, in.F, s.resSrcSlot(k)) > 0 {
			s.res.ask, s.res.askP = raOptSac, p
			s.res.peekI, s.res.peekN = 0, 0
			s.res.left = uint8(min(n, 255))
			s.ctl = ctlResolve
			return false
		}
		p = s.repPop()
	}
	return true
}

// optDiscardGen poses an Optional$ discard in a generated table (a
// filtered or opponent-chosen one). The chooser semantics follow gorge's
// two arms: chooseMode (ChooseOpp) asks the controller 0..min(NumCards,
// eligible) with no election and no strict-supersets gate; TgtChoose asks
// the discarding player a may-election first, then a Min 1 pick.
func (s *State) optDiscardGen(k *prep, in *kop.Instr, e *stackEntry) (ok, done bool) {
	if in.F == 0 && in.C&kop.ChooseOpp == 0 {
		return true, false // the core's optDiscard
	}
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		return true, false // the core's (breaks: nothing)
	}
	pl := &s.pl[p]
	m := 0
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		if s.matchCardGen(k, in.F, s.objs[o].card, p, e.ctrl, o) {
			m++
		}
	}
	if m == 0 {
		return true, true // gorge: eligible == 0 -> nothing
	}
	n := s.value(k, in.A)
	if n < 1 && in.C&kop.OptAny == 0 {
		return true, true // discardBounds keeps a resolved zero at zero: askMax == 0 skips the ask
	}
	if in.C&kop.OptAny == 0 && in.C&kop.ChooseOpp == 0 {
		s.res.ask, s.res.askP, s.res.chosen = raOptDiscMay, p, p // the may-election
		s.ctl = ctlResolve
		return false, true
	}
	maxv := min(n, m)
	if in.C&kop.OptAny != 0 {
		maxv = m // AnyNumber$ bounds: 0..eligible, NumCards unread
	}
	askP := p
	if in.C&kop.ChooseOpp != 0 {
		askP = e.ctrl // chooseMode: the controller chooses from the revealed hand
	}
	s.optDiscardAsk(askP, p, maxv)
	return false, true
}

// optDiscardAsk poses the optional discard's card pick: askP answers,
// chosen discards, left the Max (Min is re-derived from the op: 0 for
// AnyNumber$ and chooseMode, 1 after a may-election).
func (s *State) optDiscardAsk(askP, chosen uint8, maxv int) {
	s.res.ask, s.res.peekI, s.res.peekN = raOptDisc, 0, 0
	s.res.askP = askP
	s.res.chosen = chosen
	s.res.left = uint8(min(maxv, 255))
	s.ctl = ctlResolve
}

// optDiscardAccept applies a may-election's "yes": the whole hand (Mode$
// Hand), the single eligible card (gorge's strict-supersets rule), or the
// Min 1 pick ask. Reports whether a pick ask is now live.
func (s *State) optDiscardAccept(k *prep) bool {
	r := &s.res
	in := &s.prog(k)[r.pc]
	p := r.chosen
	pl := &s.pl[p]
	if in.C&kop.OptAll != 0 { // Mode$ Hand: the whole hand
		for pl.handN > 0 {
			s.discardObj(k, pl.hand[0])
		}
		r.ask = raNone
		return false
	}
	m := 0
	var only Obj
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		if s.discardOK(k, o) {
			m++
			only = o
		}
	}
	if m <= 1 { // one (or zero) eligible: discarded with no question (strict supersets)
		if m == 1 {
			s.discardObj(k, only)
		}
		r.ask = raNone
		return false
	}
	n := s.value(k, in.A)
	if n < 1 {
		n = 1
	}
	s.optDiscardAsk(p, p, min(n, m))
	return true
}

// optaskLeft reports whether another unpicked pick exists for the live
// optional ask (gorge's repose re-offers every unpicked candidate; when
// none remains the answer commits).
func (s *State) optaskLeft(k *prep) bool {
	r := &s.res
	switch r.ask {
	case raOptDisc:
		pl := &s.pl[r.chosen]
		for i := 0; i < int(pl.handN); i++ {
			o := pl.hand[i]
			if !s.optaskPicked(o) && s.discardOK(k, o) {
				return true
			}
		}
		return false
	case raOptSac:
		in := &s.prog(k)[r.pc]
		for m := s.ctrlM[r.askP]; m.any(); m = m.rest() {
			sl := m.first()
			if !s.optaskPicked(s.perms[sl].obj) && s.sacMatch(k, in.F, sl, r.askP, s.resSrcSlot(k)) {
				return true
			}
		}
		return false
	}
	return false
}

// optaskPicked reports whether o is among the picks recorded so far.
func (s *State) optaskPicked(o Obj) bool {
	r := &s.res
	for j := 0; j < int(r.peekI); j++ {
		if r.arr[j] == o {
			return true
		}
	}
	for j := 0; j < int(r.peekN); j++ {
		if r.peek[j] == o {
			return true
		}
	}
	return false
}

// optaskFireDiscard discards the picks recorded so far, in pick order. Gorge
// moves the answered discards in answer order and asks the CR 616.1 order
// inside the first card's move, so the first pick whose own move competes and
// every later pick wait in the queue, competing or not: at the ask a later
// answered card is still in hand.
func (s *State) optaskFireDiscard(k *prep) {
	r := &s.res
	n := int(r.peekI) + int(r.peekN)
	first := -1
	for i := 0; i < n; i++ {
		if s.graveMoveCompetes(k, s.optaskPickAt(r, i)) {
			first = i
			break
		}
	}
	if first < 0 {
		first = n // none competes: every pick moves now
	}
	for i := 0; i < first; i++ {
		s.discardObj(k, s.optaskPickAt(r, i))
	}
	for i := first; i < n; i++ {
		o := s.optaskPickAt(r, i)
		if s.park(k, o, parkDiscard, 0) {
			continue
		}
		if !s.parkDiscardQuiet(o, 0) {
			s.capFull(StallReplCands)
			break
		}
	}
	r.peekI, r.peekN = 0, 0
}

// optaskPickAt is the i-th recorded pick, arr first and then peek.
func (s *State) optaskPickAt(r *resCtx, i int) Obj {
	if i < int(r.peekI) {
		return r.arr[i]
	}
	return r.peek[i-int(r.peekI)]
}

// optaskFireSac sacrifices the picks recorded so far together, in pick order
// (CR 701.21a: gorge moves them as one batch, so a permanent that sees
// another die, Rakish Crew's outlaws, sees the other picks go with it).
func (s *State) optaskFireSac(k *prep) {
	r := &s.res
	for i := 0; i < int(r.peekI); i++ {
		s.addSac(s.objs[r.arr[i]].slot)
	}
	for i := 0; i < int(r.peekN); i++ {
		s.addSac(s.objs[r.peek[i]].slot)
	}
	r.peekI, r.peekN = 0, 0
	s.flushSac(k)
}

// optaskPick records a pick; the arrays hold one slot per hand card (arr) plus
// peek, so the flush branch is a guard. Reports whether another pick is still
// wanted: gorge's KModes multi-pick re-poses after each pick while the count is
// under the Max, offering Done once the count reaches the Min (resolveOptions
// reads the running peekI), and closes on Done or when the unpicked eligible
// set runs dry.
func (s *State) optaskPick(k *prep, a Action, fire func(*prep, Obj)) bool {
	r := &s.res
	o := Obj(a.Source())
	if int(r.peekI) < len(r.arr) {
		r.arr[r.peekI] = o
		r.peekI++
	} else if int(r.peekN) < len(r.peek) {
		r.peek[r.peekN] = o
		r.peekN++
	} else {
		// Arrays full (more picks than the hand holds): flush in pick order,
		// then keep the new one. Firing the new pick immediately rotated the
		// graveyard order (CR 404.3).
		for i := 0; i < int(r.peekI); i++ {
			fire(k, r.arr[i])
		}
		for i := 0; i < int(r.peekN); i++ {
			fire(k, r.peek[i])
		}
		r.peekI, r.peekN = 0, 0
		r.arr[0] = o
		r.peekI = 1
	}
	r.left--
	return r.left > 0 && s.optaskLeft(k)
}

// optDiscardMin is the optional discard pick's Min: 0 for AnyNumber$ and
// chooseMode (the chooser may take nothing outright), 1 after a
// may-election (gorge re-walks the TgtChoose arm with min 1).
func (s *State) optDiscardMin(k *prep) int {
	in := &s.prog(k)[s.res.pc]
	if in.C&kop.OptAny != 0 || in.C&kop.ChooseOpp != 0 {
		return 0
	}
	return 1
}
