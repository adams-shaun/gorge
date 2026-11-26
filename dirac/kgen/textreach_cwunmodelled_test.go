package kgen

import "testing"

// CardDef.CWUnmodelled (textreach.go textWordsMasks) is the colour words a
// face reads on a surface the kernel's colour-word model does not rewrite
// (dirac/static_colorwords.go): the kernel stalls only on those. A target
// spec's ValidTgts$, the printed Protection keywords, the display text, the
// description parameters and an Animate activation's (AB$) word-list Colors$
// are modelled or inert; any other parameter is
// not, even when the same word also sits on a modelled surface.
func TestColorWordsUnmodelledMask(t *testing.T) {
	const (
		white = 1 << 0
		black = 1 << 2
		red   = 1 << 3
		green = 1 << 4
		blue  = 1 << 1
	)
	for _, tc := range []struct {
		card       string
		words      uint16 // the colour bits TextWords must hold
		unmodelled uint16 // CWUnmodelled's exact value
	}{
		// "Destroy target nonblack creature": the target spec alone
		{"Doom Blade", black, 0},
		// Animate Colors$ Red,Green beside the Oracle text naming both: the
		// record carries the colour-word colour frozen as the ability was put
		// on the stack (static_colorwords.go cwFreeze, ovCw), so the words
		// are read
		{"Raging Ravine", red | green, 0},
		// "a 1/1 blue Bird creature token": the description is display text
		{"Ordered Migration", blue, 0},
		// K:etbCounter's Adamant$ White: the gate's colour is read live at the
		// kernel's ExAdamant (kw_evoke.go exAdamant, the composed colour-word
		// colour of the source face's spend); the keyword line beside its
		// parsed expansion is inert
		{"Ardenvale Paladin", white, 0},
		// S:Mode$ Continuous Affected$ Creature.Blue: the kernel reads the
		// static's Affected$ filter's colour clauses on the static's source
		// face (matchPermOne, cwTextObj of the source slot)
		{"Sunken City", blue, 0},
		// A:AB$ Pump KWChoice$ Protection from red: the choice's grant
		// (OpGrantKeyword) reads the term's colour by the resolving entry's
		// colour-word colour (cwProtGrant)
		{"Jodah's Avenger", red, 0},
		// S:Mode$ AlternativeCost ValidSA$ Spell.Self Cost$ ExileFromHand<1/
		// Card.Blue+Other>: the alternative cost is read from the cast card's
		// printed face in its cast zone (gorge alternativeCosts at
		// beginCastWith, statics_altcost.go) and frozen for the pick
		// (cast_asks.go exAsk), so the word is read but never rewritten
		{"Snapback", blue, 0},
	} {
		c := lowerNamedCard(t, tc.card)
		if c.TextWords&tc.words != tc.words {
			t.Errorf("%s: TextWords = %#x, want %#x set (precondition: the face reads the word)", tc.card, c.TextWords, tc.words)
		}
		if c.CWUnmodelled != tc.unmodelled {
			t.Errorf("%s: CWUnmodelled = %#x, want %#x", tc.card, c.CWUnmodelled, tc.unmodelled)
		}
	}
}
