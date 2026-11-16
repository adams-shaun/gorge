package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// delayedLKIMany admits a RememberObjects$ RememberedLKI delayed trigger whose
// remembered set is several objects and whose body returns that set as one
// effect (CR 603.7). The kernel's DpMany program carries up to four objects of
// the resolution's remembered set (dirac/trig_delayedmany.go) and its OpReturn
// body walks them as one entering event (kop.RetDelayedMany;
// dirac/returnrem.go). The set is one of two writes, and the body is exactly:
//
//   - one target slot whose only remembering write is the Exile of that slot
//     (RememberChanged$ True), so the set is the targets that left (Hide on
//     the Ceiling's X targets, Temporal Tribulation's two); or
//   - no target slot, and the only remembering write of the chain is one
//     ChangeZoneAll Remember sweep (Planar Guide's "Exile all creatures",
//     RememberChanged$ True), so the set is everything the sweep moved;
//   - a body of one ChangeZone Origin$ Exile, Destination$ Battlefield of
//     Defined$ DelayTriggerRememberedLKI and nothing else.
//
// Capacity: the carrier holds four objects, so Hide on the Ceiling with X >= 5,
// Eerie Interlude exiling five or more and Planar Guide sweeping five or more
// live non-tokens are counted supported but stall (StallDProg, declared, never
// a partial return); dirac.TestDelayedManyCapacity pins it.
//
// It returns the reason the chain is another shape, "" when it is this one.
func (lc *lowerCtx) delayedLKIManyShape(exec *cards.SA) string {
	if len(lc.ab.Targets) != 1 {
		return "not exactly one target slot"
	}
	writes := 0
	exiled := false
	for _, in := range lc.ab.Prog {
		if !kremWrites(in) {
			continue
		}
		writes++
		exiled = in.Op == "Exile" && in.Obj.Kind == rTarget && in.Obj.N == 0
	}
	if writes != 1 || !exiled {
		return "the remembered set is not the target slot's exile"
	}
	return delayedLKIReturnBody(exec)
}

// delayedLKISweepMulti reports whether the chain has no target slot and its
// only remembering write is one ChangeZoneAll Remember sweep: RememberChanged$
// remembers every object the sweep moved (Planar Guide's "Exile all
// creatures"), an unbounded set, so the kernel's one-object delayed record
// would return only the sweep's first object.
func (lc *lowerCtx) delayedLKISweepMulti() bool {
	if len(lc.ab.Targets) != 0 {
		return false
	}
	writes, sweep := 0, false
	for _, in := range lc.ab.Prog {
		if !kremWrites(in) {
			continue
		}
		writes++
		sweep = in.Op == "ChangeZoneAll" && strings.Contains(in.CS, "Remember")
	}
	return writes == 1 && sweep
}

// delayedLKIReturnBody is the one body both many shapes return exactly.
func delayedLKIReturnBody(exec *cards.SA) string {
	if exec.API != "ChangeZone" || exec.Sub != nil || len(exec.Params) != 3 ||
		exec.Params["Defined"] != "DelayTriggerRememberedLKI" ||
		exec.Params["Origin"] != "Exile" || exec.Params["Destination"] != "Battlefield" {
		return "the body is not one ChangeZone Exile->Battlefield of DelayTriggerRememberedLKI"
	}
	return ""
}

// delayedLKIMulti reports whether any target slot of the chain takes a variable
// or more than one object.
func (lc *lowerCtx) delayedLKIMulti() bool {
	for _, t := range lc.ab.Targets {
		if !t.Max.isConst() || t.Max.N > 1 {
			return true
		}
	}
	return false
}
