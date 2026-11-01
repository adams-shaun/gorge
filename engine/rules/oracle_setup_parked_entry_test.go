package rules

import (
	"slices"
	"strings"
	"testing"
)

// XMage's addCard(Zone.BATTLEFIELD) places a setup
// permanent without "entering", so no enters-the-battlefield trigger fires
// at the setup checkpoint. Two shapes escaped placeSetupCard's drop window:
//
//   - a PARKED entry: an as-enters choice (Gathering Stone's "choose a
//     creature type", CR 614.12) suspends the move until the setup drive
//     answers it, after the window has closed, so the ETB trigger queued
//     then fired alongside the turn-1 upkeep trigger (setup hand
//     [Grizzly Bears] vs XMage's [], graveyard [Jace Beleren] in both);
//   - an entry trigger scripted with no Destination$ (Bullseye, Death
//     Dealer's "When NICKNAME enters", ValidCard$ Card.Self only), which
//     the Destination$-Battlefield gate did not classify (setup life 18 vs
//     XMage's 20).
//
// Each row is the measured diverge's setup checkpoint.
func TestOracleSetupDropsParkedAndDestinationlessEntryTriggers(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		check     func(t *testing.T, snap OracleSnapshot, transcript string)
	}{
		{
			// The ETB look must not fire; the turn-1 upkeep look (the
			// trigger's Phase half) still does: it mills the non-Bear top.
			name: "Gathering Stone parked as-enters choice",
			raw:  `{"name":"stone","setup":{"p0":{"battlefield":["Gathering Stone"],"library_top":["Jace Beleren","Grizzly Bears","Forest","Glorious Anthem","Shock","Plains","Ornithopter"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[]}`,
			check: func(t *testing.T, snap OracleSnapshot, transcript string) {
				if h := snap.Players[0].Hand; len(h) != 0 {
					t.Errorf("p0 hand = %v at setup, want [] (the ETB look fired)", h)
				}
				if g := snap.Players[0].Graveyard; !slices.Equal(g, []string{"Jace Beleren"}) {
					t.Errorf("p0 graveyard = %v at setup, want [Jace Beleren] (the upkeep look)", g)
				}
				if strings.Contains(transcript, "trigger_order") {
					t.Errorf("setup ordered two Gathering Stone triggers; the ETB one must be dropped:\n%s", transcript)
				}
			},
		},
		{
			name: "Bullseye entry trigger without Destination$",
			raw:  `{"name":"bullseye","setup":{"p0":{"battlefield":["Bullseye, Death Dealer","Ornithopter","Sol Ring"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[]}`,
			check: func(t *testing.T, snap OracleSnapshot, transcript string) {
				if l := snap.Players[0].Life; l != 20 {
					t.Errorf("p0 life = %d at setup, want 20 (the ETB damage fired)", l)
				}
				if setupPermCount(snap, "Ornithopter") != 1 {
					t.Errorf("Ornithopter sacrificed at setup (the ETB fired): %+v", snap.Permanents)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, run := runFixtureScenario(t, tc.raw)
			if run.e.setupEntryDrops != nil {
				t.Errorf("setupEntryDrops = %+v after setup, want nil", run.e.setupEntryDrops)
			}
			tc.check(t, res.Snapshots[0], strings.Join(res.Transcript, "\n"))
		})
	}
}

// A triggered "you may pay" cost no potential mana can cover (Esoteric
// Duplicator's {2} with one Llanowar Elves) is posed decline-only, so no
// mana floats before the decline (XMage never activates a mana ability for a
// cost it cannot pay: its pool is empty at the measured checkpoint). Under
// the pay-first model (respay.go) the engine itself withholds both the pay
// and the mana window there, so the harness's "done" fallback
// (unpayableTriggerCostWindowDone) is never reached. A payable cost is
// offered pay first, and the harness's first-option fallback pays it, the
// planner tapping both Elves.
func TestOracleUnpayableTriggerCostWindowAnswersDone(t *testing.T) {
	res, _ := runFixtureScenario(t, `{"name":"duplicator","setup":{"p0":{"battlefield":["Esoteric Duplicator","Ornithopter","Llanowar Elves"],"hand":["Deadly Dispute"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Deadly Dispute","mana":"CB","answers":[{"kind":"choose","pick":["Ornithopter"]}]},{"op":"resolve","seat":0}]}`)
	last := res.Snapshots[len(res.Snapshots)-1]
	if pool := last.Players[0].Pool; pool != "" {
		t.Errorf("p0 pool = %q after the declined {2}, want empty (the fallback tapped Llanowar Elves)", pool)
	}
	transcript := strings.Join(res.Transcript, "\n")
	if strings.Contains(transcript, `choose -> ["Done"]`) || !strings.Contains(transcript, `choose -> ["Do not pay"]`) {
		t.Errorf("want the decline-only ask answered with no mana window:\n%s", transcript)
	}

	// A payable {2} (two Llanowar Elves) still taps through the window.
	res, _ = runFixtureScenario(t, `{"name":"duplicator-pay","setup":{"p0":{"battlefield":["Esoteric Duplicator","Ornithopter","Llanowar Elves","Llanowar Elves"],"hand":["Deadly Dispute"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Deadly Dispute","mana":"CB","answers":[{"kind":"choose","pick":["Ornithopter"]}]},{"op":"resolve","seat":0}]}`)
	if transcript := strings.Join(res.Transcript, "\n"); !strings.Contains(transcript, `choose -> ["Pay 2"]`) {
		t.Errorf("a payable cost must be offered and paid:\n%s", transcript)
	}
	if pool := res.Snapshots[len(res.Snapshots)-1].Players[0].Pool; pool != "" {
		t.Errorf("p0 pool = %q after the paid {2}, want empty (the planner tapped exactly the cost)", pool)
	}
}
