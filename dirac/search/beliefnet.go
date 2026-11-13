package search

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Learned beliefs over the opponent's hidden cards, on a fixed
// learned-belief weights contract: hand networks (format 1) and deck
// networks (formats 1-3) trained offline load here and predict exactly what
// the reference implementation of the contract predicts
// (TestBeliefNetsMatchReference), and the dealt rows below (DealtHandMix,
// DealtPool) are the trainers' input.
//
// One small MLP over BeliefFeatures observable features of the opponent
// (standardised by the file's mean and std) and, for a network with a
// vocabulary, the opponent's seen cards as a bag (min(count, 4)/4 a card)
// with up to three heads:
//
//   - hand (wh, bh; w2, b2 in a format-1 hand file): the shares of the
//     opponent's hidden hand that are lands, instant-speed spells
//     (instants and flash) and other cards;
//   - colours (wp, bp): the deck's colour configuration, one of the 25
//     mono / pair / triple configurations (BeliefConfigMask);
//   - cards (wd, bd): logits over the vocabulary for the hidden pool (hand
//     plus library).
//
// Both are active only under ClosedList with a corpus (Searcher.Belief),
// never under Clairvoyant, and only for a Search's worlds (and
// DealtHandMix / DealtPool): the pregame DealUnseen deals as without them.
// :handbelief=PATH (Searcher.SetBeliefs' hand) uses the hand head only:
// each world still draws the closed-list corpus list minus the seen cards,
// and the hand is a weighted sample of that pool, each card weighted by
// max(share, 1e-3) / max(the class's share of the pool, 1e-3), so the
// dealt composition follows the belief. :deckbelief=PATH (the deck)
// replaces the corpus and, when both are set, the hand network: each world
// draws a configuration from the colour head, then the hidden pool from
// the card head -- a card outside the configuration's colours (CardColours)
// keeps `offcolour` of its weight (version 3), or is cut with a `splash`
// share of draws over every card instead (versions 1-2) -- at most 3
// copies of a non-basic counting the seen ones (8 draws over the cap in a
// row deal a basic land of the configuration's colours), and deals the
// hand by the deck network's hand head. A world the deck belief cannot
// deal (no card of the drawn configuration in the table, no basic to fall
// back on) draws from the corpus.
//
// The root's prediction is computed once per Search (beliefRoot), so a
// world costs only its draws: no map, no allocation per simulation. Off
// (Searcher.bel nil) the search is exactly as without this file: redeal
// tests one pointer.
//
// The features (BeliefFeatureVector, the contract's HandFeatures, in its
// order and scaling; all from the searching seat's view, no hidden card
// read; "opp" is the opponent, the player whose cards are predicted; the
// opp counts are over the objects opp controls):
//
//	 0 turn / 20              1 opp hand size / 7     2 opp library size / 40
//	 3 opp lands / 8          4 opp untapped lands / 8
//	 5 opp non-land permanents / 10                  6 opp non-land creatures / 8
//	 7 opp graveyard / 20     8 opp graveyard lands / 8
//	 9 opp graveyard instant-speed cards / 8        10 opp exiled cards / 8
//	11 opp is the active player (0/1)               12 the stack is not empty (0/1)
//	13 opp mulligans / 2     14 opp lands / max(1, turn/2 + 1)
//	15 the seat's own hand size / 7
//	16-20 colours (CardColours) among opp's seen cards, W U B R G (0/1)
//	21 opp has two or more untapped lands and is not the active player (0/1)

// BeliefFeatures is the learned beliefs' feature count.
const BeliefFeatures = 22

// BeliefConfigs is the number of colour configurations the deck belief's
// colour head predicts.
const BeliefConfigs = 25

// BeliefConfigMask is each configuration's colours (WUBRG bits), in the
// trainer's order: itertools.combinations(range(5), r) for r = 1, 2, 3.
var BeliefConfigMask = func() (m [BeliefConfigs]uint8) {
	k := 0
	for a := 0; a < 5; a++ {
		m[k] = 1 << a
		k++
	}
	for a := 0; a < 5; a++ {
		for b := a + 1; b < 5; b++ {
			m[k] = 1<<a | 1<<b
			k++
		}
	}
	for a := 0; a < 5; a++ {
		for b := a + 1; b < 5; b++ {
			for c := b + 1; c < 5; c++ {
				m[k] = 1<<a | 1<<b | 1<<c
				k++
			}
		}
	}
	return m
}()

