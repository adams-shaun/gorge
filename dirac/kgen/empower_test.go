package kgen

import "testing"

// TestEmpowerGrantedLoyaltySVars pins where a cross-object granted ability
// reads its SVars: off the GRANTOR's face, as gorge does
// (engine/rules/resolution_ability_ctx.go grantedAbilitySVars). Avatar of
// Burgeoning Echoes' granted [-10] counts the lands you control on the Jace
// token that has no SVar table of its own, and Food Fight's DealDamage counts
// the named-source permanents on whichever artifact activates it.
func TestEmpowerGrantedLoyaltySVars(t *testing.T) {
	cards := lowerNamed(t, []string{"Avatar of Burgeoning Echoes", "Food Fight"})
	seen := 0
	for _, c := range cards {
		if len(c.Refusals) != 0 {
			t.Errorf("%s: refused: %v", c.Name, c.Refusals)
			continue
		}
		switch c.Name {
		case "Avatar of Burgeoning Echoes":
			seen++
			ab := c.Abilities[1]
			if !ab.Loyalty {
				t.Fatalf("%s: granted ability is not a loyalty ability", c.Name)
			}
			if len(ab.Prog) != 1 || ab.Prog[0].Op != "PutCounter" {
				t.Fatalf("%s: program %v", c.Name, ab.Prog)
			}
			if v := ab.Prog[0].A; v.Kind != vCount {
				t.Errorf("%s: CounterNum$ X = %v, want the grantor's Count body", c.Name, v)
			}
		case "Food Fight":
			seen++
			ab := c.Abilities[0]
			if ab.Name != "Granted" {
				t.Fatalf("%s: ability 0 is %q, want the granted ability", c.Name, ab.Name)
			}
			if len(ab.Prog) != 1 || ab.Prog[0].Op != "DealDamage" {
				t.Fatalf("%s: program %v", c.Name, ab.Prog)
			}
			if v := ab.Prog[0].A; v.Kind != vCount {
				t.Errorf("%s: DealDamage amount = %v, want the grantor's Count body", c.Name, v)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("saw %d carriers, want 2", seen)
	}
}
