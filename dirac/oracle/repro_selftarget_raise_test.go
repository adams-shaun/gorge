package oracle

import (
	"os"
	"path/filepath"
	"testing"
)

// Icefall Regent carries "Spells your opponents cast that target CARDNAME
// cost {2} more to cast", a battlefield static (lowered with Zone 0). It taxes
// spells that target the permanent; it does not price the card's own cast from
// hand (CR 113.6, 601.2f), so on the autopay surface both engines plan the
// cast from untapped Islands, and the kernel no longer offers only pass.
//
// The game is recorded by TestSelfTargetRaiseReproRecord
// (SELF_TARGET_RAISE_RECORD=1) on the kernel table built from the decks; the
// repro row is TestStdSetReprosLockstep's
// battlefield_self_target_raise_does_not_decline_own_cast.
func selfTargetRaiseCase() effectLongCase {
	return effectLongCase{
		name:   "battlefield_self_target_raise_does_not_decline_own_cast",
		prefer: []string{"Icefall Regent", "Wall of Wood"},
		decks: [2][]string{
			append(deckOf(8, "Icefall Regent"), deckOf(16, "Island")...),
			append(deckOf(8, "Wall of Wood"), deckOf(16, "Forest")...),
		},
		surface: "autopay",
		reached: durationHeld("Icefall Regent", "Wall of Wood"),
	}
}

// TestSelfTargetRaiseReprosReach replays the recorded game: no divergence,
// every scripted action offered, and Icefall Regent's enters trigger tapped
// the Wall through one of its controller's untap steps. The reach check keeps
// the lockstep entry honest: a script that skipped the cast would not lock.
func TestSelfTargetRaiseReprosReach(t *testing.T) {
	reg := synthReg(t)
	c := selfTargetRaiseCase()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatalf("the recorded game never reaches the lock of %s", c.name)
	}
}

// TestSelfTargetRaiseReproRecord plays the case's policy over seeds until one
// reaches the lock, and writes the repro. It runs only with
// SELF_TARGET_RAISE_RECORD=1; the game it writes is the committed repro.
func TestSelfTargetRaiseReproRecord(t *testing.T) {
	if os.Getenv("SELF_TARGET_RAISE_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := selfTargetRaiseCase()
	set := printedDelveSet(t, c.decks)
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
			Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
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
	t.Fatalf("no seed reached the lock of %s", c.name)
}