// Belief classes of a card (the hand head's outputs).
const (
	BeliefLand    = 0
	BeliefInstant = 1 // instant-speed: an instant, or a non-land with flash
	BeliefOther   = 2
)

// BeliefClass is card c's hand class.
func BeliefClass(t *kop.Table, c dirac.CardID) uint8 {
	cd := &t.Cards[c]
	switch {
	case cd.Types&kop.TLand != 0:
		return BeliefLand
	case cd.Types&kop.TInstant != 0 || cd.Keywords&kop.KwFlash != 0:
		return BeliefInstant
	}
	return BeliefOther
}

// BeliefFeatureVector writes the belief features of seat's opponent at s,
// from seat's view (the order above): with more than two seats, the next
// seat in turn order (beliefOpp).
func BeliefFeatureVector(s *dirac.State, seat uint8, f *[BeliefFeatures]float64) {
	var seen [2 * dirac.MaxDeck]dirac.CardID
	ns := s.SeenCards(beliefOpp(s, seat), seat, seen[:])
	beliefFeatures(s, seat, seen[:ns], f)
}

// beliefOpp is the opponent the belief features describe: with two seats the
// other seat, with more the next seat in turn order. The learned beliefs are a
// two-seat model, so a search deals by them only with two seats (beliefRoot).
func beliefOpp(s *dirac.State, seat uint8) uint8 {
	return (seat + 1) % s.NumSeats()
}

// HandFeatures is BeliefFeatureVector appended to out[:0] (the contract's
// HandFeatures signature; allocates when out is short).
func HandFeatures(s *dirac.State, seat uint8, out []float64) []float64 {
	var f [BeliefFeatures]float64
	BeliefFeatureVector(s, seat, &f)
	return append(out[:0], f[:]...)
}

// beliefFeatures is BeliefFeatureVector given the opponent's seen cards.
func beliefFeatures(s *dirac.State, seat uint8, seen []dirac.CardID, f *[BeliefFeatures]float64) {
	opp := beliefOpp(s, seat)
	t := s.Table()
	var lands, untapped, perms, creatures, gy, gyLand, gyInst, exile float64
	for i := 0; i < s.NumObjs(); i++ {
		o := dirac.Obj(i)
		if s.Controller(o) != opp {
			continue
		}
		cd := &t.Cards[s.ObjCard(o)]
		switch s.ObjZone(o) {
		case kop.ZoneBattlefield:
			if cd.Types&kop.TLand != 0 {
				lands++
				if !s.Tapped(o) {
					untapped++
				}
			} else {
				perms++
				if cd.Types&kop.TCreature != 0 {
					creatures++
				}
			}
		case kop.ZoneGraveyard:
			gy++
			switch BeliefClass(t, s.ObjCard(o)) {
			case BeliefLand:
				gyLand++
			case BeliefInstant:
				gyInst++
			}
		case kop.ZoneExile:
			exile++
		}
	}
	var cols uint8
	for _, c := range seen {
		cols |= CardColours(t, c)
	}
	b := func(x bool) float64 {
		if x {
			return 1
		}
		return 0
	}
	turn := float64(s.Turn())
	*f = [BeliefFeatures]float64{
		turn / 20, float64(s.HandSize(opp)) / 7, float64(s.LibrarySize(opp)) / 40,
		lands / 8, untapped / 8, perms / 10, creatures / 8, gy / 20, gyLand / 8, gyInst / 8, exile / 8,
		b(s.ActivePlayer() == opp), b(s.StackLen() > 0), float64(s.Mulligans(opp)) / 2,
		lands / math.Max(1, turn/2+1), float64(s.HandSize(seat)) / 7,
		b(cols&1 != 0), b(cols&2 != 0), b(cols&4 != 0), b(cols&8 != 0), b(cols&16 != 0),
		b(untapped >= 2 && s.ActivePlayer() != opp),
	}
}

