package oracle

import (
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// layerSrcRandomDecks is the sweep harness's deterministic two-colour deck
// generator: two colours, 23 spells (up to two copies each) and 17 basics.
func layerSrcRandomDecks(set cardset.Set, n int, seed uint64) [][]cardset.Entry {
	r := rand.New(rand.NewPCG(seed, 99))
	sup := set.Supported()
	basics := [5]string{"Plains", "Island", "Swamp", "Mountain", "Forest"}
	cols := [5]kop.ColorMask{kop.ColW, kop.ColU, kop.ColB, kop.ColR, kop.ColG}
	var spells []int
	for i := 0; i < set.NumCards() && i < len(sup); i++ {
		if !sup[i] || set.Inert(i) {
			continue
		}
		if set.Def(i).Types&kop.TLand != 0 {
			continue
		}
		spells = append(spells, i)
	}
	var out [][]cardset.Entry
	for len(out) < n {
		a, b := r.IntN(5), r.IntN(5)
		for b == a {
			b = r.IntN(5)
		}
		m := cols[a] | cols[b]
		var cand []int
		for _, i := range spells {
			if set.Def(i).Colors&^m == 0 {
				cand = append(cand, i)
			}
		}
		if len(cand) < 12 {
			continue
		}
		cnt := map[string]int{}
		for k := 0; k < 23; k++ {
			cnt[set.Names()[cand[r.IntN(len(cand))]]]++
		}
		keys := make([]string, 0, len(cnt))
		for k := range cnt {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var es []cardset.Entry
		for _, k := range keys {
			es = append(es, cardset.Entry{Name: k, Count: cnt[k]})
		}
		es = append(es, cardset.Entry{Name: basics[a], Count: 9}, cardset.Entry{Name: basics[b], Count: 8})
		out = append(out, es)
	}
	return out
}

// TestLayerSrcCollectionMatchesDirectPools runs the layer walk's table-
// precomputed source records and the old per-ability statM scan over real
// std-trc and std-dft games at the same states, and requires the collected
// effects to be identical (dirac.LayerSrcCheck; the edh pods run the same
// check in dirac).
func TestLayerSrcCollectionMatchesDirectPools(t *testing.T) {
	for _, pool := range []string{"std-trc", "std-dft"} {
		t.Run(pool, func(t *testing.T) {
			set := cardsettest.Synth(t, pool)
			decks := layerSrcRandomDecks(set, 64, 7)
			var st dirac.LayerSrcStats
			games := 0
			for c := 0; games < 4 && c < 4*len(decks)+64; c++ {
				pick := make([][]cardset.Entry, 2)
				for j := range pick {
					pick[j] = decks[(c*2+j*7+j)%len(decks)]
				}
				tbl, ids, err := set.MatchupN(pick)
				if err != nil {
					continue
				}
				var ds [dirac.MaxSeats][]dirac.CardID
				copy(ds[:], ids)
				got, err := dirac.LayerSrcCheck(tbl, ds, 2, uint64(games)+1, 400)
				dirac.ReleaseTable(tbl)
				if err != nil {
					t.Fatalf("%s game %d: %v", pool, games, err)
				}
				games++
				st.States += got.States
				st.Sources += got.Sources
				st.Slots += got.Slots
			}
			if games == 0 || st.States == 0 || st.Slots == 0 || st.Sources == 0 {
				t.Fatalf("%s: %d games, %+v; the equivalence check did not reach a walked source", pool, games, st)
			}
			t.Logf("%s: %d games, %+v", pool, games, st)
		})
	}
}
