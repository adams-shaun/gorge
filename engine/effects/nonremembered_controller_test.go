package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestFracturedIdentityGivesEveryOtherPlayerACopy(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Fractured Identity")
	if !ok {
		t.Fatal("Fractured Identity missing from corpus")
	}
	if len(card.Faces) == 0 || len(card.Faces[0].Abilities) == 0 {
		t.Fatal("Fractured Identity has no spell ability")
	}
	// Upstream spells the copy leg over the spell's own target
	// (Defined$ Targeted | Controller$ NonTargetedController), read through
	// the repeat-player table (choose_control.go repeatPlayers).
	root := card.Faces[0].Abilities[0]
	sa := cards.ResolveSVar(card.Faces[0].SVars, "DBClone")
	if root.API != "ChangeZone" || sa == nil || sa.API != "CopyPermanent" ||
		sa.Params["Defined"] != "Targeted" || sa.Params["Controller"] != "NonTargetedController" {
		t.Fatalf("Fractured Identity DBClone = %#v (root %#v), want the targeted NonTargetedController copy", sa, root)
	}

	h := newHost(t, 4)
	h.g.Tokens = reg.Tokens
	source := h.g.AddObject(card, 0)
	source.Zone = state.ZStack
	// Owned by seat 1, controlled by seat 2: the exile resets the live
	// controller to the owner, so "its controller" is the last-known one.
	target := h.g.AddObject(mkCard(t, "Name:Targeted permanent\nTypes:Creature\nPT:3/3\nOracle:x\n"), 1)
	target.Zone = state.ZBattlefield
	target.Controller = 2
	h.g.SetZone(state.ZBattlefield, 2, []state.ObjID{target.ID})
	c := &Ctx{Source: source.ID, Controller: 0, Targets: []state.Target{{Obj: target.ID}}}
	Resolve(h, c, root)
	if z := h.g.Obj(target.ID).Zone; z != state.ZExile {
		t.Fatalf("target zone = %v, want exile", z)
	}

	counts := make([]int, len(h.g.Players))
	for i := range h.g.Objs {
		o := &h.g.Objs[i]
		if o.IsToken && o.Face() != nil && o.Face().Name == "Targeted permanent" {
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

// TestCopyPermanentNonTargetedControllerRaw: the bare upstream selector gives
// every player but the exiled target's last-known controller a copy.
func TestCopyPermanentNonTargetedControllerRaw(t *testing.T) {
	h := newHost(t, 4)
	target := h.g.AddObject(mkCard(t, "Name:Probe permanent\nTypes:Creature\nPT:3/3\nOracle:x\n"), 1)
	target.Zone = state.ZExile
	src := h.g.AddObject(mkCard(t, "Name:Probe source\nTypes:Sorcery\nOracle:x\n"), 0)
	src.Zone = state.ZStack
	c := &Ctx{Source: src.ID, Controller: 0, Targets: []state.Target{{Obj: target.ID}},
		Snap: LKISnapshots{TargetController: map[state.ObjID]state.PlayerID{target.ID: 2}}}
	effCopyPermanent(h, c, sa(t, "DB$ CopyPermanent | Controller$ NonTargetedController | Defined$ Targeted"))
	counts := make([]int, len(h.g.Players))
	for i := range h.g.Objs {
		o := &h.g.Objs[i]
		if o.IsToken && o.Face() != nil && o.Face().Name == "Probe permanent" {
			counts[o.Controller]++
		}
	}
	if counts[0] != 1 || counts[1] != 1 || counts[2] != 0 || counts[3] != 1 {
		t.Fatalf("copy owners = %v, want [1 1 0 1]; events=%+v", counts, h.log)
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
	if len(got) != 4 {
		t.Fatalf("NonRememberedController with nothing remembered = %v; want all 4 players", got)
	}
	for i, want := range []state.PlayerID{0, 1, 2, 3} {
		if !got[i].IsPlayer || got[i].Player != want {
			t.Fatalf("NonRememberedController[%d] with nothing remembered = %+v, want player %d", i, got[i], want)
		}
	}
	got = Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "OppNonRememberedController"}})
	if len(got) != 3 {
		t.Fatalf("OppNonRememberedController with nothing remembered = %v; want 3 opponents", got)
	}
	for i, want := range []state.PlayerID{1, 2, 3} {
		if !got[i].IsPlayer || got[i].Player != want {
			t.Fatalf("OppNonRememberedController[%d] with nothing remembered = %+v, want player %d", i, got[i], want)
		}
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

	// The player-visible "who can't" case: with nothing remembered the
	// complement is every player, so all four seats discard.
	c.Remembered = nil
	hands = make([]state.ObjID, len(h.g.Players))
	for p := range h.g.Players {
		handCard := h.g.AddObject(mkCard(t, "Name:second hand card\nTypes:Sorcery\nOracle:x\n"), state.PlayerID(p))
		handCard.Zone = state.ZHand
		h.g.SetZone(state.ZHand, state.PlayerID(p), []state.ObjID{handCard.ID})
		hands[p] = handCard.ID
	}
	for p, id := range hands {
		if h.g.Obj(id).Zone != state.ZHand {
			t.Fatalf("precondition: seat %d's second card is in %s, want hand", p, h.g.Obj(id).Zone)
		}
	}
	effDiscard(h, c, discard)
	for p, id := range hands {
		if h.g.Obj(id).Zone != state.ZGraveyard {
			t.Fatalf("Plaguecrafter with nothing remembered did not discard seat %d (zone %s)", p, h.g.Obj(id).Zone)
		}
	}
}

// TestNonRememberedControllerExcludesEveryRememberedController pins the set
// complement: with two remembered cards controlled by seats 1 and 2, both
// seats are excluded -- the old first-live-only read left seat 2 in.
func TestNonRememberedControllerExcludesEveryRememberedController(t *testing.T) {
	h := newHost(t, 4)
	source := h.g.AddObject(mkCard(t, "Name:Remembering source\nTypes:Enchantment\nOracle:x\n"), 0)
	source.Zone = state.ZBattlefield
	first := h.g.AddObject(mkCard(t, "Name:First remembered\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	first.Zone = state.ZExile
	second := h.g.AddObject(mkCard(t, "Name:Second remembered\nTypes:Creature\nPT:1/1\nOracle:x\n"), 2)
	second.Zone = state.ZExile
	c := &Ctx{Source: source.ID, Controller: 0, Remembered: []state.Target{{Obj: first.ID}, {Obj: second.ID}}}
	if got := PlayerOf(h, c, c.Remembered[0]); got != 1 {
		t.Fatalf("precondition: first remembered anchor maps to seat %d, want 1", got)
	}
	if got := PlayerOf(h, c, c.Remembered[1]); got != 2 {
		t.Fatalf("precondition: second remembered anchor maps to seat %d, want 2", got)
	}

	wantPlayers(t, definedPlayerIDs(h, c, "NonRememberedController"), []state.PlayerID{0, 3},
		"definedPlayerIDs(NonRememberedController) with controllers 1 and 2 remembered")
	wantPlayers(t, definedPlayerIDs(h, c, "OppNonRememberedController"), []state.PlayerID{3},
		"definedPlayerIDs(OppNonRememberedController) with controllers 1 and 2 remembered")
}
