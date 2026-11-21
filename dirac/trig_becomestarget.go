package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// the self form of the BecomesTarget watcher --
// "whenever this becomes the target of a spell or ability an opponent
// controls" (gorge trigmatch/actions.go becomesTargetMatches: ValidTarget$
// Card.Self matched against the TargetsChosen event's chosen Ids,
// ValidSource$ SpellAbility.OppCtrl resolved against the targeting stack
// object's controller). The trigger builder lowers it to a TfSelf line with
// WhoseOpp (dirac/kgen/lower_trigger.go, build_ab.go); btWatchers walks it
// beside the other watchers, and objTrigGen's TfSelf match fires it only for
// the targeted permanent itself. Cold: reached only from btWatchers.

// DebugBtSelfOff makes btWatchers skip a self-filtered BecomesTarget line,
// the mutation proof that the oracle's self triggers run through this path.
// Never set in production.
var DebugBtSelfOff bool

// BtStats, when DebugBtStats is non-nil, counts the object-targeted
// BecomesTarget triggers the games queued (Queues), the player half's
// (PlayerQueues) and those whose ValidSource$ is an ability
// (SrcAbilityQueues). The plain self walk's queues also count in Queues.
// SrcCtrlReads counts the TrigNSrcCtrl triggers that resolved
// (trig_srcctrllive.go), SrcGone those whose targeting object had left the
// stack and the battlefield by then (a countered spell, a sacrificed source).
// The lockstep tests read them to prove the games reached the walks; nothing
// else reads them.
type BtStats struct {
	Queues, PlayerQueues, SrcAbilityQueues int
	SrcCtrlReads, SrcGone                  int
}

// DebugBtStats is the lockstep hook of the BecomesTarget walk. Never set in
// production.
var DebugBtStats *BtStats

// btSelfOff reports whether listener ability a is the self form and the
// mutation switch is on. TfSelf is the structural test: the trigger builder
// sets it only when the whole filter is the bare self clause, so a future
// self spelling cannot slip past a hand-maintained parameter list.
func btSelfOff(a *kop.Ability) bool {
	return DebugBtSelfOff && a.Trig.Flags&kop.TfSelf != 0
}

// BecomesTarget watchers (CR 603.2; gorge trigmatch
// becomesTargetMatches with ValidTarget$ other than Card.Self and
// ValidSource$ SpellAbility.OppCtrl: Pawpatch Recruit, Surrak, Elusive
// Hunter). Gorge emits one TargetsChosen event per chosen target, so a
// watcher triggers once per matching target of an opponent's spell or
// ability; its trigger object is the targeted object (becomesTargetReferent),
// which "other than that creature" (F2NotTriggerObj) reads. Cold and out of
// line: the hot site is targetedOne's subs[TrigBecomesTarget] gate, and
// these run only on a table with a watcher (genPrep.btWatch).
//
// The self form "whenever this becomes the target of a spell or ability an
// opponent controls" is the same gorge arm with
// ValidTarget$ Card.Self: its filter is the bare self clause (TfSelf), so
// objTrigGen's self match fires it only for the targeted permanent itself,
// and the WhoseOpp gate below keeps the controller's own spell or ability
// from firing it. Gorge source: engine/rules/trigmatch/actions.go
// becomesTargetMatches / becomesTargetSourceMatches (one TargetsChosen event
// per chosen target).

// btWatchers fires the non-Valiant BecomesTarget triggers for the permanent
// at sl (object o) becoming a target of stack entry e: a watcher whose Filter
// the permanent matches when e's controller is an opponent, and any other
// non-Valiant line through objTrigGen's usual match (the Valiant lines are
// becameTarget's).
//
//go:noinline
func (s *State) btWatchers(k *prep, sl Slot, o Obj, e *stackEntry) {
	ctrl := s.perms[sl].ctrl
	n0 := s.npend
	s.eachTrigger(k, kop.TrigBecomesTarget, func(l trigListener, a *kop.Ability) {
		if a.Trig.N == kop.TrigNValiant {
			return
		}
		if btSelfOff(a) {
			return // DebugBtSelfOff: the mutation must drop the self form
		}
		if !s.btSrcOK(k, l, a, e) {
			return
		}
		n := s.npend
		s.objTrigGen(k, kop.TrigBecomesTarget, l, a, sl, o, s.btTriggerPlayer(l, a, e, ctrl), 0)
		if s.npend > n && a.Trig.N == kop.TrigNSrcAbility {
			if st := DebugBtStats; st != nil {
				st.SrcAbilityQueues++
			}
		}
	})
	s.stampBtSA(e, n0)
	if st := DebugBtStats; st != nil && s.npend > n0 {
		st.Queues++
	}
}

// btSrcOK is a BecomesTarget line's ValidSource$ gate: the targeting stack
// entry's controller against the line's Whose, then the source form. A
// TrigNSrcSpell line also reads its card filter (btSpellSrcOK); a
// TrigNSrcAbility line requires an ability on the stack, not a spell
// (Loki, God of Mischief's Ability.YouCtrl).
func (s *State) btSrcOK(k *prep, l trigListener, a *kop.Ability, e *stackEntry) bool {
	if !whoseOK(a.Trig.Whose, l.ctrl, e.ctrl) {
		return false
	}
	switch a.Trig.N {
	case kop.TrigNSrcSpell:
		return s.btSpellSrcOK(k, l, a, e)
	case kop.TrigNSrcAbility:
		return e.kind != seSpell
	}
	return true
}

