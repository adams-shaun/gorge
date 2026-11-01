package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestNoStateBasedActionsBetweenSacrificeCosts pins CR 704.3: state-based
// actions wait for priority, which an activation withholds until its costs are
// paid. A token sacrificed as the first of two costs into a replacement ask
// stays where it went until then, and ceases to exist only at that boundary
// (CR 704.5d).
func TestNoStateBasedActionsBetweenSacrificeCosts(t *testing.T) {
	t.Parallel()
	const (
		exilerA = "Name:Exiler A\nTypes:Enchantment\nR:Event$ Moved | ActiveZones$ Battlefield | Destination$ Graveyard | ValidCard$ Card | ReplaceWith$ Banish | Description$ Exile whatever would go to a graveyard.\nSVar:Banish:DB$ ChangeZone | Hidden$ True | Origin$ All | Destination$ Exile | Defined$ ReplacedCard\nOracle:x\n"
		exilerB = "Name:Exiler B\nTypes:Enchantment\nR:Event$ Moved | ActiveZones$ Battlefield | Destination$ Graveyard | ValidCard$ Card | ReplaceWith$ Banish | Description$ Banish whatever would go to a graveyard.\nSVar:Banish:DB$ ChangeZone | Hidden$ True | Origin$ All | Destination$ Exile | Defined$ ReplacedCard\nOracle:x\n"
		altar   = "Name:Altar\nTypes:Enchantment\nA:AB$ GainLife | Cost$ Sac<2/Artifact> | LifeAmount$ 1 | SpellDescription$ Gain 1 life.\nOracle:x\n"
		trinket = "Name:Trinket\nTypes:Artifact\nOracle:x\n"
	)
	e := handEngine(t)
	onBoard(t, e, 0, exilerA)
	onBoard(t, e, 0, exilerB)
	source := onBoard(t, e, 0, altar)
	tokens := []state.ObjID{onBoard(t, e, 0, trinket), onBoard(t, e, 0, trinket)}
	for _, id := range tokens {
		e.G.Obj(id).IsToken = true
	}
	activateAbilityOn(t, e, source)
	exiledDuringAsk := false
	for i := 0; i < 10; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		if d.Kind == decision.KReplacement {
			for _, id := range tokens {
				switch e.G.Obj(id).Zone {
				case state.ZExile:
					exiledDuringAsk = true
				case state.ZCeased:
					t.Fatalf("token %d ceased to exist before anyone received priority", id)
				}
			}
			submitChoices(t, e, 0)
			continue
		}
		choices := make([]int, 0, d.Min)
		for j := 0; j < d.Min; j++ {
			choices = append(choices, j)
		}
		submitChoices(t, e, choices...)
	}
	if !exiledDuringAsk {
		t.Fatal("fixture never reached a replacement ask with a token already sacrificed")
	}
	for _, id := range tokens {
		if z := e.G.Obj(id).Zone; z != state.ZCeased {
			t.Fatalf("token %d in zone %v once priority came, want it gone", id, z)
		}
	}
}
