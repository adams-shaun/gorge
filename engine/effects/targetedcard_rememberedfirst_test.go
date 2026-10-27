package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func corpusSVar(t *testing.T, name, svar string) (*cards.Card, *cards.SA) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup(name)
	if !ok || card == nil || len(card.Faces) == 0 {
		t.Fatalf("missing corpus card %s", name)
	}
	body := cards.ResolveSVar(card.Faces[0].SVars, svar)
	if body == nil {
		t.Fatalf("%s has no SVar %s", name, svar)
	}
	return card, body
}

// TestTargetedCardRememberedFirstSelectors pins the two Defined$ selectors:
// TargetedCard is the chain's object targets only (no player, never the
// source), RememberedFirst the first remembered object.
func TestTargetedCardRememberedFirstSelectors(t *testing.T) {
	h := newHost(t, 3)
	card := mkCard(t, "Name:Fixture\nTypes:Creature\nPT:1/1\nOracle:x\n")
	src := h.g.AddObject(card, 0).ID
	b := h.g.AddObject(card, 1).ID
	cc := h.g.AddObject(card, 1).ID
	if src == b || b == cc {
		t.Fatal("precondition: distinct objects")
	}
	tc := sa(t, "DB$ Pump | Defined$ TargetedCard | KW$ Trample")
	rf := sa(t, "DB$ ChangeZone | Defined$ RememberedFirst | Origin$ Exile | Destination$ Hand")

	c := &Ctx{Source: src, Controller: 0,
		Targets:    []state.Target{{Player: 1, IsPlayer: true}, {Obj: b}},
		Remembered: []state.Target{{Obj: b}, {Obj: cc}}}
	if got, ok := knownDefinedTargets(h, c, "TargetedCard"); !ok || !slices.Equal(got, []state.Target{{Obj: b}}) {
		t.Fatalf("TargetedCard = %v ok=%v, want [B]", got, ok)
	}
	if got := Defined(h, c, tc); !slices.Equal(got, []state.Target{{Obj: b}}) {
		t.Fatalf("Defined TargetedCard = %v, want [B]", got)
	}
	if got, ok := knownDefinedTargets(h, c, "RememberedFirst"); !ok || !slices.Equal(got, []state.Target{{Obj: b}}) {
		t.Fatalf("RememberedFirst = %v ok=%v, want [B]", got, ok)
	}
	if got := Defined(h, c, rf); !slices.Equal(got, []state.Target{{Obj: b}}) {
		t.Fatalf("Defined RememberedFirst = %v, want [B]", got)
	}

	empty := &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	for _, sel := range []string{"TargetedCard", "RememberedFirst"} {
		if got, ok := knownDefinedTargets(h, empty, sel); !ok || len(got) != 0 {
			t.Fatalf("%s with nothing bound = %v ok=%v, want empty ok", sel, got, ok)
		}
	}
	if got := Defined(h, empty, tc); len(got) != 0 {
		t.Fatalf("Defined TargetedCard with only a player target = %v, want none", got)
	}
	if got := Defined(h, empty, rf); len(got) != 0 {
		t.Fatalf("Defined RememberedFirst with nothing remembered = %v, want none", got)
	}
}

// TestTargetedCardRememberedFirstWillOfTheSultai resolves Will of the
// Sultai's real DBPump: the targeted creature gains trample, not the sorcery.
func TestTargetedCardRememberedFirstWillOfTheSultai(t *testing.T) {
	spell, body := corpusSVar(t, "Will of the Sultai", "DBPump")
	h := newHost(t, 2)
	src := h.g.AddObject(spell, 0).ID
	bear := h.g.AddObject(mkCard(t, "Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{bear})
	h.g.Obj(bear).Zone = state.ZBattlefield
	Resolve(h, &Ctx{Source: src, Controller: 0, SVars: spell.Faces[0].SVars,
		Targets: []state.Target{{Obj: bear}}}, body)
	if len(h.continuous) != 1 {
		t.Fatalf("continuous = %+v, want one trample grant", h.continuous)
	}
	ce := h.continuous[0]
	if ce.Source != bear || !slices.Equal(ce.AddKeywords, []string{"Trample"}) {
		t.Fatalf("grant = %+v, want Trample on the creature %d (sorcery is %d)", ce, bear, src)
	}
}

// TestTargetedCardRememberedFirstParallelThoughts resolves Parallel Thoughts'
// real replacement body: the first card of the exiled pile goes to hand.
func TestTargetedCardRememberedFirstParallelThoughts(t *testing.T) {
	ench, body := corpusSVar(t, "Parallel Thoughts", "RepParallelThoughts")
	h := &askHost{}
	h.g = state.NewGame(names(2))
	src := h.g.AddObject(ench, 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src})
	h.g.Obj(src).Zone = state.ZBattlefield
	fillLibrary(h.g, 0, mkCard(t, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"), 2)
	ids := append([]state.ObjID(nil), h.g.Zone(state.ZLibrary, 0)[:2]...)
	for _, id := range ids {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZExile})
	}
	if h.g.Obj(ids[0]).Zone != state.ZExile || h.g.Obj(ids[1]).Zone != state.ZExile {
		t.Fatal("precondition: both pile cards in exile")
	}
	Resolve(h, &Ctx{Source: src, Controller: 0, SVars: ench.Faces[0].SVars,
		Remembered: []state.Target{{Obj: ids[0]}, {Obj: ids[1]}}}, body)
	if got := h.g.Obj(ids[0]).Zone; got != state.ZHand {
		t.Fatalf("first pile card in %s, want Hand", got)
	}
	if got := h.g.Obj(ids[1]).Zone; got != state.ZExile {
		t.Fatalf("second pile card in %s, want Exile", got)
	}
}
