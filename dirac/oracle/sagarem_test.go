package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Saga exiled-with lockstep: The Measure of a Man's chapter III returns
// the card its chapter I exiled (linked abilities, CR 607.2a, 714.2b). gorge
// reads the cards exiled with the Saga (Defined$ ExiledWith, the exiled
// card's ExiledWith link); the kernel reads its exiled-with links
// (zone_exiledwith.go). Naturalize destroys the Saga, sometimes between its
// chapters, so the exiled card stays exiled, and a later Saga's chapter III
// must not return it. Counters from gorge's event log:
//
//  1. a chapter I exiles a creature,
//  2. a chapter III returns it,
//  3. a Saga left the battlefield holding a card it exiled.
var sagaremDecks = [][2][]cardset.Entry{
	{
		{
			{Name: "The Measure of a Man", Count: 8}, {Name: "Savannah Lions", Count: 6}, {Name: "Walking Corpse", Count: 6},
			{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 10},
		},
		{
			{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Naturalize", Count: 6},
			{Name: "Shock", Count: 4}, {Name: "Unsummon", Count: 2},
			{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Island", Count: 2},
		},
	},
	{ // both seats play the Saga: each exiles the other's creature
		{
			{Name: "The Measure of a Man", Count: 8}, {Name: "Savannah Lions", Count: 6}, {Name: "Walking Corpse", Count: 4},
			{Name: "Disenchant", Count: 2}, {Name: "Plains", Count: 12}, {Name: "Swamp", Count: 8},
		},
		{
			{Name: "The Measure of a Man", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
			{Name: "Naturalize", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 4},
		},
	},
}

type sagaremTally struct {
	evAt                   int
	exiled, returned, left int
}

func (p *sagaremTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.MoveZone {
			continue
		}
		switch {
		case e.From == state.ZBattlefield && e.To == state.ZExile:
			p.exiled++ // the pool's only battlefield exile is chapter I's
		case e.From == state.ZExile && e.To == state.ZBattlefield:
			p.returned++
		case e.From == state.ZBattlefield && chName(g, e.Obj) == "The Measure of a Man":
			if o := g.Obj(e.Obj); o != nil {
				for _, r := range o.Remembered {
					if ro := g.Obj(r.Obj); !r.IsPlayer && ro != nil && ro.Zone == state.ZExile {
						p.left++
						break
					}
				}
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

// sagaremRun plays the matchups, each both ways, for seeds seeds each. With
// mutated set a divergence is returned instead of failing the test.
func sagaremRun(t *testing.T, seeds int, mutated bool) (p *sagaremTally, games int, diverged bool) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("sagarem")
	if err != nil {
		t.Fatal(err)
	}
	p = &sagaremTally{}
	var matchups [][2][]cardset.Entry
	for _, d := range sagaremDecks {
		matchups = append(matchups, d, [2][]cardset.Entry{d[1], d[0]})
	}
	for mi, md := range matchups {
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
		var tbl *kop.Table
		if tbl, _, _, err = set.Matchup(md[0], md[1]); err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"sr-a", "sr-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := fbPolicy(seed) // plays at random among its plays
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			p.evAt = 0
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 80, Check: p.check})
			CloseSide(k)
			games++
			if mutated {
				if res.Div != nil {
					return p, games, true
				}
				continue
			}
			if res.Stall != "" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return p, games, diverged
}

func TestSagaRememberedLockstep(t *testing.T) {
	t.Parallel()
	p, games, _ := sagaremRun(t, 40, false)
	t.Logf("%d games: chapter I exiled %d, chapter III returned %d, a Saga left holding an exiled card %d",
		games, p.exiled, p.returned, p.left)
	for _, c := range []struct {
		name string
		n    int
	}{{"chapter I exiles", p.exiled}, {"chapter III returns", p.returned}, {"Sagas gone between the chapters", p.left}} {
		if c.n < 5 {
			t.Errorf("%d %s, want at least 5", c.n, c.name)
		}
	}
}

// TestSagaRememberedMutation: with every card in exile read as exiled with
// the reading Saga, a chapter III also returns the cards another Saga exiled,
// which gorge's ExiledWith link does not.
func TestSagaRememberedMutation(t *testing.T) {
	dirac.DebugExwAnySource = true
	defer func() { dirac.DebugExwAnySource = false }()
	if _, _, diverged := sagaremRun(t, 64, true); !diverged {
		t.Fatal("no divergence in 64 seeds with any exiled card read as exiled with the Saga")
	}
}
