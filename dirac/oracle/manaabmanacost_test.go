package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The first-only filter lockstep: a filter as a card's
// first and only mana ability -- Abzan Devotee's "{1}: Add {W}, {B}, or
// {G}" and Three Tree Mascot's "{1}: Add one mana of any color", each once
// each turn; Prophetic Prism's and Giant's Boulder's "{1}, {T}: Add one
// mana of any color" -- in a W/B/G deck whose spells want B and G over
// mostly Plains. gorge's planner census prices none of them (a paid gap), so
// their mana is only ever reached by a scripted prefix (rules
// potentialPrefixScript): the kernel's filtSearch must find the same path
// (gap order, funding source, colour) at every play, and must not reuse a
// Devotee or a Mascot in the same turn. The expected answer at every
// decision is gorge's.
var kmanaabmanacostDecks = [2][]cardset.Entry{
	{
		{Name: "Abzan Devotee", Count: 4}, {Name: "Three Tree Mascot", Count: 4}, {Name: "Prophetic Prism", Count: 3},
		{Name: "Giant's Boulder", Count: 2}, {Name: "Grizzly Bears", Count: 4}, {Name: "Doom Blade", Count: 3},
		{Name: "Vampire Nighthawk", Count: 3}, {Name: "Skinrender", Count: 2}, {Name: "Serra Angel", Count: 2},
		{Name: "Savannah Lions", Count: 2},
		{Name: "Plains", Count: 9}, {Name: "Swamp", Count: 2},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Wind Drake", Count: 4}, {Name: "Pestermite", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

// kmanaabmanacostComboDecks play the four Devotees whose Produced$ Combo is
// printed out of WUBRG order (Jeskai "U R W", Mardu
// "R W B", Sultai "B G U", Temur "G U R") over mixed basics, against the
// plain blue-red creature deck: the kernel's colour ask must try the colours
// in gorge's printed order (kop.Ability.ManaColOrd).
var kmanaabmanacostComboDecks = [2][]cardset.Entry{
	{
		{Name: "Jeskai Devotee", Count: 3}, {Name: "Mardu Devotee", Count: 3}, {Name: "Sultai Devotee", Count: 3},
		{Name: "Temur Devotee", Count: 3}, {Name: "Wind Drake", Count: 2}, {Name: "Serra Angel", Count: 3},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Doom Blade", Count: 3}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 3}, {Name: "Mountain", Count: 3},
		{Name: "Forest", Count: 3}, {Name: "Swamp", Count: 2},
	},
	kmanaabmanacostDecks[1],
}

// kmanaabmanacostComboNames fixes the order the Combo tally is read in.
var kmanaabmanacostComboNames = []string{"Jeskai Devotee", "Mardu Devotee", "Sultai Devotee", "Temur Devotee"}

// kmanaabmanacostFilters are the pool's first-only filter carriers.
var kmanaabmanacostFilters = map[string]bool{
	"Abzan Devotee": true, "Three Tree Mascot": true, "Prophetic Prism": true, "Giant's Boulder": true,
	"Jeskai Devotee": true, "Mardu Devotee": true, "Sultai Devotee": true, "Temur Devotee": true,
}

// kmanaabmanacostTally counts, on gorge's side, the scripted plays whose
// prefix activated a first-only filter, and those that activated two.
type kmanaabmanacostTally struct {
	one, two, games int
	byName          map[string]int // activations by carrier name (read in a fixed order)
}

// kmanaabmanacostRun plays seeds 1..n, failing on each divergence (report)
// or stopping at the first; it returns the divergent games.
func kmanaabmanacostRun(t *testing.T, n uint64, report bool, tal *kmanaabmanacostTally) int {
	t.Helper()
	return kmanaabmanacostRunDecks(t, kmanaabmanacostDecks, n, report, tal)
}

// kmanaabmanacostRunDecks is kmanaabmanacostRun over the deck pair md.
func kmanaabmanacostRunDecks(t *testing.T, md [2][]cardset.Entry, n uint64, report bool, tal *kmanaabmanacostTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("manaabmanacost")
	if err != nil {
		t.Fatal(err)
	}
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	if tal != nil && tal.byName == nil {
		tal.byName = map[string]int{}
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mf-a", "mf-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		// per script (GorgeSide.Stats.Scripted, counted before its prefix
		// runs): the first-only filter activations its prefix made
		script, acts := -1, 0
		flush := func() {
			if tal != nil && acts >= 1 {
				tal.one++
				if acts >= 2 {
					tal.two++
				}
			}
			script, acts = -1, 0
		}
		check := func(ref *GorgeSide, _ Side) *Divergence {
			if ref.E.ManaAbilityHook == nil && tal != nil {
				e := ref.E
				e.ManaAbilityHook = func(_ state.PlayerID, src state.ObjID, sa *cards.SA) {
					if o := e.G.Obj(src); o == nil || o.Face() == nil || !kmanaabmanacostFilters[o.Face().Name] || sa.API != "Mana" {
						return
					}
					if ref.Stats.Scripted != script {
						flush()
						script = ref.Stats.Scripted
					}
					acts++
					tal.byName[e.G.Obj(src).Face().Name]++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: check})
		CloseSide(k)
		flush()
		if tal != nil {
			tal.games++
		}
		if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return div
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div
}

// TestManaabmanacostLockstep: 256 seeds, no divergence, no kernel stall,
// and the first-only filters paid for plays on gorge's side.
func TestManaabmanacostLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var tal kmanaabmanacostTally
	kmanaabmanacostRun(t, 256, true, &tal)
	t.Logf("%d games: scripts activating a first-only filter %d, two or more %d", tal.games, tal.one, tal.two)
	if tal.one < 30 || tal.two < 5 {
		t.Errorf("first-only filter scripts: %d (two or more %d); want at least 30 and 5", tal.one, tal.two)
	}
}

// TestManaabmanacostMutation: with a once-each-turn filter's activation
// limit ignored, some seed diverges.
func TestManaabmanacostMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugFiltFirstNoLimit = true
	n := kmanaabmanacostRun(t, 64, false, nil)
	dirac.DebugFiltFirstNoLimit = false
	if n == 0 {
		t.Error("no seed diverged with the filters' activation limit ignored")
	}
}

// TestManaabmanacostComboLockstep: 256 seeds over the Combo-order Devotees,
// no divergence, no kernel stall, and each Devotee activated on gorge's side.
func TestManaabmanacostComboLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var tal kmanaabmanacostTally
	kmanaabmanacostRunDecks(t, kmanaabmanacostComboDecks, 256, true, &tal)
	t.Logf("%d games: scripts activating a first-only filter %d, two or more %d", tal.games, tal.one, tal.two)
	for _, n := range kmanaabmanacostComboNames {
		t.Logf("%s activated %d times", n, tal.byName[n])
		if tal.byName[n] == 0 {
			t.Errorf("%s was never activated on gorge's side", n)
		}
	}
}
