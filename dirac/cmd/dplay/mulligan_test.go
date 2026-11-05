package main

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	botpolicy "github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/mull"
	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMulliganPolicyNames pins dplay's policy list to gorge's registry: a
// name gorge registers that dplay lacks (or vice versa) fails the build's
// test run.
func TestMulliganPolicyNames(t *testing.T) {
	gorge := botpolicy.MulliganNames()
	want := slices.Clone(mullPolicies)
	slices.Sort(want)
	if !slices.Equal(want, gorge) {
		t.Fatalf("dplay policy names %v != gorge registry's %v", want, gorge)
	}
	for _, n := range gorge {
		if !slices.Contains(mullPolicies, n) {
			t.Errorf("gorge policy %q missing from dplay's list", n)
		}
	}
	if !slices.Contains(mullPolicies, "constructed") {
		t.Errorf("constructed missing from dplay's list")
	}
	o := options{a: seatRandom, b: seatRandom, config: configAutopay, mullPolicy: "noland"}
	err := o.check()
	if err == nil || !strings.HasPrefix(err.Error(), `-mulligan-policy "noland": want one of `) {
		t.Errorf("unknown name error %v", err)
	}
}

// --- the lockstep parity harness -------------------------------------------
//
// Each seeded case is one hand of 7 facts: `lands` land cards (0..7) and
// 7-lands spells, `bottom` prior mulligans (0..2). The SAME facts feed both
// sides: the dplay port (dirac/search/mull) and gorge's botpolicy policy (a
// Board of Cards built from the facts, the registry policy asked through
// MulliganByName). Coin is compared by its draw sequence: both sides are
// given the same drawn lot v (0..2) -- dplay's CoinMulls(v) against gorge's
// default arm driven by a stub rng whose IntN(3) returns v -- so the
// comparison is over the answer as a function of the draw, not over the
// draw's outcome; the stub also counts draws, pinning the one-draw contract.

// factCard is one generated hand card: its mull facts and the gorge Card
// the same facts build.
type factCard struct {
	f mull.Card
	g botpolicy.Card
}

// landFacts builds one land card, variant v:
// 0 a basic White plains, 1 a basic Blue island, 2 a non-basic land with no
// recorded production (Evolving Wilds), 3 a real Produced$ Combo gain land
// (Bloodfell Caves, comboLandFacts), 4 a colourless-only producer ({C}).
func landFacts(v int) factCard {
	if v%5 == 3 {
		return comboLandFacts()
	}
	f := mull.Card{Land: true}
	var g botpolicy.Card
	var mp cards.ManaProduction
	switch v % 5 {
	case 0:
		f.Prod, f.Produces[0] = true, true
		mp.Colour[0] = 1
		g.Basic = true
	case 1:
		f.Prod, f.Produces[1] = true, true
		mp.Colour[1] = 1
		g.Basic = true
	case 4:
		f.Prod = true
		mp.Colour[5] = 1 // colourless only: no WUBRG colour
	}
	// cases 0/1/3/4 record production; case 2 records none (wilds)
	g = botpolicy.Card{PrintedName: fmt.Sprintf("land%d", v%5), Basic: g.Basic,
		ManaCost: "no cost", Castable: true, Produces: mp}
	return factCard{f: f, g: g}
}

// comboLandFacts is a real Produced$ Combo dual, Bloodfell Caves, as both
// sides read it: dplay's mull.CardOf of the generated card, and gorge's
// ManaProduction (Any plus its B/R counts). Using the generated record, not
// a hand-built Any land, is what makes the lockstep catch a CardOf that
// loses the Combo choice and reports B/R only.
func comboLandFacts() factCard {
	set := cardset.MustByName("fdn")
	i, ok := set.Lookup("Bloodfell Caves")
	if !ok {
		panic("the fdn pool has no Bloodfell Caves")
	}
	f := mull.CardOf(*set.Def(i))
	var mp cards.ManaProduction
	mp.Any = true
	mp.Colour[2], mp.Colour[3] = 1, 1 // B, R
	return factCard{f: f, g: botpolicy.Card{PrintedName: "Bloodfell Caves",
		ManaCost: "no cost", Castable: true, Produces: mp}}
}

