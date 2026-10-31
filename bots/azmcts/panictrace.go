package azmcts

import "runtime/debug"

// PanicTrace, when set, receives every panic the search recovers inside a
// world (the simulations Stats.Panics counts), with where it was recovered
// and the stack at the recover. A recovered panic is a discarded simulation,
// so without this hook its cause is invisible; a driver that wants to find
// the root cause sets it. Diagnostic only: it never
// changes what the search plays.
var PanicTrace func(where string, p any, stack []byte)

func tracePanic(where string, p any) {
	if PanicTrace != nil {
		PanicTrace(where, p, debug.Stack())
	}
}
