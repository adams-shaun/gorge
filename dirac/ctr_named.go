package dirac

// DebugCtrXDropDivinity makes every placement of a DIVINITY counter a no-op
// that still reports success: the counter goes to no sparse slot. The
// ctrdivinity lockstep's mutation test flips it to prove the pool exercises
// Myojin of Night's Reach's divinity counter -- without it the creature
// never has the counter, so the counters_GE1_DIVINITY static grants no
// indestructible and the SubCounter activation cost cannot be paid.
//
// The test-only hook sits in perm.setCtrN, the one accessor every counter
// placement goes through, so it covers the enters replacement
// (OpEnterWithCounters), OpPutCounter, OpPutCounterAll and the other put
// paths alike. Removal is untouched: only a placement (a count that would
// rise) is dropped.
var DebugCtrXDropDivinity bool
