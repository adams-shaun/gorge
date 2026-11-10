package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// All-Fates Scroll's shape: an artifact with a printed unrestricted
// "{T}: Add one mana of any color". Mm'menon, the Right Hand grants it a
// second, restricted "{T}: Add {U}", so its mana wheel has two options and a
// payment plan can name only the printed one (pay.PaymentAbility).
const grantedWheelScrollSrc = "Name:Wheel Scroll\nTypes:Artifact\n" +
	"A:AB$ Mana | Cost$ T | Produced$ Any | SpellDescription$ Any colour.\nOracle:x\n"

func TestManaWheelMarksGrantedAbility(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Mm'menon, the Right Hand"))
	mm := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, mm)
	scroll := onBoard(t, e, 0, grantedWheelScrollSrc)
	e.priorityRound()
	d := e.Pending()
	act := -1
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == scroll {
			act = o.Index
		}
	}
	if act < 0 {
		t.Fatalf("no activate option for the scroll: %+v", d.Options)
	}
	submitChoices(t, e, act)
	d = e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Choose a mana ability of Wheel Scroll") {
		t.Fatalf("want the mana ability wheel, got %+v", d)
	}
	var printed, granted int
	for _, o := range d.Options {
		switch o.Label {
		case "Add any color":
			printed++
			if o.Granted {
				t.Errorf("the printed any-colour option is marked Granted: %+v", o)
			}
		case "Add U":
			granted++
			if !o.Granted {
				t.Errorf("the granted {U} option is not marked Granted: %+v", o)
			}
		}
	}
	// Precondition: both abilities are on the wheel, so the marks above ran.
	if printed != 1 || granted != 1 {
		t.Fatalf("wheel = %+v, want one printed any-colour and one granted U option", d.Options)
	}
}
