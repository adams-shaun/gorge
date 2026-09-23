package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The real Memory Leak fetch names a targeted player's hand AND graveyard.
// Neither zone may silently fall through to the source-default object mover.
func TestMemoryLeakCompoundFetchPlayerChoosesFromBothZones(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Memory Leak")
	if !ok {
		t.Fatal("corpus missing Memory Leak")
	}
	var fetch *cards.SA
	for _, ability := range card.Faces[0].Abilities {
		if ability.Params["Origin"] == "Hand,Graveyard" && ability.Params["DefinedPlayer"] == "Targeted" {
			fetch = ability
			break
		}
		for sub := ability.Sub; sub != nil; sub = sub.Sub {
			if sub.Params["Origin"] == "Hand,Graveyard" && sub.Params["DefinedPlayer"] == "Targeted" {
				fetch = sub
				break
			}
		}
	}
	if fetch == nil {
		t.Fatal("corpus pin moved: Memory Leak fetch selector absent")
	}
	h := &askHost{}
	h.g = state.NewGame(names(2))
	source := h.g.AddObject(card, 0)
	source.Zone = state.ZStack
	hand := h.g.AddObject(mkCard(t, "Name:Hand Creature\nTypes:Creature\nPT:2/2\nOracle:x\n"), 1)
	hand.Zone = state.ZHand
	grave := h.g.AddObject(mkCard(t, "Name:Grave Creature\nTypes:Creature\nPT:3/3\nOracle:x\n"), 1)
	grave.Zone = state.ZGraveyard
	other := h.g.AddObject(mkCard(t, "Name:Other Hand Creature\nTypes:Creature\nPT:4/4\nOracle:x\n"), 0)
	other.Zone = state.ZHand
	h.g.SetZone(state.ZHand, 1, []state.ObjID{hand.ID})
	h.g.SetZone(state.ZGraveyard, 1, []state.ObjID{grave.ID})
	h.g.SetZone(state.ZHand, 0, []state.ObjID{other.ID})
	if hand.Zone == grave.Zone || hand.Owner == other.Owner || hand.ID == grave.ID {
		t.Fatal("test candidates do not differ in zone and owner")
	}
	c := &Ctx{Source: source.ID, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	effChangeZone(h, c, fetch)
	if h.asked == nil || h.asked.ResumeKind != "search" || h.asked.Player != 0 {
		t.Fatalf("decision = %+v, want controller's compound search", h.asked)
	}
	seen := map[state.ObjID]bool{}
	for _, opt := range h.asked.Options {
		seen[opt.Obj] = true
		if opt.Player != 1 {
			t.Fatalf("candidate owned by %d, want targeted player", opt.Player)
		}
	}
	if !seen[hand.ID] || !seen[grave.ID] || seen[other.ID] {
		t.Fatalf("options = %+v, want targeted player's hand and graveyard only", h.asked.Options)
	}
	c.Search, c.SearchDone = []state.ObjID{grave.ID}, true
	c.LibraryTarget = 0
	effChangeZone(h, c, fetch)
	if got := h.g.Obj(grave.ID).Zone; got != state.ZExile {
		t.Fatalf("chosen grave card zone = %v, want exile", got)
	}
	if got := h.g.Obj(hand.ID).Zone; got != state.ZHand {
		t.Fatalf("unchosen hand card zone = %v, want hand", got)
	}
}

// Player-valued Defined$ and ValidTgts$ selectors share the same fetch
// contract as DefinedPlayer$: their answer identifies whose zones to offer,
// not a card to move. A battlefield/hand union is included as well.
func TestCompoundOriginPlayerSelectorKinds(t *testing.T) {
	for _, tc := range []struct {
		name, script string
	}{
		{"defined", "DB$ ChangeZone | Defined$ Targeted | Origin$ Library,Hand | Destination$ Exile | ChangeType$ Creature"},
		{"valid-target", "DB$ ChangeZone | ValidTgts$ Player | Origin$ Hand,Graveyard | Destination$ Exile | ChangeType$ Creature"},
		{"battlefield-hand", "DB$ ChangeZone | DefinedPlayer$ Targeted | Origin$ Battlefield,Hand | Destination$ Exile | ChangeType$ Creature"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &askHost{}
			h.g = state.NewGame(names(2))
			source := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Sorcery\nOracle:x\n"), 0)
			hand := h.g.AddObject(mkCard(t, "Name:Hand Creature\nTypes:Creature\nPT:2/2\nOracle:x\n"), 1)
			hand.Zone = state.ZHand
			otherZone := state.ZGraveyard
			switch tc.name {
			case "defined":
				otherZone = state.ZLibrary
			case "battlefield-hand":
				otherZone = state.ZBattlefield
			}
			other := h.g.AddObject(mkCard(t, "Name:Other Creature\nTypes:Creature\nPT:3/3\nOracle:x\n"), 1)
			other.Zone = otherZone
			h.g.SetZone(state.ZHand, 1, []state.ObjID{hand.ID})
			h.g.SetZone(otherZone, 1, []state.ObjID{other.ID})
			if hand.Zone == other.Zone || hand.ID == other.ID {
				t.Fatal("candidates must differ in zone and identity")
			}
			c := &Ctx{Source: source.ID, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
			s := sa(t, tc.script)
			effChangeZone(h, c, s)
			if h.asked == nil || h.asked.ResumeKind != "search" {
				t.Fatalf("decision = %+v, want compound search", h.asked)
			}
			seen := map[state.ObjID]bool{}
			for _, opt := range h.asked.Options {
				seen[opt.Obj] = true
				if opt.Player != 1 {
					t.Fatalf("option %+v not owned by selected player", opt)
				}
			}
			if !seen[hand.ID] || !seen[other.ID] {
				t.Fatalf("options = %+v, want selected player's hand and %v", h.asked.Options, otherZone)
			}
		})
	}
}
