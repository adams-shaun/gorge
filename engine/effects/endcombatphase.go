package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { Register("EndCombatPhase", effEndCombatPhase) }

// effEndCombatPhase implements DB$/SP$/AB$ EndCombatPhase (Forge's
// EndCombatPhaseEffect; the corpus carriers are Mandate of Peace and Merciful
// Mandate). CR 724.2's "end the combat phase" is a resolution-time
// turn-structure change, so like EndTurn it goes through events: one
// events.EndCombatPhase whose IDs is a snapshot of the whole stack taken
// BEFORE the emit, exiling every object on it, the resolving spell included
// (724.2b), and removing every creature and planeswalker from combat
// (724.2d). The rules engine then skips to the phase after combat (see
// Engine.finishEndCombatPhase).
//
// CR 724.2g: an effect that tries to end the combat phase outside a combat
// phase does nothing, whatever the card's own ConditionPhases$ says. A
// declined Optional$ election ("you may end the combat phase") leaves the
// resolving spell to finish normally.
func effEndCombatPhase(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	if g.Step < state.StepBeginCombat || g.Step > state.StepEndCombat {
		return
	}
	if strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKOptional)), "True") {
		chooser := c.Controller
		d := &decision.Decision{Player: chooser, Kind: decision.KChoose, Min: 1, Max: 1,
			Source: c.Source, ResumeKind: "endcombatphase_optional", ResumeSA: sa,
			Prompt: "End the combat phase?", Options: []decision.Option{
				{Index: 0, Kind: "yes", Label: "Yes", Player: chooser},
				{Index: 1, Kind: "no", Label: "No", Player: chooser},
			}}
		ans, ok := AskTape(h, d)
		if !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "EndCombatPhase Optional$ declined (no engine host to ask)"})
			return
		}
		if len(ans) == 0 || ans[0].Kind != "yes" {
			return
		}
	}
	ids := append([]state.ObjID(nil), g.Stack...)
	h.Emit(events.Event{Kind: events.EndCombatPhase, IDs: ids})
}
