package state

// PendingTrigger is a triggered ability that has matched but is not yet on
// the stack: it is waiting for its controller to order it against its
// siblings (CR 603.3b, decision.KTriggerOrder) or for its decider to accept
// it (decision.KTriggerOptional). It lives in state rather than rules so
// that view can describe it without importing rules — the same move-it-down
// precedent as ContinuousEffect.
type PendingTrigger struct {
	Source     ObjID // the permanent whose trigger fired
	Controller PlayerID
	Label      string   // what a client shows: the source's name and the card's own TriggerDescription$
	Optional   bool     // true when a yes/no is (or will be) asked
	Decider    PlayerID // who answers that yes/no; meaningful only when Optional
}
