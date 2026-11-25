package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Suspend (CR 702.62; gorge rules turn.go's upkeep walk queues the tick,
// emit.go's TIME hook queues the may-cast, and engine/rules/suspend_trigger.go
// resolves both; cast_commit.go's suspend branch). 0 B of State:
//
//   - a suspended card is a mayPlay record with until = kop.DurSuspend
//     (obj/ver = the exiled card, ctrl = its owner, turn = its time counters);
//     the generic play surfaces skip it (ownWalkerRecord) and cleanup keeps
//     it while the card stays exiled (plotLive);
//   - the special action is AkPlot with mode plotModeSuspend, offered from
//     the hand under the card's own cast timing and paid like plot's (a
//     special action, never a spell);
//   - the owner's upkeep queues a REAL triggered ability per suspended card
//     (gorge's SuspendTick trigger, CR 603.3): an abDelayed entry marked
//     suspendTickAmt, placed and passed like any trigger, whose RESOLUTION
//     (resolveSuspendTick) rechecks the intervening-if (CR 603.4) and removes
//     one time counter. The last counter queues the may-cast trigger
//     (suspendCastAmt; gorge's SuspendLast), whose resolution poses the
//     owner's CR 702.62a cast-or-not ask (suspend_cast_yes /
//     suspend_cast_no) -- only if it could be cast: no CantBeCast static and
//     a legal target ("if able");
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

// A DurSuspend record's kind: counting (0) or queued for the final-counter
// cast trigger.
const (
	suspendQueued uint8 = 1
	// suspendHaste marks the record of a creature on the battlefield that
	// was cast through the last counter: haste while ctrl controls it.
	suspendHaste uint8 = 3
)

// The abDelayed entry's trigAmt markers naming suspend's two triggers (speed's
// is -1, the venture's -2, the monarch's -3/-4, the rad drain's -5, the Ring
// emblem's -6/-7/-8, the extra-turn rider's -9, the staged loss copy's -10):
// free offsets.
const (
	suspendTickAmt = -11 // the upkeep time-counter tick (CR 702.62b)
	suspendCastAmt = -12 // the last-counter may-cast (CR 702.62a)
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
	*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: ob.ver, ctrl: ob.owner(), until: kop.DurSuspend, turn: mayWord(uint16(n))}
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
	s.placeFrom(o, kop.ZoneExile, false, from, false, false)
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

// suspendTick queues each of the active player's suspended cards' CR 702.62b
// upkeep trigger as the upkeep begins (gorge's SuspendTick, queued by
// turn.go's walk and placed by the ordinary drain): one abDelayed entry
// marked suspendTickAmt per card, in exile order, after the step's other
// triggers. The counter leaves only as the trigger RESOLVES
// (resolveSuspendTick), the intervening-if rechecked there.
func (s *State) suspendTick(k *prep) {
	if DebugSuspendNoTick || s.g.nmay == 0 || s.npend+1 >= MaxPending {
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
		s.enqueue(o, uint8(s.objs[o].ver), s.objs[o].card, abDelayed, r.ctrl, NoObj, r.ctrl, suspendTickAmt)
	}
}

// resolveSuspendTick is the upkeep tick trigger's resolution (gorge
// startSuspendTick): the intervening-if — the card is still the exiled
// suspended card the trigger names (CR 603.4) — is rechecked here, a card
// that moved or lost its record fizzles with no effect, and the tick removes
// exactly one time counter through the ordinary removal event
// (fireCtrRemovedExile). The last counter converts the record to the
// may-cast queue and queues the may-cast trigger (gorge's SuspendLast) after
// the ones the removal event made.
func (s *State) resolveSuspendTick(k *prep, e *stackEntry) {
	o := e.obj
	ob := &s.objs[o]
	i := s.suspendRec(o)
	if ob.zone != kop.ZoneExile || ob.ver != e.srcVer || i < 0 || DebugSuspendNoTick {
		s.finishResolve(k, false) // the intervening-if no longer holds (CR 603.4)
		return
	}
	r := s.mayAt(i)
	if r.turn == 0 || r.ctrl != s.active || k.t.Cards[ob.card].Suspend == 0 {
		s.finishResolve(k, false)
		return
	}
	r.turn--
	s.fireCtrRemovedExile(k, o, int(kop.CtrTime), 1, int(r.turn)) // trig_counterremoved.go
	if r.turn == 0 {
		q := *r
		q.kind = suspendQueued
		s.dropMay(i)
		*s.mayAt(int(s.g.nmay)) = q
		s.g.nmay++
		s.enqueue(o, uint8(ob.ver), ob.card, abDelayed, q.ctrl, NoObj, q.ctrl, suspendCastAmt)
	}
	s.finishResolve(k, false)
}

// resolveSuspendCast is the may-cast trigger's resolution (gorge
// askSuspendCast): a card that left exile, a CantBeCast static forbids or
// that has no legal target consumes the record without an ask ("if able");
// otherwise the owner's CR 702.62a cast-or-not ask is posed as a resolution
// ask (raSuspendCast, like the Siege defeat's) and the resolution is spent —
// suspendCastApply finishes it. It reports whether the ask was posed.
func (s *State) resolveSuspendCast(k *prep, e *stackEntry) bool {
	i := s.suspendRec(e.obj)
	if i < 0 || s.mayAt(i).kind != suspendQueued {
		return false
	}
	r := *s.mayAt(i)
	ob := &s.objs[r.obj]
	if ob.zone != kop.ZoneExile || ob.ver != r.ver || k.t.Cards[ob.card].Suspend == 0 ||
		k.gen.anyCantCast && s.castForbidden(k, r.ctrl, ob.card, r.obj) || !s.spellTargetsFeasible(k, ob.card, r.ctrl) {
		s.dropMay(i) // "if able": nothing to choose, the card stays in exile
		return false
	}
	s.res.ask, s.res.askP = raSuspendCast, r.ctrl
	s.ctl = ctlResolve
	return true
}

// suspendCastOptions fills the ask (gorge's KChoose suspend_cast_yes /
// suspend_cast_no).
//
//kmanifest:askopts raSuspendCast
func (s *State) suspendCastOptions(k *prep, d *Decision) {
	d.Sub = ChSuspendCast
	d.add(MakeAction(AkChoose, TCode(s.res.e.obj), 0, 1))
	d.add(MakeAction(AkChoose, TCode(s.res.e.obj), 0, 0))
}

// suspendCastApply applies the ask's answer (gorge tapeSuspendAnswer): the
// record is consumed either way and the trigger leaves the stack; yes casts
// the card for no mana cost, the priority holder keeping priority after (CR
// 117.3b, gorge's emit Priority(active) before beginCast).
//
//kmanifest:askapply raSuspendCast
func (s *State) suspendCastApply(k *prep, a Action) {
	s.res.ask = raNone
	if i := s.suspendRec(s.res.e.obj); i >= 0 {
		s.dropMay(i)
	}
	s.finishResolve(k, false)
	if a.Mode() == 0 {
		return // suspend_cast_no: it stays in exile
	}
	o := s.res.e.obj
	if s.objs[o].zone != kop.ZoneExile {
		return
	}
	holder := s.prio
	s.prio = s.res.e.ctrl
	s.beginCast(k, o, castFree, 0)
	s.bld().e.trigPl = trigPlSuspend
	s.prio = holder
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
