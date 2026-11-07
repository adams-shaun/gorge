package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the Increment keyword (CR 702.xx, Secrets of
// Strixhaven; 601.2h, 603.2): "Whenever you cast a spell, if the amount of
// mana you spent is greater than this creature's power or toughness, put a
// +1/+1 counter on this creature." Gorge lowers the bare K:Increment into
// a SpellCast trigger carrying the Increment$ marker and reads it at fire
// time (trigmatch.incrementAdmits, rules/trigmatch/cast.go): the cast's
// pool mana spent (ManaSpentForCast, the log scan since the spell's own
// PutOnStack) against the SOURCE's current power/toughness, the LAYER
// OUTPUT, an OR of the two comparisons; a source with no face fails
// closed. The condition is an intervening "if" (CR 603.4), rechecked as the
// ability resolves against the source as it is then (rules'
// triggerResolvingCheckHolds, trigmatch.IncrementHolds): the source must
// still be a creature on the battlefield, and the captured spend must still
// exceed its power or toughness. The spend is event-relative, so gorge
// carries it on the trigger context (TriggerIncrementSpent); the kernel
// carries it on the trigger entry's trigAmt, captured when the trigger is
// queued and read by incrementResolves.
//
// The kernel's spend is the spell entry's recorded spend (recordSpent,
// copy_asenters.go), written at the payment (histCast), which runs before the
// cast triggers fire (onCast) -- the same number gorge's log scan derives:
// both count every pool mana the caster spent on the cast, X included,
// convoke picks excluded. A table with an Increment trigger turns on the
// spend record (genPrep.anySpent, set in gen.go beside prepOpus) but NOT
// anyCastSpent: an Increment permanent is not a spend reader, and gorge's
// planner does not decline casts for it (cast_pricing.go
// triggeredCastSpendReaderOut scans TriggeredCard$CastTotalManaSpent
// mentions only). castTrigGen (triggers_gen.go) gates on the flag; the
// deferred cast trigger fires after the payment, so the entry already
// holds the spend. The entry's trigAmt is otherwise the trigger amount, which
// Increment's fixed counter body never reads.

// DebugIncrementAnd is the mutation switch of the Increment lockstep
// (dirac/oracle/increment_test.go): require BOTH comparisons (&&)
// instead of gorge's OR; the lockstep must diverge.
var DebugIncrementAnd bool

// DebugIncrementLog records every Increment admission decision (the
// oracle probe's counts only).
var DebugIncrementLog bool

// IncrementRec is one Increment admission decision: the cast's spend and
// the source's power/toughness as it fired, the trigger's controller and
// the caster's, and whether it admitted.
type IncrementRec struct {
	Spent, Power, Tough int
	Ctrl, Caster        uint8
	Admitted            bool
}

// IncrementLog is the admission log DebugIncrementLog appends to.
var IncrementLog []IncrementRec

// incrementAdmits is castTrigGen's cold gate for Tf2Increment: the source is
// a creature on the battlefield and the cast's spend is greater than its
// power OR its toughness (gorge trigmatch.IncrementHolds; CR 702.xx). Gorge
// fails closed on a source with no face or one that is not a battlefield
// creature; a kernel battlefield listener is a card or a pool token, always
// faced, and a stack-local listener carries NoSlot.
//
//go:noinline
func (s *State) incrementAdmits(k *prep, l trigListener, e *stackEntry) bool {
	if l.sl == NoSlot || !s.isCreature(k, l.sl) {
		return false
	}
	spent := int(s.castSpent(e.obj))
	p, t := s.pt(k, l.sl)
	admit := incrementSpendExceeds(spent, p, t)
	if DebugIncrementLog {
		IncrementLog = append(IncrementLog, IncrementRec{Spent: spent, Power: p, Tough: t, Ctrl: l.ctrl, Caster: e.ctrl, Admitted: admit})
	}
	return admit
}

// incrementSpendExceeds is the ONE home of Increment's spend comparison: the
// spend is greater than the source's power OR its toughness, read at fire
// time and again as the trigger resolves. DebugIncrementAnd flips it to BOTH
// (the lockstep's mutation switch).
func incrementSpendExceeds(spent, p, t int) bool {
	if DebugIncrementAnd {
		return spent > p && spent > t
	}
	return spent > p || spent > t
}

// incrementResolves rechecks Increment's intervening "if" (CR 702.191a,
// 603.4) as the trigger resolves (trigResolveOK). The spell is gone, so the
// spend captured on the trigger entry (trigAmt, set by castTrigGen when the
// trigger was queued) is compared against the SOURCE's current power and
// toughness. The source must still be the same object and a creature on the
// battlefield: a counter or a pump from a trigger that resolved first moves
// the comparison, and a source that left or stopped being a creature fails
// closed. Gorge mirrors this in rules.triggerResolvingCheckHolds through
// trigmatch.IncrementHolds.
func (s *State) incrementResolves(k *prep, e *stackEntry) bool {
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer || !s.isCreature(k, ob.slot) {
		return false
	}
	p, t := s.pt(k, ob.slot)
	return incrementSpendExceeds(int(e.trigAmt), p, t)
}