// BeliefNet is a trained belief network: a hand belief (format
// 1: version, features, mean, std, w1, b1, w2, b2) or deck belief
// (formats 1-3: version, features, cards, mean, std, w1/b1, wh/bh, wp/bp,
// wd/bd, splash, and from 3 offcolour). Read-only once loaded; shared by
// every Searcher.
type BeliefNet struct {
	Version   int
	Cards     []string // the vocabulary (empty: a features-only network)
	H         int
	Offcolour float64 // > 0: an off-colour card keeps this share of its weight
	Splash    float64 // the share of draws over every card (offcolour 0)
	mean, std [BeliefFeatures]float64
	w1t       []float64 // (features + vocabulary) x H, input-major
	b1        []float64
	wh        []float64 // 3 x H
	bh        [3]float64
	wp, bp    []float64 // BeliefConfigs x H, BeliefConfigs (nil: no colour head)
	wd, bd    []float64 // V x H, V (nil: no card head)
}

// HasDeck reports whether the network has the colour and card heads
// :deckbelief needs.
func (n *BeliefNet) HasDeck() bool { return n.wp != nil && n.wd != nil }

// The newest formats ParseBeliefNet reads: a hand file (w2/b2) and a deck
// file (wh/bh, the colour and card heads).
const (
	HandBeliefVersion = 1
	DeckBeliefVersion = 3
)

type beliefJSON struct {
	Version   int         `json:"version"`
	Features  int         `json:"features"`
	Splash    float64     `json:"splash"`
	Offcolour float64     `json:"offcolour"`
	Cards     []string    `json:"cards"`
	Mean      []float64   `json:"mean"`
	Std       []float64   `json:"std"`
	W1        [][]float64 `json:"w1"`
	B1        []float64   `json:"b1"`
	W2        [][]float64 `json:"w2"`
	B2        []float64   `json:"b2"`
	Wh        [][]float64 `json:"wh"`
	Bh        []float64   `json:"bh"`
	Wp        [][]float64 `json:"wp"`
	Bp        []float64   `json:"bp"`
	Wd        [][]float64 `json:"wd"`
	Bd        []float64   `json:"bd"`
}

