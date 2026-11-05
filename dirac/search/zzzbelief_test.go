package search

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// testBeliefNet is a belief network whose outputs are its biases alone
// (every weight 0, the trunk's one unit 1): hand logits bh, and with cards
// the configuration logits bp and card logits bd.
func testBeliefNet(t testing.TB, cards []string, bh []float64, bp, bd []float64, offcolour float64) *BeliefNet {
	return testBeliefNetH(t, 1, cards, bh, bp, bd, offcolour)
}

// testBeliefNetH is testBeliefNet with H hidden units (the first 1, the
// rest 0).
func testBeliefNetH(t testing.TB, H int, cards []string, bh []float64, bp, bd []float64, offcolour float64) *BeliefNet {
	t.Helper()
	zero := func(rows, cols int) [][]float64 {
		m := make([][]float64, rows)
		for i := range m {
			m[i] = make([]float64, cols)
		}
		return m
	}
	j := map[string]any{
		"version": 3, "features": BeliefFeatures, "cards": cards, "offcolour": offcolour,
		"mean": make([]float64, BeliefFeatures), "std": slices.Repeat([]float64{1}, BeliefFeatures),
		"w1": zero(H, BeliefFeatures+len(cards)), "b1": append([]float64{1}, make([]float64, H-1)...),
		"wh": zero(3, H), "bh": bh,
	}
	if bp != nil {
		j["wp"], j["bp"], j["wd"], j["bd"] = zero(BeliefConfigs, H), bp, zero(len(cards), H), bd
	}
	raw, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	n, err := ParseBeliefNet(raw)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// beliefGame is a vanilla game whose seat 1 plays a mono-red list of
// Mountains, Lightning Bolts and Hill Giants.
func beliefGame(seed uint64) *dirac.State {
	var b []dirac.CardID
	for i := 0; i < 40; i++ {
		b = append(b, []dirac.CardID{vanilla.Mountain, vanilla.LightningBolt, vanilla.HillGiant}[i%3])
	}
	s := new(dirac.State)
	s.Init(vanilla.Table, vanilla.Decks[0], b, seed, dirac.Config{})
	return s
}

func hiddenOf(s *dirac.State, p uint8) []dirac.CardID {
	var buf [2 * dirac.MaxDeck]dirac.CardID
	return slices.Clone(buf[:s.HiddenCards(p, buf[:])])
}

func sortedOf(l []dirac.CardID) []dirac.CardID {
	l = slices.Clone(l)
	slices.Sort(l)
	return l
}

// TestHandBeliefFollowsComposition: :handbelief deals the opponent's hand
// by the hand head (all lands, then all instant-speed cards) from the
// corpus list's pool (here the opponent's own list, so the hidden multiset
// is kept); DealUnseen (pregame) and a search without ClosedList ignore
// it; and SetBeliefs(nil, nil) restores the plain deal exactly.
func TestHandBeliefFollowsComposition(t *testing.T) {
	s := beliefGame(7)
	opp := uint8(1)
	h := s.HandSize(opp)
	want := sortedOf(hiddenOf(s, opp))
	corpus := NewBelief(vanilla.Table, [][]dirac.CardID{want})
	for _, tc := range []struct {
		bh   []float64
		card dirac.CardID
	}{{[]float64{10, -10, -10}, vanilla.Mountain}, {[]float64{-10, 10, -10}, vanilla.LightningBolt}} {
		sr := NewSearcher(DefaultOptions(), nil, nil)
		sr.Belief = corpus
		sr.SetBeliefs(nil, testBeliefNet(t, nil, tc.bh, nil, nil, 0).Bind(vanilla.Table))
		sr.seat = 0
		sr.beliefRoot(s, 0)
		for i := 0; i < 20; i++ {
			var c dirac.State
			c.CopyFrom(s)
			a, b := redealSeed(uint64(i), i)
			sr.redeal(&c, a, b)
			got := hiddenOf(&c, opp)
			for _, x := range got[:h] {
				if x != tc.card {
					t.Fatalf("world %d: hand %v, want only card %d", i, got[:h], tc.card)
				}
			}
			if !slices.Equal(sortedOf(got), want) {
				t.Fatalf("world %d: the hidden multiset changed", i)
			}
		}
		plain := NewSearcher(DefaultOptions(), nil, nil)
		plain.Belief = corpus
		var c1, c2 dirac.State
		c1.CopyFrom(s)
		c2.CopyFrom(s)
		sr.DealUnseen(&c1, 0, 3, 4)
		plain.DealUnseen(&c2, 0, 3, 4)
		if !slices.Equal(hiddenOf(&c1, opp), hiddenOf(&c2, opp)) {
			t.Fatal("DealUnseen used the hand belief")
		}
		sr.Opts.ClosedList = false
		sr.beliefRoot(s, 0)
		if sr.bel.handOK || sr.bel.deckOK {
			t.Fatal("a belief predicted without ClosedList")
		}
	}
	// off: identical to a searcher that never had a belief
	plain := NewSearcher(DefaultOptions(), nil, nil)
	off := NewSearcher(DefaultOptions(), nil, nil)
	plain.Belief, off.Belief = corpus, corpus
	off.SetBeliefs(nil, testBeliefNet(t, nil, []float64{10, -10, -10}, nil, nil, 0).Bind(vanilla.Table))
	off.SetBeliefs(nil, nil)
	plain.seat, off.seat = 0, 0
	for i := 0; i < 5; i++ {
		var c1, c2 dirac.State
		c1.CopyFrom(s)
		c2.CopyFrom(s)
		a, b := redealSeed(uint64(i), i)
		plain.redeal(&c1, a, b)
		off.redeal(&c2, a, b)
		if !slices.Equal(hiddenOf(&c1, opp), hiddenOf(&c2, opp)) || !slices.Equal(hiddenOf(&c1, 0), hiddenOf(&c2, 0)) {
			t.Fatal("SetBeliefs(nil, nil) changed the deal")
		}
	}
}

// TestDeckBeliefDealsFromHeads: :deckbelief draws the hidden pool from the
// card head within the drawn configuration (mono-red): never a card the
// table lacks, never an off-colour card at offcolour 0, at most 3 copies
// of a non-basic (counting the seen ones); the hand by the hand head. At
// offcolour 1 the heavier off-colour card is dealt.
func TestDeckBeliefDealsFromHeads(t *testing.T) {
	name := func(c dirac.CardID) string { return vanilla.Table.Cards[c].Name }
	cards := []string{name(vanilla.Mountain), name(vanilla.LightningBolt), name(vanilla.HillGiant), name(vanilla.Island), "Not A Card"}
	bp := make([]float64, BeliefConfigs)
	for k, m := range BeliefConfigMask {
		bp[k] = -10
		if m == 1<<3 { // R
			bp[k] = 10
		}
	}
	bd := []float64{0, 2, -20, 3, 50}
	s := beliefGame(9)
	opp := uint8(1)
	h, n := s.HandSize(opp), s.HandSize(opp)+s.LibrarySize(opp)
	for _, off := range []float64{0, 1} {
		net := testBeliefNet(t, cards, []float64{10, -10, -10}, bp, bd, off)
		if !net.HasDeck() {
			t.Fatal("HasDeck false")
		}
		sr := NewSearcher(DefaultOptions(), nil, nil)
		sr.Belief = NewBelief(vanilla.Table, vanilla.Decks[:])
		sr.SetBeliefs(net.Bind(vanilla.Table), nil)
		sr.seat = 0
		sr.beliefRoot(s, 0)
		if !sr.bel.deckOK || !sr.bel.handOK {
			t.Fatalf("offcolour %v: prediction not ready", off)
		}
		islands := 0
		for i := 0; i < 30; i++ {
			var c dirac.State
			c.CopyFrom(s)
			a, b := redealSeed(uint64(i)+100, i)
			sr.redeal(&c, a, b)
			got := hiddenOf(&c, opp)
			if len(got) != n {
				t.Fatalf("pool of %d, want %d", len(got), n)
			}
			bolts := 0
			for j, x := range got {
				switch x {
				case vanilla.Mountain:
				case vanilla.LightningBolt:
					bolts++
				case vanilla.Island:
					islands++
					if off == 0 {
						t.Fatalf("offcolour 0 dealt an off-colour Island")
					}
				default:
					t.Fatalf("dealt card %d (%s)", x, name(x))
				}
				if j < h && x != vanilla.Mountain && x != vanilla.Island {
					t.Fatalf("hand card %s, want lands only", name(x))
				}
			}
			if bolts > 3 {
				t.Fatalf("%d Lightning Bolts dealt (cap 3)", bolts)
			}
		}
		if off == 1 && islands == 0 {
			t.Fatal("offcolour 1: no Island dealt")
		}
	}
}

// TestBeliefsSearch: with the beliefs set and then cleared the searches
// match a searcher that never had them, step for step; with them on a
// closed-list game plays to the end and a search allocates nothing.
func TestBeliefsSearch(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 24
	name := func(c dirac.CardID) string { return vanilla.Table.Cards[c].Name }
	var cards []string
	for c := range vanilla.Table.Cards {
		cards = append(cards, name(dirac.CardID(c)))
	}
	bp := make([]float64, BeliefConfigs)
	bd := make([]float64, len(cards))
	for i := range bd {
		bd[i] = float64(i%7) / 3
	}
	deck := testBeliefNet(t, cards, []float64{0.5, 0.2, 0}, bp, bd, 0.2).Bind(vanilla.Table)
	hand := testBeliefNet(t, nil, []float64{1, -1, 0}, nil, nil, 0).Bind(vanilla.Table)
	mk := func() *Searcher {
		sr := NewSearcher(o, &fakeNet{val: 0.5}, nil)
		sr.Belief = NewBelief(vanilla.Table, vanilla.Decks[:])
		return sr
	}
	a := mk()
	b := mk()
	b.SetBeliefs(deck, hand)
	b.SetBeliefs(nil, nil)
	ra, wa := seatGame(a, 3)
	rb, wb := seatGame(b, 3)
	if wa != wb || len(ra) != len(rb) {
		t.Fatalf("cleared beliefs: %d/%d steps, winners %d/%d", len(ra), len(rb), wa, wb)
	}
	for i := range ra {
		if !sameSearch(&ra[i].res, &rb[i].res) {
			t.Fatalf("cleared beliefs: step %d differs", i)
		}
	}
	on := mk()
	on.SetBeliefs(deck, hand)
	ro, _ := seatGame(on, 3)
	diff := len(ro) != len(ra)
	for i := 0; !diff && i < len(ro); i++ {
		diff = !sameSearch(&ro[i].res, &ra[i].res)
	}
	if !diff {
		t.Fatal("beliefs on: the searches never changed")
	}
	s := beliefGame(5)
	var d dirac.Decision
	for i := 0; i < 400; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone || d.Player == 0 && d.N > 1 {
			break
		}
		s.Apply(d.Options[0])
	}
	if d.Kind == dirac.DkNone {
		t.Skip("no seat-0 decision")
	}
	on.Search(s, 1)
	if allocs := testing.AllocsPerRun(5, func() { on.Search(s, 1) }); allocs != 0 {
		t.Fatalf("a search with the beliefs on allocates %.1f times", allocs)
	}
}

