package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestExplorersScopeDigPosesOptionalTake(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Explorer's Scope")
	if !ok {
		t.Fatal("corpus is missing Explorer's Scope")
	}
	var effect *cards.SA
	var svars map[string]string
	for _, face := range card.Faces {
		if _, ok := face.SVars["TrigDig"]; ok {
			effect = cards.ResolveSVar(face.SVars, "TrigDig")
			svars = face.SVars
			break
		}
	}
	if effect == nil || effect.API != "Dig" || effect.Params["OptionalAbilityPrompt"] == "" {
		t.Fatalf("precondition: Explorer's Scope TrigDig is not the expected Dig shape: %+v", effect)
	}
	h := &askHost{}
	h.g = state.NewGame(names(2))
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	id := h.g.AddObject(land, 0).ID
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{id})
	Resolve(h, &Ctx{Controller: 0, SVars: svars}, effect)
	if h.asked == nil || h.asked.Kind != decision.KChoose || h.asked.Min != 0 {
		t.Fatalf("Explorer's Scope decision = %+v, want optional take election", h.asked)
	}
}

func TestDigOptionalPromptParametersAskForOptionalTake(t *testing.T) {
	for _, param := range []string{"PromptToSkipOptionalAbility", "OptionalAbilityPrompt"} {
		t.Run(param, func(t *testing.T) {
			h, ids := digAskFixture(t)
			value := "True"
			if param == "OptionalAbilityPrompt" {
				value = "Would you like to put the land onto the battlefield tapped?"
			}
			saLine := "SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 2 | " + param + "$ " + value + " | ChangeValid$ Land | DestinationZone$ Hand"
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
