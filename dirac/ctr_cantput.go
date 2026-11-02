package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugCtrBlockOff is TestCantPutCounterReplMutation's switch: set, the
// kernel ignores every StatCantPutCounter, so a counter the prohibition
// swallows in gorge is placed here and the lockstep diverges.
var DebugCtrBlockOff bool

// putCounterBlocked reports whether a StatCantPutCounter static forbids
// putting a counter of kind on the permanent at sl (CR 122.1; gorge
// PutCounterBlocked's object form, rules/layers_restrict.go). The static's
// Filter is matched with the static's source and controller, like
// staticFlagGen. Static.A is the CounterType$ kind, -1 for every kind.
//
// The check is the counter-placement choke point: callers run it BEFORE the
// AddCounter replacements, because gorge's Engine.emit swallows the
// CounterChange outright (the placement never happens, so neither the event
// nor any replacement of it runs, and no CounterAddedOnce trigger fires).
// The player form (ValidPlayer$) is not lowered; kgen refuses it.
//
//go:noinline
func (s *State) putCounterBlocked(k *prep, sl Slot, kind int) bool {
	if DebugCtrBlockOff || k.gen == nil || !k.gen.anyCtrBlock {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantPutCounter {
			return true
		}
		if a.Static.A >= 0 && int(a.Static.A) != kind {
			return true
		}
		if !s.statCond(k, st) {
			return true
		}
		if s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// ctrPutMode selects the AddCounter replacement path a placement uses.
type ctrPutMode uint8

const (
	ctrPutEffect ctrPutMode = iota // replCounters: no published putter
	ctrPutBy                       // replCountersBy: the effect's controller is the putter
	ctrPutCost                     // replCostCtr: a cost's placement (EffectOnly$ doublers excluded)
	ctrPutRaw                      // no AddCounter replacement yet: the caller already resolved the amount
)

// placeCounters is the one funnel every permanent counter placement goes
// through. It applies the CantPutCounter prohibition first, then the
// AddCounter replacements selected by mode, marks the slot and stores the
// counters. It returns the number actually placed (0 when the prohibition
// swallowed the event or the amount was not positive); a storage-capacity
// failure stalls loudly as before.
//
// A blocked placement does not touch the slot: gorge emits nothing at all,
// so there is no state change to mark.
//
//go:noinline
func (s *State) placeCounters(k *prep, sl Slot, kind, n int, ctrl uint8, mode ctrPutMode) int {
	if n <= 0 {
		return 0
	}
	if s.putCounterBlocked(k, sl, kind) {
		return 0
	}
	var m int
	switch mode {
	case ctrPutEffect:
		m = s.replCounters(k, sl, kind, n)
	case ctrPutBy:
		m = s.replCountersBy(k, sl, kind, n, ctrl)
	case ctrPutCost:
		m = s.replCostCtr(k, sl, kind, n, ctrl)
	case ctrPutRaw:
		m = n
	}
	s.touchSl(sl)
	if m <= 0 {
		return 0
	}
	if !s.perms[sl].addCtrN(s, kind, m) {
		s.ctrStall()
		return 0
	}
	return m
}
