package oracle

import "testing"

// Sword Coast Serpent's adventure face was offered by the kernel while gorge
// held it back at priority. The recording is kfuzz -synth dr-mbc-bbaa pair 154.
func TestMBCSwordCoastSerpentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sword_coast_serpent")
}
