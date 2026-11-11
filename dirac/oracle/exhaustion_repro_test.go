package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Exhaustion ("Creatures and lands target opponent controls don't untap during
// their next untap step", CR 611.2b) played decision by decision against
// gorge: the opponent's Grizzly Bears (attackers, so tapped) and Forests tapped when it resolves stay
// tapped through that player's next untap step and untap in the one after.
// TestStdSetReprosLockstep replays the repro; TestExhaustionReprosReach
// requires the recorded game to reach the observable below.
// TestExhaustionRecord (EXHAUSTION_RECORD=1) writes it.
const exhaustionRepro = "exhaustion_target_stays_tapped_through_its_untap_step"

func exhaustionCase() exileMovedCase {
	const bear = "Grizzly Bears"
	return exileMovedCase{
		effectLongCase: effectLongCase{
			name:   exhaustionRepro,
			prefer: []string{bear, "Exhaustion"},
			decks: [2][]string{
				append(deckOf(8, "Exhaustion"), deckOf(16, "Island")...),
				append(deckOf(8, bear), deckOf(16, "Forest")...),
			},
			surface: "autopay",
			reached: exhaustionHeld,
		},
		gated: "Exhaustion", needOut: bear,
	}
}

// exhaustionHeld: after seat 0's Exhaustion resolved, at least one permanent
// seat 1 had tapped (at least one of them a land, and at least one a
// creature) stayed tapped through seat 1's next untap step, and the permanents
// of that set untapped in a later one.
func exhaustionHeld(g *GorgeSide) bool {
	var spell string
	active := state.PlayerID(0)
	tapped := map[state.ObjID]bool{}
	var held map[state.ObjID]bool // the tapped set when Exhaustion resolved
	inUntap, heldStep, steps := false, false, 0
	var heldLand, heldCreature, released bool
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.Tap:
			tapped[e.Obj] = true
		case events.Untap:
			if held != nil && held[e.Obj] {
				switch {
				case inUntap && steps == 1:
					return false // untapped in the step the Effect should have skipped
				case steps >= 2:
					released = true
				}
			}
			delete(tapped, e.Obj)
		case events.TurnChange:
			active = e.Player
		case events.StepChange:
			inUntap = e.Step == state.StepUntap
			if inUntap && held != nil && active == 1 {
				steps++
				heldStep = heldStep || steps == 1
			}
		case events.Resolve:
			if o := g.E.G.Obj(e.Obj); held == nil && o != nil && o.Face() != nil && o.Face().Name == "Exhaustion" && o.Controller == 0 {
				spell = o.Face().Name
				held = map[state.ObjID]bool{}
				for id := range tapped {
					if p := g.E.G.Obj(id); p != nil && p.Controller == 1 && p.Zone == state.ZBattlefield && p.Face() != nil {
						held[id] = true
						heldLand = heldLand || p.Face().IsLand()
						heldCreature = heldCreature || p.Face().IsCreature()
					}
				}
			}
		}
	}
	return spell != "" && heldStep && heldLand && heldCreature && released
}

func TestExhaustionReprosReach(t *testing.T) {
	reg := synthReg(t)
	c := exhaustionCase()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c.effectLongCase, r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatalf("the recorded game never reaches %s", c.name)
	}
}

func TestExhaustionRecord(t *testing.T) {
	if os.Getenv("EXHAUSTION_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	c := exhaustionCase()
	set := printedDelveSet(t, c.decks)
	for seed := uint64(1); seed <= 600; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, exileMovedPolicy(m, c, seed))
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