// TestBeliefFeatureVector: the features read the opponent's public state
// from the seat's view, in the contract's order and scaling (HandFeatures).
func TestBeliefFeatureVector(t *testing.T) {
	s := beliefGame(11)
	var f [BeliefFeatures]float64
	BeliefFeatureVector(s, 0, &f)
	opp := uint8(1)
	if f[0] != float64(s.Turn())/20 || f[1] != float64(s.HandSize(opp))/7 || f[2] != float64(s.LibrarySize(opp))/40 ||
		f[11] != map[bool]float64{true: 1, false: 0}[s.ActivePlayer() == opp] || f[15] != float64(s.HandSize(0))/7 {
		t.Fatalf("features %v", f)
	}
	for i := 3; i <= 10; i++ {
		if f[i] != 0 {
			t.Fatalf("feature %d = %v at the start", i, f[i])
		}
	}
	if g := HandFeatures(s, 0, nil); !slices.Equal(g, f[:]) {
		t.Fatalf("HandFeatures %v, BeliefFeatureVector %v", g, f)
	}
	// play on until the opponent has a land out: lands, colours, rate
	var d dirac.Decision
	pol := dirac.NewRandomPolicy(3)
	for i := 0; i < 2000; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		BeliefFeatureVector(s, 0, &f)
		if f[3] > 0 {
			if f[19] != 1 || f[14] <= 0 || f[4] > f[3] {
				t.Fatalf("a Mountain out but features %v", f)
			}
			return
		}
		s.Apply(pol.Pick(&d))
	}
	t.Fatal("the opponent never played a land")
}

