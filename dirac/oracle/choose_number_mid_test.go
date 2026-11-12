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
		{
			// The no-Max$ shape: the bare "Choose a number" defaults to 0..12.
			name:   "choose_number_mid_void",
			prefer: []string{"Void"},
			decks: [2][]string{
				append(deckOf(4, "Void"), append(deckOf(18, "Swamp"), deckOf(18, "Mountain")...)...),
				deckOf(40, "Swamp"),
			},
			surface: "full",
			reached: chosenNumberOn("Void"),
		},
		{
			// Random$ True: the draw must match gorge's engine rng exactly.
			name:   "choose_number_mid_hapatos_might",
			prefer: []string{"Hapato's Might", "Black Knight"},
			decks: [2][]string{
				append(deckOf(4, "Hapato's Might"), append(deckOf(4, "Black Knight"), deckOf(32, "Swamp")...)...),
				deckOf(40, "Swamp"),
			},
			surface: "full",
			reached: chosenNumberOn("Hapato's Might"),
		},
		{
			// Max$ 13: a literal above the default 0..12 list.
			name:   "choose_number_mid_by_invitation_only",
			prefer: []string{"By Invitation Only"},
			decks: [2][]string{
				append(deckOf(4, "By Invitation Only"), deckOf(36, "Plains")...),
				deckOf(40, "Plains"),
			},
			surface: "full",
			reached: chosenNumberOn("By Invitation Only"),
		},
		// The "pay any amount of {E}" family: Max$ Count$YourCountersEnergy,
		// read after the card's own PutCounter. A target creature is provided
		// for the burn shapes, though the number is chosen before the target.
		{
			name:   "choose_number_mid_a_galvanic_discharge",
			prefer: []string{"A-Galvanic Discharge", "Grizzly Bears"},
			decks: [2][]string{
				append(deckOf(4, "A-Galvanic Discharge"), deckOf(36, "Mountain")...),
				append(deckOf(4, "Grizzly Bears"), deckOf(36, "Forest")...),
			},
			surface: "full",
			reached: chosenNumberOn("A-Galvanic Discharge"),
		},
		{
			name:   "choose_number_mid_aether_spike",
			prefer: []string{"Aether Spike", "Lightning Bolt"},
			decks: [2][]string{
				append(deckOf(4, "Aether Spike"), deckOf(36, "Island")...),
				append(deckOf(8, "Lightning Bolt"), deckOf(32, "Mountain")...),
			},
			surface: "full",
			reached: chosenNumberOn("Aether Spike"),
		},
		{
			name:   "choose_number_mid_die_young",
			prefer: []string{"Die Young", "Grizzly Bears"},
			decks: [2][]string{
				append(deckOf(4, "Die Young"), deckOf(36, "Swamp")...),
				append(deckOf(4, "Grizzly Bears"), deckOf(36, "Forest")...),
			},
			surface: "full",
			reached: chosenNumberOn("Die Young"),
		},
		{
			name:   "choose_number_mid_galvanic_discharge",
			prefer: []string{"Galvanic Discharge", "Grizzly Bears"},
			decks: [2][]string{
				append(deckOf(4, "Galvanic Discharge"), deckOf(36, "Mountain")...),
				append(deckOf(4, "Grizzly Bears"), deckOf(36, "Forest")...),
			},
			surface: "full",
			reached: chosenNumberOn("Galvanic Discharge"),
		},
		{
			name:   "choose_number_mid_harnessed_lightning",
			prefer: []string{"Harnessed Lightning", "Grizzly Bears"},
			decks: [2][]string{
				append(deckOf(4, "Harnessed Lightning"), deckOf(36, "Mountain")...),
				append(deckOf(4, "Grizzly Bears"), deckOf(36, "Forest")...),
			},
			surface: "full",
			reached: chosenNumberOn("Harnessed Lightning"),
		},
		{
			// Max$ Count$ValidHand: a dynamic literal-less bound.
			name:   "choose_number_mid_rites_of_initiation",
			prefer: []string{"Rites of Initiation"},
			decks: [2][]string{
				append(deckOf(4, "Rites of Initiation"), deckOf(36, "Mountain")...),
				deckOf(40, "Mountain"),
			},
			surface: "full",
			reached: chosenNumberOn("Rites of Initiation"),
		},
		{
			// The same chosen byte across the as-enters choice and the upkeep
			// re-choice: at least two Choose events prove the mid ask ran.
			name:   "choose_number_mid_shapeshifter",
			prefer: []string{"Shapeshifter"},
			decks: [2][]string{
				append(deckOf(4, "Shapeshifter"), deckOf(36, "Wastes")...),
				deckOf(40, "Wastes"),
			},
			surface: "full",
			reached: chosenNumberAtLeast("Shapeshifter", 2),
		},
		{
			name:   "choose_number_mid_wrath_of_the_skies",
			prefer: []string{"Wrath of the Skies", "Grizzly Bears"},
			decks: [2][]string{
				append(deckOf(4, "Wrath of the Skies"), deckOf(36, "Plains")...),
				append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...),
			},
			surface: "full",
			reached: chosenNumberOn("Wrath of the Skies"),
		},
	}
}

// chosenNumberOn reads the finished gorge log for the card's own number
// choice (the Choose event api:ChooseNumber emits, Counter "number").
func chosenNumberOn(name string) func(g *GorgeSide) bool {
	return chosenNumberAtLeast(name, 1)
}

// chosenNumberAtLeast requires n number choices on the card: Shapeshifter
// chooses as it enters and again at each upkeep, so one event alone would not
// prove the resolution-time ask ran.
func chosenNumberAtLeast(name string, n int) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		got := 0
		for _, e := range g.E.L.Events {
			if e.Kind != events.Choose || e.Counter != "number" {
				continue
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
				got++
			}
		}
		return got >= n
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
