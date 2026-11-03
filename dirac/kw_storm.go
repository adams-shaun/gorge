package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugIgnoreStorm disables the native storm cast trigger (the oracle's
// mutation test: the lockstep must diverge without it).
var DebugIgnoreStorm bool

// stormProg is the resolution of a storm trigger (CR 702.40a, gorge
// effects/copy.go effCopySpellAbility over the K:Storm expansion): copy the
// spell that was cast, once per spell cast before it this turn (the count the
// entry carries, fixed when the trigger fired), its controller choosing new
// targets for the copies. The count rides the entry, not the instruction
// (execCopySpell).
var stormProg = [1]kop.Instr{
	{Op: kop.OpCopySpell, Obj: kop.RefTriggerObj, C: 1},
}

// castFromStack queues the cast triggers the spell itself carries, in gorge's
// stack walk: its printed "when you cast this spell" triggers, then a granted
// storm.
func (s *State) castFromStack(k *prep, e *stackEntry) {
	if k.selfCast {
		s.fireSelfCast(k, e)
	}
	if k.gen != nil && k.gen.anyStorm && !DebugIgnoreStorm {
		s.queueStorm(k, e)
	}
}

// queueStorm queues the storm trigger of the spell e just cast when a live
// StatStackKw Storm grant of its caster reaches it. A spell has the keyword or
// not (gorge hasKeywordH): two granting statics give one trigger. The count is
// every player's spells cast this turn, the spell itself excluded; histCast
// has already folded this cast in.
func (s *State) queueStorm(k *prep, e *stackEntry) {
	g, _ := s.stackKwN(k, kop.StackKwStorm, e.card, e.obj, e.ctrl, s.castFromHandBy(e, e.ctrl))
	if g == 0 {
		return
	}
	n := int(s.g.hist[0][histSpells]) + int(s.g.hist[1][histSpells]) - 1
	s.enqueue(e.obj, s.objs[e.obj].ver, e.card, abStorm, e.ctrl, e.obj, e.ctrl, int16(max(n, 0)))
}
