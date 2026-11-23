package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The ChangesZoneAll Origin$ Any -> Graveyard batch: "whenever one or more
// land cards are put into your graveyard from anywhere" (Titania, Voice of
// Gaea's line). The fixture pool carries the lowering's carriers
// (Turntimber Sower, Sand Scout, The Gitrog Monster, Robot Domination) and
// three sources (plus Mephitic Vapors' surveil 2 and Dryad Arbor, an animated land the state-based actions bin): Tome Scour's mill, Mind Rot's discard
// and Creeping Mold / Armageddon's destroy-land. One resolving action is one
// batch, so a mill of several land cards or one Armageddon makes ONE Plant,
// not one per land. DebugCzallNoLatch queues the line per card and the
// lockstep must diverge.

var kczallanyDecks = [2][]cardset.Entry{
	{
		{Name: "Turntimber Sower", Count: 3}, {Name: "Sand Scout", Count: 3},
		{Name: "The Gitrog Monster", Count: 3}, {Name: "Robot Domination", Count: 2},
		{Name: "Tome Scour", Count: 2}, {Name: "Mind Rot", Count: 2},
		{Name: "Creeping Mold", Count: 3}, {Name: "Armageddon", Count: 3},
		{Name: "Mephitic Vapors", Count: 3}, {Name: "Dryad Arbor", Count: 3},
		{Name: "Forest", Count: 5}, {Name: "Swamp", Count: 4},
		{Name: "Plains", Count: 2}, {Name: "Island", Count: 2},
	},
	{
		{Name: "Turntimber Sower", Count: 4}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Day of Judgment", Count: 4}, {Name: "Mind Rot", Count: 3},
		{Name: "Tome Scour", Count: 3}, {Name: "Creeping Mold", Count: 3},
		{Name: "Mephitic Vapors", Count: 3}, {Name: "Dryad Arbor", Count: 3},
		{Name: "Forest", Count: 4}, {Name: "Plains", Count: 4},
		{Name: "Swamp", Count: 4}, {Name: "Island", Count: 3},
	},
}

const (
	czallSower  = "Turntimber Sower"
	czallScout  = "Sand Scout"
	czallGitrog = "The Gitrog Monster"
	czallRobot  = "Robot Domination"
)

// kczallanyTally is what the games reached: watcher triggers and resolved
// mass-destroy spells, so a clean lockstep is not a green vacuous game.
type kczallanyTally struct {
	triggers, armageddons, casts, vapors int
}

// kczallanyPolicy prefers casting the fixture's spells (so the sources
// resolve), else a random option.
func kczallanyPolicy(seed uint64, tl *kczallanyTally) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x637a6c79))
	return PolicyFunc(func(c *PickCtx) Canon {
		var casts []Canon
		for _, a := range c.D.Options {
			if a.Kind() == ActCast {
				casts = append(casts, a)
			}
		}
		if len(casts) == 0 || seed%5 == 0 {
			return rnd.Pick(c)
		}
		a := casts[r.IntN(len(casts))]
		if c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok && o.Kind == "cast" {
				tl.casts++
			}
		}
		return a
	})
}

// czallanyScan counts the watcher triggers the gorge side actually queued
// and the resolved mass-destroy spells.
func czallanyScan(g *GorgeSide, tl *kczallanyTally) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		o := g.E.G.Obj(ev.Obj)
		name := ""
		if o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		switch ev.Kind {
		case events.TriggerPush:
			switch name {
			case czallSower, czallScout, czallGitrog, czallRobot:
				tl.triggers++
			}
		case events.Resolve:
			switch name {
			case "Armageddon":
				tl.armageddons++
			case "Mephitic Vapors":
				tl.vapors++ // a surveil 2 resolved
			}
		}
	}
}

func runCzallany(t *testing.T, seats, seeds int, mutate bool) (games, divs, stalls int, tl kczallanyTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set := cardsettest.Set(t, "czallany")
	old := dirac.DebugCzallNoLatch
	dirac.DebugCzallNoLatch = mutate
	defer func() { dirac.DebugCzallNoLatch = old }()

	var entries [][]cardset.Entry
	var names []string
	var decks [][]*cards.Card
	for s := 0; s < seats; s++ {
		d := kczallanyDecks[s%2]
		entries = append(entries, d)
		names = append(names, "czallany-"+string(rune('a'+s)))
		var cs []*cards.Card
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs = append(cs, c)
			}
		}
		if len(cs) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(cs))
		}
		decks = append(decks, cs)
	}

	play := func(seed uint64, m *Matchup, tbl *kop.Table) {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kczallanyPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 20})
		czallanyScan(g, &tl)
		games++
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				lo := d.At - 10
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
	}

	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		if seats == 2 {
			kt, _, _, err := set.Matchup(entries[0], entries[1])
			if err != nil {
				t.Fatal(err)
			}
			m, err := NewMatchup(reg, [2]string{names[0], names[1]}, [2][]*cards.Card{decks[0], decks[1]}, 1)
			if err != nil {
				t.Fatal(err)
			}
			play(seed, m, kt)
		} else {
			kt, _, err := set.MatchupN(entries)
			if err != nil {
				t.Fatal(err)
			}
			m, err := NewMatchupN(reg, names, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			play(seed, m, kt)
		}
	}
	return games, divs, stalls, tl
}

func TestCzallanyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runCzallany(t, 2, 128, false)
	t.Logf("%d games, %d divergences, %d stalls, %d watcher triggers, %d Armageddons, %d surveils, %d casts",
		games, divs, stalls, tl.triggers, tl.armageddons, tl.vapors, tl.casts)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if tl.triggers < 5 || tl.armageddons < 1 || tl.vapors < 1 {
		t.Errorf("thin coverage: %d watcher triggers (want 5), %d Armageddons (want 1), %d surveils (want 1)", tl.triggers, tl.armageddons, tl.vapors)
	}
}

func TestCzallanyFourSeatLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runCzallany(t, 4, 32, false)
	t.Logf("%d games, %d divergences, %d stalls, %d watcher triggers, %d Armageddons, %d casts",
		games, divs, stalls, tl.triggers, tl.armageddons, tl.casts)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if tl.triggers < 1 {
		t.Errorf("thin coverage: %d watcher triggers", tl.triggers)
	}
}

func TestCzallanyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runCzallany(t, 2, 96, true)
	if divs == 0 {
		t.Fatalf("DebugCzallNoLatch: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
