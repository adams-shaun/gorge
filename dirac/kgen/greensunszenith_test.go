package kgen

import "testing"

// Green Sun's Zenith: a search for a green creature card of mana value X or
// less onto the battlefield, then the spell shuffles itself into its owner's
// library. It lowers with no refusal: the search's filter keeps both the
// colour and the X bound, and the self-shuffle is the Zenith SpellToLib.
var greenSunsZenithForms = []formCase{
	{card: "Green Sun's Zenith", form: "Creature.Green+cmcLEX search filter", lir: "Search obj=You A=1 C=1 F={types=0x2 color=G cmp(2 2 0 X )}"},
	{card: "Green Sun's Zenith", form: "Origin$ Stack Defined$ Parent shuffle", lir: "1 SpellToLib"},
}

func TestGreenSunsZenithForm(t *testing.T) { checkForms(t, greenSunsZenithForms) }
