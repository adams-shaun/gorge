package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The painland lockstep: the ten painlands' "{T}: Add {C}" and
// "{T}: Add one of two colours. CARDNAME deals 1 damage to you", and Elves
// of Deep Shadow's pain-only "{T}: Add {B}" (gorge pay: the painful
// alternative is last resort, Consequence{Damage: 1}; phase 2 only, cost 3
// a point, never a lethal plan). Colour-hungry spells force the painful
// alternatives, burn lowers life into the lethal guard, a counterspell and
// Swords to Plowshares exercise instant-speed payments; Mana Leak's
// unless cost and Boros Recruit's hybrid pip take gorge's manual window, whose
// ability ask answers a painland's ability 0 ({C}) and taps Elves of Deep
// Shadow with its damage. The expected answer
// at every decision is gorge's.
var kpainDecks = [2][]cardset.Entry{
	{
		{Name: "Adarkar Wastes", Count: 4}, {Name: "Battlefield Forge", Count: 3}, {Name: "Shivan Reef", Count: 3},
		{Name: "Plains", Count: 2}, {Name: "Island", Count: 2}, {Name: "Mountain", Count: 2},
		{Name: "Savannah Lions", Count: 4}, {Name: "Wind Drake", Count: 4}, {Name: "Serra Angel", Count: 2},
		{Name: "Counterspell", Count: 1}, {Name: "Mana Leak", Count: 2}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Boros Recruit", Count: 2}, {Name: "Swords to Plowshares", Count: 1},
	},
	{
		{Name: "Llanowar Wastes", Count: 3}, {Name: "Karplusan Forest", Count: 3}, {Name: "Sulfurous Springs", Count: 3},
		{Name: "Underground River", Count: 2}, {Name: "Caves of Koilos", Count: 1}, {Name: "Yavimaya Coast", Count: 1},
		{Name: "Brushland", Count: 1}, {Name: "Swamp", Count: 2}, {Name: "Forest", Count: 1}, {Name: "Wastes", Count: 1},
		{Name: "Elves of Deep Shadow", Count: 4}, {Name: "Llanowar Elves", Count: 2}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Walking Corpse", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
	},
}

// painCarriers are the pool's pain sources (their damage is counted from
// gorge's life totals: every point of life a seat loses outside combat and
// burn is a painful activation).
var painCarriers = map[string]bool{"Adarkar Wastes": true, "Battlefield Forge": true, "Shivan Reef": true,
	"Llanowar Wastes": true, "Karplusan Forest": true, "Sulfurous Springs": true, "Underground River": true,
	"Caves of Koilos": true, "Yavimaya Coast": true, "Brushland": true, "Elves of Deep Shadow": true}

func TestPainLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("pain")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kpainDecks {
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
	tbl, _, _, err := set.Matchup(kpainDecks[0], kpainDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(1000)
	if s := os.Getenv("KPAIN_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	casts, clean := 0, 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kpain-a", "kpain-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
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
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate {
				casts++
			}
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			continue
		}
		clean++
	}
	t.Logf("%d seeds, %d clean; %d casts/activations", seeds, clean, casts)
}

// TestPainLockstepMutation proves the pool exercises the pain riders: with
// the kernel's damage dropped, some seed diverges from gorge.
func TestPainLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("pain")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kpainDecks {
		for _, e := range d {
			c, _ := reg.Lookup(e.Name)
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(kpainDecks[0], kpainDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugNoPainDamage = true
	defer func() { dirac.DebugNoPainDamage = false }()
	for seed := uint64(1); seed <= 20; seed++ {
		m, err := NewMatchup(reg, [2]string{"kpain-a", "kpain-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d diverges without the pain damage (decision %d: %s)", seed, res.Div.At, res.Div.What)
			return
		}
	}
	t.Fatal("no seed diverged with the kernel's pain damage dropped: the pool does not exercise it")
}
