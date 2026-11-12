package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// sweepHits is the battlefield permanents matching f for the resolving
// entry, in gorge's sweep order: seat 0's battlefield in entry order, then
// seat 1's (effPutCounterAll / effUntapAll walk AliveFrom(0)).
func (s *State) sweepHits(k *prep, f kop.FilterID, ord *[MaxPerm]Slot) int {
	e := &s.res.e
	src := s.objRef(k, kop.RefSelf)
	var hit slotMask
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.matchPerm(k, f, sl, e.ctrl, src) {
			hit = hit.with(sl)
		}
	}
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		n = s.battlefieldOrder(p, hit.and(s.ctrlM[p]), ord[:], n)
	}
	return n
}

// putCounterAll: gorge putCounterAllSweep -- one CounterChange per matching
// permanent (CR 122.1; Doubling Season's replacement per permanent), a
// zero or negative amount places nothing.
func (s *State) putCounterAll(k *prep, in *kop.Instr) {
	n := s.value(k, in.A)
	if n <= 0 {
		return
	}
	var ord [MaxPerm]Slot
	cnt := s.sweepHits(k, in.F, &ord)
	if in.B.N == 1 { // lowered from Defined$ Valid <filter> (A)
		s.kdefPutHit(cnt)
	}
	s.g.lastCK = kop.CounterKind(in.C) // CounterAddedOnce reads the kind
	if in.B.N == kop.PutCtrCellFirst {
		s.g.ctrCell = slotMask{}
	}
	var placed [MaxPerm]Slot
	nplaced := 0
	for _, sl := range ord[:cnt] {
		m := s.placeCounters(k, sl, int(in.C), n, s.perms[sl].ctrl, ctrPutEffect)
		if m > 0 {
			// RememberPut$ names the recipients whose counter actually landed,
			// after replacements (gorge rememberPlaced over the placed list).
			placed[nplaced] = sl
			nplaced++
		}
		if m > 0 && k.anyCtrAdded {
			s.ctrNote(k, sl, int(in.C))
		}
		if in.B.N == kop.PutCtrRemAmount {
			s.putCounterRemAmount(k, in, m) // RememberAmount$ (act_remember.go)
		}
		if in.B.N == kop.PutCtrCellFirst || in.B.N == kop.PutCtrCellNext {
			s.fireCounterCell(k, sl, m) // trig_countercell.go
		} else if m > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
			s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(m))
		}
	}
	if in.B.N == kop.PutCtrRememberPut && !DebugRemPutNever {
		for _, sl := range placed[:nplaced] {
			s.remember(s.perms[sl].obj)
		}
	}
}

// untapAll: gorge effUntapAll -- every TAPPED matching permanent gets an
// Untap event; a stun counter is removed instead (CR 122.1d, as OpUntap),
// and an any-event ReplAnyUntap match replaces the event (untapBlocked).
func (s *State) untapAll(k *prep, in *kop.Instr) {
	var ord [MaxPerm]Slot
	cnt := s.sweepHits(k, in.F, &ord)
	var pre slotMask
	for _, sl := range ord[:cnt] {
		p := &s.perms[sl]
		if p.flags&pfTapped == 0 {
			continue
		}
		s.touchSl(sl)
		if s.untapBlocked(k, sl) {
			continue
		}
		if p.ctrLow(int(kop.CtrStun)) > 0 {
			p.seeCtr(s)
			p.subCtrN(s, int(kop.CtrStun), 1) // a stun counter is removed instead (CR 122.1d)
			continue
		}
		p.flags &^= pfTapped
		s.untapEnds(k, p.obj) // the lock and the host-tapped grants end at the first untap (static_animatewhiletapped.go)
		pre = pre.with(sl)
	}
	s.fireUntapsGen(k, pre)
}

// fireGraveSac queues the Sacrificed triggers that work from a graveyard
// (TriggerZones$ Graveyard, Curious Cadaver; CR 113.6k) for the sacrifice at sl,
// after the battlefield's.
//
//go:noinline
func (s *State) fireGraveSac(k *prep, sl Slot) {
	ev := &s.perms[sl]
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if s.objs[o].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind == kop.AbTriggered && a.Trig.Zone == kop.ZoneGraveyard && a.Trig.Event == kop.TrigSacrificed {
						s.objTrigGen(k, kop.TrigSacrificed, trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}, a, sl, ev.obj, ev.ctrl, 0)
					}
				}
			}
		}
	}
}

// fireGraveDies queues the Dies triggers that work from a graveyard
// (TriggerZones$ Graveyard, Furious Forebear) for the death at sl, after the
// battlefield's.
//
//go:noinline
func (s *State) fireGraveDies(k *prep, sl Slot) {
	ev := &s.perms[sl]
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if s.objs[o].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind == kop.AbTriggered && a.Trig.Zone == kop.ZoneGraveyard && a.Trig.Event == kop.TrigDies {
						s.objTrigGen(k, kop.TrigDies, trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}, a, sl, ev.obj, ev.ctrl, 0)
					}
				}
			}
		}
	}
}

// tapAll is OpTapAll: gorge effTapAll walks seat 0's battlefield then seat
// 1's in entry order and taps (EmitTap) each UNTAPPED permanent matching
// ValidCards$; each tap fires its Taps triggers as it happens. Obj (a
// target player) restricts the walk to that player's battlefield.
func (s *State) tapAll(k *prep, in *kop.Instr) {
	only := uint8(255)
	if in.Obj != kop.RefNone {
		if only = s.playerRef(k, in.Obj); only == 255 {
			return
		}
	}
	var ord [MaxPerm]Slot
	cnt := s.sweepHits(k, in.F, &ord)
	tb := s.tapBatchOpen(k) // one TapAll resolution is one tapping action (trig_tapall.go)
	defer s.tapBatchClose(tb)
	for _, sl := range ord[:cnt] {
		p := &s.perms[sl]
		if p.flags&pfTapped != 0 || only != 255 && p.ctrl != only {
			continue
		}
		p.flags |= pfTapped
		s.touchSl(sl)
		if s.subs[kop.TrigTaps].any() {
			s.fireTapsBy(k, sl, s.effTapper(sl))
		}
		s.markTapped(k, sl)
	}
}
