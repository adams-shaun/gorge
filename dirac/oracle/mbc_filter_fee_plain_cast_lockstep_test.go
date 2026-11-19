package oracle

import "testing"

// CR 605.3b with CR 601.2g-h: a first-and-only filter mana ability whose
// activation carries a literal mana fee (Apprentice Wizard's "{U}, {T}: Add
// {C}{C}{C}") pays a plain cast inside the offer's affordability read. The
// gorge side offered Arcane Melee and Meloku the Clouded Mirror; the kernel
// priced only the four-mana casts (decision 428; tail-66-c000 pair 4, seed
// 48471, focus Ebon Dragon).
func TestMBCArcaneMeleeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_fee_plain_cast")
}
