package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// staticCtx is one static ability's lowering state while its Mode's body
// runs: the static being built (st), the statics that replace it (extra), and
// the inputs every body reads. (*lowerCtx).staticBody (lower_trigger.go)
// reads the conditions every mode shares, runs the body, then checks the
// unread params.
type staticCtx struct {
	lc    *lowerCtx
	s     cards.Static
	sp    *params
	st    lStatic
	extra []lStatic
	readF func(k string) (*lFilter, error)
}

// staticModes maps a static Mode$ to its body. A body returns done with the
// statics to emit (or an error) to end the lowering there; otherwise
// staticBody goes on to its shared checks. Each static_<family>.go registers
// its modes in init(); a new mode is a new body and one staticMode line.
var staticModes = map[string]func(*staticCtx) ([]lStatic, bool, error){}

// staticMode registers the body of Mode$ name; a second registration panics.
func staticMode(name string, h func(*staticCtx) ([]lStatic, bool, error)) {
	if _, dup := staticModes[name]; dup {
		panic("kgen: static mode " + name + " registered twice")
	}
	staticModes[name] = h
}
