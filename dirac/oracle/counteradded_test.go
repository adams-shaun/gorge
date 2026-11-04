package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Nth-counter lockstep: Death to Our Enemies'
// "When the fourth plan counter is put on this enchantment, sacrifice it"
// (Mode$ CounterAdded, CounterAmount$ EQ4) is a crossing: it fires once when
// the plan count reaches four, not on every later put. The random policy
// keeps spells in hand, so one is sometimes cast in response to the sacrifice
// trigger and puts a fifth plan counter on the enchantment; the kernel's
// counterAddedCrossed must not queue a second sacrifice, as gorge's
// counterAddedMatches does not.

var kcounteraddedDecks = [2][]cardset.Entry{
	{
		{Name: "Death to Our Enemies", Count: 4}, {Name: "Contagion Clasp", Count: 2}, {Name: "Shock", Count: 9},
		{Name: "Burst Lightning", Count: 9}, {Name: "Mountain", Count: 16},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Gray Ogre", Count: 8}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
	},
}

// The per-counter + ActivationLimit$ 1 lockstep:
// Knight of Wundagore ("whenever you put a +1/+1 counter on another creature,
// put a +1/+1 counter on this creature; only once each turn") and Ant-Man,
// Colony Commander (the same shape with an Insect token) are plain
// Mode$ CounterAdded lines, one trigger per counter. Since gorge the
// per-counter loop re-checks both limit gates, so the first counter of the
// turn queues the one trigger and the rest are refused: Big Play's and
// Ant-Man's counters trigger once a
// turn, which the kernel carries as its one-per-batch event under
// TfOncePerTurn.
var klimitDecks = [2][]cardset.Entry{
	{
		{Name: "Knight of Wundagore", Count: 4}, {Name: "Ant-Man, Colony Commander", Count: 4}, {Name: "Big Play", Count: 9},
		{Name: "Incremental Growth", Count: 3}, {Name: "Forest", Count: 11}, {Name: "Island", Count: 9},
	},
	kcounteraddedDecks[1],
}

// runCounteradded plays the seeds and returns the games, the divergences,
// the Death to Our Enemies that ended in a graveyard (the sacrifice resolved).
func runCounteradded(t *testing.T, seeds int, mutate bool) (games, divs, sacs int) {
	t.Helper()
	games, divs, sacs, _ = runCounteraddedDecks(t, kcounteraddedDecks, seeds, mutate)
	return
}

// runCounteraddedDecks is runCounteradded for any decks of the pool; tokens
// counts the Insect tokens standing at the end of the games.
func runCounteraddedDecks(t *testing.T, kdecks [2][]cardset.Entry, seeds int, mutate bool) (games, divs, sacs, tokens int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("counteradded")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCounterAddedNoCross
	dirac.DebugCounterAddedNoCross = mutate
	defer func() { dirac.DebugCounterAddedNoCross = old }()
	var decks [2][]*cards.Card
	for s, d := range kdecks {
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
	tbl, _, _, err := set.Matchup(kdecks[0], kdecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcounteradded-a", "kcounteradded-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := RandomPolicy(seed)
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		for _, id := range g.E.G.Zone(state.ZGraveyard, 0) {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Death to Our Enemies" {
				sacs++
			}
		}
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
				if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Insect Token" {
					tokens++
				}
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, sacs, tokens
}

func TestCounteraddedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, sacs := runCounteradded(t, 256, false)
	t.Logf("%d games, %d divergences, plan-crossing sacrifices %d", games, divs, sacs)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if sacs < 5 {
		t.Errorf("only %d plan-crossing sacrifices (want at least 5)", sacs)
	}
}

// TestCounteraddedMutation drops the crossing's before < n half: a spell cast
// in response to the sacrifice trigger puts a fifth plan counter and the
// plan card re-fires, so the games must diverge. A green mutation (no
// divergence) means the lockstep never reaches a counter put past the Nth.
func TestCounteraddedMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _ := runCounteradded(t, 256, true)
	if divs == 0 {
		t.Fatalf("mutation re-fired the plan card in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}

func TestCounteraddedLimitLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, tokens := runCounteraddedDecks(t, klimitDecks, 256, false)
	t.Logf("%d games, %d divergences, Insect tokens %d", games, divs, tokens)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if tokens < 5 {
		t.Errorf("only %d Insect tokens (want at least 5): the limited trigger never fired", tokens)
	}
}

// kmikeyDecks plays Mikey & Leo, Chaos & Order ("whenever you put a counter on
// a creature you control, draw a card; only once each turn": Mode$ CounterAdded
// with no CounterType$, ValidSource$ You, ActivationLimit$ 1) beside +1/+1
// sources (Big Play, Incremental Growth) and Contagion Clasp's -1/-1 counter.
// The opponent plays Contagion Clasps too: a counter it puts on Mikey's
// owner's creature must not draw (ValidSource$ You).
var kmikeyDecks = [2][]cardset.Entry{
	{
		{Name: "Mikey & Leo, Chaos & Order", Count: 4}, {Name: "Big Play", Count: 8}, {Name: "Incremental Growth", Count: 3},
		{Name: "Contagion Clasp", Count: 3}, {Name: "Forest", Count: 12}, {Name: "Island", Count: 10},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Gray Ogre", Count: 8}, {Name: "Contagion Clasp", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
	},
}

func TestCounteraddedMikeyLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "counteradded", kmikeyDecks, 1000, "KCOUNTERADDED_SEEDS")
}

// TestCounteraddedMikeyMutation narrows the any-kind draw to one counter
// kind: to -1/-1 the +1/+1 puts no longer draw, to +1/+1 Contagion Clasp's
// -1/-1 put no longer draws; each must diverge some seed.
func TestCounteraddedMikeyMutation(t *testing.T) {
	t.Parallel()
	for _, k := range []kop.CounterKind{kop.CtrM1M1, kop.CtrP1P1} {
		n := lockstepRun(t, "counteradded", kmikeyDecks, 1000, false, func(tbl *kop.Table) {
			for i := range tbl.Cards {
				for j := range tbl.Cards[i].Abilities {
					a := &tbl.Cards[i].Abilities[j]
					if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigCounterAddedOnce && a.Trig.N == kop.TrigNAnyCounter {
						a.Trig.N = int16(k)
					}
				}
			}
		})
		if n == 0 {
			t.Errorf("no seed diverged with the any-kind draw narrowed to counter kind %d", k)
		}
	}
}