// mountainFacts is a basic Mountain, the non-green land the combo-land
// regression bottoms.
func mountainFacts() factCard {
	f := mull.Card{Land: true, Prod: true, Produces: [5]bool{false, false, false, true, false}}
	var mp cards.ManaProduction
	mp.Colour[3] = 1
	return factCard{f: f, g: botpolicy.Card{PrintedName: "Mountain", Basic: true,
		ManaCost: "no cost", Castable: true, Produces: mp}}
}

// spellFacts builds one spell with `generic` generic mana and the WUBRG pip
// counts p; its CMC is generic+sum(p), never 0 (a costless noncreature
// nonland would read as a land in dplay's facts and not in gorge's
// printed-cost test; the FDN pool has no such card).
func spellFacts(generic int, p [5]int32) factCard {
	sum := int32(0)
	for _, v := range p {
		sum += v
	}
	f := mull.Card{CMC: int32(generic) + sum, Pips: p}
	var parts []string
	if generic > 0 {
		parts = append(parts, fmt.Sprint(generic))
	}
	for col, n := range p {
		for k := int32(0); k < n; k++ {
			parts = append(parts, "WUBRG"[col:col+1])
		}
	}
	mc := strings.Join(parts, " ")
	if mc == "" {
		mc = "1" // CMC 0 refused above, so some part always exists
	}
	g := botpolicy.Card{PrintedName: "spell " + mc, Creature: true, CMC: f.CMC,
		ManaCost: mc, Castable: true}
	return factCard{f: f, g: g}
}

// mullSrc is a stub gorge rng whose IntN(3) returns v, counting its draws.
// IntN(3) is uint64n(3), Lemire's method: u*3 = hi*2^64 + lo, answer hi,
// rejected while lo < (2^64-3)%3 = 1. v=0 needs u*3 < 2^64 (hi=0) with
// lo >= 1, so u=1; v=1 and v=2 need hi=v with lo>=1, the ceilings of
// v*2^64/3 (lo = 2 and 4). Each case is exactly one draw.
type mullSrc struct {
	v int
	n int
}

var mullSrcU = [3]uint64{1, 6148914691236517206, 12297829382473034412}

func (s *mullSrc) Uint64() uint64 { s.n++; return mullSrcU[s.v] }

// keepDecision is engine/rules/mulligan.go's keep/mulligan ask, gorge
// keepMulliganPrompt's wording (keepBottomCount reads the bottoming penalty
// off it).
func keepDecision(bottom int, canMulligan bool) *decision.Decision {
	n := fmt.Sprintf("%d cards", bottom)
	if bottom == 1 {
		n = "1 card"
	}
	prompt := "p0 plays first. Keep your hand (put " + n + " on the bottom of your library) or take a mulligan?"
	d := &decision.Decision{Seq: 3, Player: 0, Kind: decision.KMulligan, Min: 1, Max: 1,
		Prompt: prompt, Options: []decision.Option{{Index: 0, Kind: "keep", Label: "keep"}}}
	if canMulligan {
		d.Options = append(d.Options, decision.Option{Index: 1, Kind: "mulligan", Label: "mulligan"})
	}
	return d
}

// bottomDecision is the bottoming ask: one "bottom" option per hand card,
// ObjIDs 1..7 in hand order, m picks.
func bottomDecision(m int) *decision.Decision {
	d := &decision.Decision{Seq: 9, Player: 0, Kind: decision.KMulligan, Min: m, Max: m}
	for i := 0; i < 7; i++ {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "bottom", Obj: state.ObjID(i + 1)})
	}
	return d
}

// gorgeBoard builds gorge's Board for a hand of fact cards, with OwnDeck
// holding deckTotal cards deckLands of them lands (nil when deckTotal is 0:
// gorge landWindow's no-manifest arm).
func gorgeBoard(rule botpolicy.MulliganRule, hand []factCard, deckTotal, deckLands int) botpolicy.Board {
	m := map[state.ObjID]botpolicy.Card{}
	for i, fc := range hand {
		m[state.ObjID(i+1)] = fc.g
	}
	b := botpolicy.Board{Mulligan: rule, Cards: botpolicy.TableOf(m), HandSize: int32(len(hand))}
	if deckTotal > 0 {
		b.OwnDeck = &deck.Manifest{Name: "d", Main: []deck.ManifestRow{
			{Name: "lands", Count: deckLands, Land: true},
			{Name: "spells", Count: deckTotal - deckLands},
		}}
	}
	return b
}

