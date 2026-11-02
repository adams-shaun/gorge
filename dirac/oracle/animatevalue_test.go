package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The value-driven Animate lockstep: constructed matchups over the generated
// animatevalue pool. Every carrier has an Animate or AnimateAll whose
// Power$/Toughness$ is a value (a hand size, a mana value, a count or a
// creature's P/T), read once at resolution. The frozen FDN and BLB pair
// pools exercise none of these arms. The X carriers (Chimeric Staff, Minsc,
// Mirror Entity, Testament of Faith) stay out of the constructed pool; their
// Power$/Toughness$ lowering (pt=X/X) is pinned by dirac/kgen's
// TestAnimateXValue.
var animatevalueDecks = [][2][]cardset.Entry{
	{
		{
			{Name: "Belligerent Yearling", Count: 2}, {Name: "Obuun, Mul Daya Ancestor", Count: 2},
			{Name: "Jolrael, Mwonvuli Recluse", Count: 2}, {Name: "Unruly Krasis", Count: 2},
			{Name: "Frenzied Raptor", Count: 2}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Wind Drake", Count: 4}, {Name: "Hill Giant", Count: 4},
			{Name: "Plains", Count: 4}, {Name: "Mountain", Count: 4},
			{Name: "Forest", Count: 6}, {Name: "Island", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
			{Name: "Walking Corpse", Count: 4}, {Name: "Wind Drake", Count: 4},
			{Name: "Frenzied Raptor", Count: 4},
			{Name: "Island", Count: 5}, {Name: "Swamp", Count: 5},
			{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
		},
	},
	{
		{
			{Name: "Karn's Touch", Count: 2}, {Name: "Karn, Silver Golem", Count: 2},
			{Name: "Sydri, Galvanic Genius", Count: 2}, {Name: "Toymaker", Count: 2},
			{Name: "Zur, Eternal Schemer", Count: 2}, {Name: "Riptide Mangler", Count: 2},
			{Name: "Eldrazi Mimic", Count: 2}, {Name: "Wall of Tombstones", Count: 2},
			{Name: "Ornithopter", Count: 2}, {Name: "Carrion Feeder", Count: 2},
			{Name: "Walking Corpse", Count: 4}, {Name: "Darksteel Relic", Count: 4},
			{Name: "Island", Count: 4}, {Name: "Swamp", Count: 5}, {Name: "Plains", Count: 3},
		},
		{
			{Name: "Walking Corpse", Count: 4}, {Name: "Wind Drake", Count: 4},
			{Name: "Ornithopter", Count: 4}, {Name: "Darksteel Relic", Count: 4},
			{Name: "Carrion Feeder", Count: 4},
			{Name: "Island", Count: 8}, {Name: "Swamp", Count: 12},
		},
	},
}

// animatevalueCarriers names the cards whose casts and activations the test
// counts (each must be played at least once).
var animatevalueCarriers = []string{
	"Belligerent Yearling", "Obuun, Mul Daya Ancestor",
	"Jolrael, Mwonvuli Recluse", "Unruly Krasis",
	"Karn's Touch", "Karn, Silver Golem",
	"Sydri, Galvanic Genius", "Toymaker", "Zur, Eternal Schemer",
	"Riptide Mangler", "Eldrazi Mimic", "Wall of Tombstones",
}

func TestAnimatevalueLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("animatevalue")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	for mi, md := range animatevalueDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 512; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"animatevalue-a", "animatevalue-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := eagerPolicy(seed, played)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				if res.Stall == KernelStallPrefix+"tables" {
					// S4: repeated value-driven Animate
					// activations on one permanent can reach the ninth
					// override record; the capacity stall is the loud end,
					// not a divergence.
					continue
				}
				t.Errorf("matchup %d seed %d: kernel stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, carrier plays %v", games, played)
	for _, n := range animatevalueCarriers {
		if played[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
}

// TestAnimatevalueMutation flips the debug switch that makes a value-driven
// override use 0/0 instead of its Power$/Toughness$ and requires the lockstep
// to diverge: the kernel's P/T no longer matches gorge's resolution-time
// value.
func TestAnimatevalueMutation(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("animatevalue")
	if err != nil {
		t.Fatal(err)
	}
	md := animatevalueDecks[0]
	var decks [2][]*cards.Card
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugIgnoreAnimateValue
	dirac.DebugIgnoreAnimateValue = true
	defer func() { dirac.DebugIgnoreAnimateValue = old }()
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"animatevalue-a", "animatevalue-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			return
		}
	}
	t.Fatal("debug switch did not diverge in 256 seeds")
}
