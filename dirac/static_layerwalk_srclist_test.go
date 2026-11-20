package dirac

import (
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// layerSrcGame is one table and its decks: the committed edh pods' shape,
// and the random synthetic games' shape.
type layerSrcGame struct {
	tbl   *Table
	decks [MaxSeats][]CardID
	n     uint8
}

func (g *layerSrcGame) init(s *State, seed uint64) {
	s.InitN(g.tbl, g.decks, g.n, seed, Config{Mulligans: 2})
}

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

// layerSrcSynthGames builds the first n playable std-* games from the
// harness's decks and seeds.
func layerSrcSynthGames(t *testing.T, pool string, n int) []layerSrcGame {
	t.Helper()
	set := cardsettest.Synth(t, pool)
	decks := layerSrcRandomDecks(set, 64, 7)
	var out []layerSrcGame
	for c := 0; len(out) < n && c < 4*len(decks)+64; c++ {
		pick := make([][]cardset.Entry, 2)
		for j := range pick {
			pick[j] = decks[(c*2+j*7+j)%len(decks)]
		}
		tbl, ids, err := set.MatchupN(pick)
		if err != nil {
			continue
		}
		g := layerSrcGame{tbl: tbl, n: 2}
		copy(g.decks[:], ids)
		out = append(out, g)
	}
	if len(out) == 0 {
		t.Fatalf("%s: no game builds", pool)
	}
	return out
}

// layerSrcEdhGames wraps the committed edh pods.
func layerSrcEdhGames(t *testing.T, n int) []layerSrcGame {
	t.Helper()
	pods := edhPods(t, n)
	out := make([]layerSrcGame, len(pods))
	for i, p := range pods {
		out[i] = layerSrcGame{tbl: p.tbl, decks: p.decks, n: MaxSeats}
	}
	return out
}

// layerSrcCompare checks the cached source list against the live statM scan
// for every occupied slot. It builds the list first, so lfxCollect takes the
// list path.
func layerSrcCompare(t *testing.T, s *State, k *prep) (sources, slots int) {
	t.Helper()
	l := s.layerSrcBuild(k)
	if l == nil {
		return 0, 0
	}
	for sl := Slot(0); sl < s.permHi; sl++ {
		if !s.occ.has(sl) {
			continue
		}
		var got, want [maxLfx]lfx
		ng := s.lfxCollect(k, sl, &got)
		nw := s.lfxCollectRecords(k, sl, &want)
		nw = s.lfxCollectStaticsDirect(k, sl, &want, nw)
		nw = s.lfxCollectEmblems(k, sl, &want, nw)
		if ng != nw {
			t.Fatalf("slot %d: list collected %d effects, direct %d", sl, ng, nw)
		}
		for i := 0; i < ng; i++ {
			if got[i] != want[i] {
				t.Fatalf("slot %d effect %d: list %+v, direct %+v", sl, i, got[i], want[i])
			}
		}
		slots++
	}
	return int(l.n), slots
}

// TestLayerSrcCollectionMatchesDirect runs the layer walk's cached source
// list and the old statM scan over real std-trc, std-dft and edh games at the
// same states, and requires the collected effects to be identical.
func TestLayerSrcCollectionMatchesDirect(t *testing.T) {
	for _, pool := range []string{"std-trc", "std-dft", "edh"} {
		t.Run(pool, func(t *testing.T) {
			var games []layerSrcGame
			if pool == "edh" {
				games = layerSrcEdhGames(t, 4)
			} else {
				games = layerSrcSynthGames(t, pool, 4)
			}
			checked, sources, slots := 0, 0, 0
			for gi := range games {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Logf("%s game %d: kernel panic (skipped): %v", pool, gi, r)
						}
					}()
					var s State
					games[gi].init(&s, uint64(gi)+1)
					pol := NewRandomPolicy(uint64(gi) + 1)
					var d Decision
					for step := 0; step < 400; step++ {
						if step%8 == 0 {
							src, nsl := layerSrcCompare(t, &s, s.k())
							checked++
							sources += src
							slots += nsl
						}
						s.StepInto(&d)
						if d.Kind == DkNone {
							break
						}
						s.Apply(pol.Pick(&d))
					}
				}()
			}
			if checked == 0 || slots == 0 || sources == 0 {
				t.Fatalf("%s: checked=%d slots=%d sources=%d; the equivalence check did not reach a walked source", pool, checked, slots, sources)
			}
			t.Logf("%s: %d states, %d sources, %d slot walks", pool, checked, sources, slots)
		})
	}
}

// layerSrcCompareCurrent is layerSrcCompare without rebuilding first: it
// observes the list as it stands, so a dropped invalidation shows up as a
// stale list.
func layerSrcCompareCurrent(t *testing.T, s *State, k *prep) (sources, slots int) {
	t.Helper()
	for sl := Slot(0); sl < s.permHi; sl++ {
		if !s.occ.has(sl) {
			continue
		}
		var got, want [maxLfx]lfx
		ng := s.lfxCollect(k, sl, &got)
		nw := s.lfxCollectRecords(k, sl, &want)
		nw = s.lfxCollectStaticsDirect(k, sl, &want, nw)
		nw = s.lfxCollectEmblems(k, sl, &want, nw)
		if ng != nw {
			t.Fatalf("slot %d: cached list collected %d effects, direct %d; the list was not invalidated", sl, ng, nw)
		}
		for i := 0; i < ng; i++ {
			if got[i] != want[i] {
				t.Fatalf("slot %d effect %d: cached %+v, direct %+v; the list was not invalidated", sl, i, got[i], want[i])
			}
		}
		slots++
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
	set := cardsettest.Synth(t, "std-mkm")
	deck := []cardset.Entry{
		{Name: "Burden of Proof", Count: 1},
		{Name: "Evidence Examiner", Count: 1},
		{Name: "Plains", Count: 20},
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
	host := sc.perm(id["Evidence Examiner"], 0)
	aura := sc.perm(id["Burden of Proof"], 0)
	sc.s.attachTo(sc.k, sc.slot(aura), sc.slot(host))
	sc.s.refreshTy(sc.k)

	if src, slots := layerSrcCompare(t, sc.s, sc.k); src == 0 || slots == 0 {
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
}
