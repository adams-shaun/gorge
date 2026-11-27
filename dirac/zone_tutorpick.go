package dirac

// TutorPickStats counts the library searches an opponent answered (SrChooserOpp
// on an OpSearchEx: Intuition's and Gifts Ungiven's "target opponent chooses",
// gorge searchChooser's Chooser$ Targeted over DefinedPlayer$ You). The
// lockstep test asserts it is non-zero, so a green game is not one where the
// new shape never fired.
type TutorPickStats struct{ OppAnswered int64 }

// DebugTutorPickStats, when set, receives the opponent-answered search count.
var DebugTutorPickStats *TutorPickStats
