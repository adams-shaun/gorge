package hbot

// Debugf, when set (tests, the parity harness), receives the port's
// intermediate facts. Nil in production: no call, no allocation.
var Debugf func(format string, args ...any)
