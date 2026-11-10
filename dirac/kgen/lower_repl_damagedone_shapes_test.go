package kgen

import (
	"fmt"
	"strings"
	"testing"
)

const antiVenomProbe = `Name:Probe Venom
ManaCost:W
Types:Creature Symbiote
PT:5/5
R:Event$ DamageDone | ActiveZones$ Battlefield | ValidTarget$ Card.Self | ReplaceWith$ Counters | PreventionEffect$ True | AlwaysReplace$ True%s | Description$ probe.
SVar:Counters:DB$ PutCounter | Defined$ ReplacedTarget | CounterType$ %s | CounterNum$ X
SVar:X:ReplaceCount$DamageAmount
`

const wolverineProbe = `Name:Probe Wolverine
ManaCost:R
Types:Creature Mutant
PT:3/5
R:Event$ DamageDone | ActiveZones$ Battlefield | ValidTarget$ Card.Self | ReplaceWith$ Healing%s | Description$ probe.
SVar:Healing:DB$ HealDamage | Defined$ %s
`

// TestLowerReplDamagedoneShapes: Anti-Venom's prevent-and-counters and
// Wolverine's heal-then-deal DamageDone replacements on damage to
// themselves lower (lower_replmult.go selfDamageRepl); each variant off
// either shape still refuses by name.
func TestLowerReplDamagedoneShapes(t *testing.T) {
	checkCarrier(t, []string{"Anti-Venom, Horrifying Healer", "Wolverine, Fierce Fighter"}, nil,
		[]string{"PreventDamageCounters", "DamageHealFirst"})
	for _, c := range []struct{ name, src, want string }{
		{"Probe Venom", fmt.Sprintf(antiVenomProbe, "", "P1P1"), ""},
		{"Probe Venom", fmt.Sprintf(antiVenomProbe, " | ReplacementResult$ Updated", "P1P1"), "DamageDone prevent-and-counters shape"},
		{"Probe Venom", fmt.Sprintf(antiVenomProbe, " | ValidSource$ Creature", "P1P1"), "DamageDone self replacement ValidSource"},
		{"Probe Venom", fmt.Sprintf(antiVenomProbe, " | IsCombat$ True", "P1P1"), "DamageDone self replacement IsCombat"},
		{"Probe Venom", fmt.Sprintf(antiVenomProbe, "", "M1M1"), "DamageDone prevent-and-counters body"},
		{"Probe Wolverine", fmt.Sprintf(wolverineProbe, " | ReplacementResult$ Updated", "ReplacedTarget"), ""},
		{"Probe Wolverine", fmt.Sprintf(wolverineProbe, "", "ReplacedTarget"), "DamageDone heal replacement shape"},
		{"Probe Wolverine", fmt.Sprintf(wolverineProbe, " | ReplacementResult$ Updated | PreventionEffect$ True", "ReplacedTarget"), "DamageDone heal replacement shape"},
		{"Probe Wolverine", fmt.Sprintf(wolverineProbe, " | ReplacementResult$ Updated", "Self"), "DamageDone heal replacement body"},
	} {
		got := lowerScript(t, c.name, c.src)
		if c.want == "" {
			if len(got) != 0 {
				t.Errorf("%s in shape: refused %v\n%s", c.name, got, c.src)
			}
			continue
		}
		if !strings.Contains(strings.Join(got, "\n"), c.want) {
			t.Errorf("%s: want a refusal containing %q, got %v\n%s", c.name, c.want, got, c.src)
		}
	}
}
