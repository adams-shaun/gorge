package oracle

import "testing"

// War Machine, Avenging Arsenal's attacks trigger grants double strike to
// each attacking modified creature its controller has (CR 700.9), and a
// Convulsing Licid its Animate has turned into an Aura attached to the
// attacker is a modification (CR 613.1d layer-4 type line, not the printed
// face). gorge read the attached object's printed type line, missed the
// Licid, and skipped the grant (decision 1037 of pair 41 of the
// tail-63-c000 evidence loop at land ae57987d13, projection
// Cards[50].Chars.Keywords ref=256 kernel=264).
func TestMBCFlamebornHellionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "modified_licid_aura")
}
