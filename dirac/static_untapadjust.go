package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// StatUntapAdjust is Static Orb's player keyword
// (`Affected$ Player | AddKeyword$ UntapAdjust:Permanent:2`, lowered by
// kgen/lower_playerstatic.go): during each player's untap step at most
// Static.A permanents matching Static.Filter untap, and that player chooses
// which of the tapped ones stay tapped (CR 702.19e's "can't untap more than N
// permanents" family; gorge rules/untap.go untapAdjustCap and turn.go
// finishUntapStep). The static is read live from every battlefield, because
// `Affected$ Player` covers both seats, under Cond2 (the printed
// `IsPresent$ Card.Self+untapped` gate). No State bytes.

// untapAdjustCap returns the most restrictive live cap, with the capped
// permanents' filter and the static's source/controller for the filter's
// perspective. A card with no cap reports ok=false.
func (s *State) untapAdjustCap(k *prep) (filter kop.FilterID, limit int, src Slot, ctrl uint8, ok bool) {
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatUntapAdjust || !s.statCond(k, st) {
			return true
		}
		n := int(a.Static.A)
		if n < 0 {
			return true
		}
		if !ok || n < limit {
			filter, limit, src, ctrl, ok = a.Static.Filter, n, st.sl, st.ctrl, true
		}
		return true
	})
	return
}

// untapAdjustCands lists the tapped permanents in ord that the cap counts and
// that would untap this step. The exclusions mirror gorge's
// untapAdjustCandidates: a one-shot skip (exert / CantUntapNextStep), an
// Effect's don't-untap lock, and a stun counter that replaces the untap.
func (s *State) untapAdjustCands(k *prep, ord []Slot, filter kop.FilterID, src Slot, ctrl uint8) []Slot {
	var out []Slot
	for _, sl := range ord {
		p := &s.perms[sl]
		if p.flags&pfTapped == 0 || p.flags&pfSkipUntap != 0 {
			continue
		}
		if s.skipsUntap(k, sl) {
			continue
		}
		if p.ctrLow(int(kop.CtrStun)) > 0 && !s.untapLocked(p.obj, false) {
			continue
		}
		if filter != 0 && !s.matchPermGen(k, filter, sl, ctrl, src, layerFull) {
			continue
		}
		out = append(out, sl)
	}
	return out
}

// untapCapAsk poses the next cap pick when the board still has more tapped
// candidates than the cap lets untap. It parks the step on ctlGenAsk with
// untapSl == NoSlot (the cap marker; the per-permanent keep-tapped election
// keeps a real slot there), and returns false when the election is done.
func (s *State) untapCapAsk(k *prep, ord []Slot) bool {
	filter, limit, src, ctrl, ok := s.untapAdjustCap(k)
	if !ok {
		return false
	}
	if len(s.untapAdjustCands(k, ord, filter, src, ctrl)) <= limit {
		return false
	}
	s.g.untapOn, s.g.untapNext, s.g.untapUp = true, 0, slotMask{}
	s.g.untapSl, s.g.untapAns = NoSlot, 1
	s.g.ctlRet, s.ctl = ctlStepBegin, ctlGenAsk
	return true
}

// untapCapOptions poses one pick of the cap election: every remaining
// candidate as keep_tapped over its permanent, in battlefield order. There is
// no AkDone: the cap's count fixes the number of picks, so the last answer
// finishes the election.
func (s *State) untapCapOptions(k *prep, d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChGenKeepTapped, s.active
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(s.active, s.ctrlM[s.active], ord[:], 0)
	filter, _, src, ctrl, ok := s.untapAdjustCap(k)
	if !ok {
		return
	}
	for _, sl := range s.untapAdjustCands(k, ord[:n], filter, src, ctrl) {
		d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 1))
	}
}

// untapCapAnswer records one keep-tapped pick on its permanent. The resumed
// scan consumes pfSkipUntap exactly as an exert skip does, so the keep set
// needs no State field.
func (s *State) untapCapAnswer(a Action) {
	if a.Kind() == AkChoose {
		if o := Obj(a.Source()); o != NoObj {
			if sl := s.objs[o].slot; sl != NoSlot {
				s.perms[sl].flags |= pfSkipUntap
				s.touchSl(sl)
			}
		}
	}
	s.ctl = s.g.ctlRet
}
