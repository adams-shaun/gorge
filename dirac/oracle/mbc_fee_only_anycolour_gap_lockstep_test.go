package oracle

import "testing"

// Energy Refractor's "{2}: Add one mana of any color" beside a lone Swamp
// (tail-02-c000 pair 123, seed 7536856847925902617, decision 373, surface
// full, Lifebane Zombie's {1}{B}{B} in hand): the payer's floating pool is
// empty and only the Swamp makes {B}, so the plain census prices no cast of
// the Zombie. gorge's census does not price it either — a fee-only source
// that nets no mana adds no supply (potential_nogain.go manaFeeNetsNoMana,
// CR 106.1, 605.3b) — but it makes the census unrelaxable (a repeatable
// fee-only ability has no once-only relaxation, paymentPlanRelaxedAlternatives),
// so every play the walk lists takes a prefix search, and that search funds
// the Refractor's {2} by activating printed sources (one per fee, CR
// 601.2b), floats the Refractor's one mana of a chosen colour, and pays the
// cast. The kernel's census gap registers the ability (censusGapMana) but no
// model classifies it unrelaxable and no search step activates it, so
// cast(#55{Lifebane Zombie},-,0) stays off the kernel's menu and the pair
// parts at the options (options options).
func TestMBCLifebaneZombieReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "fee_only_anycolour_gap")
}
