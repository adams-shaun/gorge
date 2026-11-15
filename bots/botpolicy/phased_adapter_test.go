package botpolicy_test

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestPhasedOutAdaptersAgree pins CR 702.26b on both Board adapters: a
// phased-out permanent is treated as though it does not exist, so the
// projection drops it from its battlefield (view.zoneCard) and the game half
// must too. The game half used to keep it in Creatures, Cards and
// PublicCards, so the hosted bot (game half) and a View-fed seat saw
// different boards whenever something phased out: a hunt Commander duel
// (seed 389, Boreal Druid phased out) attacked with a different creature
// set on the two routes. Checked from the controller's seat and an
// opponent's, the Boards must agree table by table, order included, and
// neither may hold the phased-out creature.
func TestPhasedOutAdaptersAgree(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	mk := func(name string, owner state.PlayerID) state.ObjID {
		src := fmt.Sprintf("Name:%s\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n", name)
		card, diags := cards.ParseBytes("phased_adapter_test.go", []byte(src))
		if len(diags) != 0 {
			t.Fatalf("parse: %v", diags)
		}
		card.Link()
		o := g.AddObject(card, owner)
		o.Zone = state.ZBattlefield
		g.SetZone(state.ZBattlefield, owner, append(g.Zone(state.ZBattlefield, owner), o.ID))
		return o.ID
	}
	phased := mk("Phased Elf", 0)
	present := mk("Present Elf", 0)
	foreign := mk("Foreign Elf", 1)
	events.Apply(g, events.Event{Kind: events.PhaseOut, Obj: phased, Amount: 1})
	if !g.Obj(phased).PhasedOut {
		t.Fatal("precondition: the creature did not phase out")
	}
	for _, viewer := range []state.PlayerID{0, 1} {
		gameBoard := botpolicy.BoardFromGame(g, printedChars{}, viewer)
		viewBoard := heuristic.BoardFromView(view.Project(g, nil, viewer, nil))
		if d := botpolicy.AdaptersDiffer(viewBoard, gameBoard); d != "" {
			t.Fatalf("viewer %d: the adapters disagree with a phased-out permanent: %s", viewer, d)
		}
		for name, b := range map[string]botpolicy.Board{"game": gameBoard, "view": viewBoard} {
			if _, ok := b.Creatures.Lookup(phased); ok {
				t.Errorf("viewer %d, %s half: the phased-out creature is in Creatures", viewer, name)
			}
			if _, ok := b.Cards.Lookup(phased); ok {
				t.Errorf("viewer %d, %s half: the phased-out creature is in Cards", viewer, name)
			}
			if _, ok := b.PublicCards.Lookup(phased); ok {
				t.Errorf("viewer %d, %s half: the phased-out creature is in PublicCards", viewer, name)
			}
			for _, id := range []state.ObjID{present, foreign} {
				if _, ok := b.Creatures.Lookup(id); !ok {
					t.Errorf("viewer %d, %s half: phased-in creature %d is missing from Creatures", viewer, name, id)
				}
			}
		}
	}
}
