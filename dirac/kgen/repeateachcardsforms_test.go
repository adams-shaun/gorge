package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// repeatEachCardsLoopForms: api:RepeatEach over cards without
// ChangeZoneTable$ True lowers as a plain per-card OpRepeatEach loop
// (C=RepeatPlain), the iterated card bound as the body's Remembered
// (RefEach). The carriers whose body the loop cannot hold refuse past the
// loop, on their body's own gap, never on ChangeZoneTable$.
var repeatEachCardsLoopForms = []formCase{
	{card: "Balan, Wandering Knight", form: "RepeatCards$ Equipment.YouCtrl, Attach Object$ Remembered Defined$ Self", lir: "RepeatEachCard C=1 F={types=0x0 sub=Equipment ctrl=1} skip=1\n     1 AttachObj obj=Self obj2=Each"},
	{card: "Mercadia's Downfall", form: "RepeatCards$ Creature.attacking, Pump Defined$ Remembered (its X filter is the gap)", refused: `unknown property "DefenderCtrlForRemembered"`},
	{card: "Cleansing", form: "RepeatCards$ Land, Destroy UnlessCost$ (a body that asks: the unless ask, then the Destroy)", lir: "RepeatEachCard C=1 F={types=0x1} skip=2\n     1 UnlessPaid obj=EachPlayer A=1 CS=PayLife skip=1\n     2 Destroy obj=Each"},
	{card: "Asinine Antics", form: "ChangeZoneTable$ True Role Token keeps its batched loop (C=0)", lir: "RepeatEachCard F="},
}

func TestRepeatEachCardsLoopForms(t *testing.T) {
	checkForms(t, repeatEachCardsLoopForms)
	// the Role probe without ChangeZoneTable$ is the plain loop (C=1); with
	// it, the batched one (C=0)
	for _, tc := range []struct {
		each string
		c    int
	}{
		{"RepeatCards$ Creature.OppCtrl | Zone$ Battlefield", kop.RepeatPlain},
		{"RepeatCards$ Creature.OppCtrl | Zone$ Battlefield | ChangeZoneTable$ True", 0},
	} {
		c := lowerScriptCard(t, "Probe Antics", repeatCardsProbe(tc.each, probeRole))
		saw := false
		for _, a := range c.Abilities {
			for _, in := range a.Prog {
				if in.Op == "RepeatEachCard" {
					saw = true
					if in.C != tc.c {
						t.Errorf("%s: RepeatEachCard C %d, want %d", tc.each, in.C, tc.c)
					}
				}
			}
		}
		if !saw || len(c.Refusals) != 0 {
			t.Errorf("%s: lowered no RepeatEachCard (refusals %v)", tc.each, c.Refusals)
		}
	}
}
