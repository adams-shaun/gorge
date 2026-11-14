package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// apiOps maps an effect's api: name to its lowering. Each api_<family>.go
// registers its effects in init(); a new effect is its lowering (in its own
// lower_<mechanic>.go) plus one registration line, never a case in a shared
// switch. (*lowerCtx).api (lower_effect.go) dispatches through it.
var apiOps = map[string]func(lc *lowerCtx, sa *cards.SA, ps *params) error{}

// apiOp registers the lowering of the api: name; a second registration of one
// name panics at init.
func apiOp(name string, h func(lc *lowerCtx, sa *cards.SA, ps *params) error) {
	if _, dup := apiOps[name]; dup {
		panic("kgen: api " + name + " registered twice")
	}
	apiOps[name] = h
}

// apiPS registers a lowering that reads only the effect's params.
func apiPS(name string, h func(lc *lowerCtx, ps *params) error) {
	apiOp(name, func(lc *lowerCtx, _ *cards.SA, ps *params) error { return h(lc, ps) })
}

// apiSA registers a lowering that reads the whole ability.
func apiSA(name string, h func(lc *lowerCtx, sa *cards.SA) error) {
	apiOp(name, func(lc *lowerCtx, sa *cards.SA, _ *params) error { return h(lc, sa) })
}
