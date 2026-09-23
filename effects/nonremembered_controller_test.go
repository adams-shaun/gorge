package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestFracturedIdentityGivesEveryOtherPlayerACopy(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Fractured Identity")
	if !ok {
		t.Fatal("Fractured Identity missing from corpus")
	}
	if len(card.Faces) == 0 {
		t.Fatal("Fractured Identity has no face")
	}
	sa := cards.ResolveSVar(card.Faces[0].SVars, "DBClone")
	if sa == nil || sa.API != "CopyPermanent" || sa.Params["Controller"] != "NonRememberedController" {
		t.Fatalf("Fractured Identity DBClone = %#v, want CopyPermanent Controller$ NonRememberedController", sa)
	}

	h := newHost(t, 4)
	h.g.Tokens = reg.Tokens
	source := h.g.AddObject(card, 0)
	source.Zone = state.ZStack
	excluded := h.g.AddObject(mkCard(t, "Name:Remembered permanent\nTypes:Creature\nPT:3/3\nOracle:x\n"), 2)
	excluded.Zone = state.ZExile
	c := &Ctx{Source: source.ID, Controller: 0, Remembered: []state.Target{{Obj: excluded.ID}}}
	effCopyPermanent(h, c, sa)

	counts := make([]int, len(h.g.Players))
	for i := range h.g.Objs {
		o := &h.g.Objs[i]
		if o.IsToken && o.Face() != nil && o.Face().Name == "Remembered permanent" {
			counts[o.Controller]++
		}
	}
	if counts[0] != 1 || counts[1] != 1 || counts[2] != 0 || counts[3] != 1 {
		t.Fatalf("Fractured Identity copy owners = %v, want [1 1 0 1]; events=%+v", counts, h.log)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			t.Fatalf("unexpected unsupported-selector note: %q", ev.Text)
		}
	}
}

func TestNonRememberedControllerDefinedPlayers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Plaguecrafter")
	if !ok {
		t.Fatal("Plaguecrafter missing from corpus")
	}
	var discard *cards.SA
	for _, face := range card.Faces {
		candidate := cards.ResolveSVar(face.SVars, "Discard")
		if candidate != nil && candidate.API == "Discard" && candidate.Params["Defined"] == "NonRememberedController" {
			discard = candidate
		}
	}
	if discard == nil {
		t.Fatal("Plaguecrafter corpus selector moved")
	}

	h := newHost(t, 4)
	source := h.g.AddObject(card, 0)
	source.Zone = state.ZBattlefield
	anchor := h.g.AddObject(mkCard(t, "Name:Remembered card\nTypes:Creature\nPT:1/1\nOracle:x\n"), 2)
	anchor.Zone = state.ZExile
	c := &Ctx{Source: source.ID, Controller: 0, Remembered: []state.Target{{Obj: anchor.ID}}}
	got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "NonRememberedController"}})
	if len(got) != 3 {
		t.Fatalf("Plaguecrafter Defined$ NonRememberedController = %v; want 3 live players", got)
	}
	for i, want := range []state.PlayerID{0, 1, 3} {
		if !got[i].IsPlayer || got[i].Player != want {
			t.Fatalf("NonRememberedController[%d] = %+v, want player %d", i, got[i], want)
		}
	}
	got = Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "OppNonRememberedController"}})
	if len(got) != 2 || got[0].Player != 1 || got[1].Player != 3 {
		t.Fatalf("OppNonRememberedController = %v; want players 1 and 3", got)
	}
	c.Remembered = nil
	got = Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "NonRememberedController"}})
	if len(got) != 0 {
		t.Fatalf("unbound NonRememberedController = %v; want empty set", got)
	}

	c.Remembered = []state.Target{{Obj: anchor.ID}}
	hands := make([]state.ObjID, len(h.g.Players))
	for p := range h.g.Players {
		handCard := h.g.AddObject(mkCard(t, "Name:hand card\nTypes:Sorcery\nOracle:x\n"), state.PlayerID(p))
		handCard.Zone = state.ZHand
		h.g.SetZone(state.ZHand, state.PlayerID(p), []state.ObjID{handCard.ID})
		hands[p] = handCard.ID
	}
	effDiscard(h, c, discard)
	for p, id := range hands {
		wantDiscarded := p != 2
		gotDiscarded := h.g.Obj(id).Zone == state.ZGraveyard
		if gotDiscarded != wantDiscarded {
			t.Fatalf("Plaguecrafter discarded seat %d card=%v, want %v (zone %s)", p, gotDiscarded, wantDiscarded, h.g.Obj(id).Zone)
		}
	}
}
