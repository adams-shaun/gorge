package kgen

import "testing"

// heavySlots bounds the heavy parallel tests that run at once: each builds
// over the whole corpus (about 1 GB live), so two at a time keep the
// package inside the test budget's 4 GB while they overlap.
var heavySlots = make(chan struct{}, 2)

// heavyParallel marks a heavy read-only test parallel and holds one of
// heavySlots until it ends, so the package's serial phase stays short.
func heavyParallel(t *testing.T) {
	t.Helper()
	t.Parallel()
	heavySlots <- struct{}{}
	t.Cleanup(func() { <-heavySlots })
}
