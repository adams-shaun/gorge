package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// devotionChosenProbe reads Count$Devotion.Chosen (Nyx Lotus's amount): the
// controller's devotion to the colour recorded on the source.
const devotionChosenProbe = `Name:Devotion Chosen Probe
ManaCost:2
Types:Artifact
A:AB$ GainLife | Cost$ T | LifeAmount$ X | SpellDescription$ probe
SVar:X:Count$Devotion.Chosen
`

// TestDevotionChosenLowers: Count$Devotion.Chosen lowers to vDevotionChosen,
// not the fixed-mask vDevotion, and the build carries it as ExDevotionChosen.
func TestDevotionChosenLowers(t *testing.T) {
	c := lowerScriptCard(t, "Devotion Chosen Probe", devotionChosenProbe)
	for _, rf := range c.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	var got []lValue
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			for _, v := range []lValue{in.A, in.B} {
				if v.Kind == vDevotionChosen || v.Kind == vDevotion {
					got = append(got, v)
				}
			}
		}
	}
	if len(got) != 1 || got[0].Kind != vDevotionChosen {
		t.Fatalf("devotion values %v, want one vDevotionChosen", got)
	}
}

// TestDevotionChosenCorpusRefusals pins the four corpus Count$Devotion.Chosen
// users: none is refused for the devotion value, and Nyx Lotus / Nykthos stay
// refused for their ChooseColor mana chain. Every one is refused before the
// lowering reaches X, so each card's own X body is also lowered through
// devotionHead and must give vDevotionChosen.
func TestDevotionChosenCorpusRefusals(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name       string
		chooseMana bool
	}{
		{"Nyx Lotus", true},
		{"Nykthos, Shrine to Nyx", true},
		{"Nyx", false},
		{"Hotel of Fears", false},
	} {
		card, ok := reg.Lookup(tc.name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", tc.name)
		}
		rest, found := strings.CutPrefix(card.Faces[0].SVars["X"], "Count$")
		if !found || rest != "Devotion.Chosen" {
			t.Fatalf("%s: SVar X = %q, want Count$Devotion.Chosen", tc.name, card.Faces[0].SVars["X"])
		}
		if v, ok, err := devotionHead(rest); !ok || err != nil || v.Kind != vDevotionChosen {
			t.Errorf("%s: X lowers to %v (head %v, err %v), want vDevotionChosen", tc.name, v, ok, err)
		}
		c := lowerCorpusCard(t, tc.name)
		if len(c.Refusals) == 0 {
			t.Fatalf("%s: no refusals, want the pinned refusal list", tc.name)
		}
		choose := false
		for _, rf := range c.Refusals {
			if strings.Contains(rf.Reason, "Devotion") {
				t.Errorf("%s refused for devotion: %s", tc.name, rf.Reason)
			}
			if strings.Contains(rf.Reason, "ChooseColor outside an as-enters replacement") {
				choose = true
			}
		}
		if tc.chooseMana && !choose {
			t.Errorf("%s: no ChooseColor mana-ability refusal in %v", tc.name, c.Refusals)
		}
	}
}
