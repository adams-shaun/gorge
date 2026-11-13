package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Suspend (CR 702.62; gorge rules turn.go finishEnteredStep /
// startSuspendedCast, cast_commit.go's suspend branch). 0 B of State:
//
//   - a suspended card is a mayPlay record with until = kop.DurSuspend
//     (obj/ver = the exiled card, ctrl = its owner, turn = its time counters);
//     the generic play surfaces skip it (ownWalkerRecord) and cleanup keeps
//     it while the card stays exiled (plotLive);
//   - the special action is AkPlot with mode plotModeSuspend, offered from
//     the hand under the card's own cast timing and paid like plot's (a
//     special action, never a spell);
//   - the owner's upkeep removes a counter from each of its suspended cards
//     in exile order (gorge ticks the exile zone without the stack); a card
//     whose last counter went joins the ask queue (the record's kind), and
//     the owner is asked, before priority, whether to cast it free (CR
//     702.62a's "may", gorge's suspend_cast_yes / suspend_cast_no) -- only if
//     it could be cast: no CantBeCast static and a legal target;
//   - a creature spell cast that way gains haste until its caster loses
//     control of the spell or the permanent: a suspendHaste record (obj/ver =
//     the permanent, ctrl = the caster) that ctrKw reads while the permanent
//     is the caster's. Like gorge's grant (Card.Self+YouCtrl against the
//     caster) it resumes if control returns.
//
// Everything is behind prep.anySuspend.

// plotModeSuspend is the mode byte of AkPlot that names the suspend special
// action (mode 0 is plot's, mode 1 is foretell's).
const plotModeSuspend uint8 = 2

// FeatPlotSuspend is plotModeSuspend for the oracle adapter.
const FeatPlotSuspend = plotModeSuspend

// A DurSuspend record's kind: counting (0), queued for the final-counter cast
// ask, or its ask on screen.
const (
	suspendQueued uint8 = 1
	suspendPosed  uint8 = 2
	// suspendHaste marks the record of a creature on the battlefield that
	// was cast through the last counter: haste while ctrl controls it.
	suspendHaste uint8 = 3
)

// DebugSuspendNoHaste skips the haste grant and DebugSuspendNoTick the upkeep
// time-counter removal (mutation testing only: the lockstep must catch it).
var DebugSuspendNoHaste, DebugSuspendNoTick bool

// suspendRec is the index of o's DurSuspend record, or -1.
func (s *State) suspendRec(o Obj) int {
	ver := s.objs[o].ver
	for i := 0; i < int(s.g.nmay); i++ {
		if r := s.mayAt(i); r.until == kop.DurSuspend && r.kind != suspendHaste && r.obj == o && r.ver == ver {
			return i
		}
	}
	return -1
}

// suspendAdd records o, just exiled, as suspended with n time counters. It
// reports false when the permission pool is full (stalled).
func (s *State) suspendAdd(o Obj, n int) bool {
	ob := &s.objs[o]
	if s.mayFull() {
		s.capFull(StallMayPlay)
		return false
	}
	*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: ob.ver, ctrl: ob.owner(), until: kop.DurSuspend, turn: uint16(n)}
	s.g.nmay++
	return true
}

// suspendTime is the time counters on the exiled card o (0 when not
// suspended): the projection's TIME counter.
func (s *State) suspendTime(o Obj) uint8 {
	if i := s.suspendRec(o); i >= 0 {
		return uint8(s.mayAt(i).turn)
	}
	return 0
}

// offerSuspendHand offers hand card o's suspend special action. The caller
// has checked the card's cast timing and prohibition (gorge offers it
// beside the plain cast, under the same gates, and without a target check).
func (s *State) offerSuspendHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	s.offerSpecialHand(k, d, p, o, c, av, k.t.Cards[c].SuspendCost, MakeAction(AkPlot, TCode(o), 0, plotModeSuspend))
}

// suspendAction performs the suspend special action (CR 702.62a): pay the
// suspend cost, exile the card with its time counters. Not a cast: nothing
// is pushed and no cast trigger fires.
func (s *State) suspendAction(k *prep, o Obj, p uint8) {
	ob := &s.objs[o]
	c := ob.card
	cd := &k.t.Cards[c]
	s.payHandSpecial(k, p, s.adjustCost(k, p, c, o, cd.SuspendCost), o)
	from := s.detach(o)
	s.placeFrom(o, kop.ZoneExile, false, from, false)
	s.suspendAdd(o, int(cd.Suspend))
	s.passes = 0
}

// suspendExiled records o, a card its own spell just exiled with n time
// counters (Rousing Refrain, Inspiring Refrain): a card with Suspend in
// exile with a time counter is suspended however it got there (CR 702.62b).
func (s *State) suspendExiled(k *prep, o Obj, n int) {
	if s.objs[o].zone != kop.ZoneExile || n < 1 || k.t.Cards[s.objs[o].card].Suspend == 0 || s.suspendRec(o) >= 0 {
		return
	}
	s.suspendAdd(o, n)
}

