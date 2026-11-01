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
	s.eachTrigger(k, kop.TrigBecomesTarget, func(l trigListener, a *kop.Ability) {
		if a.Trig.N == kop.TrigNValiant {
			return
		}
		if btSelfOff(a) {
			return // DebugBtSelfOff: the mutation must drop the self form
		}
		if a.Trig.Whose == kop.WhoseOpp && e.ctrl == l.ctrl {
			return
		}
		s.objTrigGen(k, kop.TrigBecomesTarget, l, a, sl, o, ctrl, 0)
	})
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
		s.queueTrig(k, l, a, o, sp.ctrl, 0)
	})
}
