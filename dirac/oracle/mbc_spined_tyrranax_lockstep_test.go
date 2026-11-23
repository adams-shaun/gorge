package oracle

import "testing"

// CR 117.12 / 601.2h: Spined Tyrranax's "you may pay {2}{G}" at the beginning
// of combat, with every land tapped and one Eldrazi Spawn (Path of Annihilation
// lets it tap for any colour, and it can also be sacrificed for {C}) as the only
// source left: that is two mana at most, so the cost cannot be paid. gorge
// priced the any-colour source as unbounded and offered trigger_cost_pay beside
// the decline (decision 365, tail-10 pair 60); dirac offered the decline alone.
func TestMBCSpinedTyrranaxReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_pay_unfundable_any_color_spawn")
}