// LoadBeliefNet reads a belief network file.
func LoadBeliefNet(path string) (*BeliefNet, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	n, err := ParseBeliefNet(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return n, nil
}

// ParseBeliefNet parses a belief network (allocates; init only). It
// refuses a version it does not know and weights of the wrong shape.
func ParseBeliefNet(raw []byte) (*BeliefNet, error) {
	var j beliefJSON
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, err
	}
	handFile := j.W2 != nil || j.B2 != nil
	if handFile {
		if j.Version < 1 || j.Version > HandBeliefVersion {
			return nil, fmt.Errorf("hand belief format version %d (this build reads 1..%d)", j.Version, HandBeliefVersion)
		}
		if j.Wh != nil || j.Wp != nil || j.Wd != nil || len(j.Cards) != 0 {
			return nil, fmt.Errorf("belief network: both a hand file's w2 and a deck file's heads")
		}
		j.Wh, j.Bh = j.W2, j.B2
	} else if j.Version < 1 || j.Version > DeckBeliefVersion {
		return nil, fmt.Errorf("deck belief format version %d (this build reads 1..%d)", j.Version, DeckBeliefVersion)
	}
	if j.Features != BeliefFeatures {
		return nil, fmt.Errorf("belief network: %d features, want %d", j.Features, BeliefFeatures)
	}
	if len(j.Mean) != BeliefFeatures || len(j.Std) != BeliefFeatures {
		return nil, fmt.Errorf("belief network: mean/std want %d entries", BeliefFeatures)
	}
	if !(j.Splash >= 0 && j.Splash < 1) || !(j.Offcolour >= 0 && j.Offcolour <= 1) {
		return nil, fmt.Errorf("belief network: splash %v, offcolour %v (want 0 <= splash < 1, 0 <= offcolour <= 1)", j.Splash, j.Offcolour)
	}
	V := len(j.Cards)
	in := BeliefFeatures + V
	H := len(j.B1)
	if H == 0 || len(j.W1) != H {
		return nil, fmt.Errorf("belief network: w1 has %d rows, b1 %d", len(j.W1), H)
	}
	n := &BeliefNet{Version: j.Version, Cards: j.Cards, H: H, Splash: j.Splash, Offcolour: j.Offcolour, b1: j.B1}
	copy(n.mean[:], j.Mean)
	copy(n.std[:], j.Std)
	for i, x := range n.std {
		if !(x > 0) {
			return nil, fmt.Errorf("belief network: std[%d] = %v, want > 0", i, x)
		}
	}
	n.w1t = make([]float64, in*H)
	for h, row := range j.W1 {
		if len(row) != in {
			return nil, fmt.Errorf("belief network: w1 row %d has %d inputs, want %d (22 features + %d cards)", h, len(row), in, V)
		}
		for i, x := range row {
			n.w1t[i*H+h] = x
		}
	}
	mat := func(name string, rows [][]float64, b []float64, want int) ([]float64, error) {
		if len(rows) != want || len(b) != want {
			return nil, fmt.Errorf("belief network: %s has %d rows and %d biases, want %d", name, len(rows), len(b), want)
		}
		out := make([]float64, 0, want*H)
		for i, r := range rows {
			if len(r) != H {
				return nil, fmt.Errorf("belief network: %s row %d has %d entries, want %d", name, i, len(r), H)
			}
			out = append(out, r...)
		}
		return out, nil
	}
	var err error
	if n.wh, err = mat("wh", j.Wh, j.Bh, 3); err != nil {
		return nil, err
	}
	copy(n.bh[:], j.Bh)
	if j.Wp != nil || j.Wd != nil {
		if n.wp, err = mat("wp", j.Wp, j.Bp, BeliefConfigs); err != nil {
			return nil, err
		}
		n.bp = j.Bp
		if V == 0 {
			return nil, fmt.Errorf("belief network: a card head needs the cards vocabulary")
		}
		if n.wd, err = mat("wd", j.Wd, j.Bd, V); err != nil {
			return nil, err
		}
		n.bd = j.Bd
	}
	return n, nil
}

// BeliefTable is a network bound to a card table (the vocabulary's names
// resolved to the table's CardIDs).
type BeliefTable struct {
	Net    *BeliefNet
	tbl    *kop.Table
	id     []dirac.CardID // vocabulary index -> table card (kop.NoCard: not in the table)
	vi     []int32        // table card -> vocabulary index (-1: none)
	cfg    []uint32       // vocabulary: bit k set when configuration k holds the card's colours
	col    []uint8        // vocabulary: CardColours
	basic  []bool         // vocabulary: a basic land (no copy cap)
	basics []int32        // the vocabulary's basic lands in the table, in vocabulary order
}

// Bind resolves n's vocabulary against t (allocates; once per table).
// A vocabulary name the table lacks is never dealt; a table card the
// vocabulary lacks is never dealt by the deck belief and reads as unseen.
func (n *BeliefNet) Bind(t *kop.Table) *BeliefTable {
	bt := &BeliefTable{Net: n, tbl: t}
	bt.vi = make([]int32, len(t.Cards))
	for i := range bt.vi {
		bt.vi[i] = -1
	}
	byName := make(map[string]dirac.CardID, len(t.Cards))
	for i := range t.Cards {
		if _, ok := byName[t.Cards[i].Name]; !ok {
			byName[t.Cards[i].Name] = dirac.CardID(i)
		}
	}
	V := len(n.Cards)
	bt.id = make([]dirac.CardID, V)
	bt.cfg = make([]uint32, V)
	bt.col = make([]uint8, V)
	bt.basic = make([]bool, V)
	for v, name := range n.Cards {
		c, ok := byName[name]
		if !ok {
			bt.id[v] = kop.NoCard
			continue
		}
		bt.id[v] = c
		for i := range t.Cards { // every table card of that name (copies under other ids)
			if t.Cards[i].Name == name && bt.vi[i] < 0 {
				bt.vi[i] = int32(v)
			}
		}
		bt.col[v] = CardColours(t, c)
		for k, m := range BeliefConfigMask {
			if bt.col[v]&^m == 0 {
				bt.cfg[v] |= 1 << k
			}
		}
		if bt.basic[v] = t.Cards[c].Types&kop.TBasic != 0; bt.basic[v] {
			bt.basics = append(bt.basics, int32(v))
		}
	}
	return bt
}

