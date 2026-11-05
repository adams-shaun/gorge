package main

import (
	"fmt"
	"slices"
	"testing"

	botpolicy "github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/mull"
	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// forgeLand builds the same land facts on both sides: a basic with one
// coloured production, or a non-producing land (Evolving Wilds).
func forgeLand(name string, colour int, basic, produces bool) factCard {
	f := mull.Card{PrintedName: name, Land: true, Basic: basic}
	var mp cards.ManaProduction
	if produces {
		f.Prod = true
		f.Produces[colour] = true
		mp.Colour[colour] = 1
	}
	return factCard{f: f, g: botpolicy.Card{PrintedName: name, Basic: basic,
		ManaCost: "no cost", Castable: true, Produces: mp}}
}

func forgePlains() factCard { return forgeLand("Plains", 0, true, true) }
func forgeSwamp() factCard  { return forgeLand("Swamp", 2, true, true) }
func forgeWilds() factCard  { return forgeLand("Evolving Wilds", 0, false, false) }

func forgeCreature(name string, cmc, power int32) factCard {
	f := mull.Card{PrintedName: name, Creature: true, Power: power, CMC: cmc}
	g := botpolicy.Card{PrintedName: name, Creature: true, Power: power, CMC: cmc,
		ManaCost: fmt.Sprint(cmc), Castable: true}
	return factCard{f: f, g: g}
}

func forgeInstant(name string, cmc int32) factCard {
	f := mull.Card{PrintedName: name, CMC: cmc}
	g := botpolicy.Card{PrintedName: name, InstantSpeed: true, CMC: cmc,
		ManaCost: fmt.Sprint(cmc), Castable: true}
	return factCard{f: f, g: g}
}

// forgeSeven is gorge's sevenWith: `lands` Plains and 7-lands 2/2 creatures.
func forgeSeven(lands int) []factCard {
	hand := make([]factCard, 0, mull.HandSize)
	for i := 0; i < mull.HandSize; i++ {
		if i < lands {
			hand = append(hand, forgePlains())
		} else {
			hand = append(hand, forgeCreature("Bear", 2, 2))
		}
	}
	return hand
}

func forgeFacts(hand []factCard) []mull.Card {
	facts := make([]mull.Card, len(hand))
	for i := range hand {
		facts[i] = hand[i].f
	}
	return facts
}

// TestKeepForgeMatchesGorge copies gorge's mulligan_forge_test.go keep cases
// onto dplay's facts: every case must get the same keep/mulligan answer
// from mull.KeepForge as from botpolicy's forge policy on the same hand,
// mulligan count and deck census.
func TestKeepForgeMatchesGorge(t *testing.T) {
	rule, err := botpolicy.MulliganByName("forge")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		name                 string
		hand                 []factCard
		deckTotal, deckLands int
		bottom, handSize     int
	}
	add := func(name string, hand []factCard, deckTotal, deckLands, bottom int) {
		cases = append(cases, struct {
			name                 string
			hand                 []factCard
			deckTotal, deckLands int
			bottom, handSize     int
		}{name: name, hand: hand, deckTotal: deckTotal, deckLands: deckLands,
			bottom: bottom, handSize: len(hand)})
	}
	for lands := 0; lands <= 7; lands++ {
		add(fmt.Sprintf("opening seven, %d lands", lands), forgeSeven(lands), 60, 24, 0)
	}
	flood := append([]factCard{forgePlains(), forgePlains(), forgePlains(),
		forgePlains(), forgePlains(), forgePlains()}, forgeCreature("Bear", 2, 2))
	add("six lands and a spell after one mulligan", flood, 60, 24, 1)
	add("seven lands after one mulligan", forgeSeven(7), 60, 24, 1)
	add("no-land seven after one mulligan", forgeSeven(0), 60, 24, 1)
	for _, lands := range []int{0, 7} {
		add(fmt.Sprintf("%d lands after two mulligans", lands), forgeSeven(lands), 60, 24, 2)
	}
	add("one land in an 8/60 deck", forgeSeven(1), 60, 8, 0)
	add("one land in a 10/60 deck", forgeSeven(1), 60, 10, 0)
	for _, lands := range []int{7, 6} {
		add(fmt.Sprintf("%d lands in a 40/60 deck", lands), forgeSeven(lands), 60, 40, 0)
	}
	add("no-land deck", forgeSeven(0), 60, 0, 0)
	add("ignores colour", []factCard{forgeSwamp(), forgeSwamp(), forgeSwamp(),
		forgeCreature("Bear", 2, 2), forgeCreature("Bear", 2, 2),
		forgeCreature("Bear", 2, 2), forgeCreature("Bear", 2, 2)}, 60, 24, 0)
	add("nil deck, one land", forgeSeven(1), 0, 0, 0)
	add("nil deck, three lands", forgeSeven(3), 0, 0, 0)
	add("hand order: one land first plus six free artifacts",
		[]factCard{forgePlains(), forgeInstant("Free 1", 0), forgeInstant("Free 2", 0),
			forgeInstant("Free 3", 0), forgeInstant("Free 4", 0),
			forgeInstant("Free 5", 0), forgeInstant("Free 6", 0)}, 60, 8, 1)
	// gorge's census-mismatch case (Board.HandSize != 7) has no dplay
	// analogue: the dplay hand is always HandSize while the seat decides.

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := forgeFacts(tc.hand)
			b := gorgeBoard(rule, tc.hand, tc.deckTotal, tc.deckLands)
			b.HandSize = int32(tc.handSize)
			ch, ok := botpolicy.MulliganAnswer(b, rule, keepDecision(tc.bottom, true))
			if !ok || len(ch) != 1 {
				t.Fatalf("gorge answer %+v ok=%v", ch, ok)
			}
			gorgeKeep := ch[0] == 0
			var wk mull.Work
			kernelKeep := mull.KeepForge(facts, tc.bottom, tc.deckTotal, tc.deckLands, &wk)
			if gorgeKeep != kernelKeep {
				t.Fatalf("gorge keep=%v, dplay keep=%v (hand %v)", gorgeKeep, kernelKeep, facts)
			}
		})
	}
}

