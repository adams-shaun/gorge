package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Learn (CR 701.48a; gorge effLearn, dirac/act_learn.go): the learner may
// reveal a Lesson card they own from outside the game and put it into their
// hand, discard a card to draw a card, or do neither. Both seats carry a
// sideboard of Lessons (and one card that is not a Lesson, which must never be
// offered) in both engines, and the kernel must pose gorge's one election
// (nothing, each Lesson in sideboard order, then the rummage) and the
// rummage's discard pick, decision for decision. The learn fixture pool holds
// the seventeen paper carriers.

var learnDecks = [2][]cardset.Entry{
	{
		{Name: "Eyetwitch", Count: 3}, {Name: "Hunt for Specimens", Count: 2}, {Name: "Poet's Quill", Count: 2},
		{Name: "Igneous Inspiration", Count: 2}, {Name: "First Day of Class", Count: 2}, {Name: "Enthusiastic Study", Count: 2},
		{Name: "Rise of Extus", Count: 2}, {Name: "Cram Session", Count: 2},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 7}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Arcane Subtraction", Count: 2}, {Name: "Pop Quiz", Count: 2}, {Name: "Study Break", Count: 2},
		{Name: "Professor of Symbology", Count: 2}, {Name: "Guiding Voice", Count: 2}, {Name: "Gnarled Professor", Count: 2},
		{Name: "Overgrown Arch", Count: 2}, {Name: "Field Trip", Count: 2}, {Name: "Sparring Regimen", Count: 2},
		{Name: "Island", Count: 8}, {Name: "Plains", Count: 7}, {Name: "Forest", Count: 7},
	},
}

var learnSides = [2][]cardset.Entry{
	{
		{Name: "Environmental Sciences", Count: 1}, {Name: "Introduction to Annihilation", Count: 1}, {Name: "Pest Summoning", Count: 1},
		{Name: "Necrotic Fumes", Count: 1}, {Name: "Reduce to Memory", Count: 1}, {Name: "Grizzly Bears", Count: 1},
	},
	{
		{Name: "Introduction to Prophecy", Count: 1}, {Name: "Expanded Anatomy", Count: 1}, {Name: "Spirit Summoning", Count: 1},
		{Name: "Inkling Summoning", Count: 1}, {Name: "Mascot Exhibition", Count: 1}, {Name: "Thrilling Discovery", Count: 1},
		{Name: "Island", Count: 1},
	},
}

// learnReach counts what the games did with the learn, read from gorge's
// event log: Lessons moved from a sideboard to a hand, and the learns cast or
// activated by name.
type learnReach struct {
	lessons, learnCasts         int
	nothing, kLessons, rummages int64 // the kernel's answers by arm (dirac.LearnStats)
}

// learnCarrierNames is the deck cards that learn.
func learnCarrierNames() map[string]bool {
	m := map[string]bool{}
	for _, d := range learnDecks {
		for _, e := range d {
			switch e.Name {
			case "Swamp", "Mountain", "Plains", "Forest", "Island":
			default:
				m[e.Name] = true
			}
		}
	}
	return m
}

// runLearn plays seeds 1..seeds of the learn matchup, both autopay surfaces;
// it returns the games, the divergences, the stalls and the reach.
func runLearn(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, reach learnReach) {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("learn")
	if err != nil {
		t.Fatal(err)
	}
	old, oldSt := dirac.DebugLearnNoLesson, dirac.DebugLearnStats
	st := &dirac.LearnStats{}
	dirac.DebugLearnNoLesson, dirac.DebugLearnStats = mutate, st
	defer func() { dirac.DebugLearnNoLesson, dirac.DebugLearnStats = old, oldSt }()
	defer func() {
		reach.nothing, reach.kLessons, reach.rummages = st.Nothing.Load(), st.Lessons.Load(), st.Rummages.Load()
	}()
	var decks, sides [2][]*cards.Card
	for s := range learnDecks {
		decks[s] = expandEntries(t, reg, learnDecks[s])
		sides[s] = expandEntries(t, reg, learnSides[s])
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(append(append([]cardset.Entry(nil), learnDecks[0]...), learnSides[0]...), append(append([]cardset.Entry(nil), learnDecks[1]...), learnSides[1]...))
	if err != nil {
		t.Fatal(err)
	}
	learners := learnCarrierNames()
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchupSide(reg, [2]string{"learn-a", "learn-b"}, decks, sides, 1)
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
		casts := map[string]int{}
		pol := poisonhighPolicy(seed, casts, map[string]int{}) // casts whenever it can
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		for name, n := range casts {
			if learners[name] {
				reach.learnCasts += n
			}
		}
		for _, e := range g.E.L.Events {
			if e.Kind == events.MoveZone && e.From == state.ZSideboard && e.To == state.ZHand {
				reach.lessons++
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return
}

func TestLearnLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, reach := runLearn(t, 64, false)
	t.Logf("%d games, %d divergences, %d stalls; %+v", games, divs, stalls, reach)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if reach.lessons < 20 || reach.learnCasts < 40 || reach.rummages < 10 || reach.nothing < 10 || reach.kLessons != int64(reach.lessons) {
		t.Errorf("reach %+v: want at least 20 Lessons fetched (each a kernel lesson answer), 40 learn casts, 10 rummages and 10 declines", reach)
	}
}

// TestLearnMutation never offers a Lesson in the kernel: the games must
// diverge. A green mutation means the lockstep never fetched a Lesson.
func TestLearnMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runLearn(t, 16, true)
	if divs == 0 {
		t.Fatalf("mutation dropped every Lesson in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
