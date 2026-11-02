package dirac

// DebugStationGrantNoGate drops the charge-counter threshold of a Station
// self grant (dirac/kgen lower_stationgrant.go): the granted trigger exists
// at zero counters. A mutation switch for TestStationgrantLockstep.
var DebugStationGrantNoGate bool