// TestParseBeliefNetShapes refuses malformed networks.
func TestParseBeliefNetShapes(t *testing.T) {
	for _, raw := range []string{
		`{}`,
		`{"version":3,"features":22,"mean":[0],"std":[1],"w1":[[0]],"b1":[1],"wh":[[0],[0],[0]],"bh":[0,0,0]}`,
		`{"version":3,"features":21}`,
	} {
		if _, err := ParseBeliefNet([]byte(raw)); err == nil {
			t.Fatalf("%s: parsed", raw)
		}
	}
	// the trainers' files load; a version past each format's and a wrong
	// feature count are refused
	for _, tc := range []struct{ file, from, to string }{
		{"handbelief_v1.json", `"version": 1`, `"version": 2`},
		{"handbelief_v1.json", `"features": 22`, `"features": 21`},
		{"deckbelief_v3.json", `"version": 3`, `"version": 4`},
		{"deckbelief_v3.json", `"features": 22`, `"features": 21`},
	} {
		raw, err := os.ReadFile(filepath.Join("testdata", tc.file))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseBeliefNet(raw); err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		bad := []byte(strings.Replace(string(raw), tc.from, tc.to, 1))
		if string(bad) == string(raw) {
			t.Fatalf("%s: no %s", tc.file, tc.from)
		}
		if _, err := ParseBeliefNet(bad); err == nil {
			t.Fatalf("%s: accepted %s", tc.file, tc.to)
		}
	}
	n := testBeliefNet(t, nil, []float64{0, 0, 0}, nil, nil, 0)
	if n.HasDeck() {
		t.Fatal("a hand-only network has deck heads")
	}
}

