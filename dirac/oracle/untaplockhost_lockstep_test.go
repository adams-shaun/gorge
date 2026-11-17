package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The untap lock of Duration$ AsLongAsInPlay (Somnophore, Tidebinder Mage):
// the locked creature stays tapped through its controller's untap steps for as
// long as the source is on the battlefield, whoever controls the source
// (dirac/act_untaplock.go ulHost; OpSkipUntap C kop.SkipUntapLockHostStep).
// The blue deck taps the red and green creatures of the other, which only a
// lock keeps tapped (nothing else here holds a creature through an untap
// step), and the games are played decision by decision against gorge.

var untapLockHostNames = [2][]string{
	append(append(append(deckOf(4, "Somnophore"), deckOf(4, "Tidebinder Mage")...), deckOf(8, "Wind Drake")...), deckOf(24, "Island")...),
	append(append(deckOf(12, "Grizzly Bears"), deckOf(12, "Raging Goblin")...), append(deckOf(8, "Forest"), deckOf(8, "Mountain")...)...),
}

func untapLockHostDecks(all [2][]string) (d [2][]cardset.Entry) {
	for s, names := range all {
		n := map[string]int{}
		var order []string
		for _, c := range names {
			if n[c] == 0 {
				order = append(order, c)
			}
			n[c]++
		}
		for _, c := range order {
			d[s] = append(d[s], cardset.Entry{Name: c, Count: n[c]})
		}
	}
	return d
}

// untapLockKept counts, in g's events, the untap steps after which a creature
// that was tapped when the step began (and belongs to the step's player) is
// still tapped when the step ends.
func untapLockKept(g *GorgeSide) int {
	tapped := map[state.ObjID]bool{}
	var snap []state.ObjID
	active := state.PlayerID(0)
	inStep, kept := false, 0
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.Tap:
			tapped[e.Obj] = true
		case events.Untap:
			delete(tapped, e.Obj)
		case events.MoveZone:
			if e.From == state.ZBattlefield {
				delete(tapped, e.Obj)
			}
		case events.TurnChange:
			active = e.Player
		case events.StepChange:
			if inStep {
				for _, o := range snap {
					if tapped[o] {
						kept++
					}
				}
				inStep, snap = false, nil
			}
			if e.Step == state.StepUntap {
				inStep = true
				for o := range tapped {
					if ob := g.E.G.Obj(o); ob != nil && ob.Controller == active && ob.Face() != nil && ob.Face().IsCreature() {
						snap = append(snap, o)
					}
				}
			}
		}
	}
	return kept
}

// untapLockHostRun plays seeds 1..n on a table of the decks, returning the
// divergent games and the untap steps a lock held a creature tapped through.
func untapLockHostRun(t *testing.T, n uint64) (div, kept int) {
	t.Helper()
	return untapLockHostRunDecks(t, untapLockHostNames, n)
}

// untapLockHostRunDecks is untapLockHostRun over the given decks.
func untapLockHostRunDecks(t *testing.T, all [2][]string, n uint64) (div, kept int) {
	t.Helper()
	decks := untapLockHostDecks(all)
	reg := synthReg(t)
	set := printedDelveSet(t, all)
	var gd [2][]*cards.Card
	for s, names := range all {
		for _, nm := range names {
			c, ok := reg.Lookup(nm)
			if !ok {
				t.Fatalf("%s not in the corpus", nm)
			}
			gd[s] = append(gd[s], c)
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"untaplockhost-a", "untaplockhost-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
		rc.Finish = func(g *GorgeSide) {
			kept += untapLockKept(g)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if res.Div != nil {
			div++
			t.Logf("seed %d diverges (decision %d: %s %s %v %s) after %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, actionStrings(res.Actions[max(res.Div.At-8, 0):res.Div.At]))
		}
	}
	return div, kept
}

// TestUntapLockHostLockstep: no seed diverges, and in some a locked creature
// stayed tapped through its controller's untap step.
func TestUntapLockHostLockstep(t *testing.T) {
	div, kept := untapLockHostRun(t, 60)
	if div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	if kept == 0 {
		t.Fatal("no locked creature stayed tapped through an untap step; the lockstep proves nothing")
	}
	t.Logf("untap steps that left a locked creature tapped: %d", kept)
}

// TestUntapLockHostMutation: with the ulHost records never live the kernel
// untaps what gorge keeps tapped, so the same games diverge.
func TestUntapLockHostMutation(t *testing.T) {
	dirac.DebugNoUntapLockHost = true
	defer func() { dirac.DebugNoUntapLockHost = false }()
	if div, _ := untapLockHostRun(t, 60); div == 0 {
		t.Fatal("no seed diverged with the host untap locks disabled")
	}
}
