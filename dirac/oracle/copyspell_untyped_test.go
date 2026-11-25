package oracle

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The untyped copy-target-spell lockstep (CR 707.10, 707.10c): Sigil Tracer,
// Uyo, Silent Prophet, Rootha, Mercurial Artist, Echo Mage's level abilities
// and Dual Casting's granted ability copy a target instant or sorcery spell
// (ValidTgts$ Instant,Sorcery with no TargetType$), choosing new targets for
// the copy. The expected answer at every decision is gorge's; the copies are
// counted from gorge's event log, where the activated copy resolves.
var copyspellUntypedDecks = [2][]cardset.Entry{
	{
		{Name: "Sigil Tracer", Count: 3}, {Name: "Uyo, Silent Prophet", Count: 2}, {Name: "Rootha, Mercurial Artist", Count: 2},
		{Name: "Echo Mage", Count: 2}, {Name: "Dual Casting", Count: 3}, {Name: "Prodigal Sorcerer", Count: 3},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Opt", Count: 3},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 7},
	},
	{
		{Name: "Sigil Tracer", Count: 2}, {Name: "Rootha, Mercurial Artist", Count: 2}, {Name: "Dual Casting", Count: 2},
		{Name: "Prodigal Sorcerer", Count: 3}, {Name: "Fire Imp", Count: 2}, {Name: "Wind Drake", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Opt", Count: 3}, {Name: "Divination", Count: 2},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 5},
	},
}

var copyspellUntypedCopiers = map[string]bool{
	"Sigil Tracer": true, "Uyo, Silent Prophet": true, "Rootha, Mercurial Artist": true, "Echo Mage": true,
	// Dual Casting grants the ability to the creature it enchants, so the
	// resolving copy's source is that creature: counted as "other"
}

// copyspellUntypedSeeds is each half's seed count: the lockstep is two tests
// (seeds 1..100 and 101..200) so each fits the one-minute budget.
const copyspellUntypedSeeds = 100

type copyspellUntypedTally struct {
	evAt    int
	lastSrc string
	byCard  map[string]int
	other   int
}

// check counts stack_copy events by the source of the resolving stack
// object (the activated copy ability's card).
func (c *copyspellUntypedTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt = 0
	}
	for j := c.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind.String() {
		case "stack_resolve":
			c.lastSrc = ""
			if o := g.Obj(e.Obj); o != nil {
				src := o
				if o.Source != 0 {
					if so := g.Obj(o.Source); so != nil {
						src = so
					}
				}
				if src.Card != nil {
					c.lastSrc = src.Face().Name
				}
			}
		case "stack_copy":
			if copyspellUntypedCopiers[c.lastSrc] {
				c.byCard[c.lastSrc]++
			} else {
				c.other++
			}
		}
	}
	c.evAt = len(evs)
	return nil
}

func TestCopySpellUntypedLockstepLow(t *testing.T) {
	t.Parallel()
	copyspellUntypedLockstep(t, 1)
}

func TestCopySpellUntypedLockstepHigh(t *testing.T) {
	t.Parallel()
	copyspellUntypedLockstep(t, 1+copyspellUntypedSeeds)
}

// copyspellUntypedLockstep plays seeds first..first+copyspellUntypedSeeds-1.
func copyspellUntypedLockstep(t *testing.T, first uint64) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("copyspelltgt")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range copyspellUntypedDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	byCard := map[string]int{}
	other, games := 0, 0
	seedChunks(t, first, first+copyspellUntypedSeeds-1, 25, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		// a table of its own: a kernel table is not shared across games in flight
		tbl, _, _, err := set.Matchup(copyspellUntypedDecks[0], copyspellUntypedDecks[1])
		if err != nil {
			t.Fatal(err)
		}
		tal := &copyspellUntypedTally{byCard: map[string]int{}}
		n := 0
		for seed := lo; seed <= hi; seed++ {
			res := copyspellUntypedRun(t, reg, set, tbl, gd, seed, tal.check)
			n++
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		mu.Lock()
		for k, v := range tal.byCard {
			byCard[k] += v
		}
		other += tal.other
		games += n
		mu.Unlock()
	})
	t.Logf("%d games: copies by source %v, %d other (Dual Casting's bearer)", games, byCard, other)
	if games == copyspellUntypedSeeds {
		for _, n := range []string{"Sigil Tracer", "Rootha, Mercurial Artist"} {
			if byCard[n] < 5 {
				t.Errorf("%d copies by %s, want at least 5: the pool does not exercise it", byCard[n], n)
			}
		}
	}
}

func copyspellUntypedRun(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *kop.Table, gd [2][]*cards.Card, seed uint64, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	m, err := NewMatchup(reg, [2]string{"copyspelltgt-a", "copyspelltgt-b"}, gd, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, seed, k, permcopyPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 60, Check: check})
	CloseSide(k)
	return res
}
