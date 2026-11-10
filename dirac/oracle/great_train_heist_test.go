package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// greatTrainHeistName is the card this repro pins.
const greatTrainHeistName = "Great Train Heist"

// greatTrainHeistRepro is the recorded game's name (the repro row in
// TestStdSetReprosLockstep).
const greatTrainHeistRepro = "great_train_heist_extra_combat_in_own_combat"

// greatTrainHeistCase is the recorded game's decks: seat 0 holds the Heist and
// Mountains, seat 1 only Forests, so a combat phase with four lands untapped
// can cast the first Spree mode.
func greatTrainHeistCase() [2][]string {
	return [2][]string{
		append(deckOf(8, greatTrainHeistName), deckOf(32, "Mountain")...),
		deckOf(40, "Forest"),
	}
}

// greatTrainHeistReached: the extra-phase queue received Great Train Heist's
// combat grant spliced after end of combat, so its ConditionPhases$ gate held
// when the spell resolved.
func greatTrainHeistReached(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind != events.ExtraPhase || e.Amount <= 0 || e.Step != state.StepEndCombat || len(e.IDs) == 0 || state.Step(e.IDs[0]) != state.StepBeginCombat {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == greatTrainHeistName {
			return true
		}
	}
	return false
}

// greatTrainHeistPolicy plays a land, casts Great Train Heist in a combat
// phase (the one Spree mode given, then Done), and otherwise passes; the game
// needs no other play. picked is set when it answers that mode pick.
func greatTrainHeistPolicy(m *Matchup, mode int, picked *bool) Policy {
	return PolicyFunc(func(c *PickCtx) Canon {
		switch c.D.Kind {
		case DPriority:
			if c.G != nil {
				st := c.G.E.G.Step
				if st >= state.StepBeginCombat && st <= state.StepEndCombat {
					for _, o := range c.D.Options {
						if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == greatTrainHeistName {
							return o
						}
					}
				}
			}
			for _, o := range c.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, o := range c.D.Options {
				if o.Kind() == ActPass {
					return o
				}
			}
		case DModes:
			for _, o := range c.D.Options {
				if o.Kind() == ActMode && int(o.Mode()) == mode {
					*picked = true
					return o
				}
			}
			for _, o := range c.D.Options {
				if o.Kind() == ActDone {
					return o
				}
			}
		}
		return c.D.Options[0]
	})
}

// TestGreatTrainHeistReproRecord plays the case's policy until a game casts
// the Heist in its controller's combat phase and writes the repro. It runs
// only with GREAT_TRAIN_HEIST_RECORD=1; the game it writes is the committed
// repro replayed by TestStdSetReprosLockstep and
// TestGreatTrainHeistReproLockstep. The full surface is the recorded one:
// gorge's autopay planner declines a Spree face (shape:modal_cost), so the
// mode cost is paid through the manual window.
func TestGreatTrainHeistReproRecord(t *testing.T) {
	if os.Getenv("GREAT_TRAIN_HEIST_RECORD") == "" {
		return // a recording tool, not coverage
	}
	recordGreatTrainHeist(t, greatTrainHeistRepro, 0, greatTrainHeistReached)
}

// recordGreatTrainHeist plays the case's policy (the Spree mode given) until a
// game reaches the state its reached check names, and writes the repro.
func recordGreatTrainHeist(t *testing.T, repro string, mode int, reachedBy func(g *GorgeSide) bool) {
	t.Helper()
	reg := synthReg(t)
	c := greatTrainHeistCase()
	set := cardsettest.Synth(t, "std-otj")
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	for seed := uint64(1); seed <= 100; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"great_train_heist_0", "great_train_heist_1"}, Cards: c,
			Seed: seed, PassElision: true, Surface: "full", Kernel: "std-otj"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: false, PassElision: true}
		k, err := SetSide(set, tbl, cfg)(m, r.Seed)
		if err != nil {
			t.Fatal(err)
		}
		reached, picked := false, false
		rc := RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20}
		rc.Finish = func(g *GorgeSide) { reached = reachedBy(g) }
		res := Lockstep(m, r.Seed, k, greatTrainHeistPolicy(m, mode, &picked), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if !reached || !picked {
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", repro+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions", seed, len(res.Actions))
		return
	}
	t.Fatal("no seed reached Great Train Heist's extra combat")
}

// TestGreatTrainHeistReproLockstep replays the recorded game on its own
// std-otj table: no divergence, and the Heist's extra combat grant is in the
// log, so the script did cast it in a combat phase.
func TestGreatTrainHeistReproLockstep(t *testing.T) {
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", greatTrainHeistRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	set := reproSet(t, "std-otj", r)
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	reached := false
	rc.Finish = func(g *GorgeSide) { reached = greatTrainHeistReached(g) }
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	script := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, script, rc)
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.StallUnmodelled {
		t.Fatalf("dirac stopped at a rule it does not model: %s", res.Stall)
	}
	if !reached {
		t.Fatal("the recorded game never added Great Train Heist's extra combat")
	}
}