// beliefNetState is one bound network's per-Searcher scratch.
type beliefNetState struct {
	bt      *BeliefTable
	z       []float64 // hidden activations
	seenN   []uint8   // vocabulary: seen copies at the root
	touched []int32   // seenN entries set
}

func (ns *beliefNetState) bind(bt *BeliefTable) {
	ns.bt = bt
	if bt == nil {
		return
	}
	if cap(ns.z) < bt.Net.H {
		ns.z = make([]float64, bt.Net.H)
	}
	ns.z = ns.z[:bt.Net.H]
	V := len(bt.Net.Cards)
	if cap(ns.seenN) < V {
		ns.seenN = make([]uint8, V)
		ns.touched = make([]int32, 0, 2*dirac.MaxDeck)
	}
	ns.seenN = ns.seenN[:V]
	clear(ns.seenN)
	ns.touched = ns.touched[:0]
}

// forward runs the trunk on f and the seen cards (table ids), leaving the
// hidden layer in z and the seen counts in seenN.
func (ns *beliefNetState) forward(f *[BeliefFeatures]float64, seen []dirac.CardID) {
	n := ns.bt.Net
	for _, v := range ns.touched {
		ns.seenN[v] = 0
	}
	ns.touched = ns.touched[:0]
	for _, c := range seen {
		if int(c) >= len(ns.bt.vi) {
			continue
		}
		if v := ns.bt.vi[c]; v >= 0 {
			if ns.seenN[v] == 0 {
				ns.touched = append(ns.touched, v)
			}
			if ns.seenN[v] < 255 {
				ns.seenN[v]++
			}
		}
	}
	H := n.H
	z := ns.z
	copy(z, n.b1)
	for i := 0; i < BeliefFeatures; i++ {
		x := (f[i] - n.mean[i]) / n.std[i]
		if x == 0 {
			continue
		}
		w := n.w1t[i*H : (i+1)*H]
		for h := range z {
			z[h] += x * w[h]
		}
	}
	for _, v := range ns.touched {
		x := float64(min(ns.seenN[v], 4)) / 4
		w := n.w1t[(BeliefFeatures+int(v))*H : (BeliefFeatures+int(v)+1)*H]
		for h := range z {
			z[h] += x * w[h]
		}
	}
	for h := range z {
		if z[h] < 0 {
			z[h] = 0
		}
	}
}

// head is row i of a head's weights dotted with z plus its bias.
func head(w, b []float64, i int, z []float64) float64 {
	H := len(z)
	row := w[i*H : (i+1)*H]
	s := b[i]
	for h, x := range z {
		s += row[h] * x
	}
	return s
}

// beliefState is a Searcher's learned-belief state (nil: off).
type beliefState struct {
	deck, hand beliefNetState
	// the root's prediction (beliefRoot)
	handOK bool
	q      [3]float64 // the hand's class shares
	deckOK bool
	pp     [BeliefConfigs]float64 // configuration probabilities
	pc     [BeliefConfigs]float64 // their cumulative sums
	cum    [][]float64            // BeliefConfigs+1 rows of cumulative card weights (the last unrestricted)
	dealt  []uint8                // vocabulary: copies dealt in this world
	used   []int32
	logit  []float64 // the card head's weights exp(logit - max) (0: not in the table)
	cls    [3]int
	w      [2 * dirac.MaxDeck]float64
	feat   [BeliefFeatures]float64
}

