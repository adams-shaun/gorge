//go:build !kprof

package neteval

func profStart() int64 { return 0 }
func profFeat(int64)   {}
func profNet(int64)    {}

// ProfNanos is the time spent encoding (feat) and in the network (net)
// since ProfReset (-tags kprof only; zero otherwise).
func ProfNanos() (feat, net int64) { return 0, 0 }
func ProfReset()                   {}
