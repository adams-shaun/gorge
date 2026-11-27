package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The Effect-created draw replacement class ("The next time you would draw a
// card this turn, ... instead": Words of Worship and its siblings), pinned on
// the real corpus card. effEffect registers the SVar's Event$ Draw line live,
// the shared Draw matcher picks it up, and the body's ExileEffect sub-ability
// retires the one-shot registration.

func wordsWorshipBoard(t *testing.T, seed uint64) (*Engine, Config, state.ObjID) {
	t.Helper()
	w := corpusAlternativeCard(t, "Words of Worship")
	e, cfg := tokenReplGame(t, seed, w)
	id := moveSeededCard(t, e, 0, w, state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: Words of Worship not on seat 0's battlefield")
	}
	return e, cfg, id
}

func activateWords(e *Engine, id state.ObjID) {
	face := e.G.Obj(id).Face()
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0, SVars: face.SVars}, face.Abilities[0])
}

func TestWordsOfWorshipEffectCreatedDrawReplacement(t *testing.T) {
	t.Parallel()
	e, cfg, id := wordsWorshipBoard(t, 71)
	activateWords(e, id)
	if hasNote(e, "continuous replacement unimplemented (DrawReplace)") {
		t.Fatal("Words of Worship's Draw registration fell through to the unimplemented Note")
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	life := e.G.Players[0].Life
	e.drawCard(0)
	if got := e.G.Players[0].Life; got != life+5 {
		t.Fatalf("first draw: life %d -> %d, want +5", life, got)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore {
		t.Fatalf("first draw: hand %d -> %d, the draw must be replaced", handBefore, got)
	}
	// One-shot: the registration retires, so the next draw is a real draw.
	e.drawCard(0)
	if got := e.G.Players[0].Life; got != life+5 {
		t.Fatalf("second draw: life %d, want still %d (one-shot)", got, life+5)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+1 {
		t.Fatalf("second draw: hand %d -> %d, want a normal draw", handBefore, got)
	}
	replayCheck(t, e, cfg)
}

func TestWordsOfWorshipEffectIgnoresOpponentDraw(t *testing.T) {
	t.Parallel()
	e, cfg, id := wordsWorshipBoard(t, 73)
	activateWords(e, id)
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life
	h1 := len(e.G.Zone(state.ZHand, 1))
	e.drawCard(1)
	if e.G.Players[0].Life != life0 || e.G.Players[1].Life != life1 {
		t.Fatalf("opponent's draw changed life: %d/%d -> %d/%d", life0, life1, e.G.Players[0].Life, e.G.Players[1].Life)
	}
	if got := len(e.G.Zone(state.ZHand, 1)); got != h1+1 {
		t.Fatalf("opponent draw: hand %d -> %d, want a normal draw", h1, got)
	}
	replayCheck(t, e, cfg)
}

// drawReplGame is tokenReplGameSeats with the named corpus card on seat 0's
// battlefield (and an optional Bear on seat 1's), asserted in place.
func drawReplGame(t *testing.T, seed uint64, name string, oppPermanent bool) (*Engine, Config, state.ObjID) {
	t.Helper()
	w := corpusAlternativeCard(t, name)
	var seat1 []*cards.Card
	if oppPermanent {
		seat1 = []*cards.Card{card(t, "Name:Opp Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")}
	}
	e, cfg := tokenReplGameSeats(t, seed, []*cards.Card{w}, seat1)
	id := moveSeededCard(t, e, 0, w, state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: %s not on seat 0's battlefield", name)
	}
	if oppPermanent {
		moveSeededCard(t, e, 1, seat1[0], state.ZBattlefield)
	}
	return e, cfg, id
}

func TestWordsOfWildingEffectDrawReplacementMakesABear(t *testing.T) {
	t.Parallel()
	e, cfg, id := drawReplGame(t, 75, "Words of Wilding", false)
	activateWords(e, id)
	hand := len(e.G.Zone(state.ZHand, 0))
	lib := len(e.G.Zone(state.ZLibrary, 0))
	bears := countTokensNamedOnSeat(t, e, 0, "Bear Token")
	e.drawCard(0)
	if got := countTokensNamedOnSeat(t, e, 0, "Bear Token"); got != bears+1 {
		t.Fatalf("first draw made %d bears, want 1", got-bears)
	}
	if len(e.G.Zone(state.ZHand, 0)) != hand || len(e.G.Zone(state.ZLibrary, 0)) != lib {
		t.Fatal("first draw still drew a card")
	}
	e.drawCard(0)
	if got := countTokensNamedOnSeat(t, e, 0, "Bear Token"); got != bears+1 {
		t.Fatalf("second draw made another bear (%d), want one-shot", got-bears)
	}
	if len(e.G.Zone(state.ZHand, 0)) != hand+1 {
		t.Fatal("second draw did not draw normally")
	}
	replayCheck(t, e, cfg)
}

func TestWordsOfWarEffectDrawReplacementDamagesTarget(t *testing.T) {
	t.Parallel()
	e, cfg, id := drawReplGame(t, 77, "Words of War", false)
	face := e.G.Obj(id).Face()
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0, SVars: face.SVars,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}, TargetsOffered: true}, face.Abilities[0])
	hand := len(e.G.Zone(state.ZHand, 0))
	e.drawCard(0)
	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("target life %d, want 18", got)
	}
	if len(e.G.Zone(state.ZHand, 0)) != hand {
		t.Fatal("the replaced draw still drew a card")
	}
	e.drawCard(0)
	if e.G.Players[1].Life != 18 || len(e.G.Zone(state.ZHand, 0)) != hand+1 {
		t.Fatal("second draw is not a plain draw (one-shot)")
	}
	replayCheck(t, e, cfg)
}

