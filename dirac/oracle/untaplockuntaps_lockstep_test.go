package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The untap lock of Duration$ UntilUntaps (Amber Prison, Rust Tick): "tap
// target permanent. It doesn't untap during its controller's untap step for as
// long as CARDNAME remains tapped" (CR 611.2b; dirac/act_untaplock.go ulUntap,
// OpSkipUntap C kop.SkipUntapLockUntapStep). The policy activates whenever it
// can and answers the "you may choose not to untap" election at random, so the
// hosts are sometimes untapped (the lock ends for good) and sometimes kept
// tapped (it holds). Both decks are four Amber Prisons and four Rust Ticks
// beside creatures and lands; the games are played decision by decision
// against gorge.

var untapLockUntapsNames = [2][]string{
	append(append(append(deckOf(4, "Amber Prison"), deckOf(4, "Rust Tick")...), deckOf(12, "Grizzly Bears")...), deckOf(20, "Forest")...),
	append(append(append(deckOf(4, "Amber Prison"), deckOf(4, "Rust Tick")...), deckOf(12, "Raging Goblin")...), deckOf(20, "Mountain")...),
}

// untapLockUntapsHost reports whether o is an Amber Prison or a Rust Tick: the
// hosts keep themselves tapped by their own election, which is no lock.
func untapLockUntapsHost(g *GorgeSide, o state.ObjID) bool {
	ob := g.E.G.Obj(o)
	if ob == nil || ob.Face() == nil {
		return false
	}
	n := ob.Face().Name
	return n == "Amber Prison" || n == "Rust Tick"
}

// untapLockUntapsStats are what a run reached, read off gorge's events.
type untapLockUntapsStats struct {
	div     int
	held    int // untap steps that left a tapped non-host permanent tapped (a lock held)
	ended   int // such a permanent untapped later, after a host had untapped (the lock ended with the host's untap)
	hostUps int // host untaps
}

// untapLockUntapsScan reads one game's events. held maps a permanent a lock is
// holding to the host-untap count when it was first held.
func untapLockUntapsScan(g *GorgeSide, st *untapLockUntapsStats) {
	tapped := map[state.ObjID]bool{}
	held := map[state.ObjID]int{}
	var snap []state.ObjID
	active := state.PlayerID(0)
	inStep := false
	hostUps := 0
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.Tap:
			tapped[e.Obj] = true
		case events.Untap:
			delete(tapped, e.Obj)
			if untapLockUntapsHost(g, e.Obj) {
				hostUps++
				st.hostUps++
			}
			if at, ok := held[e.Obj]; ok {
				if hostUps > at {
					st.ended++
				}
				delete(held, e.Obj)
			}
		case events.MoveZone:
			if e.From == state.ZBattlefield {
				delete(tapped, e.Obj)
				delete(held, e.Obj)
			}
		case events.TurnChange:
			active = e.Player
		case events.StepChange:
			if inStep {
				for _, o := range snap {
					if tapped[o] {
						st.held++
						if _, ok := held[o]; !ok {
							held[o] = hostUps
						}
					}
				}
				inStep, snap = false, nil
			}
			if e.Step == state.StepUntap {
				inStep = true
				for o := range tapped {
					if ob := g.E.G.Obj(o); ob != nil && ob.Controller == active && !untapLockUntapsHost(g, o) {
						snap = append(snap, o)
					}
				}
			}
		}
	}
}

// untapLockUntapsRun plays seeds 1..n on a table of the decks.
func untapLockUntapsRun(t *testing.T, n uint64) (st untapLockUntapsStats) {
	t.Helper()
	// Skip before synthReg. cardset.CorpusDir walks up for .cards and needs no
	// git checkout; testutil's CorpusDirOrSkip (inside synthReg) needs a repo
	// root and fails where there is none. The pr-precheck fresh clone is a
	// `git archive` scratch with no .git and no .cards.
	if _, err := cardset.CorpusDir(); err != nil {
		t.Skipf("test corpus: %v", err)
	}
	all := untapLockUntapsNames
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
		m, err := NewMatchup(reg, [2]string{"untaplockuntaps-a", "untaplockuntaps-b"}, gd, 1)
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
		rc.Finish = func(g *GorgeSide) { untapLockUntapsScan(g, &st) }
		res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if res.Div != nil {
			st.div++
			t.Logf("seed %d diverges (decision %d: %s %s %v %s) after %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, actionStrings(res.Actions[max(res.Div.At-8, 0):res.Div.At]))
		}
	}
	return st
}

// TestUntapLockUntilUntapsLockstep: no seed diverges; in some a lock held a
// permanent tapped through its controller's untap step, and in some a lock
// ended with its host's untap.
func TestUntapLockUntilUntapsLockstep(t *testing.T) {
	st := untapLockUntapsRun(t, 80)
	if st.div != 0 {
		t.Fatalf("%d seeds diverged", st.div)
	}
	if st.held == 0 {
		t.Fatal("no lock held a permanent tapped through an untap step; the lockstep proves nothing")
	}
	if st.ended == 0 {
		t.Fatal("no lock ended by its host's untap; the lockstep proves nothing")
	}
	t.Logf("untap steps a lock held through: %d; locks ended by the host's untap: %d; host untaps: %d", st.held, st.ended, st.hostUps)
}

// TestUntapLockUntilUntapsMutation: with the host's untap no longer dropping
// its records, a retap of the host revives a lock gorge has ended, so the same
// games diverge.
func TestUntapLockUntilUntapsMutation(t *testing.T) {
	dirac.DebugNoUntapLockUntapDrop = true
	defer func() { dirac.DebugNoUntapLockUntapDrop = false }()
	if st := untapLockUntapsRun(t, 80); st.div == 0 {
		t.Fatal("no seed diverged with the host's untap not dropping its locks")
	}
}
