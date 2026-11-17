package kgen

import "testing"

// czallLibraryPositionForms: ChangeZoneAll's library-destination placement
// reads LibraryPosition$ (0 top, -1 bottom) and RandomOrder$ True together
// with the CR 401.4 arrangement bracket (lower_libplace.go). Zimone's
// Experiment and Collected Conjuring put a remembered set on the bottom in a
// random order; Harmonic Convergence puts every enchantment on top.
var czallLibraryPositionForms = []formCase{
	{card: "Harmonic Convergence", form: "LibraryPosition$ 0 (top)", lir: "0 LibPlace"},
	{card: "Zimone's Experiment", form: "LibraryPosition$ -1, RandomOrder$ True", lir: "LibPlace C=5"},
	{card: "Collected Conjuring", form: "LibraryPosition$ -1, RandomOrder$ True from exile", lir: "LibPlace C=5"},
}

func TestChangeZoneAllLibraryPositionForms(t *testing.T) {
	checkForms(t, czallLibraryPositionForms)
}
