package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestCutPropulsionDoublesForGrantedFlying: "if that creature has flying, it
// deals twice that much damage to itself" reads the creature's flying as it
// stands (CR 702.9b), whether printed or granted by an Equipment or an
// Aura: Sami, Ship's Engineer under Auxiliary Boosters took 3 of the 6.
func TestCutPropulsionDoublesForGrantedFlying(t *testing.T) {
	t.Parallel()
	cut := corpusCardText(t, "c/cut_propulsion.txt")
	const (
		plain   = "Name:Plain Ox\nManaCost:1 G\nTypes:Creature Ox\nPT:2/6\nOracle:x\n"
		printed = "Name:Winged Ox\nManaCost:1 G\nTypes:Creature Ox\nPT:2/6\nK:Flying\nOracle:x\n"
		granter = "Name:Wing Gift\nManaCost:1 G\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ Flying | Description$ Yours fly.\nOracle:x\n"
	)
	for _, tc := range []struct {
		name   string
		ox     string
		grants bool
		want   int
	}{
		{"no_flying", plain, false, 2},
		{"printed_flying", printed, false, 4},
		{"granted_flying", plain, true, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, _ := newFixtureDeck(t, 91, cut, tc.ox, granter)
			ox := putCreature(t, e, 0, tc.ox)
			if tc.grants {
				moveSeeded(t, e, 0, granter, state.ZBattlefield)
			}
			addMana(t, e, 0, "RRR")
			e.Advance()
			castSpellAtTarget(t, e, findInHand(e, 0, "Cut Propulsion"), ox)
			if got := int(e.G.Obj(ox).Damage); got != tc.want {
				t.Fatalf("damage %d, want %d", got, tc.want)
			}
		})
	}
}
