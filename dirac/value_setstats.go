package dirac

// SetStats counts the RefSet instructions execSet ran and the members it ran
// their op over (dirac/oracle definedset_lockstep_test.go: a pool's games
// read a set).
type SetStats struct{ Runs, Members uint64 }

// DebugSetStats, when set, receives execSet's counts. Tests only.
var DebugSetStats *SetStats
