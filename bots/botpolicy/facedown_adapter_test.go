package botpolicy_test

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

func parseAdapterCard(t *testing.T, src string) *cards.Card {
	t.Helper()
	card, diags := cards.ParseBytes("facedown_adapter_test.go", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	card.Link()
	return card
}

// adapterBoards builds both halves for viewer over g: the game half from
// state.Game, the view half from the real projection.
func adapterBoards(g *state.Game, viewer state.PlayerID) (gameBoard, viewBoard botpolicy.Board) {
	return botpolicy.BoardFromGame(g, printedChars{}, viewer), heuristic.BoardFromView(view.Project(g, nil, viewer, nil))
}

const adapterElf = "Name:Hidden Elf\nManaCost:1 G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"

// TestFaceDownPermanentAdaptersAgree pins CR 708.2 on both Board adapters:
// an opponent's face-down permanent is a public object whose face-down
// characteristics (a Creature, its P/T) every seat may read, while its name,
// cost and colour stay hidden. The projection shows the stripped CardView
// with those characteristics (view.faceDownCard); the game half used to
// write a zero Card for it, so a hunt Commander duel (seed 280, a disguised
// creature) read it as a non-creature on the hosted route only. From the
// controller's seat the face shows on both halves.
func TestFaceDownPermanentAdaptersAgree(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	o := g.AddObject(parseAdapterCard(t, adapterElf), 0)
	o.Zone = state.ZBattlefield
	o.FaceDown = true
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})
	for _, viewer := range []state.PlayerID{0, 1} {
		gameBoard, viewBoard := adapterBoards(g, viewer)
		if d := botpolicy.AdaptersDiffer(viewBoard, gameBoard); d != "" {
			t.Fatalf("viewer %d: the adapters disagree on a face-down permanent: %s", viewer, d)
		}
	}
	gameBoard, _ := adapterBoards(g, 1)
	c := gameBoard.Cards.Get(o.ID)
	if !c.Creature || c.TypeSig != botpolicy.TypeSigOf([]string{"Creature"}) {
		t.Errorf("opponent's Card for the face-down permanent lacks its public Creature type: %+v", c)
	}
	if c.PrintedName != "" || c.ManaCost != "" || c.CMC != 0 || c.Colour != 0 {
		t.Errorf("opponent's Card for the face-down permanent leaks its face: %+v", c)
	}
}

// TestFaceDownSpellAdaptersAgree pins the public-card census for a face-down
// spell on the stack (CR 708.4): every seat sees a face-down object, its
// controller its face too. The projection gave the controller's stack card no
// FaceDown mark and an opponent no card at all, so the view half listed it
// face-up (with its colour) for the controller and left it out for an
// opponent, one PublicCards entry short (31 vs 32 in the seed-280 duel).
func TestFaceDownSpellAdaptersAgree(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	o := g.AddObject(parseAdapterCard(t, adapterElf), 0)
	o.Zone = state.ZStack
	o.FaceDown = true
	g.Stack = append(g.Stack, o.ID)
	for _, viewer := range []state.PlayerID{0, 1} {
		gameBoard, viewBoard := adapterBoards(g, viewer)
		if d := botpolicy.AdaptersDiffer(viewBoard, gameBoard); d != "" {
			t.Fatalf("viewer %d: the adapters disagree on a face-down spell: %s", viewer, d)
		}
		pc, ok := viewBoard.PublicCards.Lookup(o.ID)
		if !ok || !pc.FaceDown || pc.Colour != 0 || pc.Controller != 0 {
			t.Errorf("viewer %d: view half's public card for the face-down spell = %+v (listed %v)", viewer, pc, ok)
		}
	}
}

// TestCurrentNameAdaptersAgree pins the name both halves give a transformed
// double-faced permanent: its current face's (CR 712.8e), as for a copy its
// copiable name (CR 707.2). The view half used to read CardView.CardName, the
// printed front face, while the game half read the current face.
func TestCurrentNameAdaptersAgree(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	src := "Name:Front Elf\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:x\nAlternateMode:DoubleFaced\n" +
		"ALTERNATE\nName:Back Beast\nTypes:Creature Beast\nPT:3/3\nOracle:y\n"
	o := g.AddObject(parseAdapterCard(t, src), 0)
	o.Zone = state.ZBattlefield
	o.SetFaceIdx(1)
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})
	for _, viewer := range []state.PlayerID{0, 1} {
		gameBoard, viewBoard := adapterBoards(g, viewer)
		if d := botpolicy.AdaptersDiffer(viewBoard, gameBoard); d != "" {
			t.Fatalf("viewer %d: the adapters disagree on a transformed permanent: %s", viewer, d)
		}
		if n := viewBoard.Cards.Get(o.ID).PrintedName; n != "Back Beast" {
			t.Errorf("viewer %d: transformed permanent named %q, want its current face Back Beast", viewer, n)
		}
	}
}
