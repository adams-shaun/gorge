package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A delayed trigger remembering a multi-object target slot (RememberObjects$
// RememberedLKI) played decision by decision against gorge: Temporal
// Tribulation exiles two target permanents, Hide on the Ceiling X and Eerie
// Interlude any number of creatures,
// each returning them at the next end step. gorge registers one delayed
// trigger for the whole set and its ChangeZone returns the set as one effect,
// so the kernel's DpMany program returns every object in registration order as
// one entering event. TestStdSetReprosLockstep replays the repros;
// TestDelayedLkiMultitargetReprosReach requires each recorded game to reach
// two returns in one end step.
//
// The games are recorded by TestDelayedLkiMultitargetRecord
// (DELAYED_LKI_MULTITARGET_RECORD=1) on the custom pool.
func delayedLkiMultitargetCases() []effectLongCase {
	return []effectLongCase{
		{
			name:   "temporal_tribulation_returns_both_exiled_together",
			prefer: []string{"Temporal Tribulation", "Grizzly Bears"},
			decks: [2][]string{
				append(append(deckOf(8, "Temporal Tribulation"), deckOf(8, "Grizzly Bears")...), deckOf(8, "Island")...),
				deckOf(24, "Mountain"),
			},
			surface: "autopay",
			reached: twoReturnedInOneEndStep,
		},
		{
			name:   "hide_on_the_ceiling_returns_x_exiled_together",
			prefer: []string{"Ornithopter", "Hide on the Ceiling"},
			decks: [2][]string{
				append(append(deckOf(6, "Hide on the Ceiling"), deckOf(10, "Ornithopter")...), deckOf(8, "Island")...),
				deckOf(24, "Mountain"),
			},
			surface: "full",
			reached: twoReturnedInOneEndStep,
		},
		{
			name:   "eerie_interlude_returns_any_number_exiled_together",
			prefer: []string{"Ornithopter", "Eerie Interlude"},
			decks: [2][]string{
				append(append(deckOf(6, "Eerie Interlude"), deckOf(10, "Ornithopter")...), deckOf(8, "Plains")...),
				deckOf(24, "Mountain"),
			},
			surface: "autopay",
			reached: twoReturnedInOneEndStep,
		},
	}
}

// twoReturnedInOneEndStep: two or more cards moved from exile to the
// battlefield within one end step, and two or more left the battlefield for
// exile in the turn before it (the spell's own exile).
func twoReturnedInOneEndStep(g *GorgeSide) bool {
	step := state.Step(0)
	back, out := 0, 0
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.StepChange:
			if step == state.StepEnd && back >= 2 && out >= 2 {
				return true
			}
			step, back = e.Step, 0
			if e.Step == state.StepUpkeep {
				out = 0
			}
		case events.MoveZone:
			if e.From == state.ZBattlefield && e.To == state.ZExile {
				out++
			}
			if step == state.StepEnd && e.From == state.ZExile && e.To == state.ZBattlefield {
				back++
			}
		}
	}
	return step == state.StepEnd && back >= 2 && out >= 2
}

// TestDelayedLkiMultitargetReprosReach replays the recorded games: no
// divergence, every scripted action offered, the shape reached.
func TestDelayedLkiMultitargetReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range delayedLkiMultitargetCases() {
		t.Run(c.name, func(t *testing.T) {
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
				t.Fatalf("the recorded game never returns two exiled cards in one end step (%s)", c.name)
			}
		})
	}
}

// TestDelayedLkiMultitargetRecord plays each case's policy over seeds until one
// reaches its shape, and writes the repro. It runs only with
// DELAYED_LKI_MULTITARGET_RECORD=1.
func TestDelayedLkiMultitargetRecord(t *testing.T) {
	if os.Getenv("DELAYED_LKI_MULTITARGET_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range delayedLkiMultitargetCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 600; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
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
			t.Fatalf("no seed reached the shape of %s", c.name)
		})
	}
}

// TestDelayedLkiMultitargetMutation: with the delayed body returning only the
// first object its entry names, the recorded games diverge from gorge, which
// returns the whole set.
func TestDelayedLkiMultitargetMutation(t *testing.T) {
	reg := synthReg(t)
	dirac.DebugDelayedManyFirstOnly = true
	defer func() { dirac.DebugDelayedManyFirstOnly = false }()
	for _, c := range delayedLkiMultitargetCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			res, _ := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), &ScriptPolicy{Actions: r.Actions})
			if res.Div == nil {
				t.Fatal("the mutation did not diverge")
			}
		})
	}
}
