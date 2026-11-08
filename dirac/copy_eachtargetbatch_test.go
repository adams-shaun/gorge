package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestCopyEachTargetCopiesEnterTogether: "for each of them, create a token that's
// a copy" (Dominion Infiltration) makes the copies of its several targets in
// one zone batch (gorge effCopyPermanent, CR 603.6a), so a Captain Kathryn
// Janeway copy's "another creature you control enters" also fires for the copy
// made after it: both copies trigger twice (their own entry and the other's),
// not once for the one made first.
func TestCopyEachTargetCopiesEnterTogether(t *testing.T) {
	set := cardsettest.Synth(t, "std-trk")
	const name = "Captain Kathryn Janeway"
	if i, ok := set.Lookup(name); !ok || !set.Supported()[i] {
		t.Skipf("%s not in the std-trk table", name)
	}
	deck := []cardset.Entry{{Name: name, Count: 2}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	src := sc.perm(id[name], 1)
	s.res = resCtx{e: stackEntry{kind: seSpell, ctrl: 0, obj: src, card: id[name], trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	if !s.anyETB(k) {
		t.Fatal("nothing listens for entries: the test sees nothing")
	}
	in := kop.Instr{Op: kop.OpCopyAsTokenEx, Obj: kop.RefSelf, A: kop.Const(1)}
	s.npend = 0
	// the each-target loop's frame (execEachTarget): one frame, one copy per target
	hr := s.holdEntries()
	s.g.inLoop = true
	s.copyTokens(k, &in, id[name], 1)
	if s.npend != 0 {
		t.Fatalf("the first copy's entry check fired at once (%d pending): it must wait for the second copy", s.npend)
	}
	s.copyTokens(k, &in, id[name], 1)
	s.g.inLoop = false
	s.holdClose(k, hr)
	if s.npend != 4 {
		t.Fatalf("pending triggers %d, want 4 (each copy: its own entry and the other's)", s.npend)
	}
}
