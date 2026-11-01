package kgen

import "testing"

// TestEmpowerGrantedLoyaltySVars pins the cross-object granted LOYALTY
// ability's SVar degradation (Avatar of Burgeoning Echoes). Gorge resolves a
// granted activated ability's SVar references off the RECIPIENT's face
// (engine/rules/resolution_ability_ctx.go abilityResolutionSVars), and the
// Jace token has no SVar table, so CounterNum$ X degrades to zero. A
// non-loyalty cross-grant keeps its build-time body: Food Fight's DealDamage
// still counts the named-source permanents.
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
			if v := ab.Prog[0].A; v.Kind != vConst || v.N != 0 {
				t.Errorf("%s: CounterNum$ X = %v, want 0", c.Name, v)
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
