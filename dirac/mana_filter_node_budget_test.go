package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// gorbeFreeBranchNodes is the node count of gorge's plain prefix search
// (engine/rules scriptSearch, potentialPrefixScript) on the board below for
// the root free-{C} branch -- the Unknown Shores activation's first branch,
// the only one tried before the paid plan -- measured with a per-node trace
// of scriptSearch.visit: the whole plain search is 79 nodes, this branch 70.
const gorbeFreeBranchNodes = 70

// FilterBranchNodes is the node count of the plain filter search's root
// free-{C} branch on filtNodeBudgetBoard over tbl (a pool table holding the
// board's cards: the dirac_test half of the test lowers them with kgen from
// inline names, because a package dirac test may not read a raw pool), after
// the preconditions that make the count a measurement of the board. It is
// exported for that external test file only.
func FilterBranchNodes(t *testing.T, tbl *kop.Table) int {
	t.Helper()
	id := map[string]CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = CardID(c)
	}
	sc, a, shores := filtNodeBudgetBoard(t, tbl, id)
	return filtBranchNodes(t, sc, a, shores)
}

// FilterBranchNodesGorge is gorge's count for the same branch, pinned.
const FilterBranchNodesGorge = gorbeFreeBranchNodes

// filtNodeBudgetBoard is the tail-34 pair 25 offer (seed 255781): Vault of the
// Archangel's {2}{W}{B} activation beside Unknown Shores, two Swamps, two
// Mountains and Manabarbs. Every land tap queues a Taps trigger, so the four
// basics are Taps-deferred census gaps beside the paid filter.
func filtNodeBudgetBoard(t *testing.T, tbl *kop.Table, id map[string]CardID) (*scen, Action, Slot) {
	t.Helper()
	names := []string{"Unknown Shores", "Vault of the Archangel", "Swamp", "Swamp", "Mountain", "Mountain", "Manabarbs"}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	var shores, vault Obj
	for _, n := range names {
		switch o := sc.perm(id[n], 0); n {
		case "Unknown Shores":
			shores = o
		case "Vault of the Archangel":
			vault = o
		}
	}
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == vault && a.Mode() == 1 {
			return sc, a, sc.slot(shores)
		}
	}
	t.Fatal("precondition: Vault of the Archangel's {2}{W}{B} activation is not offered")
	return nil, 0, 0
}

// filtBranchNodes walks the branch and returns its node count. The dirac_test
// TestFilterSearchNodeBudgetParity pins the plain filter search's node count
// on the root free-{C} branch of that board at or below gorge's. The four
// Taps-deferred basics are the census gaps beside Unknown Shores, and gorge's
// activation loop skips a land that is a twin of one it already walked
// (scriptTwin); the kernel's other-gap branch returned before its twin cut,
// walked both Mountains and both Swamps in every order and spent 151-160
// nodes on a branch gorge settles in 70, which left the offer on the
// potentialScriptNodes boundary. The branch is walked directly (the root's
// free-{C} activation, freeBranchHopeless's prune bypassed).
func filtBranchNodes(t *testing.T, sc *scen, a Action, shores Slot) int {
	t.Helper()
	s, k := sc.s, sc.k
	p := uint8(0)
	var fs filtSearch
	fs.a, fs.p, fs.gaps, fs.own, fs.sp0 = a, p, s.scriptFilt(k, p), NoSlot, s.sp
	fs.need = s.playNeed(k, a)
	fs.useful, fs.usefulOK = s.filtUsefulCost(k, a)
	fs.other = s.otherGaps(k, p, a)
	fs.feeOnly = s.filtRepeatIn(k, fs.gaps)
	_, fs.own, _ = s.activationMana(k, Obj(a.Source()), a.Mode())
	fs.fees = s.filtFunding(k, p, fs.gaps)
	fs.fundN = s.filtFunders(k, p, fs.gaps.or(fs.other))
	if !fs.gaps.has(shores) || !isAbilityPlay(a) {
		t.Fatalf("precondition: Unknown Shores must be a filter gap of a printed activation (gaps %v)", fs.gaps)
	}
	if deferred := s.scriptTapsDefer(k, p); deferred.count() < 4 || fs.other != deferred {
		t.Fatalf("precondition: the four basics must be among the Taps-deferred other gaps (deferred %v, other %v)", deferred, fs.other)
	}
	tt := s.fork()
	defer s.unfork()
	tt.g.noGrant = true
	tt.g.fbProbe = s.fbProbeRoot(k, p, a)
	s.pool.inFilt++
	found := fs.try(k, tt, filtStep{sl: shores, gap: true}, 0, true)
	s.pool.inFilt--
	if found {
		t.Fatal("precondition: the free {C} branch spends the only white source, so it finds no plan")
	}
	if fs.nodes < 2 {
		t.Fatalf("precondition: the branch was not walked (%d nodes)", fs.nodes)
	}
	return fs.nodes
}