// deckShares are the own-deck land shares the constructed cases rotate
// over: 24 of 60 (hi 5), 16 of 60 (hi 4), and no manifest (the Limited 2..5
// window).
var deckShares = [][2]int{{60, 24}, {60, 16}, {0, 0}}

// TestMulliganPolicyLockstep is the four policies' lockstep test: 420 seeded
// hands covering 0..7 lands, 0..2 prior mulligans and the three deck shares,
// each answered by dplay's port and by gorge's botpolicy policy.
func TestMulliganPolicyLockstep(t *testing.T) {
	cardsettest.Require(t, "fdn")
	rng := rand.New(rand.NewPCG(20261019, 7))
	seenLands, seenBottom := map[int]bool{}, map[int]bool{}
	nHands, nAsks, nBottoms := 0, 0, 0

	for h := 0; h < 420; h++ {
		lands := h % 8
		bottom := (h / 8) % 3
		sh := deckShares[(h/24)%3]
		hand := make([]factCard, 0, 7)
		for i := 0; i < lands; i++ {
			hand = append(hand, landFacts(rng.IntN(5)))
		}
		for len(hand) < 7 {
			pips := [5]int32{}
			for col := range pips {
				if rng.IntN(3) == 0 {
					pips[col] = int32(1 + rng.IntN(2))
				}
			}
			hand = append(hand, spellFacts(rng.IntN(4), pips))
		}
		seenLands[lands], seenBottom[bottom] = true, true
		nHands++

		facts := make([]mull.Card, 7)
		for i, fc := range hand {
			facts[i] = fc.f
		}
		landN := 0
		for _, f := range facts {
			if f.Land {
				landN++
			}
		}

		// -- the keep/mulligan ask, per policy ----------------------------
		for _, name := range []string{"lands", "never", "constructed"} {
			rule, err := botpolicy.MulliganByName(name)
			if err != nil {
				t.Fatal(err)
			}
			b := gorgeBoard(rule, hand, sh[0], sh[1])
			ch, ok := botpolicy.MulliganAnswer(b, rule, keepDecision(bottom, true))
			if !ok || len(ch) != 1 {
				t.Fatalf("hand %d %s: gorge answer %+v ok=%v", h, name, ch, ok)
			}
			gorgeKeep := ch[0] == 0
			var kernelKeep bool
			switch name {
			case "lands":
				kernelKeep = mull.KeepLands(landN, len(facts), bottom)
			case "never":
				kernelKeep = mull.KeepNever()
			case "constructed":
				kernelKeep = mull.KeepConstructed(facts, bottom, sh[0], sh[1])
			}
			nAsks++
			if gorgeKeep != kernelKeep {
				t.Errorf("hand %d (%d lands, bottom %d, deck %v) %s: gorge keep=%v, dplay keep=%v",
					h, lands, bottom, sh, name, gorgeKeep, kernelKeep)
			}
		}

		// coin: compared by its draw sequence. Both sides get the same
		// drawn lot v: gorge's default arm through a stub rng whose
		// IntN(3) is v, dplay's port through CoinMulls(v). The stub's
		// draw count pins the one-draw contract (and none when no
		// mulligan is offered).
		for v := 0; v < 3; v++ {
			b := gorgeBoard(botpolicy.MulliganCoin, hand, sh[0], sh[1])
			src := &mullSrc{v: v}
			in := botpolicy.Decide(b, keepDecision(bottom, true), rand.New(src))
			if len(in.Choices) != 1 {
				t.Fatalf("hand %d coin v=%d: choices %+v", h, v, in.Choices)
			}
			gorgeMull := in.Choices[0] == 1
			if src.n != 1 {
				t.Errorf("hand %d coin v=%d: gorge drew %d times, want 1", h, v, src.n)
			}
			if gorgeMull != mull.CoinMulls(v) {
				t.Errorf("hand %d coin v=%d: gorge mull=%v, dplay mull=%v", h, v, gorgeMull, mull.CoinMulls(v))
			}
			nAsks++
			// no mulligan option offered: keep, and no draw in either engine
			src2 := &mullSrc{v: v}
			in2 := botpolicy.Decide(b, keepDecision(bottom, false), rand.New(src2))
			if len(in2.Choices) != 1 || in2.Choices[0] != 0 {
				t.Fatalf("hand %d coin v=%d no-mull ask: choices %+v", h, v, in2.Choices)
			}
			if src2.n != 0 {
				t.Errorf("hand %d coin v=%d no-mull ask: gorge drew %d times, want 0", h, v, src2.n)
			}
		}

		// -- the bottoming ask, per policy --------------------------------
		var wk mull.Work
		for _, tc := range []struct {
			name   string
			kernel func() []int
		}{
			{"lands", func() []int { return mull.BottomToLands(facts, bottom, &wk) }},
			{"constructed", func() []int { return mull.BottomToLands(facts, bottom, &wk) }},
			{"never", func() []int { return mull.BottomCheapest(facts, bottom, &wk) }},
		} {
			rule, err := botpolicy.MulliganByName(tc.name)
			if err != nil {
				t.Fatal(err)
			}
			b := gorgeBoard(rule, hand, sh[0], sh[1])
			ch, ok := botpolicy.MulliganAnswer(b, rule, bottomDecision(bottom))
			if !ok {
				t.Fatalf("hand %d %s bottom: ok=false", h, tc.name)
			}
			nBottoms++
			got := tc.kernel()
			if !slices.Equal(ch, got) {
				t.Errorf("hand %d (%d lands, bottom %d, deck %v) %s bottoming: gorge %v, dplay %v",
					h, lands, bottom, sh, tc.name, ch, got)
			}
		}
		// coin's bottoming is the default arm's chooseDiscard, no rng
		{
			b := gorgeBoard(botpolicy.MulliganCoin, hand, sh[0], sh[1])
			src := &mullSrc{}
			in := botpolicy.Decide(b, bottomDecision(bottom), rand.New(src))
			if src.n != 0 {
				t.Errorf("hand %d coin bottoming: gorge drew %d times, want 0", h, src.n)
			}
			nBottoms++
			got := mull.BottomCheapest(facts, bottom, &wk)
			if !slices.Equal(in.Choices, got) {
				t.Errorf("hand %d (%d lands, bottom %d) coin bottoming: gorge %v, dplay %v",
					h, lands, bottom, in.Choices, got)
			}
		}
	}

	for l := 0; l <= 7; l++ {
		if !seenLands[l] {
			t.Errorf("land count %d never generated", l)
		}
	}
	for b := 0; b <= 2; b++ {
		if !seenBottom[b] {
			t.Errorf("prior mulligan count %d never generated", b)
		}
	}
	t.Logf("lockstep: %d hands, %d keep/mull asks (x3 draws for coin), %d bottoming asks",
		nHands, nAsks, nBottoms)
}

