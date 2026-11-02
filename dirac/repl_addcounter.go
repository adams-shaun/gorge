package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// AddCounter replacements with no ValidCounterType$ (Loading
// Zone's doubler, Doc Samson's plus one). Their Repl.C is a wildcard
// (kop.ReplCtrAny / ReplCtrAnyYou) instead of a counter kind; replCtrOn
// (repl_counters.go) asks ctrKindMatch for each source.

// DebugReplShapes3Flip inverts the wildcard match (the mutation test): an
// any-kind replacement then applies to nothing and a kind-less putter gate
// to the wrong player.
var DebugReplShapes3Flip bool

// ctrKindMatch reports whether counter replacement a, whose source is
// controlled by ctrl, applies to counters of kind.
func (s *State) ctrKindMatch(a *kop.Ability, kind int, ctrl uint8) bool {
	c := a.Repl.C
	if c >= 0 {
		return int(c) == kind
	}
	return s.ctrWild(c, ctrl)
}

// ctrWild is ctrKindMatch for a wildcard. ReplCtrAnyYou reads the putter as
// gorge's replacement matcher does (ValidSource$ You: the in-flight counter
// adder, none known fails closed).
//
//go:noinline
func (s *State) ctrWild(c int16, ctrl uint8) bool {
	ok := c == kop.ReplCtrAny || c == kop.ReplCtrAnyYou && s.counterAdder() == ctrl
	return ok != DebugReplShapes3Flip
}

// ctrWildBits is the replBits a wildcard counter replacement sets: a
// putter-gated one needs the counter adder published at every site
// (replCountersBy tests rbDblCtrYou).
func ctrWildBits(a *kop.Ability) uint32 {
	if a.Repl.C == kop.ReplCtrAnyYou {
		return rbDblCtrYou
	}
	return 0
}

// replCostCtr is the number of counters of kind a COST puts on the permanent
// at sl (a planeswalker's [+N], a "put a counter on ~" cost, a counter-cost
// mana ability) when payer pays it. gorge closes only the EffectOnly$ class
// to a cost's placement (replacement_counter.go addCounterEffectOnlyClosed),
// so every other AddCounter replacement (Hardened Scales, Innkeeper's Talent,
// Loading Zone, Doc Samson) applies, the payer being the putter; once one has
// applied, the CR 614.16 second pass lets the EffectOnly$ doubler (Doubling
// Season) in too (Pir turns a loyalty cost's 1 into 2).
func (s *State) replCostCtr(k *prep, sl Slot, kind, n int, payer uint8) int {
	if k.gen == nil || n <= 0 || k.gen.anyRepl&(rbCtrOn|rbDblCtrYou) == 0 {
		return n
	}
	saved := s.g.ctrAdder
	s.g.ctrAdder = payer + 1
	m := s.replCtrOnX(k, sl, kind, n, true)
	s.g.ctrAdder = saved
	if m != n && k.gen.anyRepl&rbDblCtr != 0 {
		for c := s.replN(k, s.perms[sl].ctrl, rbDblCtr); c > 0 && m <= 0xffff; c-- {
			m *= 2
		}
	}
	return m
}
