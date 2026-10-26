package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestConditionTargetedReadsLKIPower pins CR 608.2h for the power/toughness
// half of a `ConditionDefined$ Targeted | ConditionPresent$
// Creature.powerLE2` gate: "if that creature had power 2 or less" reads the
// target as it last existed on the battlefield, not the card in exile.
// Driftgloom Coyote's ETB exiles the target before the gate runs, so a live
// read would recompute Burrowguard Mentor's characteristic-defining power off
// the battlefield (fewer creatures remain there). The cast-step expects assert
// the precondition: the Mentor's battlefield power is the value the gate must
// look back to.
func TestConditionTargetedReadsLKIPower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name        string
		battlefield []string
		livePT      string
		wantPT      string
		wantCounter int32
	}{
		{
			name:        "mentor power 3: no counter",
			battlefield: []string{"Burrowguard Mentor", "Grizzly Bears", "Grizzly Bears"},
			livePT:      "3/3",
			wantPT:      "3/4",
			wantCounter: 0,
		},
		{
			name:        "vanilla 2/2: counter",
			battlefield: []string{"Grizzly Bears"},
			livePT:      "2/2",
			wantPT:      "4/5",
			wantCounter: 1,
		},
		{
			name:        "mentor power 2: counter",
			battlefield: []string{"Burrowguard Mentor", "Grizzly Bears"},
			livePT:      "2/2",
			wantPT:      "4/5",
			wantCounter: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := "p1:Burrowguard Mentor"
			if len(tc.battlefield) == 1 && tc.battlefield[0] != "Burrowguard Mentor" {
				target = "p1:Grizzly Bears"
			}
			sc := oracleScenario{
				Setup: map[string]oracleSeat{
					"p0": {Hand: []string{"Driftgloom Coyote"}},
					"p1": {Battlefield: tc.battlefield},
				},
				Steps: []oracleStep{
					{Op: "cast", Seat: 0, Card: "p0:Driftgloom Coyote", Mana: "WWCCC",
						Expect: []oracleExpect{{Card: target, PT: tc.livePT}}},
					{Op: "resolve", Targets: []string{target}},
				},
				Expect: []oracleExpect{
					{Card: target, Zone: "exile"},
					{Card: "p0:Driftgloom Coyote", Zone: "battlefield", PT: tc.wantPT,
						Counters: map[string]int32{"P1P1": tc.wantCounter}},
				},
			}
			if fails, transcript, _ := runOracleScenario(reg, sc); len(fails) > 0 {
				t.Fatalf("%v\ntranscript:\n%v", fails, transcript)
			}
		})
	}
}