// TestMulliganPoliciesAllocFree pins the play-loop contract: the policies
// write into caller scratch and allocate nothing, so a dplay run stays
// allocation-free with mulligans on. AllocsPerRun warms the closure first,
// so this measures only the policies, not test setup.
func TestMulliganPoliciesAllocFree(t *testing.T) {
	cardsettest.Require(t, "fdn")
	hand := []mull.Card{
		landFacts(0).f, landFacts(1).f, landFacts(2).f, landFacts(3).f,
		spellFacts(1, [5]int32{0, 0, 0, 0, 1}).f,
		spellFacts(2, [5]int32{1, 0, 0, 0, 0}).f,
		spellFacts(3, [5]int32{0, 1, 0, 0, 0}).f,
	}
	var wk mull.Work
	got := testing.AllocsPerRun(100, func() {
		mull.KeepConstructed(hand, 1, 60, 24)
		mull.KeepLands(4, len(hand), 1)
		mull.BottomToLands(hand, 1, &wk)
		mull.BottomCheapest(hand, 1, &wk)
	})
	if got != 0 {
		t.Fatalf("mulligan policies allocate %v times per round, want 0", got)
	}
}

// TestMulliganComboLandParity is the real-Combo-land regression. Bloodfell Caves
// is Produced$ Combo B R, so gorge's ManaProduction.Any makes it produce
// every colour; a constructed hand of Caves, Mountain, two other basics and
// three green spells therefore keeps, and after one mulligan bottoms the
// Mountain (the Caves covers the green pips). A CardOf that loses the Combo
// marker reads the Caves as B/R only: it mulligans the hand and bottoms the
// Caves.
func TestMulliganComboLandParity(t *testing.T) {
	cardsettest.Require(t, "fdn")
	green := spellFacts(1, [5]int32{0, 0, 0, 0, 1}) // CMC 2, one green pip
	hand := []factCard{
		comboLandFacts(), // 0: Bloodfell Caves
		mountainFacts(),  // 1: Mountain
		landFacts(0),     // 2: Plains
		landFacts(1),     // 3: Island
		green, green, green,
	}
	facts := make([]mull.Card, len(hand))
	for i := range hand {
		facts[i] = hand[i].f
	}
	rule, err := botpolicy.MulliganByName("constructed")
	if err != nil {
		t.Fatal(err)
	}
	const deckTotal, deckLands = 60, 24
	for _, bottom := range []int{0, 1, 2} {
		b := gorgeBoard(rule, hand, deckTotal, deckLands)
		ch, ok := botpolicy.MulliganAnswer(b, rule, keepDecision(bottom, true))
		if !ok || len(ch) != 1 {
			t.Fatalf("bottom %d: gorge keep answer %+v ok=%v", bottom, ch, ok)
		}
		gorgeKeep := ch[0] == 0
		kernelKeep := mull.KeepConstructed(facts, bottom, deckTotal, deckLands)
		if gorgeKeep != kernelKeep {
			t.Errorf("bottom %d: gorge keep=%v, dplay keep=%v", bottom, gorgeKeep, kernelKeep)
		}
		if !kernelKeep {
			t.Errorf("bottom %d: the Caves must cover the green pips and keep", bottom)
		}
	}
	b := gorgeBoard(rule, hand, deckTotal, deckLands)
	ch, ok := botpolicy.MulliganAnswer(b, rule, bottomDecision(1))
	if !ok {
		t.Fatal("gorge constructed bottoming: ok=false")
	}
	var wk mull.Work
	want := mull.BottomToLands(facts, 1, &wk)
	if !slices.Equal(ch, want) {
		t.Errorf("bottoming: gorge %v, dplay %v", ch, want)
	}
	if !slices.Equal(want, []int{1}) {
		t.Errorf("dplay bottoms %v, want the Mountain (index 1)", want)
	}
}

