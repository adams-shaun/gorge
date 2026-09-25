package events

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// cipherEncodeFixture builds a two-seat game with a Bear creature under seat
// 0's control on the battlefield and a second Bear card in seat 0's graveyard
// -- the shape a resolving Cipher spell presents to the encode half
// (effects/cipher.go): the spell card leaving the stack for the graveyard, the
// chosen encoder still a battlefield permanent.
func cipherEncodeFixture(t *testing.T) (*state.Game, *Log, state.ObjID, state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"a", "b"})
	creature := g.AddObject(bearCard(), 0)
	creature.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{creature.ID})
	card := g.AddObject(bearCard(), 0)
	card.Zone = state.ZGraveyard
	g.SetZone(state.ZGraveyard, 0, []state.ObjID{card.ID})
	return g, NewLog(1), creature.ID, card.ID
}

// cipherEncodeEvents is the exact event pair effects/cipher.go's performing
// half emits (CR 702.99a): exile the spell card, then record the association
// on the chosen creature through the Imprint kind's "encoded" Text
// discriminator. Kept as one helper so the test and its replay drive the
// identical bytes.
func cipherEncodeEvents(card, creature state.ObjID) []Event {
	return []Event{
		{Kind: MoveZone, Obj: card, From: state.ZGraveyard, To: state.ZExile, Text: "encoded"},
		{Kind: Imprint, Obj: creature, IDs: []state.ObjID{card}, Text: "encoded"},
	}
}

// assertCipherAssociation checks both ends of the association the brief pins:
// the creature-side link (state.Object.EncodedCards) and the encoded card's
// own zone. The caller's want decides which shape is expected.
func assertCipherAssociation(t *testing.T, g *state.Game, creature, card state.ObjID, want []state.ObjID) {
	t.Helper()
	co := g.Obj(creature)
	if co == nil {
		t.Fatal("encoder creature object vanished")
	}
	if co.Zone != state.ZBattlefield {
		t.Fatalf("precondition: encoder zone = %v, want battlefield", co.Zone)
	}
	if got := co.EncodedCards; len(got) != len(want) {
		t.Fatalf("creature %d EncodedCards = %v, want %v", creature, got, want)
	}
	for i, id := range want {
		if co.EncodedCards[i] != id {
			t.Fatalf("creature %d EncodedCards = %v, want %v", creature, co.EncodedCards, want)
		}
	}
	ko := g.Obj(card)
	if ko == nil {
		t.Fatal("encoded card object vanished")
	}
	if ko.Zone != state.ZExile {
		t.Fatalf("encoded card zone = %v, want exile", ko.Zone)
	}
	if n := zoneCount(g, card); n != 1 {
		t.Fatalf("encoded card is in %d zones, want exactly one", n)
	}
	if n := zoneCount(g, creature); n != 1 {
		t.Fatalf("encoder is in %d zones, want exactly one", n)
	}
}

// TestCipherEncodedAssociation pins the Cipher encode/lifecycle event folds at
// the events layer, independent of the rules/effects pipeline that drives them
// (rules/cipher_test.go covers that end-to-end): the Imprint "encoded"
// discriminator establishes the association, events.Move prunes it when the
// encoded card leaves exile, clears it when the encoder leaves the
// battlefield, and a log-only replay of the emitted events rebuilds every
// intermediate shape exactly.
func TestCipherEncodedAssociation(t *testing.T) {
	g, l, creature, card := cipherEncodeFixture(t)
	// Preconditions the assertions below depend on: encoder a battlefield
	// permanent, card in the graveyard, no association yet.
	if co := g.Obj(creature); co.Zone != state.ZBattlefield {
		t.Fatalf("precondition: encoder zone = %v, want battlefield", co.Zone)
	}
	if ko := g.Obj(card); ko.Zone != state.ZGraveyard {
		t.Fatalf("precondition: encoded card zone = %v, want graveyard", ko.Zone)
	}
	if got := g.Obj(creature).EncodedCards; len(got) != 0 {
		t.Fatalf("precondition: fresh creature already encodes %v", got)
	}

	for _, e := range cipherEncodeEvents(card, creature) {
		Emit(g, l, e)
	}
	assertCipherAssociation(t, g, creature, card, []state.ObjID{card})

	// Replay the establishment from a fresh initial state: the fold alone
	// must rebuild the association.
	rp, rl, rCreature, rCard := cipherEncodeFixture(t)
	for _, e := range l.Events {
		Emit(rp, rl, e)
	}
	assertCipherAssociation(t, rp, rCreature, rCard, []state.ObjID{rCard})
	if got, want := len(rl.Events), len(l.Events); got != want {
		t.Fatalf("replay log recorded %d events, live recorded %d", got, want)
	}

	// Lifecycle A: the encoded card leaves exile by ANY path (here a plain
	// MoveZone to hand) and the stale link is pruned inside the fold.
	Emit(g, l, Event{Kind: MoveZone, Obj: card, From: state.ZExile, To: state.ZHand})
	if got := g.Obj(creature).EncodedCards; len(got) != 0 {
		t.Fatalf("encoded card left exile but the association survived: %v", got)
	}
	if ko := g.Obj(card); ko.Zone != state.ZHand {
		t.Fatalf("precondition: card zone after the move = %v, want hand", ko.Zone)
	}

	// Lifecycle B: re-establish (the card returns to the graveyard and is
	// encoded again), then the ENCODER leaves the battlefield and every
	// encoded link is dropped battlefield-stint state (CR 702.99 / 400.7);
	// the card itself stays in exile.
	Emit(g, l, Event{Kind: MoveZone, Obj: card, From: state.ZHand, To: state.ZGraveyard})
	for _, e := range cipherEncodeEvents(card, creature) {
		Emit(g, l, e)
	}
	assertCipherAssociation(t, g, creature, card, []state.ObjID{card})
	Emit(g, l, Event{Kind: MoveZone, Obj: creature, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := g.Obj(creature).EncodedCards; len(got) != 0 {
		t.Fatalf("encoder left the battlefield but kept encoded cards: %v", got)
	}
	if ko := g.Obj(card); ko.Zone != state.ZExile {
		t.Fatalf("encoder departure moved the encoded card: zone = %v, want exile", ko.Zone)
	}

	// Full-sequence replay: a fresh initial state plus the whole recorded log
	// rebuilds the final shape (and the same chain head, so the replayed fold
	// consumed byte-identical events).
	rp2, rl2, rCreature2, rCard2 := cipherEncodeFixture(t)
	for _, e := range l.Events {
		Emit(rp2, rl2, e)
	}
	if got := rp2.Obj(rCreature2).EncodedCards; len(got) != 0 {
		t.Fatalf("replayed encoder kept encoded cards after its departure: %v", got)
	}
	if ko := rp2.Obj(rCard2); ko == nil {
		t.Fatal("replayed encoded card object vanished")
	} else if ko.Zone != state.ZExile {
		t.Fatalf("replayed encoded card zone = %v, want exile", ko.Zone)
	}
	if rp2.Obj(rCreature2).Zone != state.ZGraveyard {
		t.Fatalf("replayed encoder zone = %v, want graveyard", rp2.Obj(rCreature2).Zone)
	}
	if got, want := rl2.Head(), l.Head(); got != want {
		t.Fatalf("replay chain head %q != live %q", got, want)
	}
}
