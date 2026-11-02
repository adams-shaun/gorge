package dirac

// an AtEOT$ rider's delayed trigger is sourced on the
// object that created it (CR 603.7d-e; gorge).
//
// gorge's scheduleAtEOTRider registers the creating spell/permanent beside
// the affected object (DelayedRegister |CR=<id>); at fire the pending
// trigger's Source is the creator and TriggerContext.DelayedSelf is the
// affected object, whose incarnation the registration still tracks
// (apply_delayed.go). The kernel's delayed record carries the creator
// (delayed.creator, NoObj for every non-AtEOT record); fireDelayed and
// fireDlyEndCombat enqueue it as the stack source with the affected object as
// trigObj, the body's referent (resolveDelayed, resolveSacUntracked,
// takePending's stale check).

// DebugAteotSourceAffected is a test mutation: a creator-sourced delayed
// trigger is put on the stack with the affected object as its source (the
// earlier shape); the lockstep mutation test requires a divergence.
var DebugAteotSourceAffected bool

// dlySource is the stack source of delayed record d: its creator when it has
// one other than the affected object, else the affected object.
func dlySource(d *delayed) Obj {
	if d.creator == NoObj || d.creator == d.obj || DebugAteotSourceAffected {
		return d.obj
	}
	return d.creator
}

// dlyRef is the body referent of a delayed trigger sourced on obj with
// trigObj recorded: trigObj when it names another object (a creator-sourced
// AtEOT$ rider), else obj.
func dlyRef(obj, trigObj Obj) Obj {
	if trigObj != NoObj && trigObj != obj {
		return trigObj
	}
	return obj
}