// TestMulliganCardOfVanillaFacts pins mull.CardOf on the vanilla and the fdn
// table's records: the facts the dplay worker reads are the land test
// gorge's isLandCard makes and the packed cost's pips. The colour coverage
// is the "any" contract: a basic and a {C}-only land must NOT read as any
// producers, while a Produced$ Combo dual (Bloodfell Caves) must -- gorge's
// ManaProduction.Any is set for every Combo prefix, and producesColour then
// counts all five colours. Its Produces stays B/R: the choice marker adds
// the colours the card could make, not simultaneous production.
func TestMulliganCardOfVanillaFacts(t *testing.T) {
	plains := mull.CardOf(vanilla.Table.Cards[vanilla.Plains])
	if !plains.Land || !plains.Prod || !plains.Produces[0] || plains.ProdAny ||
		plains.CMC != 0 || plains.Pips != [5]int32{} {
		t.Fatalf("plains facts %+v", plains)
	}
	set := cardsettest.Set(t, "fdn")
	if i, ok := set.Lookup("Bloodfell Caves"); !ok {
		t.Fatal("no Bloodfell Caves in the fdn pool")
	} else {
		caves := mull.CardOf(*set.Def(i))
		if !caves.Land || !caves.Prod || !caves.ProdAny ||
			caves.Produces != [5]bool{false, false, true, true, false} {
			t.Fatalf("bloodfell caves facts %+v", caves)
		}
	}
	if i, ok := set.Lookup("Rogue's Passage"); !ok {
		t.Fatal("no Rogue's Passage in the fdn pool")
	} else {
		pass := mull.CardOf(*set.Def(i))
		if !pass.Land || !pass.Prod || pass.ProdAny || pass.Produces != [5]bool{} {
			t.Fatalf("rogue's passage facts %+v", pass)
		}
	}
	bears := mull.CardOf(vanilla.Table.Cards[vanilla.GrizzlyBears])
	if bears.Land || bears.Prod || bears.Produces != [5]bool{} ||
		bears.CMC != 2 || bears.Pips != [5]int32{0, 0, 0, 0, 1} {
		t.Fatalf("grizzly bears facts %+v", bears)
	}
	shock := mull.CardOf(vanilla.Table.Cards[vanilla.Shock])
	if shock.Land || shock.CMC != 1 || shock.Pips != [5]int32{0, 0, 0, 1, 0} {
		t.Fatalf("shock facts %+v", shock)
	}
}

