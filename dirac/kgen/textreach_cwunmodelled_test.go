package kgen

import "testing"

// CardDef.CWUnmodelled (textreach.go textWordsMasks) is the colour words a
// face reads on a surface the kernel's colour-word model does not rewrite
// (dirac/static_colorwords.go): the kernel stalls only on those. A target
// spec's ValidTgts$, the printed Protection keywords, the display text and
// the description parameters are modelled or inert; any other parameter is
// not, even when the same word also sits on a modelled surface.
func TestColorWordsUnmodelledMask(t *testing.T) {
	const (
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
		// parameter is a surface the model does not rewrite
		{"Raging Ravine", red | green, red | green},
		// "a 1/1 blue Bird creature token": the description is display text
		{"Ordered Migration", blue, 0},
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
