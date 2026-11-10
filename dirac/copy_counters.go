package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExtCopyTokenCtr: CopyPermanent WithCountersType$/WithCountersAmount$
// (gorge effects/copypermanent.go folds the counters into the copy's own
// entry MoveZone through entryWithCounters). The counters are part of each
// token's entry (CR 122.6, 614.1c): s.g.enterCtr is set immediately before
// the copy enters, so enterGen puts them through the counter replacements
// with the entry, not as a later put. Counters are not copiable values
// (CR 707.2): the source's own are never read here.

// copyTokenCtr is ExtCopyTokenCtr. The plain path only (a creature or
// planeswalker with no as-enters choice): an Aura, an as-enters choice or a
// clone election would have to ask while the rider waits, so those stall
// rather than drop the counters.
//
//go:noinline
func (s *State) copyTokenCtr(k *prep, in *kop.Instr) {
	e := &s.res.e
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return
	}
	c := s.objs[o].card
	if s.objs[o].zone == kop.ZoneBattlefield {
		c = s.cardAt(s.objs[o].slot)
	}
	amt := s.value(k, in.B) // once, before any token (gorge resolves it once per call)
	kind := kop.CounterKind(in.C)
	// a clean copy instruction: the copy path reads B as CopyFlags and C as
	// the copy-exception override, neither of which this op carries
	cin := kop.Instr{Op: kop.OpCopyAsTokenEx, Obj: in.Obj, A: kop.Const(1)}
	tc, n, rewritten, ok := s.copyTokenRewrite(k, e.ctrl, c, 1, false)
	if !ok {
		return
	}
	if rewritten {
		// a CreateToken rewriter replaced the copy with a plain mint of its
		// card: no copy is left for the counters (repl_tokrewrite.go)
		s.mintRewritten(k, e.ctrl, tc, n)
		return
	}
	if n > 0 && (k.cards[c].types&kop.TAura != 0 && k.t.Cards[c].Enchant.Kind == kop.TgtObject || s.enterChoiceKind(k, c) != 0 || k.gen != nil && k.gen.anyBecomeCopy && s.cloneTokenAsk(k, &cin, c) != 0) {
		s.stall(StallCopyAllAsk)
		return
	}
	for i := 0; i < n; i++ {
		if s.nearCapacity(k) {
			s.stall(StallBoard)
			return
		}
		t := s.newToken(c, e.ctrl)
		s.objs[t].of |= ofCopy
		if amt > 0 {
			s.g.enterCtr.n, s.g.enterCtr.kind = uint8(min(amt, 255)), kind
		}
		s.copyTokenEnter(k, &cin, t, NoSlot, nil)
		s.g.enterCtr.n = 0
	}
}
