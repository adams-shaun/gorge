package oracle

import "testing"

// Base Camp's any-colour mana is spendable on "an ability of a Cleric, Rogue,
// Warrior, or Wizard" and Forge scripts that as Activated.Warrior+inZoneBattlefield:
// gorge's restrictValidTermMatches denies it to an ability whose source is off
// the battlefield. Viscera Dragger (a Warrior) in the graveyard with Study and
// Base Camp the only untapped lands: Unearth {1}{B} is not payable, because
// Base Camp's {B} is restricted to battlefield sources. dirac once offered the
// Unearth anyway (tail-29 pair 11, decision 1044: gorge [pass], kernel
// [pass, activate Viscera Dragger]).
func TestMBCVisceraDraggerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unearth_base_camp_warrior_graveyard")
}
