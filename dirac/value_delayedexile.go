package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExDelayedExileCount: the fired delayed program's remembered objects that
// are still the same object in exile. The DpMany program (trig_delayedmany.go)
// carries its set, at most four objects, into the fired entry's carriers
// (dpManyCarrier: object 1 in trigObj, 2..4 in the trigAmt, x and trigMV
// carriers, each with the version the program registered it at); an object
// whose version changed since is a new one with nothing to count (CR 400.7),
// as dpManyObj reads it.
//
// Dragonhawk, Fate's Tempest's end-step body counts them: gorge reads
// Count$ValidExile Card.IsTriggerRemembered, the delayed trigger's remembered
// set over the exile zones, and the body's /Twice suffix doubles it through
// the usual modifier (kop.ExProduct over 2). A card played from exile (the
// body's MayPlay grant) left it and counts no more.
//
//go:noinline
func (s *State) delayedExileCount(ctx evalCtx) int {
	e := ctx.e
	if e == nil || DebugDelayedExileOff {
		return 0
	}
	n := 0
	for i := 0; i < 4; i++ {
		o, ver := e.dpManyCarrier(i)
		if o == NoObj {
			continue
		}
		if !DebugDelayedExileLoose && (s.objs[o].zone != kop.ZoneExile || s.objs[o].ver != ver) {
			continue
		}
		n++
	}
	return n
}

// The mutation switches (dragonhawk_lockstep_test.go): Off counts nothing,
// Loose counts every remembered object whatever became of it.
var DebugDelayedExileOff, DebugDelayedExileLoose bool

// countExileOrAgg is kop.ExCountExile: N 1 is the exile type aggregate
// (zone_exiledwithstatic.go), any other N the plain exile count.
func (s *State) countExileOrAgg(k *prep, x *kop.Expr, ctx evalCtx) int {
	if x.N == 1 {
		return s.exileTypeAgg(k, x, ctx)
	}
	return s.countExile(k, x, ctx)
}
