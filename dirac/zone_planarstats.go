package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func init() {
	// The planar-roll replacement reads the generated replacement bit word,
	// so a table carrying the line needs a genPrep even when nothing else in
	// it does (prepareGen's gate, genneed.go).
	genNeed(func(t *kop.Table) bool { return tableHasReplKind(t, kop.ReplRollPlanarDice) })
}

// PlanarDiceStats counts a lockstep's planar-die coverage: rolls performed,
// rolls a replacement rewrote, the extra dice it added and the results it
// ignored. TestDemoPlanarDiceLockstep reads it so a green game is not one
// where the shape never fired.
var PlanarDiceStats struct {
	Rolls, Replaced, ExtraDice, Ignored atomic.Int64
}

// DebugNoPlanarReplacement, when set, makes the planar roll ignore every
// ReplRollPlanarDice line (TestDemoPlanarDiceMutation).
var DebugNoPlanarReplacement bool
