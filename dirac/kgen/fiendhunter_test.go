package kgen

import (
	"strings"
	"testing"
)

// TestFiendHunterRemPersist: a leaves trigger that returns the enters
// trigger's exiled card through Defined$ DirectRemembered (gorge's alias of
// RememberedLKI) lowers as the remembered-persist pair, as a Defined$
// Remembered return does. Fiend Hunter carried this shape until upstream
// Forge rewrote it to Defined$ ExiledWith, so a synthetic probe keeps the
// shape covered; Fiend Hunter itself now lowers through the exiled-with set.
func TestFiendHunterRemPersist(t *testing.T) {
	const probe = `Name:Remembered Return Probe
ManaCost:1 W W
Types:Creature Human Cleric
PT:1/3
T:Mode$ ChangesZone | Destination$ Battlefield | Origin$ Any | ValidCard$ Card.Self | OptionalDecider$ You | Execute$ PrbAway | TriggerDescription$ probe enters
SVar:PrbAway:DB$ ChangeZone | ValidTgts$ Creature.Other | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True | ForgetOtherRemembered$ True | TgtPrompt$ probe target
T:Mode$ ChangesZone | Destination$ Any | Origin$ Battlefield | ValidCard$ Card.Self | Execute$ PrbBack | TriggerDescription$ probe leaves
SVar:PrbBack:DB$ ChangeZone | Destination$ Battlefield | Origin$ Exile | Defined$ DirectRemembered
`
	c := lowerScriptCard(t, "Remembered Return Probe", probe)
	var reasons []string
	for _, r := range c.Refusals {
		reasons = append(reasons, r.Node+": "+r.Reason)
	}
	if len(reasons) != 0 {
		t.Fatalf("probe refused: %s", strings.Join(reasons, "; "))
	}
	if !c.RemPersist {
		t.Fatal("probe lowered without CardDef.RemPersist: the leaves trigger would read an empty set")
	}

	fh := lowerCorpusCard(t, "Fiend Hunter")
	for _, r := range fh.Refusals {
		t.Errorf("Fiend Hunter refused: %s: %s", r.Node, r.Reason)
	}
	if !fh.ExiledWith {
		t.Error("Fiend Hunter lowered without the exiled-with set its Defined$ ExiledWith return reads")
	}
}