// TestMulliganWorkerBottoming drives a real dplay London round through
// mulligans.answer: seat 0 runs the lands policy, mulligans until dplay
// poses its ChBottom round, and the picks mulligans serves, pick by pick,
// must be the policy's whole ascending set (mull.BottomToLands of the census
// taken at the round's first pick, before any hand card leaves). Serving the
// first m cards of option order instead -- planSet filled but never read --
// fails this on any hand whose set is not a prefix of the option order,
// which the 6-land deck's spell-heavy hands make the rule, not the
// exception.
func TestMulliganWorkerBottoming(t *testing.T) {
	tbl := vanilla.Table
	// 6 plains and the spell cycle Shock(1) / Grizzly Bears(2) / Serra
	// Angel(5): a seven usually holds 0-1 lands, so the lands policy
	// mulligans, and the highest-CMC spell it bottoms is rarely the hand's
	// first card in option order.
	deckA := make([]dirac.CardID, 0, 40)
	for i := 0; i < 6; i++ {
		deckA = append(deckA, vanilla.Plains)
	}
	for i := 0; i < 34; i++ {
		switch i % 3 {
		case 0:
			deckA = append(deckA, vanilla.Shock)
		case 1:
			deckA = append(deckA, vanilla.GrizzlyBears)
		case 2:
			deckA = append(deckA, vanilla.SerraAngel)
		}
	}
	var deckB []dirac.CardID
	for i := 0; i < 40; i++ {
		deckB = append(deckB, vanilla.Island)
	}

	const seed = 1
	var st dirac.State
	st.Init(tbl, deckA, deckB, seed, dirac.Config{Mulligans: 2})
	tblp := st.Table()
	m := &mulligans{}
	m.init("lands")
	m.newGame(seed, [2]deckShape{{len(deckA), 6}, {len(deckB), 40}})

	// hand census of player p in HAND order, the order gorge's policies
	// read (FeatZoneList; not the ObjID order a ChBottom ask is
	// canonicalised into).
	census := func(p uint8) (srcs []uint8, facts []mull.Card, lands int) {
		objs := st.FeatZoneList(p, kop.ZoneHand, nil)
		for _, ob := range objs {
			facts = append(facts, mull.CardOf(tblp.Cards[st.ObjCard(ob)]))
			srcs = append(srcs, uint8(ob))
			if facts[len(facts)-1].Land {
				lands++
			}
		}
		return srcs, facts, lands
	}

	mSeen, srcs, facts := 0, []uint8(nil), []mull.Card(nil)
	var served []uint8
	for {
		d := st.Step()
		switch {
		case d.Kind == dirac.DkMulligan:
			st.Apply(m.answer(&st, &d))
		case d.Kind == dirac.DkChoose && d.Sub == dirac.ChBottom:
			if d.Player == 0 && len(served) == 0 { // the round's first pick: the hand is whole
				mSeen = st.Mulligans(d.Player)
				srcs, facts, _ = census(d.Player)
				if len(facts) != mull.HandSize {
					t.Fatalf("hand of %d cards at the first pick", len(facts))
				}
			}
			a := m.answer(&st, &d)
			if d.Player == 0 { // seat 1 also plays the round; only seat 0 is under test
				served = append(served, uint8(a.Source()))
			}
			st.Apply(a)
		case d.Kind == dirac.DkPriority:
			goto round
		default:
			t.Fatalf("unexpected decision kind %v sub %v", d.Kind, d.Sub)
		}
	}
round:
	if mSeen == 0 {
		t.Fatal("seat 0 never mulliganed; pick a seed that reaches the bottoming round")
	}
	if n := len(served); n != mSeen {
		t.Fatalf("served %d picks, want %d", n, mSeen)
	}
	got := make([]int, 0, len(served))
	for _, pick := range served {
		found := false
		for i, src := range srcs {
			if src == pick {
				got, found = append(got, i), true
				break
			}
		}
		if !found {
			t.Fatalf("served source %d is not a hand card (served %v, hand sources %v)", pick, served, srcs)
		}
	}
	var wk mull.Work
	want := mull.BottomToLands(facts, mSeen, &wk)
	if !slices.Equal(got, want) {
		t.Fatalf("bottoming set served %v (sources %v), policy wanted %v; hand %v", got, served, want, facts)
	}
	t.Logf("bottoming: %d mulligans, %d picks, set %v of %d hand cards", mSeen, len(served), want, len(facts))
}