// SetBeliefs sets the learned beliefs a closed-list search deals the
// opponent's hidden cards by (:deckbelief, :handbelief): deck, when
// non-nil, needs HasDeck, and replaces hand (the contract's deck belief
// replaces its hand belief when both are set). Both nil turns them off. It allocates
// only when a network's vocabulary is larger than any before (once per
// run).
func (sr *Searcher) SetBeliefs(deck, hand *BeliefTable) {
	if deck == nil && hand == nil {
		sr.bel = nil
		return
	}
	if deck != nil {
		hand = nil
	}
	if sr.belBuf == nil {
		sr.belBuf = &beliefState{}
	}
	b := sr.belBuf
	b.deck.bind(deck)
	b.hand.bind(hand)
	b.handOK, b.deckOK = false, false
	if deck != nil {
		V := len(deck.Net.Cards)
		if cap(b.logit) < V {
			b.logit = make([]float64, V)
			b.dealt = make([]uint8, V)
			b.used = make([]int32, 0, 2*dirac.MaxDeck)
			b.cum = make([][]float64, BeliefConfigs+1)
			for k := range b.cum {
				b.cum[k] = make([]float64, V)
			}
		}
		b.logit, b.dealt = b.logit[:V], b.dealt[:V]
		for k := range b.cum {
			b.cum[k] = b.cum[k][:V]
		}
		clear(b.dealt)
		b.used = b.used[:0]
	}
	sr.bel = b
}

// softmax3 is the hand head's class shares from the trunk z.
func softmax3(n *BeliefNet, z []float64) (q [3]float64) {
	var l [3]float64
	mx := math.Inf(-1)
	for i := 0; i < 3; i++ {
		l[i] = head(n.wh, n.bh[:], i, z)
		mx = max(mx, l[i])
	}
	t := 0.0
	for i := range l {
		l[i] = math.Exp(l[i] - mx)
		t += l[i]
	}
	for i := range l {
		q[i] = l[i] / t
	}
	return q
}

// beliefRoot computes the beliefs' prediction for seat's opponent at s,
// once per search: only under ClosedList with a corpus, never under
// Clairvoyant (handOK and deckOK false: the plain deal). The beliefs read one
// opponent's features, so with more than two seats they predict nothing and
// each opponent is dealt from the corpus by its own seen cards.
func (sr *Searcher) beliefRoot(s *dirac.State, seat uint8) {
	b := sr.bel
	b.handOK, b.deckOK = false, false
	if !sr.Opts.ClosedList || sr.Belief == nil || sr.Opts.Clairvoyant || s.NumSeats() > 2 {
		return
	}
	ns := s.SeenCards(1-seat, seat, sr.seenC[:])
	seen := sr.seenC[:ns]
	beliefFeatures(s, seat, seen, &b.feat)
	hs := &b.hand
	if d := &b.deck; d.bt != nil {
		d.forward(&b.feat, seen)
		sr.deckRoot()
		hs = d
	} else {
		hs.forward(&b.feat, seen)
	}
	b.q = softmax3(hs.bt.Net, hs.z)
	b.handOK = true
}

// deckRoot turns the deck network's trunk into the configuration and card
// tables a world draws from.
func (sr *Searcher) deckRoot() {
	b := sr.bel
	d := &b.deck
	n := d.bt.Net
	V := len(n.Cards)
	mx := math.Inf(-1)
	for k := range b.pp {
		b.pp[k] = head(n.wp, n.bp, k, d.z)
		mx = max(mx, b.pp[k])
	}
	tot := 0.0
	for k := range b.pp {
		b.pp[k] = math.Exp(b.pp[k] - mx)
		tot += b.pp[k]
	}
	acc := 0.0
	for k := range b.pp {
		b.pp[k] /= tot
		acc += b.pp[k]
		b.pc[k] = acc
	}
	mx = math.Inf(-1)
	for v := 0; v < V; v++ {
		if d.bt.id[v] == kop.NoCard {
			b.logit[v] = math.Inf(-1)
			continue
		}
		b.logit[v] = head(n.wd, n.bd, v, d.z)
		mx = max(mx, b.logit[v])
	}
	if math.IsInf(mx, -1) {
		return // no vocabulary card in this table: deckOK stays false
	}
	all := b.cum[BeliefConfigs]
	acc = 0.0
	for v := 0; v < V; v++ {
		e := 0.0
		if d.bt.id[v] != kop.NoCard {
			e = math.Exp(b.logit[v] - mx)
		}
		b.logit[v] = e
		acc += e
		all[v] = acc
	}
	alpha := n.Offcolour // 0: the hard restriction (versions 1-2)
	for k := 0; k < BeliefConfigs; k++ {
		row := b.cum[k]
		acc := 0.0
		bit := uint32(1) << k
		for v := 0; v < V; v++ {
			e := b.logit[v]
			if d.bt.cfg[v]&bit == 0 {
				e *= alpha
			}
			acc += e
			row[v] = acc
		}
	}
	b.deckOK = true
}

