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
//   - the only remembering write of the chain is one ChangeZoneAll Remember
//     sweep (Planar Guide's "Exile all creatures", Sudden Disappearance's
//     player-scoped sweep beside its player target slot, RememberChanged$
//     True), so the set is everything the sweep moved;
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

// delayedLKISweepMulti reports whether the chain's only remembering write is
// one ChangeZoneAll Remember sweep: RememberChanged$ remembers every object
// the sweep moved (Planar Guide's "Exile all creatures"), an unbounded set, so
// the kernel's one-object delayed record would return only the sweep's first
// object. Target slots do not disqualify: Sudden Disappearance's sweep is
// scoped to the target player's battlefield, and the remembered set is still
// everything the sweep moved — a remembering write of a target slot would be
// a second write and fail the count.
func (lc *lowerCtx) delayedLKISweepMulti() bool {
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

// delayedAllSet reports the ability's program so far remembers exactly one
// set, and that set is a ChangeZoneAll sweep with RememberChanged$ True (no
// ForgetOtherRemembered$): the objects the sweep changed, any number of them
// (Planar Guide's exile of every creature, Ghostway's).
func (lc *lowerCtx) delayedAllSet() bool {
	writes := 0
	all := false
	for _, in := range lc.ab.Prog {
		if !kremWrites(in) {
			continue
		}
		writes++
		all = in.Op == "ChangeZoneAll" && strings.Contains(in.CS, "|Remember") && !strings.Contains(in.CS, "|ForgetOther")
	}
	return writes == 1 && all
}

// delayedAllManyShape admits a delayed trigger whose RememberObjects$
// Remembered(LKI) names the set a ChangeZoneAll RememberChanged$ sweep wrote:
// gorge registers ONE delayed trigger whose remembered set is every object the
// sweep changed, and its body returns that set as one effect (CR 603.7). The
// kernel's DpMany program carries up to four objects of the resolution's
// remembered set (dirac/trig_delayedmany.go) and its OpReturn body walks them
// as one entering event (kop.RetDelayedMany; dirac/returnrem.go). The shape is
// exactly the delayedLKIManyShape body: one ChangeZone Origin$ Exile,
// Destination$ Battlefield of Defined$ DelayTriggerRememberedLKI and nothing
// else.
//
// Capacity: the carrier holds four objects, so a sweep of five or more is
// counted supported but stalls (StallDProg, declared, never a partial return).
//
// It returns the reason the shape is another, "" when it is this one.
func (lc *lowerCtx) delayedAllManyShape(exec *cards.SA) string {
	if !lc.delayedAllSet() {
		return "the remembered set is not one ChangeZoneAll RememberChanged$ sweep"
	}
	return delayedLKIReturnBody(exec)
}

// delayedChainSet reports the ability's program's last remembering write is
// one bounded ChangeZone of the set itself — obj=Remembered, RememberChanged$
// True, ForgetOtherRemembered$ True on an Exile or Return (Laezel's
// Acrobatics' 10-20 clause: its sweep's set is returned, then re-exiled, and
// each move forgets the rest and re-remembers the moved ones). The set is
// exactly that write's moved objects, several of them, so the kernel's
// one-object delayed record would return only the first; the write itself
// still stalls beyond the four objects the DpMany carrier holds.
func (lc *lowerCtx) delayedChainSet() bool {
	last := -1
	for i, in := range lc.ab.Prog {
		if kremWrites(in) {
			last = i
		}
	}
	if last < 0 {
		return false
	}
	in := lc.ab.Prog[last]
	return in.Obj.Kind == rRemembered && (in.Op == "Exile" || in.Op == "Return") &&
		strings.Contains("|"+in.CS, "|Remember") && strings.Contains("|"+in.CS, "|ForgetOther")
}

// delayedChainManyShape admits a delayed trigger whose RememberObjects$
// Remembered names the set the chain's last bounded ChangeZone re-remembered
// (delayedChainSet): the same DpMany program, for the one shape it returns
// exactly — one ChangeZone Origin$ Exile, Destination$ Battlefield of
// Defined$ DelayTriggerRememberedLKI and nothing else.
func (lc *lowerCtx) delayedChainManyShape(exec *cards.SA) string {
	if !lc.delayedChainSet() {
		return "the set's last write is not one ChangeZone of the set itself"
	}
	return delayedLKIReturnBody(exec)
}
