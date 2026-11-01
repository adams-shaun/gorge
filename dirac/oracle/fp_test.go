package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The fp lockstep (the filter-props family of the
// MageZero sample decks): constructed matchups over the generated fp pool.
// The carriers: Shoot the Sheriff (Creature.!Outlaw), Guardian of the
// Guildpact (protection from Card.MonoColor), Preacher of the Schism
// (Attacked$ Player.withMostLife, CheckDefinedPlayer$ You.withMostLife) and
// Ruin-Lurker Bat (ValidPlayer$ You.descended), against outlaws, a
// changeling, a colourless changeling, mono-red burn and sacrifice outlets.
var kfpDecks = [2][]cardset.Entry{
	{
		{Name: "Shoot the Sheriff", Count: 4}, {Name: "Preacher of the Schism", Count: 4}, {Name: "Ruin-Lurker Bat", Count: 4},
		{Name: "Guardian of the Guildpact", Count: 3}, {Name: "Village Rites", Count: 3}, {Name: "Night's Whisper", Count: 2},
		{Name: "Doom Blade", Count: 2}, {Name: "Savannah Lions", Count: 3}, {Name: "Vampire Interloper", Count: 2},
		{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Highway Robber", Count: 3}, {Name: "Changeling Outcast", Count: 3}, {Name: "Universal Automaton", Count: 3},
		{Name: "Raging Goblin", Count: 4}, {Name: "Goblin Arsonist", Count: 3},
		{Name: "Shock", Count: 3}, {Name: "Lightning Strike", Count: 2}, {Name: "Doom Blade", Count: 2},
		{Name: "Thrill of Possibility", Count: 2}, {Name: "Mind Rot", Count: 2}, {Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 6},
	},
}

// kfpCarriers: the canonical carrier action per card (a cast -1, an
// attack -2), the least count asserted on the lowering surface.
var kfpCarriers = map[string]struct{ act, min int }{
	"Shoot the Sheriff": {-1, 100}, "Preacher of the Schism": {-2, 100}, "Ruin-Lurker Bat": {-1, 100},
	"Guardian of the Guildpact": {-1, 50},
}

// kfpHitMin: the least count of each lifted predicate branch over both
// surfaces together.
var kfpHitMin = map[int]int{
	dirac.KvfOutlawNo: 50, dirac.KvfOutlawYes: 50, dirac.KvfMonoNo: 20, dirac.KvfMonoYes: 50,
}

// TestFilterPropLockstep sets dirac.DebugValueHeadHits (a package global), so it runs
// isolated in a child process.
func TestFilterPropLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("fp")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kfpDecks {
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
	tbl, _, _, err := set.Matchup(kfpDecks[0], kfpDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(512)
	if s := os.Getenv("KFP_SEEDS"); s != "" {
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
		m, err := NewMatchup(reg, [2]string{"kfp-a", "kfp-b"}, decks, 1)
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
			c, ok := kfpCarriers[inst[x]]
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
	for n, c := range kfpCarriers {
		if surf[0][n] < c.min {
			t.Errorf("%s: %d occurrences on the lowering surface (want >= %d)", n, surf[0][n], c.min)
		}
	}
	for i, want := range kfpHitMin {
		if int(hits[i]) < want {
			t.Errorf("branch %d: %d hits (want >= %d)", i, hits[i], want)
		}
	}
}
