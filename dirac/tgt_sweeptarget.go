package dirac

// a test-only mutation for the target-player sweep
// scope. DebugTgtPlayerCtrlAny makes ctrlRoleF2's F2TgtPlayerCtrl arm true
// for every controller, so a sweep scoped to a chosen player target widens
// to both seats. A lockstep against gorge must then diverge; the mutation
// proves the scoped sweep is actually reached and read.
//
// It is a package variable rather than a State byte: State must not grow,
// and the flag is cold (only a table carrying F2TgtPlayerCtrl reads it).
var DebugTgtPlayerCtrlAny bool