// suspendTick runs as the upkeep of the active player begins (gorge
// finishEnteredStep): each of their suspended cards, in exile order, loses a
// time counter; one that lost its last joins the cast-ask queue in that
// order (gorge suspendedCasts).
func (s *State) suspendTick(k *prep) {
	if DebugSuspendNoTick || s.g.nmay == 0 {
		return
	}
	pl := &s.pl[s.active]
	var order [MaxObj]Obj
	n := copy(order[:], pl.exile[:pl.exileN])
	for _, o := range order[:n] {
		i := s.suspendRec(o)
		if i < 0 {
			continue
		}
		r := s.mayAt(i)
		if r.kind != 0 || r.turn == 0 || r.ctrl != s.active {
			continue
		}
		r.turn--
		if r.turn > 0 {
			continue
		}
		q := *r
		q.kind = suspendQueued
		s.dropMay(i)
		*s.mayAt(int(s.g.nmay)) = q
		s.g.nmay++
	}
}

// firstSuspendAsk is the index of the first queued or posed record, or -1.
func (s *State) firstSuspendAsk() int {
	for i := 0; i < int(s.g.nmay); i++ {
		if r := s.mayAt(i); r.until == kop.DurSuspend && r.kind != 0 && r.kind != suspendHaste {
			return i
		}
	}
	return -1
}

// suspendAsk poses the owner's CR 702.62a may-cast ask for the first queued
// card (gorge startSuspendedCast): a card that left exile, a CantBeCast static
// forbids or that has no legal target is dropped without an ask ("if able").
// It returns false when no ask is due, leaving control to the caller.
func (s *State) suspendAsk(k *prep, d *Decision) bool {
	for {
		i := s.firstSuspendAsk()
		if i < 0 {
			return false
		}
		r := s.mayAt(i)
		if r.kind == suspendPosed {
			return false // its ask is on screen
		}
		ob := &s.objs[r.obj]
		if ob.zone != kop.ZoneExile || ob.ver != r.ver || k.t.Cards[ob.card].Suspend == 0 ||
			k.gen.anyCantCast && s.castForbidden(k, r.ctrl, ob.card, r.obj) || !s.spellTargetsFeasible(k, ob.card, r.ctrl) {
			s.dropMay(i)
			continue
		}
		r.kind = suspendPosed
		s.g.ctlRet = s.ctl
		s.ctl = ctlGenAsk
		return s.genAskOptions(k, d)
	}
}

// suspendPosedOptions fills the ask for the posed record.
func (s *State) suspendPosedOptions(d *Decision) bool {
	i := s.firstSuspendAsk()
	if i < 0 || s.mayAt(i).kind != suspendPosed {
		return false
	}
	r := *s.mayAt(i)
	d.Kind, d.Sub, d.Player = DkChoose, ChSuspendCast, r.ctrl
	d.add(MakeAction(AkChoose, TCode(r.obj), 0, 1))
	d.add(MakeAction(AkChoose, TCode(r.obj), 0, 0))
	return true
}

// applySuspendAnswer applies the posed ask's answer (gorge suspendCastAnswer):
// the record is consumed either way; yes casts the card for no mana cost.
func (s *State) applySuspendAnswer(k *prep, a Action) bool {
	i := s.firstSuspendAsk()
	if i < 0 || s.mayAt(i).kind != suspendPosed {
		return false
	}
	r := *s.mayAt(i)
	s.dropMay(i)
	s.ctl = s.g.ctlRet
	if a.Mode() == 0 {
		return true // suspend_cast_no: it stays in exile
	}
	s.beginCast(k, r.obj, castFree, 0)
	s.bld().e.trigPl = trigPlSuspend
	return true
}

// suspendCast reports whether the spell entry e was cast by suspend's last
// counter.
func (e *stackEntry) suspendCast() bool {
	return e.kind == seSpell && e.trigPl == trigPlSuspend
}

// suspendEnter runs after the resolving permanent spell e entered at slot sl:
// a creature cast through suspend gains haste until its caster loses control
// (CR 702.62a).
func (s *State) suspendEnter(k *prep, sl Slot, e *stackEntry) {
	if DebugSuspendNoHaste || s.objs[e.obj].zone != kop.ZoneBattlefield || k.t.Cards[e.card].Types&kop.TCreature == 0 {
		return
	}
	if s.mayFull() {
		s.capFull(StallMayPlay)
		return
	}
	s.touchSl(sl)
	*s.mayAt(int(s.g.nmay)) = mayPlay{obj: e.obj, ver: s.objs[e.obj].ver, ctrl: e.ctrl, until: kop.DurSuspend, kind: suspendHaste}
	s.g.nmay++
}

// suspendHasteAt reports whether the permanent p holds suspend's haste: a
// live record whose caster still controls it.
func (s *State) suspendHasteAt(p *perm) bool {
	if !s.k().gen.anySuspend {
		return false
	}
	ver := s.objs[p.obj].ver
	for i := 0; i < int(s.g.nmay); i++ {
		if r := s.mayAt(i); r.until == kop.DurSuspend && r.kind == suspendHaste && r.obj == p.obj && r.ver == ver && r.ctrl == p.ctrl {
			return true
		}
	}
	return false
}

// suspendLive reports whether the DurSuspend record r still names its object:
// the exiled suspended card, or the battlefield creature holding its haste.
func (s *State) suspendLive(r mayPlay) bool {
	ob := &s.objs[r.obj]
	if r.kind == suspendHaste {
		return ob.ver == r.ver && ob.zone == kop.ZoneBattlefield
	}
	return s.plotLive(r)
}
