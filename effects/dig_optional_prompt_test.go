package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func TestDigOptionalPromptParametersAskForOptionalTake(t *testing.T) {
	for _, param := range []string{"PromptToSkipOptionalAbility", "OptionalAbilityPrompt"} {
		t.Run(param, func(t *testing.T) {
			h, ids := digAskFixture(t)
			saLine := "SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 2 | " + param + "$ True | ChangeValid$ Land | DestinationZone$ Hand"
			Resolve(h, &Ctx{Controller: 0}, sa(t, saLine))
			if h.asked == nil || h.asked.Kind != decision.KChoose || h.asked.Min != 0 || h.asked.Max != 2 {
				t.Fatalf("decision = %+v, want optional 0..2 take ask", h.asked)
			}
			h.asked = nil
			Resolve(h, &Ctx{Controller: 0, DigDone: true}, sa(t, saLine))
			if len(h.g.Zone(state.ZHand, 0)) != 0 {
				t.Fatal("declining the prompt moved a card")
			}
			if lib := h.g.Zone(state.ZLibrary, 0); len(lib) != len(ids) {
				t.Fatalf("library = %v, want unchanged after decline", lib)
			} else {
				for i := range ids {
					if lib[i] != ids[i] {
						t.Fatalf("library = %v, want original order %v", lib, ids)
					}
				}
			}
			if h.asked != nil {
				t.Fatalf("decline produced follow-up ask: %+v", h.asked)
			}
		})
	}
}
