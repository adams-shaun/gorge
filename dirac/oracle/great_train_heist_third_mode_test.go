package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// greatTrainHeistThirdModeRepro is the recorded game's name: Great Train
// Heist cast with its third Spree mode (choose target opponent; whenever a
// creature you control deals combat damage to that player this turn, create a
// tapped Treasure) before a Reckless Lackey attacks and connects.
const greatTrainHeistThirdModeRepro = "great_train_heist_third_mode_treasure"

// greatTrainHeistThirdModeCase: seat 0 holds Heists, Lackeys (R 1/2, haste,
// first strike) and Mountains, so the Lackey and the Heist's {R} mode cost
// are payable and the Lackey can attack at once; seat 1 holds only Forests.
func greatTrainHeistThirdModeCase() [2][]string {
	return [2][]string{
		append(append(deckOf(8, greatTrainHeistName), deckOf(8, "Reckless Lackey")...), deckOf(24, "Mountain")...),
		deckOf(40, "Forest"),
	}
}

// greatTrainHeistTreasure reports whether seat 0 controls a tapped Treasure
// token: the Heist's mode made it, so its DamageDone trigger fired for the
// Lackey's combat damage to the targeted opponent.
func greatTrainHeistTreasure(g *GorgeSide) bool {
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		o := g.E.G.Obj(id)
		if o != nil && o.IsToken && o.Tapped && o.Face() != nil && o.Face().Name == "Treasure Token" {
			return true
		}
	}
	return false
}

// greatTrainHeistThirdModePolicy plays a land, casts a Lackey, casts the Heist
// (mode 3 only) in a main phase once a Lackey has been cast, attacks with
// everything, and otherwise passes. picked is set when it answers the mode
// pick.
func greatTrainHeistThirdModePolicy(m *Matchup, picked *bool) Policy {
	lackey := false
	return PolicyFunc(func(c *PickCtx) Canon {
		switch c.D.Kind {
		case DPriority:
			for _, o := range c.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Reckless Lackey" {
					lackey = true
					return o
				}
			}
			if c.G != nil && lackey && c.G.E.G.Step == state.StepMain1 {
				for _, o := range c.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == greatTrainHeistName {
						return o
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
				if o.Kind() == ActMode && o.Mode() == 2 {
					*picked = true
					return o
				}
			}
			for _, o := range c.D.Options {
				if o.Kind() == ActDone {
					return o
				}
			}
		case DTarget:
			for _, o := range c.D.Options {
				if o.Kind() == ActTarget && o.Tgt() >= RefPlayer {
					return o
				}
			}
		case DAttackers:
			for _, o := range c.D.Options {
				if o.Kind() == ActAttack {
					return o
				}
			}
		}
		return c.D.Options[0]
	})
}

// TestGreatTrainHeistThirdModeReproRecord plays the policy until a game makes
// the Heist's Treasure and writes the repro. It runs only with
// GREAT_TRAIN_HEIST_RECORD=1. The full surface is the recorded one: gorge's
// autopay planner declines a Spree face (shape:modal_cost), so the mode cost
// is paid through the manual window.
func TestGreatTrainHeistThirdModeReproRecord(t *testing.T) {
	if os.Getenv("GREAT_TRAIN_HEIST_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := greatTrainHeistThirdModeCase()
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
		rc.Finish = func(g *GorgeSide) { reached = greatTrainHeistTreasure(g) }
		res := Lockstep(m, r.Seed, k, greatTrainHeistThirdModePolicy(m, &picked), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if !reached || !picked {
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", greatTrainHeistThirdModeRepro+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions", seed, len(res.Actions))
		return
	}
	t.Fatal("no seed in 1..100 reached the third mode's Treasure")
}

// TestGreatTrainHeistThirdModeReproLockstep replays the recorded game on its
// own std-otj table: no divergence, and the tapped Treasure is on seat 0's
// battlefield, so the mode's trigger fired on both engines (a replay that
// agrees on an empty board is not the fix).
func TestGreatTrainHeistThirdModeReproLockstep(t *testing.T) {
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", greatTrainHeistThirdModeRepro+".repro.json"))
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
	// The replay stops where the recording did: the tapped Treasure untaps at
	// its controller's next untap step, so a longer game would not hold it.
	rc.MaxDecisions = len(r.Actions)
	treasure := false
	rc.Finish = func(g *GorgeSide) { treasure = greatTrainHeistTreasure(g) }
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
	if !treasure {
		t.Fatal("seat 0 has no tapped Treasure token: the third mode's trigger never fired")
	}
}