// btTriggerPlayer is the trigger player of a BecomesTarget line: the
// targeting spell or ability's controller for a source form (gorge's
// TriggeredSourceController / TriggeredSourceSAController), else the
// listener's controller.
func (s *State) btTriggerPlayer(l trigListener, a *kop.Ability, e *stackEntry, ctrl uint8) uint8 {
	if a.Trig.N == kop.TrigNSrcSpell || a.Trig.N == kop.TrigNSrcAbility || a.Trig.N == kop.TrigNSrcCtrl && !DebugSrcCtrlListener {
		return e.ctrl
	}
	return ctrl
}

// stampBtSA records the targeting stack entry on every trigger the walk
// just queued (gorge TriggerStack, the Defined$ TriggeredSourceSA referent).
// The pending trigger's trigSA is the object + 1 so the zero value means
// none, and trigObj keeps the targeted object (TriggerTarget). An ability's
// stack identity rides the pending trigger's wardSid: its source permanent
// can be the source of a trigger too (Kira's counter trigger is on the
// creature the ability targeted), so the object alone does not name the
// entry a counter removes. Panharmonicon echoes are copies of the same
// triggering, so they carry it too.
func (s *State) stampBtSA(e *stackEntry, n0 uint8) {
	if e.obj == NoObj {
		return
	}
	sid := uint8(0)
	if e.kind != seSpell {
		sid = e.sid
	}
	for i := n0; i < s.npend; i++ {
		lo, hi := saPack(objWord(e.obj + 1)) // code-ok: the +1 is a sentinel, not a code
		s.pend[i].trigSA, s.pend[i].skw = lo, s.pend[i].skw&^skwSAHi|hi
		s.pend[i].wardSid = sid
	}
}

// becameTargetPlayer fires the BecomesTarget watchers for a player target t
// of stack entry e (gorge becomesTargetMatches' player half: ValidTarget$
// You / Player / Opponent, such as Rayne, Loki and Dormant Gomazoa). The
// filter's FPlayer alternative is matched by castTargetsPlayer relative to
// the listener's controller; the queued trigger carries the targeted seat in
// btPl (gorge TriggerTarget{Player}) and the targeting entry in trigSA
// (TriggerStack), while trigObj stays empty for the player half.
//
//go:noinline
func (s *State) becameTargetPlayer(k *prep, t TCode, e *stackEntry) {
	p := CodeSeat(t)
	n0 := s.npend
	s.eachTrigger(k, kop.TrigBecomesTarget, func(l trigListener, a *kop.Ability) {
		if a.Trig.N == kop.TrigNValiant {
			return
		}
		if a.Trig.Flags&kop.TfSelf != 0 || btSelfOff(a) {
			return // the self forms name an object, the target itself
		}
		if a.Trig.Filter == 0 || !s.castTargetsPlayer(k, a.Trig.Filter, p, l.ctrl) {
			return
		}
		if !s.btSrcOK(k, l, a, e) || !s.trigOK(k, l, a) {
			return
		}
		n := s.npend
		s.queueTrig(k, l, a, NoObj, s.btTriggerPlayer(l, a, e, l.ctrl), 0)
		for i := n; i < s.npend; i++ {
			s.pend[i].defSeat = p + 1
		}
		if s.npend > n && a.Trig.N == kop.TrigNSrcAbility {
			if st := DebugBtStats; st != nil {
				st.SrcAbilityQueues++
			}
		}
	})
	s.stampBtSA(e, n0)
	if st := DebugBtStats; st != nil && s.npend > n0 {
		st.PlayerQueues++
	}
}

// btSpellTarget fires the Tf2SpellToo watchers for spell object o on the
// stack becoming a target of stack entry e: a creature spell the watcher's
// controller controls, targeted by an opponent's spell or ability (gorge
// ValidTarget$ alternative Spell.Creature+YouCtrl).
//
//go:noinline
func (s *State) btSpellTarget(k *prep, o Obj, e *stackEntry) {
	si := -1
	for i := int(s.sp) - 1; i >= 0; i-- {
		if s.stack[i].kind == seSpell && s.stack[i].obj == o {
			si = i
			break
		}
	}
	if si < 0 {
		return
	}
	sp := &s.stack[si]
	if k.t.Cards[sp.card].Types&kop.TCreature == 0 {
		return
	}
	s.eachTrigger(k, kop.TrigBecomesTarget, func(l trigListener, a *kop.Ability) {
		if a.Trig.Flags2&kop.Tf2SpellToo == 0 || l.ctrl != sp.ctrl || e.ctrl == l.ctrl || !s.trigOK(k, l, a) {
			return
		}
		n := s.npend
		s.queueTrig(k, l, a, o, sp.ctrl, 0)
		s.stampBtSA(e, n)
	})
}

// btSpellSrcOK is the source gate of a ValidSource$ Spell form. Gorge's bare
// Spell base (effects matchesBaseSpell) is any object on the stack, so an
// ability or a triggered ability targeting the permanent matches it too;
// Spell.YouCtrl compares the stack object's controller with the listener's,
// and a type word (Spell.Aura, Instant,Sorcery) reads the object's card face,
// which only a spell has.
func (s *State) btSpellSrcOK(k *prep, l trigListener, a *kop.Ability, e *stackEntry) bool {
	if a.Trig.Whose == kop.WhoseYou && e.ctrl != l.ctrl {
		return false
	}
	if a.Trig.Filter2 == 0 {
		return true
	}
	return e.kind == seSpell && s.matchCardAt(k, a.Trig.Filter2, e.card, e.ctrl, l.ctrl, e.obj)
}

// DebugSrcCtrlListener is the lockstep mutation of kop.TrigNSrcCtrl: the
// trigger player stays the listener's controller, so Thunderbreak Regent's
// "that player" names nobody the targeting spell's controller is
// (TestFourSeatThunderbreakLockstep must then diverge).
var DebugSrcCtrlListener bool