// BenchmarkBeliefRedeal: one world's deal with the corpus, the hand belief
// and the deck belief (a 300-card vocabulary).
func BenchmarkBeliefRedeal(b *testing.B) {
	name := func(c dirac.CardID) string { return vanilla.Table.Cards[c].Name }
	cards := make([]string, 300)
	for i := range cards {
		cards[i] = "x" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	for c := range vanilla.Table.Cards {
		cards[c] = name(dirac.CardID(c))
	}
	bp := make([]float64, BeliefConfigs)
	bd := make([]float64, len(cards))
	deck := testBeliefNet(b, cards, []float64{0.5, 0.2, 0}, bp, bd, 0.2).Bind(vanilla.Table)
	hand := testBeliefNet(b, nil, []float64{1, -1, 0}, nil, nil, 0).Bind(vanilla.Table)
	s := beliefGame(13)
	for _, bc := range []struct {
		name       string
		deck, hand *BeliefTable
	}{{"corpus", nil, nil}, {"handbelief", nil, hand}, {"deckbelief", deck, nil}} {
		b.Run(bc.name, func(b *testing.B) {
			sr := NewSearcher(DefaultOptions(), nil, nil)
			sr.Belief = NewBelief(vanilla.Table, vanilla.Decks[:])
			sr.SetBeliefs(bc.deck, bc.hand)
			sr.seat = 0
			if sr.bel != nil {
				sr.beliefRoot(s, 0)
			}
			var c dirac.State
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c.CopyFrom(s)
				x, y := redealSeed(uint64(i), i)
				sr.redeal(&c, x, y)
			}
		})
	}
	b.Run("root-h128", func(b *testing.B) {
		sr := NewSearcher(DefaultOptions(), nil, nil)
		sr.SetBeliefs(testBeliefNetH(b, 128, cards, []float64{0.5, 0.2, 0}, bp, bd, 0.2).Bind(vanilla.Table), nil)
		for i := 0; i < b.N; i++ {
			sr.beliefRoot(s, 0)
		}
	})
}

