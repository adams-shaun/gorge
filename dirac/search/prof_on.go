//go:build kprof

package search

import (
	"sync/atomic"
	"time"
)

const profOn = true

type phase uint8

const (
	phRedeal phase = iota
	phReplay
	phExpand
	phBackup
	numPhases
)

var profNs [numPhases]atomic.Int64
var profT0 = time.Now()

func profStart() int64 { return int64(time.Since(profT0)) }

func profAdd(p phase, t0 int64) { profNs[p].Add(int64(time.Since(profT0)) - t0) }

// ProfNanos is the time spent per phase since ProfReset.
func ProfNanos() (out [numPhases]int64) {
	for i := range profNs {
		out[i] = profNs[i].Load()
	}
	return
}
func ProfReset() {
	for i := range profNs {
		profNs[i].Store(0)
	}
}
