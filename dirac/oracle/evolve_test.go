package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Evolve lockstep (CR 702.100a, 603.4): K:Evolve's ETB trigger and its
// resolution recheck on Experiment One (regenerate for two counters), Gyre
// Sage (mana per +1/+1 counter), Pollywog Prodigy (a noncreature-spell draw
// reading its power) and Dinosaur Egg (discover by toughness), and Fathom
// Mage's per-counter draw (CR 603.2c: the evolve counter and each of
// Dragonscale Boon's two), beside Grizzly Bears and Craw Wurm as the
// entrants and Raise the Alarm, whose two 1/1 tokens enter together: the
// first evolves Dinosaur Egg (0/3 to 1/4) and the second's recheck then
// fails. Against Ornithopters, Hill Giants, Pacifism and burn. The expected
// answer at every decision is gorge's.
var kevolveDecks = [2][]cardset.Entry{
	{
		{Name: "Experiment One", Count: 4}, {Name: "Gyre Sage", Count: 4}, {Name: "Pollywog Prodigy", Count: 3}, {Name: "Fathom Mage", Count: 4},
		{Name: "Dinosaur Egg", Count: 4}, {Name: "Raise the Alarm", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Craw Wurm", Count: 2}, {Name: "Dragonscale Boon", Count: 3}, {Name: "Forest", Count: 6}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Ornithopter", Count: 6}, {Name: "Pacifism", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 20},
	},
}

func TestEvolveLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "evolve", kevolveDecks, 1000, "KEVOLVE_SEEDS")
}

// TestEvolveLockstepMutation proves the pool evolves and rechecks: dropping
// the Evolve gate (every entrant evolves) or Fathom Mage's per-counter
// queueing (one draw per put, not per counter) each makes some seed diverge.
func TestEvolveLockstepMutation(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name string
		fn   func(*kop.Ability)
	}{
		{"gate", func(a *kop.Ability) {
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigETB {
				a.Trig.Flags2 &^= kop.Tf2Evolve
			}
		}},
		{"per counter", func(a *kop.Ability) {
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigCounterAddedOnce {
				a.Trig.Flags4 &^= kop.Tf4PerCounter
			}
		}},
	} {
		n := lockstepRun(t, "evolve", kevolveDecks, 200, false, func(tbl *kop.Table) {
			for i := range tbl.Cards {
				for j := range tbl.Cards[i].Abilities {
					m.fn(&tbl.Cards[i].Abilities[j])
				}
			}
		})
		if n == 0 {
			t.Errorf("%s: no seed diverged with it dropped: the pool never exercises it", m.name)
		}
	}
}

// TestEvolveRecheckMutation: with the resolution recheck dropped some game
// diverges, so the pool reaches two creatures entering together (Raise the
// Alarm's tokens) beside a Dinosaur Egg.
func TestEvolveRecheckMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugEvolveNoRecheck = true
	defer func() { dirac.DebugEvolveNoRecheck = false }()
	if lockstepDiverges(t, "evolve", kevolveDecks, 400) {
		return
	}
	t.Fatal("no seed diverged with the resolution recheck dropped: the pool never has two evolve triggers on one source")
}
