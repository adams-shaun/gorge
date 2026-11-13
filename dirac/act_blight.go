package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Blight (CR 701.68). Blight<N> is a cost that puts N
// -1/-1 counters on a creature the payer controls, the source included,
// and an effect that makes a player do the same. Cold and out of line:
// only a table naming OpBlight or a Blight cost reaches it.

// raGenBlightPick is OpBlight's resolution ask (gorge effBlight's
// "blight" KChoose), numbered past the earlier asks.
const raGenBlightPick = raGenManifestDread + 44

// ChGenBlightPick is OpBlight's pick: source = the creature blighted.
const ChGenBlightPick ChoiceKind = 202

// execBlight runs OpBlight: player Obj blights A (CR 701.68). With no
// creature the player controls nothing happens; with exactly one the
// counters are placed unasked (gorge effBlight's strict-supersets rule);
// with more the player is asked once.
func (s *State) execBlight(k *prep, in *kop.Instr) bool {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		return true
	}
	n := s.value(k, in.A)
	if n <= 0 {
		return true
	}
	var buf [MaxPerm]Slot
	c := s.blightCands(k, in, p, &buf)
	switch {
	case c == 0:
		return true
	case c == 1:
		s.blightSlot(buf[0], n)
		return true
	}
	s.res.ask, s.res.askP, s.res.left = raGenBlightPick, p, uint8(n)
	return false
}

// blightCands writes the battlefield permanents player p controls matching
// in.F, in battlefield order.
func (s *State) blightCands(k *prep, in *kop.Instr, p uint8, buf *[MaxPerm]Slot) int {
	src := s.resSrcSlot(k)
	n := 0
	for q := s.occ; q.any(); q = q.rest() {
		sl := q.first()
		if s.perms[sl].ctrl != p || !s.isCreature(k, sl) || !s.matchPerm(k, in.F, sl, p, src) {
			continue
		}
		buf[n] = sl
		n++
	}
	return n
}

// blightSlot puts n -1/-1 counters on the creature at sl (gorge
// blightApply's CounterChange).
func (s *State) blightSlot(sl Slot, n int) {
	k := s.k()
	if s.putCounterBlocked(k, sl, int(kop.CtrM1M1)) {
		return
	}
	p := &s.perms[sl]
	p.seeCtr(s)
	s.placeCounters(k, sl, int(kop.CtrM1M1), n, p.ctrl, ctrPutRaw)
	if n > 0 && k.anyCtrAdded {
		s.ctrNote(k, sl, int(kop.CtrM1M1))
	}
}

// askOptionsBlight builds OpBlight's pick options; false: not one.
func (s *State) askOptionsBlight(k *prep, d *Decision) bool {
	if s.res.ask != raGenBlightPick {
		return false
	}
	d.Sub = ChGenBlightPick
	var buf [MaxPerm]Slot
	n := s.blightCands(k, &s.prog(k)[s.res.pc], s.res.askP, &buf)
	for _, sl := range buf[:n] {
		d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
	}
	return true
}

// applyAskBlight applies OpBlight's pick; false: not one.
func (s *State) applyAskBlight(k *prep, a Action) bool {
	if s.res.ask != raGenBlightPick {
		return false
	}
	s.res.ask = raNone
	if a.Kind() == AkChoose {
		if ob := &s.objs[Obj(a.Source())]; ob.zone == kop.ZoneBattlefield && s.perms[ob.slot].ctrl == s.res.askP {
			s.blightSlot(ob.slot, int(s.res.left))
		}
	}
	s.continueRes(k, 0)
	return true
}
