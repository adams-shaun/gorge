package dirac

// a test-only mutation for the "target player or
// planeswalker" shape. DebugTgtAnyNoFilter makes legal.go's TgtAny object
// branch skip t.Filter, so a creature becomes a legal choice beside the
// players for a player-or-planeswalker target. A lockstep against gorge
// must then diverge; the mutation proves the pool actually reaches the
// filtered any-target shape and that the kernel reads it.
//
// It is a package variable rather than a State byte: State must not grow,
// and the flag is cold (only a TgtAny target check reads it).
var DebugTgtAnyNoFilter bool
