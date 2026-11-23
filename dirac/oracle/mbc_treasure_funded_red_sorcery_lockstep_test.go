package oracle

import "testing"

// TestMBCDestructiveTamperingPoolGlob replays the pool's recordings by glob
// (repro_glob_test.go's table needs a test to glob each pool).
func TestMBCDestructiveTamperingPoolGlob(t *testing.T) {
	t.Parallel()
	stdReprosGlobPools(t, "dr-mbc-aabb")
}
