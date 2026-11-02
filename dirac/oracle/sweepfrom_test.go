package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (CR 120.3, 702.2b, 702.15b):
// constructed decks over the generated sweepfrom pool.
//
//   - Waltz of Rage / Nibelheim Aflame: DamageAll DamageSource$ ParentTarget
//     over Creature.NotDefinedTargeted, the target creature dealing its power
//     to every other creature in one damage batch;
//   - Child of Night, Vampire Nighthawk (lifelink), Typhoid Rats, Vampire
//     Nighthawk (deathtouch) and Glistener Elf (infect) as the source;
//   - Ashen-Skin Zubera (a dies trigger) among the victims, vanilla bodies of
//     power 1-6, and Waltz's creature-dies exile effect.
var ksweepfromDecks = [2][]cardset.Entry{
	{
		{Name: "Waltz of Rage", Count: 4}, {Name: "Nibelheim Aflame", Count: 3}, {Name: "Child of Night", Count: 4}, {Name: "Typhoid Rats", Count: 3},
		{Name: "Vampire Nighthawk", Count: 3}, {Name: "Ashen-Skin Zubera", Count: 3}, {Name: "Fire Elemental", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Waltz of Rage", Count: 3}, {Name: "Nibelheim Aflame", Count: 2}, {Name: "Glistener Elf", Count: 4}, {Name: "Alpine Grizzly", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Trained Armodon", Count: 3}, {Name: "Colossal Dreadmaw", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
}

// ksfCounts are what gorge's event log showed.
type ksfCounts struct {
	sweeps     int // runs of noncombat damage from one creature to creatures
	lifelink   int // ... from a lifelink creature
	deathtouch int // ... from a deathtouch creature
	infect     int // ... from an infect creature
	exiled     int // library cards Waltz of Rage's creature-dies trigger exiled
}

func ksfScan(g *GorgeSide) ksfCounts {
	var c ksfCounts
	if g == nil {
		return c
	}
	gm := g.E.G
	prev := state.ObjID(0)
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.DamageProvenance:
			src := gm.Obj(ev.Obj)
			if ev.Amount <= 0 || len(ev.IDs) == 0 || strings.HasPrefix(ev.Text, events.DamageProvenanceCombat) || src == nil || src.Face() == nil || !src.Face().IsCreature() {
				continue
			}
			if rc := gm.Obj(ev.IDs[0]); rc == nil || rc.Face() == nil || !rc.Face().IsCreature() {
				continue
			}
			if ev.Obj == prev {
				continue
			}
			prev = ev.Obj
			c.sweeps++
			switch src.Face().Name {
			case "Child of Night":
				c.lifelink++
			case "Vampire Nighthawk":
				c.lifelink++
				c.deathtouch++
			case "Typhoid Rats":
				c.deathtouch++
			case "Glistener Elf":
				c.infect++
			}
		case events.MoveZone:
			// Waltz of Rage's trigger exiles the top card of its controller's
			// library (the only library exile in the pool)
			if ev.From == state.ZLibrary && ev.To == state.ZExile {
				c.exiled++
			}
			prev = 0
		case events.Resolve:
			prev = 0
		}
	}
	return c
}

// ksfRun plays seeds 1..seeds and returns the summed counts, the first
// divergence ("" if none) and the kernel stalls.
func ksfRun(t *testing.T, seeds uint64) (tot ksfCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("sweepfrom")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ksweepfromDecks {
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
	tbl, _, _, err := set.Matchup(ksweepfromDecks[0], ksweepfromDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ksf-a", "ksf-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		wrapped := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return pol.Pick(c)
		})
		res := Lockstep(m, seed, k, wrapped, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		c := ksfScan(gs)
		tot.sweeps += c.sweeps
		tot.lifelink += c.lifelink
		tot.deathtouch += c.deathtouch
		tot.infect += c.infect
		tot.exiled += c.exiled
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %d %v %s", seed, actionStrings(res.Actions[lo:d.At]), len(d.Diff), d.Diff, d.Detail)
			return tot, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tot, games, "", stalls
}

func TestSweepfromLockstep(t *testing.T) {
	t.Parallel()
	tot, games, div, stalls := ksfRun(t, 256)
	t.Logf("%d games; sweeps %d (lifelink source %d, deathtouch source %d, infect source %d); Waltz trigger exiles %d; stalls %v",
		games, tot.sweeps, tot.lifelink, tot.deathtouch, tot.infect, tot.exiled, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tot.sweeps < 30 || tot.lifelink < 10 || tot.deathtouch < 10 || tot.exiled < 10 {
		t.Errorf("sweeps %d, lifelink %d, deathtouch %d, Waltz trigger exiles %d, want at least 30, 10, 10 and 10",
			tot.sweeps, tot.lifelink, tot.deathtouch, tot.exiled)
	}
}

// TestSweepfromMutation: with the resolving spell as the damage source
// (no lifelink, deathtouch or infect from the creature) the lockstep must
// diverge within 64 seeds.
func TestSweepfromMutation(t *testing.T) {
	dirac.DebugSweepFromSelf = true
	defer func() { dirac.DebugSweepFromSelf = false }()
	_, games, div, _ := ksfRun(t, 64)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugSweepFromSelf", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}
