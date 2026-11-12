package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDrawnvpFirstCardInDrawStep: FirstCardInDrawStep$ True/False lowers
// onto the kernel's draw-step qualifier (gorge trigmatch.firstCardInDrawStep,
// CR 504.1), and any other spelling stays refused rather than guessed.
func TestDrawnvpFirstCardInDrawStep(t *testing.T) {
	for _, c := range []struct {
		value string
		first bool
	}{
		{"True", true},
		{"False", false},
	} {
		t.Run(c.value, func(t *testing.T) {
			lc := kdrawnvpLower(t, map[string]string{
				"ValidCard": "Card.OppOwn", "FirstCardInDrawStep": c.value, "TriggerZones": "Battlefield",
			})
			tr := kdrawnvpTrigger(t, lc)
			if tr.FirstInDrawStep != c.first || tr.NotFirstInDrawStep == c.first {
				t.Fatalf("FirstCardInDrawStep$ %s: First %v NotFirst %v", c.value, tr.FirstInDrawStep, tr.NotFirstInDrawStep)
			}
			_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			want := kop.Tf5FirstCardInDrawStep
			if !c.first {
				want = kop.Tf5NotFirstCardInDrawStep
			}
			found := false
			for _, ab := range all[0].def.Abilities {
				if ab.Kind == kop.AbTriggered && ab.Trig.Event == kop.TrigDrawn {
					found = true
					if ab.Trig.Flags5 != want {
						t.Fatalf("built Flags5 = %d, want %d", ab.Trig.Flags5, want)
					}
				}
			}
			if !found {
				t.Fatal("no Drawn trigger built")
			}
		})
	}

	enterstappedRefuseWant(t, kdrawnvpLower(t, map[string]string{
		"ValidCard": "Card.OppOwn", "FirstCardInDrawStep": "Maybe", "TriggerZones": "Battlefield",
	}).Refusals, `Drawn FirstCardInDrawStep "Maybe"`)
}
