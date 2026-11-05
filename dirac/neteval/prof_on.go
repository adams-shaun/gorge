//go:build kprof

package neteval

import (
	"sync/atomic"
	"time"
)

var t0 = time.Now()
var featNs, netNs atomic.Int64

func profStart() int64          { return int64(time.Since(t0)) }
func profFeat(s int64)          { featNs.Add(int64(time.Since(t0)) - s) }
func profNet(s int64)           { netNs.Add(int64(time.Since(t0)) - s) }
func ProfNanos() (int64, int64) { return featNs.Load(), netNs.Load() }
func ProfReset()                { featNs.Store(0); netNs.Store(0) }
