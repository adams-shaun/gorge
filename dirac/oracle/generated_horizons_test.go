package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Generated Horizons' "At the beginning of your upkeep, you create a Forest
// land token" (CopyPermanent DefinedName$ Forest) played decision by decision
// against gorge: the named basic land lowers to a table token, and its upkeep
// copy adds Forests until a cast plans off a census past 48 sources.
// TestStdSetReprosLockstep replays the repro; TestGeneratedHorizonsReprosReach
// requires the recorded game to make a Forest token and to put a spell on the
// stack with more than 48 untapped Forests, since a lockstep entry alone would
// pass a script that never cast a spell past the kernel's old 48-unit census
// cap.
//
// The game is recorded by TestGeneratedHorizonsRecord
// (GENERATED_HORIZONS_RECORD=1) on the custom table its decks need: both seats
// prefer Generated Horizons and otherwise pick at random from the seed's own
// stream.
func generatedHorizonsCase() effectLongCase {
	return effectLongCase{
		name:    "generated_horizons_copies_a_forest_past_48_sources",
		prefer:  []string{"Generated Horizons"},
		decks:   [2][]string{append(deckOf(8, "Generated Horizons"), deckOf(16, "Forest")...), deckOf(24, "Forest")},
		surface: "autopay",
		reached: generatedHorizonsPast48,
	}
}

// generatedHorizonsPast48: a Forest token entered the battlefield, and a spell
// was put on the stack while seat 0 had more than 48 untapped Forests since
// the previous cast, so an autopay cast there planned over more sources than
// the kernel's old census cap held.
func generatedHorizonsPast48(g *GorgeSide) bool {
	// The board is replayed from the event log, not read from the final state:
	// a long game can end with a player leaving it, which clears the
	// battlefield, while the cast that exercised the census happened earlier.
	// tapped is seat 0's Forest permanents by object, and peak is the high
	// water mark of untapped Forests since the previous spell was put on the
	// stack (a cast taps its sources before the card moves).
	tapped := map[state.ObjID]bool{}
	untapped, peak := 0, 0
	token, castPast48 := false, false
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.MoveZone:
			switch {
			case e.To == state.ZBattlefield:
				o := g.E.G.Obj(e.Obj)
				if o == nil || o.Face() == nil || o.Face().Name != "Forest" || e.Player != 0 {
					continue
				}
				if o.IsToken {
					token = true
				}
				tapped[e.Obj] = false
				untapped++
				if untapped > peak {
					peak = untapped
				}
			case e.From == state.ZBattlefield:
				if t, ok := tapped[e.Obj]; ok {
					if !t {
						untapped--
					}
					delete(tapped, e.Obj)
				}
			}
		case events.PutOnStack:
			if peak > 48 {
				castPast48 = true
			}
			peak = untapped
		case events.Tap:
			if t, ok := tapped[e.Obj]; ok && !t {
				tapped[e.Obj] = true
				untapped--
			}
		case events.Untap:
			if t, ok := tapped[e.Obj]; ok && t {
				tapped[e.Obj] = false
				untapped++
				if untapped > peak {
					peak = untapped
				}
			}
		}
	}
	return token && castPast48
}

// TestGeneratedHorizonsReprosReach replays the recorded game: no divergence,
// every scripted action offered, a Forest token made, and a cast past 48
// sources.
func TestGeneratedHorizonsReprosReach(t *testing.T) {
	reg := synthReg(t)
	c := generatedHorizonsCase()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	set := reproSet(t, "custom", r)
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, set, script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatal("the recorded game never makes a Forest token past 48 sources")
	}
}

// TestGeneratedHorizonsRecord plays the case's policy over seeds until one
// reaches the board, and writes the repro. It runs only with
// GENERATED_HORIZONS_RECORD=1; the game it writes is the committed repro.
func TestGeneratedHorizonsRecord(t *testing.T) {
	if os.Getenv("GENERATED_HORIZONS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := generatedHorizonsCase()
	for seed := uint64(1); seed <= 600; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
		set := reproSet(t, "custom", r)
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
