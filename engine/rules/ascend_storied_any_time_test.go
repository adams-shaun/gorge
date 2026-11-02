package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Ascend and Storied read "any time you control" (CR 702.131b, 702.195a). Thalia's
// enter-tapped replacement folds each creature's entry itself, so only the
// state-based pass sees the tenth permanent.
func TestAscendGrantsPastAReplacedEntry(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	bears := lookup(t, reg, "Grizzly Bears")
	deck0 := []*cards.Card{lookup(t, reg, "Wayward Swordtooth")}
	for range 9 {
		deck0 = append(deck0, bears)
	}
	e := corpusEngine(t, reg, deck0, []*cards.Card{lookup(t, reg, "Thalia, Heretic Cathar")})
	moveByName(t, e, 1, "Thalia, Heretic Cathar", state.ZBattlefield)
	e.checkStateBased()
	moveByName(t, e, 0, "Wayward Swordtooth", state.ZBattlefield)
	for range 9 {
		moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	}
	e.checkStateBased()
	if !e.G.Players[0].Blessing {
		t.Fatal("ten permanents and an Ascend carrier: no city's blessing")
	}
}

// A phased-out permanent is treated as though it doesn't exist (CR 702.26b).
func TestAscendDoesNotCountAPhasedOutPermanent(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	bears := lookup(t, reg, "Grizzly Bears")
	deck0 := []*cards.Card{lookup(t, reg, "Wayward Swordtooth")}
	for range 9 {
		deck0 = append(deck0, bears)
	}
	e := corpusEngine(t, reg, deck0, nil)
	moveByName(t, e, 0, "Wayward Swordtooth", state.ZBattlefield)
	first := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	e.emit(events.Event{Kind: events.PhaseOut, Obj: first, Amount: 1})
	if !e.G.Obj(first).PhasedOut {
		t.Fatal("precondition: the Bears did not phase out")
	}
	for range 8 {
		moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	}
	e.checkStateBased()
	if e.G.Players[0].Blessing {
		t.Fatal("nine phased-in permanents gave the city's blessing")
	}
}

// A permanent that becomes an artifact or legendary counts from then on, which no
// entry announces.
func TestStoriedCountsATypeGainedLater(t *testing.T) {
	for _, gained := range []string{"Artifact", "Legendary"} {
		t.Run(gained, func(t *testing.T) {
			reg := testutil.CorpusRegistry(t)
			bears := lookup(t, reg, "Grizzly Bears")
			e := corpusEngine(t, reg, []*cards.Card{bears, bears}, nil)
			onBoard(t, e, 0, "Name:Story Teller\nTypes:Legendary Creature Dwarf\nPT:1/1\nK:Storied\nOracle:x\n")
			a := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
			b := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
			e.checkStateBased()
			if e.G.Players[0].EnduringStory {
				t.Fatal("one legendary and two Bears gave an enduring story")
			}
			for _, id := range []state.ObjID{a, b} {
				e.AddContinuous(ContinuousEffect{Source: id, Layer: LType, Affects: "Card.Self", AddTypes: []string{gained}, UntilEOT: true})
			}
			e.checkStateBased()
			if !e.G.Players[0].EnduringStory {
				t.Fatalf("three permanents that are %s: no enduring story", gained)
			}
		})
	}
}
