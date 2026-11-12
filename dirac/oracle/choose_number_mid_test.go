package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// A resolution-time ChooseNumber plays identically in gorge and the kernel:
// the chooser is asked (or the Random$ draw matches), the answer is stored on
// the source and the sub-chain reads Count$ChosenNumber. Each case must reach
// a "number" Choose event on its card, and every seed must play without a
// divergence.
func chooseNumberMidCases() []effectLongCase {
	return []effectLongCase{
		{
			name:   "choose_number_mid_well_of_lost_dreams",
			prefer: []string{"Well of Lost Dreams", "Revitalize"},
			decks: [2][]string{
				append(deckOf(4, "Well of Lost Dreams"), append(deckOf(4, "Revitalize"), deckOf(32, "Plains")...)...),
				deckOf(40, "Plains"),
			},
			surface: "full",
			reached: chosenNumberOn("Well of Lost Dreams"),
		},
		{
			name:   "choose_number_mid_rampaging_aetherhood",
			prefer: []string{"Rampaging Aetherhood"},
			decks: [2][]string{
				append(deckOf(4, "Rampaging Aetherhood"), deckOf(36, "Forest")...),
				deckOf(40, "Forest"),
			},
			surface: "full",
			reached: chosenNumberOn("Rampaging Aetherhood"),
		},
		{
			name:   "choose_number_mid_territorial_aetherkite",
			prefer: []string{"Territorial Aetherkite"},
			decks: [2][]string{
				append(deckOf(4, "Territorial Aetherkite"), deckOf(36, "Mountain")...),
				deckOf(40, "Forest"),
			},
			surface: "full",
			reached: chosenNumberOn("Territorial Aetherkite"),
		},
		{
			name:   "choose_number_mid_expel_the_interlopers",
			prefer: []string{"Expel the Interlopers", "Grizzly Bears"},
			decks: [2][]string{
				append(deckOf(4, "Expel the Interlopers"), deckOf(36, "Plains")...),
				append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...),
			},
			surface: "full",
			reached: chosenNumberOn("Expel the Interlopers"),
		},
	}
}

// chosenNumberOn reads the finished gorge log for the card's own number
// choice (the Choose event api:ChooseNumber emits, Counter "number").
func chosenNumberOn(name string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		for _, e := range g.E.L.Events {
			if e.Kind != events.Choose || e.Counter != "number" {
				continue
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
				return true
			}
		}
		return false
	}
}

func TestChooseNumberMidLockstep(t *testing.T) {
	reg := synthReg(t)
	for _, c := range chooseNumberMidCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			reached := false
			for seed := uint64(1); seed <= 6; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"},
					Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				pol := effectLongPolicy(m, c, seed)
				res, got := effectLongPlay(t, reg, c, r, set, pol)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at decision %d: %s %s %v %s\nref %v\nker %v",
						seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
				}
				if got {
					reached = true
					break
				}
			}
			if !reached {
				t.Fatal("no seed reached the card's number choice")
			}
		})
	}
}
