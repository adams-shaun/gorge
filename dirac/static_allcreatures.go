package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugAllCreatNoOverride is the mutation switch of TestAllcreatMutation:
// every non-printed all-creature-types effect (an override's
// kop.Override.AllCreatureTypes, from a Continuous AddAllCreatureTypes$
// static or an Animate) is ignored. A printed changeling
// (kop.CardDef.AllCreatureTypes) is unaffected: it never rides an override.
var DebugAllCreatNoOverride bool

// ovrAllCreat reports whether override o makes its object every creature
// type. Every reader of the override field (the layer walk's layer 4, the
// fixed-order type/subtype readers and the feat view) goes through this one
// helper, so the mutation switch above covers them all -- the next reader
// that copies the field instead of calling it is the bug this shape cannot
// have.
func ovrAllCreat(o *kop.Override) bool {
	return o.AllCreatureTypes && !DebugAllCreatNoOverride
}
