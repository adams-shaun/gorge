package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Mutable Explorer's "When this creature enters, create a tapped Mutavault
// token" (CopyPermanent DefinedName$ Mutavault | TokenTapped$ True) played
// decision by decision against gorge: the copy source is the card named, not
// an object (kop.ExtCopyNamed). TestStdSetReprosLockstep replays the repro;
// TestMutableExplorerReprosReach requires the recorded game to put a
// Mutavault token onto the battlefield, since a lockstep entry alone would
// pass a script that never cast the Explorer.
//
// The game is recorded by TestMutableExplorerRecord (MUTABLE_EXPLORER_RECORD=1)
// on the std-ecl pool: both seats prefer the Explorer and otherwise pick at
// random from the seed's own stream.
func mutableExplorerCase() effectLongCase {
	return effectLongCase{
		name:    "mutable_explorer_makes_a_tapped_mutavault",
		prefer:  []string{"Mutable Explorer"},
		decks:   [2][]string{append(deckOf(8, "Mutable Explorer"), deckOf(16, "Forest")...), deckOf(24, "Forest")},
		surface: "autopay",
		reached: mutavaultTokenEntered,
	}
}

// mutavaultTokenEntered: a Mutavault token entered the battlefield.
func mutavaultTokenEntered(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind != events.MoveZone || e.To != state.ZBattlefield {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Mutavault" {
			return true
		}
	}
	return false
}

// TestMutableExplorerReprosReach replays the recorded game: no divergence,
// every scripted action offered, a Mutavault token made.
func TestMutableExplorerReprosReach(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-ecl")
	c := mutableExplorerCase()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, set, script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatal("the recorded game never makes a Mutavault token")
	}
}

// TestMutableExplorerRecord plays the case's policy over seeds until one makes
// the token, and writes the repro. It runs only with MUTABLE_EXPLORER_RECORD=1;
// the game it writes is the committed repro.
func TestMutableExplorerRecord(t *testing.T) {
	if os.Getenv("MUTABLE_EXPLORER_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-ecl")
	c := mutableExplorerCase()
	for seed := uint64(1); seed <= 600; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth std-ecl"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, effectLongPolicy(m, c, seed))
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if !reached {
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions", seed, len(res.Actions))
		return
	}
	t.Fatalf("no seed reached %s", c.name)
}
