package kgen

import "testing"

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
