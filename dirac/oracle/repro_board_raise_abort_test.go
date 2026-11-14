package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Kasmina, Enigmatic Mentor taxes every spell its controller's opponents cast
// at a creature or planeswalker Kasmina's controller controls ({2} more). A
// seat whose only creature targets are taxed and whose mana pays the printed
// cost alone (Swelter, {3}{R}, on exactly four lands) has no target it can
// pay for: gorge withdraws the cast ("cast aborted: no legal target"), and the
// kernel's target menu drops the taxed candidates the same way for a
// battlefield raise as for a raise on the cast card itself (CR 601.2c, 601.2f).
//
// The game is recorded by TestBoardRaiseAbortReproRecord
// (PINS_RECORD=1) on the kernel table built from the decks; the
// repro row is TestStdSetReprosLockstep's
// board_raise_leaves_no_affordable_target_aborts_cast.
func boardRaiseAbortCase() effectLongCase {
	return effectLongCase{
		name:   "board_raise_leaves_no_affordable_target_aborts_cast",
		prefer: []string{"Kasmina, Enigmatic Mentor", "Merfolk of the Pearl Trident", "Swelter"},
		decks: [2][]string{
			append(append(deckOf(4, "Kasmina, Enigmatic Mentor"), deckOf(8, "Merfolk of the Pearl Trident")...), deckOf(12, "Island")...),
			append(deckOf(8, "Swelter"), deckOf(16, "Mountain")...),
		},
		surface: "full",
		reached: swelterCastWithdrawnUnderKasmina,
	}
}

// swelterCastWithdrawnUnderKasmina: with Kasmina on the battlefield, a Swelter
// went to the stack and came straight back to its owner's hand.
func swelterCastWithdrawnUnderKasmina(g *GorgeSide) bool {
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	kasmina := false
	for _, e := range g.E.L.Events {
		if e.Kind != events.MoveZone {
			continue
		}
		switch {
		case e.To == state.ZBattlefield && name(e.Obj) == "Kasmina, Enigmatic Mentor":
			kasmina = true
		case kasmina && e.From == state.ZStack && e.To == state.ZHand && name(e.Obj) == "Swelter":
			return true
		}
	}
	return false
}

// TestBoardRaiseAbortReprosReach replays the recorded game: no divergence,
// every scripted action offered, and a Swelter cast was withdrawn.
func TestBoardRaiseAbortReprosReach(t *testing.T) {
	reg := synthReg(t)
	c := boardRaiseAbortCase()
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

// TestBoardRaiseAbortReproRecord plays the case's policy over seeds until one
// reaches the withdrawn cast, and writes the repro. It runs only with
// PINS_RECORD=1; the game it writes is the committed repro.
func TestBoardRaiseAbortReproRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := boardRaiseAbortCase()
	set := printedDelveSet(t, c.decks)
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
			Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom",
			Why: "Kasmina, Enigmatic Mentor taxes every spell targeting its controller's creatures, so Swelter (CR 601.2c, 601.2f) has no target an untapped {3}{R} of Mountains pays for: both engines withdraw the cast (\"cast aborted: no legal target\") with the witness mana still floating"}
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
