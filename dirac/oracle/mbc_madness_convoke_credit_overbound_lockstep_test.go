package oracle

import "testing"

// TestMBCMadnessConvokeCreditOverboundGlob replays the pool's other recordings
// by glob, so the new recording is replayed without a Go edit.
func TestMBCMadnessConvokeCreditOverboundGlob(t *testing.T) {
	stdReprosGlobPools(t, "dr-mbc-baba")
}
