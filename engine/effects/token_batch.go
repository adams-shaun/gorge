package effects

// token_batch.go is the open/close helper for the Mode$ TokenCreatedOnce
// "one or more" bracket (CR 603.2c). effects.Resolve wraps one resolution in
// it, so a body that creates several tokens -- one api:Token with
// TokenAmount$, a loop, or several sub-abilities -- queues the watcher once.
// The bracket lives in the rules engine (engine/rules/trigger_phase_gate.go's
// BeginTokenBatch/EndTokenBatch); this helper reaches it through an optional
// interface so a host double without it still runs, just with each token as
// its own batch-of-one.

// tokenBatcher is the engine's token-creation bracket. Deliberately optional
// (not part of effects.Host), the actionBatcher shape.
type tokenBatcher interface {
	BeginTokenBatch()
	EndTokenBatch()
}

// beginTokenBatch opens one token-creation bracket and returns the function
// that closes it. A host that does not implement the bracket returns a no-op
// close. Callers use it as `defer beginTokenBatch(h)()`.
func beginTokenBatch(h Host) func() {
	if b, ok := h.(tokenBatcher); ok {
		b.BeginTokenBatch()
		return b.EndTokenBatch
	}
	return func() {}
}
