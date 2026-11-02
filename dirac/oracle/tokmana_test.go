package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The granted mana ability lockstep (tokmana): constructed matchups over
// the generated tokmana pool, whose Continuous statics grant a mana
// ability to tokens (Springleaf Parade), creatures (Cryptolith Rite,
// Citanul Hierophants, Enduring Vitality), lands (Chromatic Lantern),
// Shamans (Sachi), Slivers of either player
// (Gemhide Sliver, Manaweft Sliver), nonland permanents (A Realm Reborn)
// and countered creatures (Jiang Yanggu), beside mana creatures that then
// carry a printed and a granted ability, and off-colour spells only the
// granted mana pays. gorge's planner never witnesses a granted ability: a
// play its printed sources cannot pay is offered after a scripted prefix
// of granted activations (potential_script.go), which the kernel copies
// (dirac/mana_grantedoffers.go grantSearch); the expected answer at
// every decision is gorge's.
var ktokmanaDecks = [2][]cardset.Entry{
	{
		{Name: "Springleaf Parade", Count: 3}, {Name: "Cryptolith Rite", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Citanul Hierophants", Count: 2}, {Name: "Raise the Alarm", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Divination", Count: 2}, {Name: "Lightning Strike", Count: 2},
		{Name: "Counterspell", Count: 1}, {Name: "Murder", Count: 1}, {Name: "Air Elemental", Count: 1},
		{Name: "Forest", Count: 12}, {Name: "Plains", Count: 2}, {Name: "Island", Count: 2},
	},
	{
		{Name: "Chromatic Lantern", Count: 3}, {Name: "Joiner Adept", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Elvish Mystic", Count: 3}, {Name: "Elvish Visionary", Count: 2}, {Name: "Wellwisher", Count: 1},
		{Name: "Gemhide Sliver", Count: 2}, {Name: "Manaweft Sliver", Count: 2}, {Name: "Metallic Sliver", Count: 2},
		{Name: "Sachi, Daughter of Seshiro", Count: 1}, {Name: "Jiang Yanggu, Wildcrafter", Count: 1},
		{Name: "Enduring Vitality", Count: 1}, {Name: "A Realm Reborn", Count: 1}, {Name: "Savannah Lions", Count: 2},
		{Name: "Shock", Count: 2}, {Name: "Mind Rot", Count: 1}, {Name: "Serra Angel", Count: 1},
		{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 2}, {Name: "Island", Count: 2},
	},
}

// ktokmanaCarriers are the pool's off-colour spells: casts only a granted
// colour pays on these decks (deck a has no red source but its tokens and
// creatures under Springleaf Parade / Cryptolith Rite; deck b no red at all).
var ktokmanaCarriers = map[string]bool{"Divination": true, "Lightning Strike": true, "Counterspell": true, "Murder": true, "Air Elemental": true, "Shock": true, "Mind Rot": true, "Serra Angel": true}

func TestTokmanaLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tokmana")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ktokmanaDecks {
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
	tbl, _, _, err := set.Matchup(ktokmanaDecks[0], ktokmanaDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(192) // ~40 s on 4 vCPU (the 1-minute per-test budget); KTOKMANA_SEEDS=512 ran clean
	if s := os.Getenv("KTOKMANA_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	var inst []string
	for _, d := range decks {
		for _, c := range d {
			inst = append(inst, cardName(c))
		}
	}
	var surf [2]map[string]int
	surf[0], surf[1] = map[string]int{}, map[string]int{}
	var scripted [2]int
	first := uint64(1)
	if s := os.Getenv("KTOKMANA_SEED"); s != "" { // one seed, for a repro
		first, seeds = 0, 0
		for _, ch := range s {
			first = first*10 + uint64(ch-'0')
		}
		seeds = first
	}
	for seed := first; seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"ktokmana-a", "ktokmana-b"}, decks, 1)
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
		scripted[si] += res.Stats.Scripted
		for _, a := range res.Actions {
			x := int(a.Src())
			if x >= len(inst) || a.Kind() != ActCast || !ktokmanaCarriers[inst[x]] {
				continue
			}
			surf[si][inst[x]]++
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
	t.Logf("%d seeds; off-colour casts: lowering surface %v, autopay surface %v; plays paid after a scripted prefix (gorge Stats.Scripted): lowering %d, autopay %d",
		seeds, surf[0], surf[1], scripted[0], scripted[1])
	// the scripted prefix is a granted source's activation on these decks
	// (no other census gap): gorge paid at least this many plays with
	// granted mana, and the kernel offered and paid every one of them
	if seeds >= 192 && scripted[0] < 100 {
		t.Errorf("lowering surface: %d plays paid after a scripted prefix (want >= 100)", scripted[0])
	}
	for _, n := range []string{"Divination", "Lightning Strike"} {
		if seeds >= 192 && surf[0][n] < 20 {
			t.Errorf("%s: %d casts on the lowering surface (want >= 20)", n, surf[0][n])
		}
	}
}
