package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestOpeningEffectCommaTriggersRegistersBothChildren pins
// registerOpeningEffectTriggers' split of an opening Effect's Triggers$ list.
// Forge writes the list comma-separated; a whitespace-only split looks up the
// whole list as one SVar and registers nothing, so the pregame pass silently
// arms none of the Effect's promises.
func TestOpeningEffectCommaTriggersRegistersBothChildren(t *testing.T) {
	t.Parallel()
	src := "Name:Comma Chancellor\nManaCost:0\nTypes:Creature Human\nPT:1/1\n" +
		"SVar:RevealCard:DB$ Reveal | RevealDefined$ Self | SubAbility$ OnMain | SpellDescription$ x\n" +
		"SVar:OnMain:DB$ Effect | Triggers$ TrigUpkeep,TrigMain | Duration$ Permanent\n" +
		"SVar:TrigUpkeep:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | OneOff$ True | Execute$ FixtureUpkeep | TriggerDescription$ x\n" +
		"SVar:TrigMain:Mode$ Phase | Phase$ Main1 | ValidPlayer$ You | OneOff$ True | Execute$ FixtureMain | TriggerDescription$ x\n" +
		"SVar:FixtureUpkeep:DB$ Mana | Produced$ G\n" +
		"SVar:FixtureMain:DB$ Mana | Produced$ G\n" +
		"Oracle:x\n"
	e := handEngine(t, card(t, src))
	id := e.G.Zone(state.ZHand, 0)[0]
	face := e.G.Obj(id).Face()
	if face == nil || e.G.Obj(id).Zone != state.ZHand {
		t.Fatalf("precondition: opening card zone = %v, want hand", e.G.Obj(id))
	}
	sa := cards.ResolveSVar(face.SVars, "OnMain")
	if sa == nil || sa.API != "Effect" {
		t.Fatalf("precondition: OnMain = %+v, want an Effect body", sa)
	}
	e.registerOpeningEffectTriggers(openingEffect{player: 0, card: id, svar: "RevealCard"}, sa)
	if len(e.G.Delayed) != 2 {
		t.Fatalf("comma-separated Triggers$ registered %d opening triggers, want 2: %+v", len(e.G.Delayed), e.G.Delayed)
	}
}
