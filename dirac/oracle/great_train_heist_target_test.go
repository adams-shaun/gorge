package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// greatTrainHeistTargetRepro is the recorded game's name: Great Train Heist
// cast in its controller's combat phase with its third Spree mode (choose
// target opponent; the combat-damage Treasure effect over a TargetedPlayer
// capture that gorge's event-trigger registration never carries, so the kernel
// declares the target and drops the effect).
const greatTrainHeistTargetRepro = "great_train_heist_target_opponent_mode"

// TestGreatTrainHeistTargetReproRecord writes the repro: it runs only with
// GREAT_TRAIN_HEIST_RECORD=1, like TestGreatTrainHeistReproRecord.
func TestGreatTrainHeistTargetReproRecord(t *testing.T) {
	if os.Getenv("GREAT_TRAIN_HEIST_RECORD") == "" {
		return // a recording tool, not coverage
	}
	recordGreatTrainHeist(t, greatTrainHeistTargetRepro, 2, func(*GorgeSide) bool { return true })
}

// TestGreatTrainHeistTargetReproLockstep replays that game: no divergence,
// the third mode was picked, and gorge logged the Heist's chosen targets (the
// lone opponent is declared without an ask), so the dropped effect still
// leaves the mode's target in place on both engines.
func TestGreatTrainHeistTargetReproLockstep(t *testing.T) {
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", greatTrainHeistTargetRepro+".repro.json"))
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
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	script := &ScriptPolicy{Actions: r.Actions}
	modePicked, targeted := false, false
	rc.Finish = func(g *GorgeSide) {
		for _, e := range g.E.L.Events {
			if e.Kind != events.TargetsChosen {
				continue
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == greatTrainHeistName {
				targeted = true
			}
		}
	}
	res := Lockstep(m, r.Seed, k, PolicyFunc(func(c *PickCtx) Canon {
		a := script.Pick(c)
		if c.D.Kind == DModes && a.Kind() == ActMode && a.Mode() == 2 {
			modePicked = true
		}
		return a
	}), rc)
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.StallUnmodelled {
		t.Fatalf("dirac stopped at a rule it does not model: %s", res.Stall)
	}
	if !modePicked {
		t.Fatal("the recorded game never picked Great Train Heist's third Spree mode")
	}
	if !targeted {
		t.Fatal("gorge logged no chosen targets for Great Train Heist's third Spree mode")
	}
}
