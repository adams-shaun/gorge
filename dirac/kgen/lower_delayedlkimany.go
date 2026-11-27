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
//     Defined$ DelayTriggerRememberedLKI and nothing else; or
//   - Semester's End's Effect body: the same return behind a Moved replacement
//     that gives an entering creature a +1/+1 counter and an entering
//     planeswalker a loyalty counter. The Effect affects that one move only,
//     so the counters ride the return (kop.RetDelayedTypeCtr) and the Effect
//     and its imprint cleanup are dropped (delayedLKICtrBody).
//
// Capacity: the carrier holds four objects, so Hide on the Ceiling with X >= 5,
// Eerie Interlude exiling five or more and Planar Guide sweeping five or more
// live non-tokens are counted supported but stall (StallDProg, declared, never
// a partial return); dirac.TestDelayedManyCapacity pins it.
//
// It returns the reason the chain is another shape, "" when it is this one.
func (lc *lowerCtx) delayedLKIManyShape(exec *cards.SA) string {
	if why := lc.delayedLKITargetSetShape(); why != "" {
		return why
	}
	return delayedLKIReturnBody(exec)
}

// delayedLKITargetSetShape checks that the chain's remembered set is exactly
// the one target slot's exile (RememberChanged$ on the Exile), which is the
// set both the plain many-object return and Semester's End's replacement
// return carry.
func (lc *lowerCtx) delayedLKITargetSetShape() string {
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
	return ""
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

// delayedLKICtrBody reads Semester's End's delayed body: an Effect over the
// delayed remembered set whose Moved replacement gives a returning creature an
// additional +1/+1 counter and a returning planeswalker an additional loyalty
// counter, and whose Sub returns the set from exile. The Effect exists only for
// that one move, so the lowering carries the counters on the return itself
// (kop.RetDelayedTypeCtr) and drops the Effect and its imprint cleanup, which
// affect nothing else. ok is false for every other body.
func (lc *lowerCtx) delayedLKICtrBody(exec *cards.SA) (ret *cards.SA, ok bool) {
	if exec.API != "Effect" || exec.Sub == nil || len(exec.Params) != 5 {
		return nil, false
	}
	for k := range exec.Params {
		switch k {
		case "RememberObjects", "ReplacementEffects", "ImprintOnHost", "ForgetOnMoved", "SubAbility":
		default:
			return nil, false
		}
	}
	if exec.Params["RememberObjects"] != "DelayTriggerRememberedLKI" ||
		exec.Params["ImprintOnHost"] != "True" || exec.Params["ForgetOnMoved"] != "Exile" {
		return nil, false
	}
	ret = exec.Sub
	if ret.API != "ChangeZone" || len(ret.Params) != 4 ||
		ret.Params["Defined"] != "DelayTriggerRememberedLKI" ||
		ret.Params["Origin"] != "Exile" || ret.Params["Destination"] != "Battlefield" ||
		ret.Params["SubAbility"] == "" {
		return nil, false
	}
	clean := ret.Sub
	if clean == nil || clean.API != "ChangeZone" || len(clean.Params) != 4 ||
		clean.Params["Defined"] != "Imprinted" || clean.Params["Origin"] != "Command" ||
		clean.Params["Destination"] != "Exile" || clean.Sub == nil ||
		clean.Sub.API != "Cleanup" || clean.Sub.Params["ClearImprinted"] != "True" {
		return nil, false
	}
	repl := etbParams(lc.face.SVars[exec.Params["ReplacementEffects"]])
	if repl == nil || repl["Event"] != "Moved" || repl["Destination"] != "Battlefield" ||
		repl["ReplacementResult"] != "Updated" ||
		repl["ValidCard"] != "Creature.IsRemembered,Planeswalker.IsRemembered" {
		return nil, false
	}
	for k := range repl {
		switch k {
		case "Event", "ValidCard", "Destination", "ReplaceWith", "ReplacementResult", "Description":
		default:
			return nil, false
		}
	}
	body := etbParams(lc.face.SVars[repl["ReplaceWith"]])
	if body == nil || body["DB"] != "PutCounter" || body["Defined"] != "ReplacedNewCard.Creature" ||
		body["CounterType"] != "P1P1" || body["ETB"] != "True" || body["CounterNum"] != "1" ||
		body["SubAbility"] == "" || len(body) != 6 {
		return nil, false
	}
	loy := etbParams(lc.face.SVars[body["SubAbility"]])
	if loy == nil || loy["DB"] != "PutCounter" || loy["Defined"] != "ReplacedNewCard.Planeswalker" ||
		loy["CounterType"] != "LOYALTY" || loy["ETB"] != "True" || loy["CounterNum"] != "1" ||
		len(loy) != 5 {
		return nil, false
	}
	cp := *ret
	cp.Sub = nil
	cp.Params = map[string]string{
		"Defined": "DelayTriggerRememberedLKI", "Origin": "Exile", "Destination": "Battlefield",
	}
	return &cp, true
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

// delayedLKIManyBody checks the body of a RememberedLKI delayed trigger over
// a set of several objects: the kernel's DpMany program, for the shapes it
// returns exactly (lower_delayedlkimany.go). Semester's End's Effect adds a
// type-conditional enters counter to the same return: that body is lowered as
// its plain return, csFlag the |DelayedCtrByType flag.
func (lc *lowerCtx) delayedLKIManyBody(exec *cards.SA) (body *cards.SA, csFlag string, err error) {
	if ret, ok := lc.delayedLKICtrBody(exec); ok {
		if why := lc.delayedLKITargetSetShape(); why != "" {
			return exec, "", refuse("delayed trigger RememberedLKI over a multi-object target slot: %s", why)
		}
		return ret, "|DelayedCtrByType", nil
	}
	if why := lc.delayedLKIManyShape(exec); why != "" {
		return exec, "", refuse("delayed trigger RememberedLKI over a multi-object target slot: %s", why)
	}
	return exec, "", nil
}
