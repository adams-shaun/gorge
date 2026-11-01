package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the kernel reads the replacement conditions the test adds
// (Delirium$ True, Revolt$ True, a ConditionZone$ presence count). The
// condition kinds already exist; the only new read is the filtered damage
// doubler's Repl.Cond, which dblDmgMatch used to ignore.
//
// DebugCondKeysIgnore is the mutation hook for TestCondkeysMutation: the
// three shapes read true, so the kernel applies a gate gorge knows is false.
// It is classified structurally because the table stores no provenance; the
// condkeys pool carries no other card with one of these shapes.
var DebugCondKeysIgnore bool

// condKeysIgnored reports whether c is one of the shapes
// lowers: a graveyard or exile presence count (ConditionZone$), Delirium$
// True (four or more card types in the controller's graveyard) or Revolt$
// True (a permanent left the battlefield under the controller's control
// this turn).
func (s *State) condKeysIgnored(k *prep, c *kop.Cond) bool {
	if c.Kind != kop.CondValue || c.Neg || c.Op != kop.CmpGE || c.A.K != kop.VExpr {
		return false
	}
	e := &k.t.Exprs[c.A.N]
	switch e.Kind {
	case kop.ExGraveTypes:
		return c.B == kop.Const(4)
	case kop.ExRevolt:
		return c.B == kop.Const(1)
	case kop.ExCountGraveyard, kop.ExCountExile:
		return true
	case kop.ExSum:
		for _, sub := range e.Sub {
			if sub.K != kop.VExpr {
				continue
			}
			switch k.t.Exprs[sub.N].Kind {
			case kop.ExCountGraveyard, kop.ExCountExile:
				return true
			}
		}
	}
	return false
}

// replCondHolds evaluates a replacement ability's Repl.Cond for the source
// permanent at sl, controlled by ctrl. It is the one hot-path read added by
// (dblDmgMatch); the other replacement carriers already read
// Repl.Cond in turn_gen.go, copy_asenters.go and chars_gen.go.
func (s *State) replCondHolds(k *prep, sl Slot, ctrl uint8, id kop.CondID) bool {
	return s.condEval(k, id, evalCtx{src: s.perms[sl].obj, srcSl: sl, ctrl: ctrl})
}