// TestBottomForgeMatchesGorge copies gorge's mulligan_forge_test.go bottoming
// cases: the ascending pick set must match botpolicy's forge policy exactly.
func TestBottomForgeMatchesGorge(t *testing.T) {
	rule, err := botpolicy.MulliganByName("forge")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		name string
		hand []factCard
		m    int
	}
	add := func(name string, hand []factCard, m int) {
		cases = append(cases, struct {
			name string
			hand []factCard
			m    int
		}{name, hand, m})
	}
	add("non-producing land first", []factCard{forgePlains(), forgePlains(), forgePlains(),
		forgeWilds(), forgeCreature("Bear", 2, 2), forgeCreature("Bear", 2, 2), forgeCreature("Bear", 2, 2)}, 1)
	add("basic with most copies", []factCard{forgePlains(), forgeSwamp(), forgeSwamp(), forgeSwamp(),
		forgeCreature("Bear", 2, 2), forgeCreature("Bear", 2, 2), forgeCreature("Bear", 2, 2)}, 1)
	add("uncastable spell", []factCard{forgePlains(), forgePlains(),
		forgeCreature("Bear", 2, 2), forgeCreature("Bear", 5, 2), forgeCreature("Bear", 4, 2),
		forgeCreature("Bear", 1, 2), forgeCreature("Bear", 3, 2)}, 2)
	add("double-count quirk flood", []factCard{forgePlains(), forgePlains(), forgePlains(), forgePlains(),
		forgeCreature("Bear", 2, 2), forgeCreature("Bear", 2, 2), forgeCreature("Bear", 2, 2)}, 2)
	add("worst creature", []factCard{forgePlains(), forgePlains(), forgePlains(),
		forgeCreature("One", 1, 1), forgeCreature("Three", 2, 3),
		forgeCreature("Two", 3, 2), forgeCreature("Two", 2, 2)}, 1)
	add("cheapest is last land", []factCard{forgePlains(), forgePlains(), forgePlains(),
		forgeInstant("One", 1), forgeInstant("Two A", 2), forgeInstant("Two B", 2), forgeInstant("Three", 3)}, 1)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := forgeFacts(tc.hand)
			b := gorgeBoard(rule, tc.hand, 60, 24)
			ch, ok := botpolicy.MulliganAnswer(b, rule, bottomDecision(tc.m))
			if !ok {
				t.Fatal("gorge bottoming: ok=false")
			}
			var wk mull.Work
			got := mull.BottomForge(facts, tc.m, &wk)
			if !slices.Equal(ch, got) {
				t.Fatalf("gorge %v, dplay %v (hand %v)", ch, got, facts)
			}
		})
	}
}

