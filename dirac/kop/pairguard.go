package kop

// PairRefusal says two cards cannot share a table: the kernel would stall the
// game at its start. Cards are the carriers, by table CardID.
type PairRefusal struct {
	Reason string // the stall the game would start with (dirac.StallName)
	Why    string
	Cards  []CardID
}

// PairGuard reads a built matchup table and returns its refusal, or nil. Package
// kernel sets it from its init, so a binary that can play a table refuses one it
// cannot play. It sits here because package cardset, which calls it, cannot import
// package kernel: kernel's own tests import cardset. Nil in a binary without the
// kernel, which has nothing to stall.
var PairGuard func(t *Table) *PairRefusal
