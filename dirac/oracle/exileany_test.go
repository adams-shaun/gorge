package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The any-zone dies-to-exile lockstep: Rest in Peace,
// Leyline of the Void and Dryad Militant exile a card that would be put into
// a graveyard from anywhere -- a resolving or countered spell, a discarded or
// milled card, a dying creature -- and no graveyard or dies trigger fires for
// it. The kernel's ReplExileOnLeave with Repl.C = 1 must order the exile and
// the leave events the way gorge's replacementMatches does.

// Each game holds exactly one any-zone source: gorge's resolve tape
// (resolve_mayask.go tapeBoardCompetes) does not count two competing Moved
// replacements as a possible ask, so a spell leaving the stack under two of
// them panics there ("the ask-free predicate missed an ask") before the
// kernel is compared (see the report's Gorge finding). A deck therefore holds
// its source once, and the opposing deck holds none.

// kexileanyA is the red/white side; src replaces one Hill Giant.
func kexileanyA(src string) []cardset.Entry {
	d := []cardset.Entry{
		{Name: "Hare Apparent", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Burst Lightning", Count: 3}, {Name: "Abrade", Count: 2}, {Name: "Dragon Fodder", Count: 3},
		{Name: "Goblin Surprise", Count: 2}, {Name: "Day of Judgment", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	}
	if src != "" {
		d[1].Count--
		d = append(d, cardset.Entry{Name: src, Count: 1})
	}
	return d
}

// kexileanyB is the blue/black side (discard, mill, counters, removal, dies
// triggers); src replaces one Walking Corpse.
func kexileanyB(src string) []cardset.Entry {
	d := []cardset.Entry{
		{Name: "Burglar Rat", Count: 3}, {Name: "Bloodtithe Collector", Count: 2}, {Name: "Crow of Dark Tidings", Count: 3},
		{Name: "Billowing Shriekmass", Count: 2}, {Name: "Midnight Reaper", Count: 3}, {Name: "Cancel", Count: 2},
		{Name: "Essence Scatter", Count: 2}, {Name: "Chart a Course", Count: 1}, {Name: "Duress", Count: 1},
		{Name: "Hero's Downfall", Count: 1}, {Name: "Deathmark", Count: 1}, {Name: "Walking Corpse", Count: 3},
		{Name: "Island", Count: 8}, {Name: "Swamp", Count: 8},
	}
	if src != "" {
		d[11].Count--
		d = append(d, cardset.Entry{Name: src, Count: 1})
	}
	return d
}

var kexileanyMatchups = [][2][]cardset.Entry{
	{kexileanyA("Rest in Peace"), kexileanyB("")},
	{kexileanyA(""), kexileanyB("Leyline of the Void")},
	{kexileanyA("Dryad Militant"), kexileanyB("")},
}

// runExileany plays seeds 1..seeds of the matchup and returns the games, the
// divergences, the kernel capacity stalls and the kernel's exile counters.
// skip makes the kernel skip the non-battlefield check (the mutation).
func runExileany(t *testing.T, seeds int, skip bool) (games, divs, stalls int, stats [2]uint64) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exileany")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugExileAnyBattlefieldOnly
	dirac.DebugExileAnyBattlefieldOnly = skip
	defer func() { dirac.DebugExileAnyBattlefieldOnly = old }()
	dirac.ExileAnyStats = [2]uint64{}
	for mi, md := range kexileanyMatchups {
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
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kexileany-a", "kexileany-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
			}
			if d := res.Div; d != nil {
				divs++
				if !skip {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, dirac.ExileAnyStats
}

func TestExileanyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runExileany(t, 256, false)
	t.Logf("%d games, %d divergences, %d kernel stalls, %d cards exiled instead from a non-battlefield zone, %d creatures exiled instead of dying",
		games, divs, stalls, st[0], st[1])
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if stalls*20 >= games {
		t.Errorf("%d kernel stalls in %d games (5%% cap)", stalls, games)
	}
	if st[0] < 100 {
		t.Errorf("only %d cards exiled instead from a non-battlefield zone, want 100", st[0])
	}
	if st[1] < 30 {
		t.Errorf("only %d creatures exiled instead of dying, want 30", st[1])
	}
}

// TestExileanyMutation skips the non-battlefield check (the behaviour before
// the test): the kernel then puts discarded, milled and spent cards into the
// graveyard that gorge exiles, so the games must diverge. A green mutation
// means the lockstep never reaches a non-battlefield move under an any-zone
// source.
func TestExileanyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runExileany(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation skipped the non-battlefield check in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}

// Tokens dying together under two exile-instead effects pose one order ask each,
// with no state-based check between them (CR 704.3): a token the first answer exiled
// is still there at the second ask. A discard under both stalls the kernel, which
// ends that game short of the comparison.
func TestExileAnyTokenStaysBetweenAsks(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exileany")
	if err != nil {
		t.Fatal(err)
	}
	deck := []cardset.Entry{
		{Name: "Rest in Peace", Count: 8}, {Name: "Leyline of the Void", Count: 8}, {Name: "Hare Apparent", Count: 12},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 3}, {Name: "Swamp", Count: 3},
	}
	var cs []*cards.Card
	for _, e := range deck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			cs = append(cs, c)
		}
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 60; seed++ {
		cfg := Config{GorgeAutopaySurface: true}
		m, err := NewMatchup(reg, [2]string{"a", "b"}, [2][]*cards.Card{cs, cs}, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
	}
}
