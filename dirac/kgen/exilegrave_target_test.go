package kgen

import (
	"strings"
	"testing"
)

// TestExileToGraveyardTargetLowers: a targeted ChangeZone from exile to a
// graveyard lowers to the plain Return with the exile origin mask and the
// graveyard destination (Bronzebeak Foragers, Gelatinous Cube). Without the
// target, or from a battlefield, the move stays refused: the kernel's
// targeted exile->graveyard shape is the one the activated abilities use.
func TestExileToGraveyardTargetLowers(t *testing.T) {
	const script = `Name: Exile Grave Probe
ManaCost: B
Types: Creature Human
PT: 1/1
A:AB$ ChangeZone | Cost$ B | Origin$ Exile | Destination$ Graveyard | ValidTgts$ Creature.ExiledWithSource | TgtPrompt$ Choose a creature card exiled with CARDNAME
Oracle: {B}: Put target creature card exiled with CARDNAME into its owner's graveyard.
`
	lc := lowerProbe(t, script, "Exile Grave Probe", "")
	if r := probeRefusals(lc); r != "" {
		t.Fatalf("the targeted exile->graveyard move refused: %s", r)
	}
	if len(lc.Abilities) != 1 || len(lc.Abilities[0].Prog) != 1 {
		t.Fatalf("abilities %d, program %v", len(lc.Abilities), lc.Abilities)
	}
	ab := lc.Abilities[0]
	if ab.Targets[0].Zone != zExile {
		t.Errorf("target zone %d, want exile (%d)", ab.Targets[0].Zone, zExile)
	}
	in := ab.Prog[0]
	if in.Op != "Return" || in.C != zGraveyard || !strings.Contains(in.CS, "from=Exile") {
		t.Errorf("program %+v, want Return C=graveyard from=Exile", in)
	}

	// no target: the move is the source itself, which this shape does not move
	noTgt := strings.Replace(script, " | ValidTgts$ Creature.ExiledWithSource", "", 1)
	if r := probeRefusals(lowerProbe(t, noTgt, "Exile Grave Probe", "")); !strings.Contains(r, "ChangeZone to graveyard") {
		t.Errorf("without a target: refusals %q, want the graveyard move refusal", r)
	}
	// a target that does not read ExiledWithSource (Pull from Eternity's
	// shape) is not covered
	anyTgt := strings.Replace(script, "Creature.ExiledWithSource", "Creature", 1)
	if r := probeRefusals(lowerProbe(t, anyTgt, "Exile Grave Probe", "")); !strings.Contains(r, "ChangeZone to graveyard") {
		t.Errorf("an unlinked target: refusals %q, want the graveyard move refusal", r)
	}
	// a battlefield origin is a destroy-style move, not this path
	fromBF := strings.Replace(script, "Origin$ Exile", "Origin$ Battlefield", 1)
	if r := probeRefusals(lowerProbe(t, fromBF, "Exile Grave Probe", "")); r == "" {
		t.Errorf("from the battlefield: lowered, want it refused")
	}
}