// TestForgeMulliganZeroAlloc pins the play-loop contract: KeepForge and
// BottomForge write only into the caller's Work, so a dplay run stays
// allocation-free with the forge policy.
func TestForgeMulliganZeroAlloc(t *testing.T) {
	hand := []mull.Card{
		forgePlains().f, forgeSwamp().f, forgeWilds().f, forgePlains().f,
		forgeCreature("Bear", 2, 2).f, forgeInstant("Shock", 1).f, forgeCreature("Angel", 5, 4).f,
	}
	var wk mull.Work
	got := testing.AllocsPerRun(100, func() {
		mull.KeepForge(hand, 1, 60, 24, &wk)
		mull.BottomForge(hand, 2, &wk)
	})
	if got != 0 {
		t.Fatalf("forge mulligan allocates %v times per round, want 0", got)
	}
}

// gorgeBoardFacts builds gorge's Board for a hand of dplay mull facts, in
// the order given (hand order), keyed ObjIDs 1..n. The forge policy reads
// the table in insertion order (TableOf sorts by key), so ascending ObjIDs
// are hand order, exactly as the engine's bottoming options are.
func gorgeBoardFacts(rule botpolicy.MulliganRule, facts []mull.Card) botpolicy.Board {
	m := map[state.ObjID]botpolicy.Card{}
	for i, f := range facts {
		c := botpolicy.Card{PrintedName: f.PrintedName, Basic: f.Basic,
			Creature: f.Creature, Power: f.Power, CMC: f.CMC, Castable: true}
		if f.Land {
			c.ManaCost = "no cost"
		} else {
			c.ManaCost = fmt.Sprint(f.CMC)
		}
		m[state.ObjID(i+1)] = c
	}
	return botpolicy.Board{Mulligan: rule, Cards: botpolicy.TableOf(m), HandSize: int32(len(facts))}
}

// forgeWorkerRound is one seed's forge bottoming round: the mulligan count,
// the picks mulligans served, and the hand's facts/objects in both hand
// order (the policy's census) and dplay option order (d.Options).
type forgeWorkerRound struct {
	m                   int
	served              []uint8
	handObjs, optObjs   []uint8
	handFacts, optFacts []mull.Card
}

// probeForgeSeed drives one dplay London round with seat 0 on the forge
// policy and returns its bottoming round, if seat 0 mulliganed.
func probeForgeSeed(t *testing.T, deckA, deckB []dirac.CardID, seed uint64) (forgeWorkerRound, bool) {
	m := &mulligans{}
	m.init("forge")
	m.newGame(seed, [2]deckShape{})
	var st dirac.State
	st.Init(vanilla.Table, deckA, deckB, seed, dirac.Config{Mulligans: 2})
	tblp := st.Table()
	var r forgeWorkerRound
	for {
		d := st.Step()
		switch {
		case d.Kind == dirac.DkMulligan:
			if d.Player != 0 {
				// seat 1 keeps its seven and moves on
				st.Apply(d.Options[0])
				continue
			}
			st.Apply(m.answer(&st, &d))
		case d.Kind == dirac.DkChoose && d.Sub == dirac.ChBottom:
			if d.Player != 0 {
				return r, false
			}
			if len(r.served) == 0 { // the round's first pick: the hand is whole
				r.m = int(st.Mulligans(d.Player))
				for _, ob := range st.FeatZoneList(d.Player, kop.ZoneHand, nil) {
					r.handObjs = append(r.handObjs, uint8(ob))
					r.handFacts = append(r.handFacts, mull.CardOf(tblp.Cards[st.ObjCard(ob)]))
				}
				if len(r.handFacts) != mull.HandSize {
					t.Fatalf("seed %d: hand of %d cards at the first pick", seed, len(r.handFacts))
				}
				for i := 0; i < int(d.N); i++ {
					ob := dirac.Obj(d.Options[i].Source())
					r.optObjs = append(r.optObjs, uint8(ob))
					r.optFacts = append(r.optFacts, mull.CardOf(tblp.Cards[st.ObjCard(ob)]))
				}
			}
			a := m.answer(&st, &d)
			r.served = append(r.served, uint8(a.Source()))
			st.Apply(a)
		case d.Kind == dirac.DkPriority:
			return r, r.m > 0
		default:
			t.Fatalf("seed %d: unexpected decision kind %v sub %v", seed, d.Kind, d.Sub)
		}
	}
}

