package oracle

import "testing"

// CR 305.1/613: Urban Utopia (p0's Aura) on p1's Throne of the High City gives that land
// "{T}: Add one mana of any color", which p1 (the land's controller) can use, so p1 can pay
// {G}{G} for Finale of Devastation with X=0 from a Forest and the Throne. gorge offered the
// cast at decision 335 of 335 (tail-44 pair 7); the kernel did not.
func TestMBCFinaleOfDevastationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_mana_opp_aura_finale")
}
