package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// New Perspectives' "As long as you have seven or more cards in hand, you may
// pay {0} rather than pay cycling costs" (S:Mode$ AlternativeCost | ValidSA$
// Activated.Cycling | ValidPlayer$ You | Cost$ Discard<1/CARDNAME> | CheckSVar$
// X | SVarCompare$ GE7), CR 118.9: the alternative replaces the whole
// printed cost, so the card is still discarded (the static's own Discard part)
// and the mana is dropped.
const cyclingAltStatic = "Name:Perspective Fixture\nManaCost:5 U\nTypes:Enchantment\n" +
	"S:Mode$ AlternativeCost | ValidSA$ Activated.Cycling | ValidPlayer$ You | Cost$ Discard<1/CARDNAME> | CheckSVar$ Held | SVarCompare$ GE7 | Description$ x\n" +
	"SVar:Held:Count$ValidHand Card.YouOwn\nOracle:x\n"

func cyclingAltEngine(t *testing.T, fill int, cycler string) (*Engine, state.ObjID) {
	t.Helper()
	hand := []*cards.Card{card(t, cycler)}
	for i := 0; i < fill; i++ {
		hand = append(hand, card(t, "Name:Filler\nTypes:Basic Land Mountain\nOracle:x\n"))
	}
	e := handEngine(t, hand...)
	putOnBattlefield(t, e, 0, card(t, cyclingAltStatic))
	id := e.G.Zone(state.ZHand, 0)[0]
	if got := len(e.G.Zone(state.ZHand, 0)); got != 1+fill {
		t.Fatalf("precondition: hand size %d, want %d", got, 1+fill)
	}
	if o := e.G.Obj(id); !o.Face().HasKeyword("Cycling") && !o.Face().HasKeyword("TypeCycling") {
		t.Fatalf("precondition: %q prints no cycling", o.Face().Name)
	}
	return e, id
}

func cyclingAltOptions(e *Engine, id state.ObjID) (printed, alt []decision.Option) {
	for _, o := range e.legalActions(0) {
		if o.Kind != "ability" || o.Obj != id {
			continue
		}
		if o.AltCostIndex > 0 {
			alt = append(alt, o)
		} else {
			printed = append(printed, o)
		}
	}
	return printed, alt
}

func TestCyclingAlternativeCostOffer(t *testing.T) {
	t.Parallel()
	const plain = "Name:Plain Cycler\nManaCost:3 U\nTypes:Creature Beast\nPT:2/2\nK:Cycling:2\nOracle:x\n"
	const typed = "Name:Typed Cycler\nManaCost:3 U\nTypes:Creature Beast\nPT:2/2\nK:TypeCycling:Plains:2\nOracle:x\n"
	const lifeCycler = "Name:Life Cycler\nManaCost:3 U\nTypes:Creature Beast\nPT:2/2\nK:Cycling:PayLife<2>\nOracle:x\n"
	for _, tc := range []struct {
		name, src string
		fill      int
		wantAlt   bool
	}{
		{"cycling with seven in hand", plain, 6, true},
		{"cycling with six in hand", plain, 5, false},
		{"typecycling with seven in hand", typed, 6, true},
		{"pay-life cycling with seven in hand", lifeCycler, 6, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, id := cyclingAltEngine(t, tc.fill, tc.src)
			_, alt := cyclingAltOptions(e, id)
			if (len(alt) == 1) != tc.wantAlt || len(alt) > 1 {
				t.Fatalf("alternative cost options = %d, want offered=%v", len(alt), tc.wantAlt)
			}
			if !tc.wantAlt {
				return
			}
			if !strings.Contains(alt[0].Label, "alternate cost") {
				t.Fatalf("alternative option label %q", alt[0].Label)
			}
			life := e.G.Players[0].Life
			e.beginActivation(0, alt[0])
			if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
				t.Fatalf("self-discard cost posed a KChoose: %+v", d)
			}
			if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
				t.Fatalf("cycled card in %s, want graveyard", got)
			}
			if len(e.G.Stack) != 1 {
				t.Fatalf("stack depth = %d, want the cycling ability alone", len(e.G.Stack))
			}
			if e.G.Players[0].Life != life {
				t.Fatalf("the alternative paid life: %d -> %d", life, e.G.Players[0].Life)
			}
		})
	}
}