// unit is a uniform draw in [0, 1).
func unit(r *dirac.Rand) float64 { return float64(r.Uint64()>>11) / (1 << 53) }

// pick is the first index whose cumulative weight exceeds u.
func pick(row []float64, u float64) int {
	lo, hi := 0, len(row)-1
	for lo < hi {
		m := (lo + hi) / 2
		if row[m] > u {
			hi = m
		} else {
			lo = m + 1
		}
	}
	return lo
}

// deckPool draws a world's hidden pool from the deck belief into pool and
// reports whether it could (false: the world draws from the corpus).
func (sr *Searcher) deckPool(r *dirac.Rand, pool []dirac.CardID) bool {
	b := sr.bel
	bt := b.deck.bt
	k := pick(b.pc[:], unit(r)*b.pc[BeliefConfigs-1])
	row := b.cum[k]
	if len(row) == 0 || !(row[len(row)-1] > 0) {
		return false
	}
	all := b.cum[BeliefConfigs]
	splash := bt.Net.Splash
	for _, v := range b.used {
		b.dealt[v] = 0
	}
	b.used = b.used[:0]
	seenN := b.deck.seenN
	for i, fails := 0, 0; i < len(pool); {
		rw := row
		if splash > 0 && unit(r) < splash {
			rw = all // a splash draw: the whole vocabulary
		}
		v := pick(rw, unit(r)*rw[len(rw)-1])
		if !bt.basic[v] && int(seenN[v])+int(b.dealt[v]) >= 3 {
			if fails++; fails < 8 {
				continue
			}
			if v = sr.basicOf(BeliefConfigMask[k], r); v < 0 {
				return false
			}
		}
		fails = 0
		if b.dealt[v] == 0 {
			b.used = append(b.used, int32(v))
		}
		if b.dealt[v] < 255 {
			b.dealt[v]++
		}
		pool[i] = bt.id[v]
		i++
	}
	return true
}

// basicOf is a vocabulary index of a basic land of one of mask's colours
// (among the first 8 such in vocabulary order; -1: none).
func (sr *Searcher) basicOf(mask uint8, r *dirac.Rand) int {
	bt := sr.bel.deck.bt
	var cand [8]int32
	n := 0
	for _, v := range bt.basics {
		if bt.col[v]&mask != 0 && n < len(cand) {
			cand[n] = v
			n++
		}
	}
	if n == 0 {
		return -1
	}
	return int(cand[r.IntN(n)])
}

// corpusPool deals n cards from the closed-list corpus (Belief.deal's
// world) for seat's view of player p into pool.
func (sr *Searcher) corpusPool(s *dirac.State, p, seat uint8, r *dirac.Rand, n int, pool []dirac.CardID) {
	ns := s.SeenCards(p, seat, sr.seenC[:])
	seen := sr.seenC[:ns]
	slices.Sort(seen)
	var mask uint8
	for _, c := range seen {
		mask |= CardColours(s.Table(), c)
	}
	sr.Belief.deal(r, seen, mask, n, pool)
}

