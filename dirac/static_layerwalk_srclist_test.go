package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLayerSrcCollectionMatchesDirectEdh runs the layer walk's table-
// precomputed source records and the old per-ability statM scan over the
// committed four-seat edh pods at the same states, and requires the collected
// effects to be identical. The std-trc and std-dft pools, which only the
// oracle may read, run the same check (dirac/oracle/layersrc_pools_test.go).
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

// TestLayerSrcPrepMatchesLive recomputes every card's layer-source record
// from its abilities and requires prepLayerSrc to have stored exactly that.
// Dropping a flag, a rem/kept bit or an ability from the precomputation fails
// here, before a pool game could happen not to touch the card.
func TestLayerSrcPrepMatchesLive(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	defer ReleaseTable(tbl)
	k := prepare(tbl)
	g := k.gen
	if g == nil || g.fxNeed == 0 {
		t.Fatal("precondition: the FDN table does not walk")
	}
	for i := range tbl.Cards {
		cid := CardID(i)
		var fl uint8
		if g.has(cid, gfOvrStat) {
			fl |= lsrcOvr
		}
		if g.has(cid, gfSubGrant) {
			fl |= lsrcSub
		}
		if fl != 0 && g.has(cid, gfGlobOvr) {
			fl |= lsrcGlob
		}
		var abs, rem, kept uint64
		cd := &tbl.Cards[i]
		if len(cd.Abilities) > 64 {
			fl |= lsrcMany
		} else {
			for j := range cd.Abilities {
				a := &cd.Abilities[j]
				if a.Kind != kop.AbStatic {
					continue
				}
				switch a.Static.Kind {
				case kop.StatOverride:
					if a.Static.Override == 0 {
						continue
					}
				case kop.StatContinuous:
					if !a.Static.AddSub.Any() && !a.Static.AddChosen {
						continue
					}
				default:
					continue
				}
				bit := uint64(1) << uint(j)
				abs |= bit
				if isRemover(tbl, a) {
					rem |= bit
				}
				if keptOnLoss(tbl, a) {
					kept |= bit
				}
			}
		}
		want := layerSrcCard{abs: abs, rem: rem, kept: kept, flags: fl}
		if got := k.srcs[i]; got != want {
			t.Fatalf("%s: record %+v, want %+v", tbl.Cards[i].Name, got, want)
		}
	}
}

// TestLayerSrcRemovalDropsSource removes a static source through subscribe,
// the one place statM changes, and requires the walk on its host to stop
// collecting its override. A source set cached before the removal would keep
// it; the live statM scan cannot.
func TestLayerSrcRemovalDropsSource(t *testing.T) {
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

	hostSlot, auraSlot := sc.slot(host), sc.slot(aura)
	var fx [maxLfx]lfx
	if got := sc.s.lfxCollect(sc.k, hostSlot, &fx); got != 1 {
		t.Fatalf("before removal: %d effects on the host, want the Aura's 1", got)
	}
	if !sc.s.statM.has(auraSlot) {
		t.Fatal("the Aura is not a static source before removal")
	}
	if src, slots, err := layerSrcCompare(sc.s, sc.k); err != nil || src == 0 || slots == 0 {
		t.Fatalf("setup: %d sources over %d slots, err %v; want the attached Aura seen", src, slots, err)
	}
	pc := &sc.k.cards[sc.s.cardAt(auraSlot)]
	sc.s.subscribe(auraSlot, pc, false)
	if sc.s.statM.has(auraSlot) {
		t.Fatal("subscribe did not clear statM")
	}
	if got := sc.s.lfxCollect(sc.k, hostSlot, &fx); got != 0 {
		t.Fatalf("after removal: %d effects on the host, want 0", got)
	}
	if src, slots, err := layerSrcCompare(sc.s, sc.k); err != nil || src != 0 || slots == 0 {
		t.Fatalf("after removing the only static source: %d sources over %d slots, err %v, want 0", src, slots, err)
	}
}
