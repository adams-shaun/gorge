package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestTokenCreatedPredicate(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	token := g.AddObject(mkCard(t, "Name:Food Token\nTypes:Token Artifact\nOracle:x\n"), 0)
	token.IsToken = true
	token.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{token.ID})
	card := g.AddObject(mkCard(t, "Name:Relic\nTypes:Artifact\nOracle:x\n"), 0)
	card.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), card.ID))

	if !MatchesSpec(g, "Card.tokenCreated", token.ID, 0) {
		t.Fatal("tokenCreated did not match a token")
	}
	if MatchesSpec(g, "Card.tokenCreated", card.ID, 0) {
		t.Fatal("tokenCreated matched a non-token card")
	}
	if MatchesSpec(g, "Card.!tokenCreated", token.ID, 0) {
		t.Fatal("!tokenCreated matched a token")
	}
	if !MatchesSpec(g, "Card.!tokenCreated", card.ID, 0) {
		t.Fatal("!tokenCreated did not match a non-token")
	}
	if len(UnknownPredicates("Card.tokenCreated")) != 0 {
		t.Fatal("tokenCreated was reported as unknown")
	}
}

func TestTokenCreatedThisTurnEnteredCount(t *testing.T) {
	h := newHost(t, 2)
	g := h.g
	token := g.AddObject(mkCard(t, "Name:Food Token\nTypes:Token Artifact\nOracle:x\n"), 0)
	token.IsToken = true
	token.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{token.ID})
	other := g.AddObject(mkCard(t, "Name:Relic\nTypes:Artifact\nOracle:x\n"), 0)
	other.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), other.ID))
	opponent := g.AddObject(mkCard(t, "Name:Enemy Token\nTypes:Token Artifact\nOracle:x\n"), 1)
	opponent.IsToken = true
	opponent.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 1, []state.ObjID{opponent.ID})
	g.Entered = []state.ZoneEntry{
		{Obj: token.ID, To: state.ZBattlefield, From: state.ZLibrary},
		{Obj: other.ID, To: state.ZBattlefield, From: state.ZLibrary},
		{Obj: opponent.ID, To: state.ZBattlefield, From: state.ZLibrary},
	}

	c := &Ctx{Controller: 0}
	got, ok := EvalCountOK(h, c, "Count$ThisTurnEntered_Battlefield_Card.tokenCreated+YouCtrl")
	if !ok || got != 1 {
		t.Fatalf("tokenCreated count = (%d, %v), want (1, true)", got, ok)
	}
}
