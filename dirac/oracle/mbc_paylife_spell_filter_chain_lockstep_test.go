package oracle

import "testing"

// CR 601.2g-h: Withering Boon ({1}{B}, "as an additional cost, pay 3 life",
// counter target creature spell) sits in p0's hand while its only untapped
// sources are a Swamp and Viridescent Bog ("{1}, {T}: Add {B}{G}"): the Swamp
// pays the Bog's fee and the Bog's two mana pay the spell. gorge's potential
// planner declines a fixed PayLife part ("shape:additional_cost") before any
// prefix search, so a filter land that its census cannot price never pays the
// cast and no cast is offered; the kernel offered it (tail-12, decision 117).
func TestMBCWitheringBoonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paylife_spell_filter_land_chain")
}