// beliefRedeal is redeal's opponent deal with a learned belief set: the
// pool from the deck belief (else the corpus, else the open list), then
// the hand by the hand head when there is a prediction.
func (sr *Searcher) beliefRedeal(s *dirac.State, p, seat uint8, r *dirac.Rand, n int, revealed bool) {
	b := sr.bel
	pool := sr.pool[:n]
	switch {
	case b.deckOK && sr.deckPool(r, pool):
	case sr.Opts.ClosedList && sr.Belief != nil:
		sr.corpusPool(s, p, seat, r, n, pool)
		sr.St.MarkedDealt += sr.Belief.marked(pool)
	case revealed:
		h := s.HandSize(p)
		m := s.HiddenCards(p, sr.pool[:h+n])
		copy(pool, sr.pool[h:m])
	default:
		s.HiddenCards(p, pool)
	}
	if revealed {
		r.ShuffleCards(pool)
		s.DealLibrary(p, pool)
		return
	}
	if !b.handOK {
		s.DealHidden(p, r, pool)
		return
	}
	sr.handFirst(s.Table(), r, pool, s.HandSize(p))
	s.DealHiddenOrdered(p, pool)
}

// handFirst moves a hand of h cards to the front of pool, drawn without
// replacement with each card weighted by max(its class's predicted share,
// 1e-3) over max(the class's share of the pool, 1e-3) (the contract's
// weighted hand), and shuffles the rest.
func (sr *Searcher) handFirst(t *kop.Table, r *dirac.Rand, pool []dirac.CardID, h int) {
	b := sr.bel
	n := len(pool)
	b.cls = [3]int{}
	for _, c := range pool {
		b.cls[BeliefClass(t, c)]++
	}
	var wc [3]float64
	for i := range wc {
		wc[i] = math.Max(b.q[i], 1e-3) / math.Max(float64(b.cls[i])/float64(max(n, 1)), 1e-3)
	}
	w := b.w[:n]
	tot := 0.0
	for i, c := range pool {
		w[i] = wc[BeliefClass(t, c)]
		tot += w[i]
	}
	if h > n {
		h = n
	}
	for i := 0; i < h; i++ {
		j := n - 1
		u := unit(r) * tot
		for k := i; k < n; k++ {
			u -= w[k]
			if u < 0 {
				j = k
				break
			}
		}
		tot -= w[j]
		pool[i], pool[j] = pool[j], pool[i]
		w[i], w[j] = w[j], w[i]
	}
	r.ShuffleCards(pool[h:])
}

// DealtHandMix is the mean class shares (land, instant-speed, other) of
// the opponent's (beliefOpp) hand over k worlds this Searcher deals at s for seat (its
// configured deal: open list, corpus, or the learned beliefs), from seed
// alone (a belief data row's "dealt"). It leaves s
// and the game alone; it overwrites the Searcher's scratch, so call it
// between searches.
func (sr *Searcher) DealtHandMix(s *dirac.State, seat uint8, k int, seed uint64) (mix [3]float64) {
	saveSeat, saveSt := sr.seat, sr.St
	sr.seat = seat
	if sr.bel != nil {
		sr.beliefRoot(s, seat)
	}
	opp := beliefOpp(s, seat)
	t := s.Table()
	n := 0
	for i := 0; i < k; i++ {
		sr.tmp.CopyFrom(s)
		a, b := redealSeed(seed, i)
		sr.redeal(&sr.tmp, a, b)
		h := sr.tmp.HandSize(opp)
		sr.tmp.HiddenCards(opp, sr.pool[:])
		for _, c := range sr.pool[:h] {
			mix[BeliefClass(t, c)]++
		}
		n += h
	}
	for i := range mix {
		mix[i] /= float64(max(n, 1))
	}
	sr.seat, sr.St = saveSeat, saveSt
	return mix
}

// DealtPool deals world i of this Searcher's configured deal at s for seat
// (as DealtHandMix, from seed alone) and writes the opponent's dealt
// hidden pool (hand and library) into out, returning its size: a belief
// data row's "dpool". Call it between searches.
func (sr *Searcher) DealtPool(s *dirac.State, seat uint8, i int, seed uint64, out []dirac.CardID) int {
	saveSeat, saveSt := sr.seat, sr.St
	sr.seat = seat
	if sr.bel != nil {
		sr.beliefRoot(s, seat)
	}
	sr.tmp.CopyFrom(s)
	a, b := redealSeed(seed, i)
	sr.redeal(&sr.tmp, a, b)
	n := sr.tmp.HiddenCards(beliefOpp(s, seat), out)
	sr.seat, sr.St = saveSeat, saveSt
	return n
}
