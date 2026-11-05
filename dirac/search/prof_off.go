//go:build !kprof

package search

// Phase timing is compiled in only with -tags kprof (the benchmark split);
// these are no-ops otherwise.
const profOn = false

type phase uint8

const (
	phRedeal phase = iota
	phReplay
	phExpand
	phBackup
	numPhases
)

func profStart() int64            { return 0 }
func profAdd(phase, int64)        {}
func ProfNanos() [numPhases]int64 { return [numPhases]int64{} }
func ProfReset()                  {}