// TestMulliganWorkerForgeBottoming drives a real dplay London round through
// mulligans.answer with the forge policy: the picks served must be the
// hand-order BottomForge set (and gorge's answer), not the same policy run
// on d.Options' ObjID order. The deck is land-heavy so a seven often holds
// six or seven lands, which forge mulligans and then bottoms a land; with
// every land a Plains the worst-land tie-break is purely hand order, so a
// seat that serves option order picks a different object.
func TestMulliganWorkerForgeBottoming(t *testing.T) {
	deckA := make([]dirac.CardID, 0, 40)
	for i := 0; i < 20; i++ {
		deckA = append(deckA, vanilla.Plains)
	}
	for i := 0; i < 20; i++ {
		deckA = append(deckA, vanilla.GrizzlyBears)
	}
	deckB := make([]dirac.CardID, 0, 40)
	for i := 0; i < 40; i++ {
		deckB = append(deckB, vanilla.Island)
	}

	const maxSeed = 2000
	var probe forgeWorkerRound
	found := false
	for seed := uint64(1); seed <= maxSeed && !found; seed++ {
		p, ok := probeForgeSeed(t, deckA, deckB, seed)
		if !ok {
			continue
		}
		var wk mull.Work
		wantPos := mull.BottomForge(p.handFacts, p.m, &wk)
		wantObjs := make([]uint8, len(wantPos))
		for i, x := range wantPos {
			wantObjs[i] = p.handObjs[x]
		}
		optPos := mull.BottomForge(p.optFacts, p.m, &wk)
		optObjs := make([]uint8, len(optPos))
		for i, x := range optPos {
			optObjs[i] = p.optObjs[x]
		}
		if slices.Equal(wantObjs, optObjs) {
			continue // this hand's answer is order-insensitive
		}
		probe, found = p, true
	}
	if !found {
		t.Fatalf("no order-sensitive forge bottoming round in seeds 1..%d", maxSeed)
	}

	var wk mull.Work
	wantPos := mull.BottomForge(probe.handFacts, probe.m, &wk)
	wantObjs := make([]uint8, len(wantPos))
	for i, x := range wantPos {
		wantObjs[i] = probe.handObjs[x]
	}
	if !slices.Equal(probe.served, wantObjs) {
		t.Fatalf("served %v, hand-order BottomForge wants %v (hand objs %v)",
			probe.served, wantObjs, probe.handObjs)
	}

	rule, err := botpolicy.MulliganByName("forge")
	if err != nil {
		t.Fatal(err)
	}
	b := gorgeBoardFacts(rule, probe.handFacts)
	ch, ok := botpolicy.MulliganAnswer(b, rule, bottomDecision(probe.m))
	if !ok {
		t.Fatal("gorge forge bottoming: ok=false")
	}
	gorgeObjs := make([]uint8, len(ch))
	for i, x := range ch {
		gorgeObjs[i] = probe.handObjs[x]
	}
	if !slices.Equal(probe.served, gorgeObjs) {
		t.Fatalf("served %v, gorge wants %v (hand objs %v, gorge indices %v)",
			probe.served, gorgeObjs, probe.handObjs, ch)
	}
	t.Logf("forge bottoming: %d mulligans, served %v, hand order %v",
		probe.m, probe.served, probe.handObjs)
}
