package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The vh lockstep (the value-heads family of the MageZero
// sample decks): constructed matchups over the generated vh pool, so the
// expected answer at every decision is gorge's. The carriers: Galvanic
// Blast (Count$Metalcraft.4.2), Anoint with Affliction
// (TargetedController$Counters.Poison), Witchstalker Frenzy
// (PlayerCountPlayers$AttackersDeclared in a ReduceCost), Lunar
// Convocation (PlayerCountPropertyYou$LifeLostThisTurn/Times.X), Refurbished
// Familiar (PlayerCountOpponents$Amount/Minus.Remembered$Amount) and
// Unstoppable Slasher (TriggeredTarget$LifeTotal/HalfUp, Card.Self+
// !HasCounters), beside artifacts, poison makers, life gain and discard.
var kvhDecks = [2][]cardset.Entry{
	{
		{Name: "Galvanic Blast", Count: 4}, {Name: "Refurbished Familiar", Count: 3}, {Name: "Anoint with Affliction", Count: 3},
		{Name: "Unstoppable Slasher", Count: 3}, {Name: "Witchstalker Frenzy", Count: 3}, {Name: "Ornithopter", Count: 3},
		{Name: "Memnite", Count: 3}, {Name: "Frogmite", Count: 2}, {Name: "Ichor Rats", Count: 2}, {Name: "Plague Stinger", Count: 2},
		{Name: "Darksteel Citadel", Count: 2}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Lunar Convocation", Count: 4}, {Name: "Ajani's Pridemate", Count: 3}, {Name: "Healer's Hawk", Count: 3},
		{Name: "Soul Warden", Count: 3}, {Name: "Vampire Nighthawk", Count: 2}, {Name: "Inspiring Overseer", Count: 2},
		{Name: "Savannah Lions", Count: 3}, {Name: "Duress", Count: 2}, {Name: "Mind Rot", Count: 2}, {Name: "Leonin Skyhunter", Count: 2},
		{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 7},
	},
}

// kvhCarriers: the canonical carrier action per card (a cast -1, an
// activation the ability index, an attack -2), the least count asserted on
// the lowering surface.
var kvhCarriers = map[string]struct{ act, min int }{
	"Galvanic Blast": {-1, 200}, "Refurbished Familiar": {-1, 100}, "Anoint with Affliction": {-1, 100},
	"Unstoppable Slasher": {-2, 100}, "Witchstalker Frenzy": {-1, 50}, "Lunar Convocation": {-1, 100},
}

// kvhHitMin: the least count of each lifted branch over the lowering and
// autopay surfaces together.
var kvhHitMin = map[int]int{
	dirac.KvfPoisonLow: 50, dirac.KvfPoisonHigh: 5, dirac.KvfProductZero: 50, dirac.KvfProductNonzero: 20,
	dirac.KvfAttackersNonzero: 20, dirac.KvfNoCountersTrue: 20, dirac.KvfNoCountersFalse: 3,
}

// TestValueHeadLockstep sets dirac.DebugValueHeadHits (a package global), so it does not
// run in parallel.
func TestValueHeadLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("vh")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kvhDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kvhDecks[0], kvhDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(512)
	if s := os.Getenv("KVH_SEEDS"); s != "" {
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		seeds = n
	}
	var inst []string
	for _, d := range decks {
		for _, c := range d {
			inst = append(inst, cardName(c))
		}
	}
	var hits [dirac.ValueHeadNumHits]uint32
	dirac.DebugValueHeadHits = &hits
	defer func() { dirac.DebugValueHeadHits = nil }()
	var surf [2]map[string]int
	surf[0], surf[1] = map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kvh-a", "kvh-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		si := 0
		if seed%4 == 0 {
			cfg, si = Config{GorgeAutopaySurface: true}, 1
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		for _, a := range res.Actions {
			x := int(a.Src())
			if x >= len(inst) {
				continue
			}
			c, ok := kvhCarriers[inst[x]]
			if !ok {
				continue
			}
			switch k := a.Kind(); {
			case c.act == -1 && k == ActCast, c.act == -2 && k == ActAttack, c.act >= 0 && k == ActActivate && int(a.Mode()) == c.act:
				surf[si][inst[x]]++
			}
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d seeds; carrier occurrences: lowering surface %v, autopay surface %v; branch hits %v", seeds, surf[0], surf[1], hits)
	if seeds < 512 {
		return // a short run reports the counts without asserting them
	}
	for n, c := range kvhCarriers {
		if surf[0][n] < c.min {
			t.Errorf("%s: %d occurrences on the lowering surface (want >= %d)", n, surf[0][n], c.min)
		}
	}
	for i, want := range kvhHitMin {
		if int(hits[i]) < want {
			t.Errorf("branch %d: %d hits (want >= %d)", i, hits[i], want)
		}
	}
}
