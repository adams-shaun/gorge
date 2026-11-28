package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// execCounter is exec's OpCounter arm. It counters the named stack entry:
// a stack-ability target slot (tgt_stackability.go), a BecomesTarget
// TriggeredSourceSA ability by its identity, or an object ref's stack entry.
// It returns false when the caller must return false (a CR 616.1 order ask
// parked the counter, repl_uncounterable.go); true continues the program.
func (s *State) execCounter(k *prep, in *kop.Instr) bool {
	if k.gen != nil && k.gen.anyStackTgt && s.counterStackTgt(k, in) {
		return true // a stack-ability target (tgt_stackability.go)
	}
	// A BecomesTarget counter names the targeting ability by its stack
	// identity when it has one: the ability's source permanent can also be
	// the source of this very trigger (Kira, Great Glass-Spinner's counter
	// sits on the creature the ability targeted), so the object alone would
	// find the trigger instead (trig_becomestarget.go).
	i := -1
	if in.Obj == kop.RefTriggerSource && s.res.e.trigSASid != 0 {
		// The recorded ability identity is authoritative: once it has left
		// the stack the counter finds nothing (gorge's vanished TriggerStack
		// names no object), never a sibling on the source.
		i = s.stackSidIndex(s.res.e.trigSASid)
	} else if o := s.objRefObj(in.Obj); o != NoObj {
		i = s.stackIndexOf(o)
		// A SpellCast trigger's "counter that spell" names a spell: once it
		// has left the stack, the entry sharing its object is an ability (a
		// self-cast trigger, maybe this resolving one), never the target.
		if in.Obj == kop.RefTriggerObj && i >= 0 && s.stack[i].kind != seSpell {
			i = -1
		}
	}
	if i < 0 {
		return true
	}
	unc, park := false, false
	if s.stack[i].kind == seSpell && in.C&kop.CtrDestMove == 0 {
		// gorge effCounter's counterStopped: a spell two or more
		// uncounter replacements protect asks CR 616.1's order
		// (repl_uncounterable.go) before it is skipped
		if unc, park = s.uncGate(k, i); park {
			return false
		}
	}
	if unc {
		return true
	}
	s.g.stkCt = s.stack[i].ctrl + 1 // "its controller" reads the spell as it last existed (CR 608.2h)
	if in.B.N != 0 {
		s.counterRiders(k, i, int(in.B.N)) // tgt_countershapes.go
	}
	if in.C != 0 {
		s.counterTo(k, i, in.C&^kop.CtrDestMove)
		return true
	}
	s.counterAt(k, i)
	return true
}
