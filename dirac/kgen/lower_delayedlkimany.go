package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// delayedLKIMany admits a RememberObjects$ RememberedLKI delayed trigger whose
// chain target slot takes several objects (Hide on the Ceiling's X targets,
// Temporal Tribulation's two): gorge registers ONE delayed trigger whose
// remembered set is every object the chain's RememberChanged$ exile moved, and
// its body returns that set as one effect (CR 603.7). The kernel's DpMany
// program carries up to four objects of the resolution's remembered set
// (dirac/trig_delayedmany.go) and its OpReturn body walks them as one entering
// event (kop.RetDelayedMany; dirac/returnrem.go). The shape is exactly:
//
//   - one target slot, and the only remembering write of the chain is the
//     Exile of that slot (RememberChanged$ True), so the set is the targets
//     that left;
//   - a body of one ChangeZone Origin$ Exile, Destination$ Battlefield of
//     Defined$ DelayTriggerRememberedLKI and nothing else.
//
// Capacity: the carrier holds four objects, so Hide on the Ceiling with X >= 5
// and Eerie Interlude exiling five or more are counted supported but stall
// (StallDProg, declared, never a partial return); dirac.TestDelayedManyCapacity
// pins it.
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
	if exec.API != "ChangeZone" || exec.Sub != nil || len(exec.Params) != 3 ||
		exec.Params["Defined"] != "DelayTriggerRememberedLKI" ||
		exec.Params["Origin"] != "Exile" || exec.Params["Destination"] != "Battlefield" {
		return "the body is not one ChangeZone Exile->Battlefield of DelayTriggerRememberedLKI"
	}
	return ""
}
