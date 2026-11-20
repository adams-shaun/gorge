package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestLayerSrcCollectionMatchesDirectEdh runs the layer walk's cached source
// list and the old statM scan over the committed four-seat edh pods at the
// same states, and requires the collected effects to be identical. The
// std-trc and std-dft pools, which only the oracle may read, run the same
// check (dirac/oracle/layersrc_pools_test.go).
func TestLayerSrcCollectionMatchesDirectEdh(t *testing.T) {
	var st LayerSrcStats
	for i, p := range edhPods(t, 4) {
		got, err := LayerSrcCheck(p.tbl, p.decks, MaxSeats, uint64(i)+1, 400)
		if err != nil {
			t.Fatalf("edh pod %d: %v", i, err)
		}
		st.States += got.States
		st.Sources += got.Sources
		st.Slots += got.Slots
	}
	if st.States == 0 || st.Slots == 0 || st.Sources == 0 {
		t.Fatalf("edh: %+v; the equivalence check did not reach a walked source", st)
	}
	t.Logf("edh: %+v", st)
}

// layerSrcCompareCurrent is layerSrcCompare without rebuilding first: it
// observes the list as it stands, so a dropped invalidation shows up as a
// stale list.
func layerSrcCompareCurrent(t *testing.T, s *State, k *prep) (sources, slots int) {
	t.Helper()
	slots, err := layerSrcCompareSlots(s, k, "cached list")
	if err != nil {
		t.Fatalf("%v; the list was not invalidated", err)
	}
	if l := coldLayerSrc.of(s); len(l) != 0 {
		sources = int(l[0].n)
	}
	return sources, slots
}

// TestLayerSrcInvalidationDropsRemovedSource removes a static source through
// subscribe, the one place statM changes, and requires the next collection to
// see the list without it. Dropping the invalidation in subscribe leaves the
// stale Aura in the list and this test fails.
func TestLayerSrcInvalidationDropsRemovedSource(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	deck := []cardset.Entry{
		{Name: "Witness Protection", Count: 1},
		{Name: "Ambush Wolf", Count: 1},
		{Name: "Plains", Count: 20},
	}
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	host := sc.perm(id["Ambush Wolf"], 0)
	aura := sc.perm(id["Witness Protection"], 0)
	sc.s.attachTo(sc.k, sc.slot(aura), sc.slot(host))
	sc.s.refreshTy(sc.k)

	src, slots, err := layerSrcCompare(sc.s, sc.k)
	if err != nil {
		t.Fatal(err)
	}
	if src == 0 || slots == 0 {
		t.Fatalf("setup has %d sources over %d slots; want the attached Aura in the list", src, slots)
	}
	auraSlot := sc.slot(aura)
	if !sc.s.statM.has(auraSlot) {
		t.Fatal("the Aura is not a static source before removal")
	}
	pc := &sc.k.cards[sc.s.cardAt(auraSlot)]
	sc.s.subscribe(auraSlot, pc, false)
	if sc.s.statM.has(auraSlot) {
		t.Fatal("subscribe did not clear statM")
	}
	if src, slots := layerSrcCompareCurrent(t, sc.s, sc.k); src != 0 || slots == 0 {
		t.Fatalf("after removing the only static source: list has %d sources over %d slots, want 0", src, slots)
	}
	// a rebuild leaves no stale entry past n: State.SameGame compares the
	// cold bytes, so equal games must hold equal lists.
	l := sc.s.layerSrcBuild(sc.k)
	if l == nil || l.n != 0 {
		t.Fatalf("rebuilt list = %+v, want empty", l)
	}
	if l.srcs[0] != (layerSrc{}) {
		t.Fatalf("the removed source's entry survived the rebuild: %+v", l.srcs[0])
	}
}
