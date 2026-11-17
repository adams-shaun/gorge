package oracle

import "testing"

// CR 602.2b and CR 605.3b: Katilda, Dawnhart Prime grants the Humans you
// control "{T}: Add one mana of any of this creature's colors" and has her own
// "{4}{G}{W}, {T}: Put a +1/+1 counter on each creature you control". Paying
// the activation taps Katilda as part of its cost, so Katilda's own granted
// mana is one source the activation cannot use; the lands beside her (one under
// Contaminated Ground, whose tap is a deferred source) and Opaline Unicorn pay
// {4}{G}{W}, so the activation is legal. gorge offers it. The kernel's grant
// search dropped the Taps-deferred land because Katilda was the only holder of
// the grant and the activation's own source is excluded from the search.
func TestMBCKatildaActivationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "katilda_activation_own_tap_grant")
}
