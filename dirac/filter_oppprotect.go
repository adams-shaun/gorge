package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwOppProtect: FwOppProtect, the battlefield battle's CR 310.10 protector is
// an opponent of the evaluating controller (gorge wordOppProtect,
// engine/effects/filter_word.go:934: the object's protector against
// SpecContext.You). The kernel records the protector in perm.chosen
// (face_battle.go); the evaluator is the reading source's controller
// (fctx.ctrl), so a battle the evaluator itself protects does not match. No
// recorded protector, a non-battle permanent or a card off the battlefield
// fails closed.
//
//kmanifest:fword FwOppProtect
func (s *State) fwOppProtect(c *fctx, _ *kop.FLit) bool {
	if !c.perm() || s.typesAt(c.k, c.sl)&kop.TBattle == 0 {
		return false
	}
	pr, ok := s.protector(c.sl)
	return ok && pr != c.ctrl && int(pr) < len(s.pl) && !s.pl[pr].lost
}
