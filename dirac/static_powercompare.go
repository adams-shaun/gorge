package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// pwcmp: a CantBlockBy static whose ValidBlocker$ compares the blocker's
// power with the static's host ("creatures with power less than this
// creature's power can't block it": Formation Breaker, Prehistoric Pet;
// ). gorge matches the clause through MatchesStaticSpec, which
// resolves the numeric right-hand side X = Count$CardPower against the
// static's host, so X is the host's CURRENT power (CR 208.3, the layered
// power). kop.Static.PwVsSrc is 1 for powerLTX (blocker < host) and 2 for
// powerGTX (blocker > host); blockForbiddenGen consults it behind
// prep.anyPwVsSrc.

// DebugPwVsSrcLE makes pwVsSrc read case 1 as <= (the pwcmp mutation
// test's proof that the pool puts a blocker level with the host's power).
var DebugPwVsSrcLE bool

// cardPwVsSrc reports whether some CantBlockBy static of c carries PwVsSrc.
func cardPwVsSrc(c *kop.CardDef) bool {
	for i := range c.Abilities {
		a := &c.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.PwVsSrc != 0 {
			return true
		}
	}
	return false
}

// pwVsSrc reports whether blocker b's power stands in the relation mode
// names to the host's (slot host); mode 0 is no compare.
//
//go:noinline
func (s *State) pwVsSrc(k *prep, b, host Slot, mode int8) bool {
	if mode == 0 {
		return true
	}
	bp, _ := s.ptGen(k, b)
	hp, _ := s.ptGen(k, host)
	if mode == 1 {
		if DebugPwVsSrcLE {
			return bp <= hp
		}
		return bp < hp
	}
	return bp > hp
}