// dzBeliefCase is one state's features and predictions as the contract's
// reference implementation computes them (zzzbelief_dz_test.go).
type dzBeliefCase struct {
	seed        uint64
	steps, seat int
	f           [BeliefFeatures]float64
	hand        map[string][3]float64
	pair        map[string][BeliefConfigs]float64
	card        map[string]map[string]float64
}

// TestBeliefNetsMatchDZ: on fixed vanilla states, the features and every
// head's prediction of the synthetic network files in testdata (two
// sparse ones and two dense ones, a format-1 hand file and a format-3 deck
// file) equal what the contract's reference implementation (HandFeatures,
// HandBelief.Predict, DeckBelief.predict) computes, within 1e-5: the same
// feature order and scaling, standardisation, trunk, seen bag, heads and
// softmaxes, so networks trained to the contract predict the same here.
func TestBeliefNetsMatchDZ(t *testing.T) {
	nets := map[string]*BeliefNet{}
	for _, f := range []string{"handbelief_v1.json", "handbelief_x1.json", "deckbelief_v3.json", "deckbelief_x3.json"} {
		n, err := LoadBeliefNet(filepath.Join("testdata", f))
		if err != nil {
			t.Fatal(err)
		}
		nets[f] = n
	}
	const tol = 1e-5
	for _, c := range dzBeliefCases {
		s := new(dirac.State)
		s.Init(vanilla.Table, vanilla.Decks[0], vanilla.Decks[1], c.seed, dirac.Config{})
		pol := dirac.NewRandomPolicy(c.seed ^ 0x77)
		var d dirac.Decision
		for i := 0; i < c.steps; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			s.Apply(pol.Pick(&d))
		}
		seat := uint8(c.seat)
		near := func(what string, got, want float64) {
			t.Helper()
			if math.Abs(got-want) > tol {
				t.Fatalf("seed %d steps %d seat %d: %s: %.9f, reference %.9f", c.seed, c.steps, c.seat, what, got, want)
			}
		}
		var f [BeliefFeatures]float64
		BeliefFeatureVector(s, seat, &f)
		for i := range f {
			near(fmt.Sprintf("feature %d", i), f[i], c.f[i])
		}
		for name, want := range c.hand {
			sr := NewSearcher(DefaultOptions(), nil, nil)
			sr.Belief = NewBelief(vanilla.Table, vanilla.Decks[2:])
			bt := nets[name].Bind(vanilla.Table)
			if nets[name].HasDeck() {
				sr.SetBeliefs(bt, nil)
			} else {
				sr.SetBeliefs(nil, bt)
			}
			sr.beliefRoot(s, seat)
			if !sr.bel.handOK {
				t.Fatalf("%s: no hand prediction", name)
			}
			for k := range want {
				near(fmt.Sprintf("%s hand %d", name, k), sr.bel.q[k], want[k])
			}
			if !nets[name].HasDeck() {
				continue
			}
			if !sr.bel.deckOK {
				t.Fatalf("%s: no deck prediction", name)
			}
			for k, p := range c.pair[name] {
				near(fmt.Sprintf("%s configuration %d", name, k), sr.bel.pp[k], p)
			}
			tot := 0.0
			for _, w := range sr.bel.logit {
				tot += w
			}
			got := map[string]float64{}
			for v, w := range sr.bel.logit {
				if w > 0 {
					got[nets[name].Cards[v]] = w / tot
				}
			}
			if len(got) != len(c.card[name]) {
				t.Fatalf("%s: %d cards dealt, reference %d", name, len(got), len(c.card[name]))
			}
			for card, p := range c.card[name] {
				near(name+" card "+card, got[card], p)
			}
		}
	}
}
