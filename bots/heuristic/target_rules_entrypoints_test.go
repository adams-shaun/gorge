package heuristic

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/state"
)

// targetRuleBoard is the A2 fixture at the heuristic layer: a removal target
// decision offering an off-colour land (option 0) and the dominant-colour
// land (option 1), over a seat whose own manifest is mono-black. The plain
// ranking takes option 0 on the tie; the land-priority rule takes option 1.
func targetRuleBoard() botpolicy.Board {
	m := deck.Manifest{Main: []deck.ManifestRow{
		{Name: "Swamp", Count: 22, Land: true, Colour: cards.ColourMaskBlack},
	}}
	b := botpolicy.Board{
		Cards:     botpolicy.TableOf(map[state.ObjID]botpolicy.Card{}),
		Creatures: botpolicy.TableOf(map[state.ObjID]botpolicy.Creature{}),
		Life:      botpolicy.TableOf(map[state.PlayerID]int32{}),
		OwnDeck:   &m,
	}
	b.Cards.Set(10, botpolicy.Card{PrintedName: "Mountain", Land: true, Colour: cards.ColourMaskRed, CMC: 1})
	b.Cards.Set(11, botpolicy.Card{PrintedName: "Swamp", Land: true, Colour: cards.ColourMaskBlack, CMC: 1})
	return b
}

func targetRuleDecision() decision.Decision {
	return decision.Decision{
		Seq: 1, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1,
		TargetEffect: &decision.TargetEffect{API: "Destroy", Removal: &decision.RemovalEffect{Kind: "destroy"}},
		Options: []decision.Option{
			{Index: 0, Kind: "permanent", Obj: 10, Player: 1},
			{Index: 1, Kind: "permanent", Obj: 11, Player: 1},
		},
	}
}

// TestTargetRulesReachEveryEntryPoint pins the invariant that made A2 fail
// silently once already: the opt-in targeting rules travel on the Board like
// cast and mulligan, so a bot with them armed must apply them no matter
// which entry point built the Board.
//
// The caller-built Board here deliberately carries a ZERO Targets, which is
// what a real caller passes (botpolicy.BoardFromGameInto cannot know the
// bot's policy configuration). A rule applied only at one entry point -- the
// shape this test guards against -- leaves option 0 chosen and fails here.
func TestTargetRulesReachEveryEntryPoint(t *testing.T) {
	const wantArmed = 1 // the dominant-colour land

	bot := NewBotWithTargetRules(1, botpolicy.TargetRules{LandPriority: true})

	// Control: the same Board through the unarmed default bot keeps the
	// plain ranking, so this test fails if the fixture stops discriminating.
	plain, err := NewBot(1).DecideBoard(context.Background(), targetRuleBoard(), targetRuleDecision())
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Choices) != 1 || plain.Choices[0] != 0 {
		t.Fatalf("unarmed choices = %v, want [0]; the fixture no longer discriminates", plain.Choices)
	}

	viaBoard, err := bot.DecideBoard(context.Background(), targetRuleBoard(), targetRuleDecision())
	if err != nil {
		t.Fatal(err)
	}
	if len(viaBoard.Choices) != 1 || viaBoard.Choices[0] != wantArmed {
		t.Fatalf("DecideBoard choices = %v, want [%d] (rules must reach every entry point)", viaBoard.Choices, wantArmed)
	}
}
