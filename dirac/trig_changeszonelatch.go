package dirac

// DebugCzallNoLatch disables the per-batch latch of a ChangesZoneAll
// (Tf2Batch) line: test only, a Tf2Batch line is queued per member and not
// latched per batch.
var DebugCzallNoLatch bool
