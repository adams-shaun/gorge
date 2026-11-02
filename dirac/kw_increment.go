package dirac

// the Increment keyword (CR 702.xx, Secrets of
// Strixhaven; 601.2h, 603.2): "Whenever you cast a spell, if the amount of
// mana you spent is greater than this creature's power or toughness, put a
// +1/+1 counter on this creature." Gorge lowers the bare K:Increment into
// a SpellCast trigger carrying the Increment$ marker and reads it at fire
// time (trigmatch.incrementAdmits, rules/trigmatch/cast.go): the cast's
// pool mana spent (ManaSpentForCast, the log scan since the spell's own
// PutOnStack) against the SOURCE's current power/toughness, the LAYER
// OUTPUT, an OR of the two comparisons; a source with no face fails
// closed. No resolution-time recheck exists in gorge (no Condition$ on the
// body), so the kernel adds none (CR 603.4's intervening-if is not
// copied).
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
// holds the spend.

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

// incrementAdmits is castTrigGen's cold gate for Tf2Increment: the cast's
// spend is greater than the source's power OR its toughness (gorge
// trigmatch.incrementAdmits; CR 702.xx). Gorge fails closed on a source
// with no face; a kernel battlefield listener is a card or a pool token,
// always faced, and a stack-local listener carries NoSlot.
//
//go:noinline
func (s *State) incrementAdmits(k *prep, l trigListener, e *stackEntry) bool {
	if l.sl == NoSlot {
		return false
	}
	spent := int(s.castSpent(e.obj))
	p, t := s.pt(k, l.sl)
	admit := spent > p || spent > t
	if DebugIncrementAnd {
		admit = spent > p && spent > t
	}
	if DebugIncrementLog {
		IncrementLog = append(IncrementLog, IncrementRec{Spent: spent, Power: p, Tough: t, Ctrl: l.ctrl, Caster: e.ctrl, Admitted: admit})
	}
	return admit
}
