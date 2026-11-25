package kgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// learnCarriers carry DB$ Learn or AB$ Learn (CR 701.48a) and nothing else
// kgen refuses; each lowers its learn to one Learn instruction (OpExt
// ExtLearn, dirac/act_learn.go) over gorge's Lesson spec.
var learnCarriers = []string{
	"Arcane Subtraction", "Cram Session", "Enthusiastic Study", "Eyetwitch", "Field Trip",
	"First Day of Class", "Gnarled Professor", "Guiding Voice", "Hunt for Specimens", "Igneous Inspiration",
	"Overgrown Arch", "Poet's Quill", "Pop Quiz", "Professor of Symbology", "Rise of Extus",
	"Sparring Regimen", "Study Break",
}

func TestLearnLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range learnCarriers {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		// Lesson is in the corpus vocabulary (a deck pool's typeWords), not
		// on a Learn carrier's own type line
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg,
			typeWords: map[string]bool{"Lesson": true}}
		c := lowerCard(p, card.Faces[0].Name, card, false)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
			continue
		}
		n := 0
		for _, ab := range c.Abilities {
			for _, in := range ab.Prog {
				if in.Op == "Learn" {
					n++
					if in.F == nil || !strings.Contains(fmt.Sprintf("%+v", *in.F), "Lesson") {
						t.Errorf("%s: Learn filter %v, want the Lesson spec", name, in.F)
					}
				}
			}
		}
		if n == 0 {
			t.Errorf("%s: no Learn instruction lowered", name)
		}
	}
}
