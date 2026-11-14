//go:build dirac_mp

package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/search"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// TestFourSeatSearchKeepsTheActivationBudget: four max^n search seats play
// four-seat 100-card cmd5 pods whose decks each carry Lightning Greaves
// (Equip {0}) in place of eight basics. The search used to move a free
// Equipment between two creatures while its visits edged out pass's, filling
// the kernel's per-turn activation census (63 under dirac_mp: StallActs) in
// 7% of four-seat MBC pods. With the search's per-source budget
// (search.ActBudget, the bot's own) no object passes the budget in a turn,
// the census never fills, and the Greaves are still equipped. Without the
// budget pod 3 stalls StallActs at turn 27 (one object 63 times in a turn).
func TestFourSeatSearchKeepsTheActivationBudget(t *testing.T) {
	base, _ := testCorpus(t)
	dir, remove, err := ExtractSlice("cmd5")
	if err != nil {
		t.Fatal(err)
	}
	defer remove()
	pool, err := ReadPool(filepath.Join(dir, "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	c := &Corpus{Reg: base.Reg, DecksDir: filepath.Join(dir, "decks"), decks: map[string][]*cards.Card{}}
	var names []string
	for _, p := range pool {
		if p.Name != "Reign of Dragons" { // StallTwoSeat at four seats (kop.CardPodLimited)
			names = append(names, p.Name)
		}
	}
	full, err := MiniCommanderDecks(c, names, 100)
	if err != nil {
		t.Fatal(err)
	}
	// the two cards a four-seat pod refuses (kop.CardPodLimited) play as basics
	sub := map[string]string{"Thunderbreak Regent": "Mountain", "Grasp of Fate": "Plains"}
	for k, f := range full {
		for i := range f.Cards {
			if to, ok := sub[f.Cards[i].Name]; ok {
				f.Cards[i].Name = to
			}
		}
		f.Cards = greavesForBasics(t, k, f.Cards, 8)
		full[k] = f
	}
	c.AddDecks(full)
	set, err := cardset.ByName("cmd5")
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	pp := make([]PoolDeck, len(names))
	for i, n := range names {
		pp[i] = PoolDeck{Name: n}
	}
	equips, peak := 0, 0
	for pod := 0; pod < 4; pod++ {
		p := PodAt(pp, pod, 1, 4)
		m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
		if err != nil {
			t.Fatalf("pod %d: %v", pod, err)
		}
		for s := 0; s < 4; s++ {
			if n := len(m.SeatDeck(s)); n != 100 {
				t.Fatalf("pod %d seat %d: %d cards, want 100", pod, s, n)
			}
		}
		side, err := RealKernel(tbl, nil, Config{PassElision: true, GorgeAutopaySurface: true})(m, p.Seed)
		if err != nil {
			t.Fatalf("pod %d: %v", pod, err)
		}
		ks, kd, _ := KernelStateOf(side)
		f := HBotFacts(c.Reg, tbl, nil)
		var srs [4]*search.Searcher
		for s := range srs {
			o := search.DefaultOptions()
			o.Sims, o.MaxN = 16, true
			srs[s] = search.NewSearcher(o, search.HeuristicLeaf{}, hbot.New(f))
		}
		for dec := 0; dec < 4000; dec++ {
			ks.StepInto(kd)
			if kd.Kind == dirac.DkNone {
				break
			}
			r := srs[kd.Player].Search(ks, p.Seed*131+uint64(dec))
			for _, a := range r.Acts[:r.NActs] {
				if a.Kind() == dirac.AkActivate && tbl.Cards[ks.ObjCard(dirac.Obj(a.Source()))].Name == "Lightning Greaves" {
					equips++
				}
			}
			r.Play(ks)
			for o := 0; o < ks.NumObjs(); o++ {
				peak = max(peak, ks.FeatActivations(dirac.Obj(o)))
			}
		}
		if ks.Stalled() {
			t.Fatalf("pod %d stalled %s at turn %d", pod, dirac.StallName(ks.StallReason()), ks.Turn())
		}
	}
	t.Logf("Greaves equips %d, most activations of one object in a turn %d", equips, peak)
	if peak > search.ActBudget {
		t.Errorf("an object reached %d activations in a turn, past the search's budget of %d", peak, search.ActBudget)
	}
	if equips == 0 {
		t.Error("no search seat equipped Lightning Greaves: the pods did not reach the budget's case")
	}
}

// greavesForBasics trades n basic lands of a deck for n Lightning Greaves.
func greavesForBasics(t *testing.T, name string, es []deck.Entry, n int) []deck.Entry {
	t.Helper()
	for i := range es {
		if cardset.IsSynthBasic(es[i].Name) && es[i].Count > n {
			es[i].Count -= n
			return append(es, deck.Entry{Name: "Lightning Greaves", Count: n})
		}
	}
	t.Fatalf("%s: no basic land with more than %d copies", name, n)
	return nil
}