func TestWordsOfWasteEffectDrawReplacementDiscardsOpponent(t *testing.T) {
	t.Parallel()
	e, cfg, id := drawReplGame(t, 79, "Words of Waste", false)
	activateWords(e, id)
	h0, h1 := len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZHand, 1))
	e.drawCard(0)
	if got := len(e.G.Zone(state.ZHand, 1)); got != h1-1 {
		t.Fatalf("opponent hand %d -> %d, want one discard", h1, got)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != h0 {
		t.Fatalf("own hand %d -> %d, the draw must be replaced", h0, got)
	}
	e.drawCard(0)
	if len(e.G.Zone(state.ZHand, 1)) != h1-1 || len(e.G.Zone(state.ZHand, 0)) != h0+1 {
		t.Fatal("second draw is not a plain draw (one-shot)")
	}
	replayCheck(t, e, cfg)
}

func TestWordsOfWindEffectDrawReplacementBouncesEachPlayersPermanent(t *testing.T) {
	t.Parallel()
	e, cfg, id := drawReplGame(t, 81, "Words of Wind", true)
	activateWords(e, id)
	lib := len(e.G.Zone(state.ZLibrary, 0))
	e.drawCard(0)
	if got := len(e.G.Zone(state.ZBattlefield, 0)) + len(e.G.Zone(state.ZBattlefield, 1)); got != 0 {
		t.Fatalf("%d permanents remain, want both players' single permanent bounced", got)
	}
	if len(e.G.Zone(state.ZLibrary, 0)) != lib {
		t.Fatal("the replaced draw still drew a card")
	}
	if o := e.G.Obj(id); o.Zone != state.ZHand {
		t.Fatalf("Words of Wind in zone %v, want its owner's hand", o.Zone)
	}
	e.drawCard(0)
	if len(e.G.Zone(state.ZLibrary, 0)) != lib-1 {
		t.Fatal("second draw is not a plain draw (one-shot)")
	}
	replayCheck(t, e, cfg)
}

