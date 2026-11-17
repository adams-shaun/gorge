package oracle

import "testing"

// Sword Coast Serpent's adventure face is cast with a sacrificed mana source
// whose trigger asks the other player (a target, CR 603.3d). gorge's lowering
// answers that foreign decision and keeps the play (oracle gorge.go lower), so
// the kernel offers the face too (faceWitnessReaches). The recording is kfuzz
// -synth dr-mbc-bbaa pair 154.
func TestMBCSwordCoastSerpentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sword_coast_serpent")
}