func TestAladdinsLampEffectDrawReplacementDigsX(t *testing.T) {
	t.Parallel()
	e, cfg, id := drawReplGame(t, 83, "Aladdin's Lamp", false)
	face := e.G.Obj(id).Face()
	c := effects.NewCtxPtr(id, 0, effects.CtxInit{SVars: face.SVars, X: 3})
	lib := e.G.Zone(state.ZLibrary, 0)
	top3 := append([]state.ObjID(nil), lib[:3]...)
	n := len(lib)
	effects.Resolve(e, c, face.Abilities[0])
	if hasNote(e, "continuous replacement unimplemented (DrawReplace)") {
		t.Fatal("Lamp's Draw registration fell through to the unimplemented Note")
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	e.drawCard(0)
	hand := e.G.Zone(state.ZHand, 0)
	if len(hand) != handBefore+1 || len(e.G.Zone(state.ZLibrary, 0)) != n-1 {
		t.Fatalf("hand %d -> %d, library %d -> %d: want exactly one card drawn", handBefore, len(hand), n, len(e.G.Zone(state.ZLibrary, 0)))
	}
	drawn := hand[len(hand)-1]
	idx := -1
	for i, o := range top3 {
		if o == drawn {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("drew %d, want one of the original top three %v", drawn, top3)
	}
	after := e.G.Zone(state.ZLibrary, 0)
	bottom := map[state.ObjID]bool{after[len(after)-1]: true, after[len(after)-2]: true}
	for i, o := range top3 {
		if i != idx && !bottom[o] {
			t.Fatalf("looked-at card %d is not on the library's bottom two %v", o, after[len(after)-2:])
		}
	}
	e.drawCard(0)
	if len(e.G.Zone(state.ZLibrary, 0)) != n-2 {
		t.Fatal("second draw is not a plain draw (one-shot)")
	}
	replayCheck(t, e, cfg)
}

func TestRingOfMarufEffectDrawReplacementTakesSideboardCard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	ring := corpusAlternativeCard(t, "Ring of Ma'rûf")
	bears := mustCorpusCard(t, reg, "Grizzly Bears")
	cfg := seatZeroStart(Config{Seed: 42, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks:      [][]*cards.Card{append([]*cards.Card{ring}, mountainDeck(t, 39)...), mountainDeck(t, 40)},
		Sideboards: [][]*cards.Card{{bears}, nil}})
	e := New(cfg)
	e.Advance()
	id := moveSeededCard(t, e, 0, ring, state.ZBattlefield)
	if got := len(e.G.Zone(state.ZSideboard, 0)); got != 1 {
		t.Fatalf("precondition: sideboard has %d cards, want 1", got)
	}
	activateWords(e, id)
	lib := len(e.G.Zone(state.ZLibrary, 0))
	e.drawCard(0)
	if len(e.G.Zone(state.ZSideboard, 0)) != 0 {
		t.Fatal("the sideboard card did not leave the sideboard")
	}
	if len(e.G.Zone(state.ZLibrary, 0)) != lib {
		t.Fatal("the replaced draw still drew a library card")
	}
	found := false
	for _, o := range e.G.Zone(state.ZHand, 0) {
		found = found || e.G.Obj(o).Face().Name == "Grizzly Bears"
	}
	if !found {
		t.Fatal("Grizzly Bears is not in hand")
	}
	replayCheck(t, e, cfg)
}

// Mangara's Tome (its Effect carries Triggers$ and a RememberedFirst pile) and
// Urabrask (an open-ended IsRemembered draw hook) stay on the loud Note, never
// a live registration that swallows a draw with no card moved.
func TestUnmodelledDrawEffectCarriersKeepTheirNote(t *testing.T) {
	t.Parallel()
	e, _, id := drawReplGame(t, 85, "Mangara's Tome", false)
	activateWords(e, id)
	if !hasNote(e, "continuous replacement unimplemented (DrawReplace)") {
		t.Fatal("Mangara's Tome did not keep its unimplemented Note")
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	e.drawCard(0)
	if len(e.G.Zone(state.ZHand, 0)) != hand+1 {
		t.Fatal("Mangara's Tome's draw was swallowed")
	}
}
