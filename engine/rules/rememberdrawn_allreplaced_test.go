package rules

// RememberDrawn$ AllReplaced (cr-g244): Forge's AllReplaced marker counts
// every card the draw machinery moves into the drawing player's own hand
// under the cause, including the cards a replacement instruction draws
// (CR 121.6c/614.1). RememberDrawn$ True keeps recording only the effect's
// own pre-draw top card. The reported scenario is XMage's
// DrawEffectsTest.testAncientExcavation: Thought Reflection turns Ancient
// Excavation's one-card draw into two, and both must count for the
// subsequent "discard a card for each card drawn this way".

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// rememberDrawnTrueSpell is the True-spelling control fixture: the same
// "draw X, then discard Remembered$Amount" shape Ancient Excavation has, so
// the only difference from the corpus card is the parameter spelling.
const rememberDrawnTrueSpell = "Name:RememberDrawn True Test\nManaCost:U\nTypes:Sorcery\n" +
	"A:SP$ Draw | NumCards$ X | RememberDrawn$ True | SubAbility$ DBDiscard | SpellDescription$ Draw a card for each card in your hand, then discard that many.\n" +
	"SVar:DBDiscard:DB$ Discard | Defined$ You | Mode$ TgtChoose | NumCards$ Y | SubAbility$ DBCleanup\n" +
	"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True\n" +
	"SVar:X:Count$ValidHand Card.YouOwn\n" +
	"SVar:Y:Remembered$Amount\n" +
	"Oracle:x\n"

// rememberDrawnBoard is the reported scenario's board: seat 0 holds the
// spell and Shock, its library is Healing Salve above Giant Growth, and
// Thought Reflection (when requested) is on the battlefield. The batch pin
// also puts Alms Collector on seat 1 and a spare hand card in seat 0's hand
// (the batch needs a two-card draw, so a third pre-cast hand card makes X=2).
type rememberDrawnBoard struct {
	e          *Engine
	cfg        Config
	spell      state.ObjID
	shock      state.ObjID
	salve      state.ObjID
	growth     state.ObjID
	reflection state.ObjID
	spare      state.ObjID
	alms       state.ObjID
}

// rememberDrawnSetup selects the extra pieces a board carries. The zero
// value is the reported two-card-hand scenario.
type rememberDrawnSetup struct {
	// spellSrc replaces the corpus Ancient Excavation with a fixture script
	// (the RememberDrawn$ True control).
	spellSrc string
	// reflection puts Thought Reflection on seat 0's battlefield.
	reflection bool
	// almsCollector puts Alms Collector on seat 1's battlefield and a spare
	// card in seat 0's hand, so the resolution's draw is a two-card batch
	// that Alms Collector's DrawCards replacement consumes.
	almsCollector bool
}

// rememberDrawnGame builds a two-seat game from seat 0's deck and stops at
// turn 1's first main, where every board below is set up.
func rememberDrawnGame(t *testing.T, seed uint64, seat0 []*cards.Card) (*Engine, Config) {
	t.Helper()
	return rememberDrawnGameDecks(t, seed, seat0, nil)
}

// rememberDrawnGameDecks is rememberDrawnGame with an explicit seat 1 deck;
// nil keeps the all-Mountain deck.
func rememberDrawnGameDecks(t *testing.T, seed uint64, seat0, seat1 []*cards.Card) (*Engine, Config) {
	t.Helper()
	deck := append([]*cards.Card(nil), seat0...)
	deck = append(deck, mountainDeck(t, 40-len(deck))...)
	if seat1 == nil {
		seat1 = mountainDeck(t, 40)
	} else {
		seat1 = append([]*cards.Card(nil), seat1...)
		seat1 = append(seat1, mountainDeck(t, 40-len(seat1))...)
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{deck, seat1},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// setupRememberDrawnBoard builds the board with logged moves only, so
// replayCheck can rebuild it. Every other seat-0 hand/library card is exiled
// to make the hand and library counts exact.
func setupRememberDrawnBoard(t *testing.T, seed uint64, s rememberDrawnSetup) *rememberDrawnBoard {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	spellCard := mustCorpusCard(t, reg, "Ancient Excavation")
	if s.spellSrc != "" {
		spellCard = card(t, s.spellSrc)
	}
	seat0 := []*cards.Card{spellCard,
		mustCorpusCard(t, reg, "Shock"),
		mustCorpusCard(t, reg, "Healing Salve"),
		mustCorpusCard(t, reg, "Giant Growth")}
	if s.reflection {
		seat0 = append(seat0, mustCorpusCard(t, reg, "Thought Reflection"))
	}
	var seat1 []*cards.Card
	if s.almsCollector {
		seat0 = append(seat0, mustCorpusCard(t, reg, "Grizzly Bears"))
		seat1 = append(seat1, mustCorpusCard(t, reg, "Alms Collector"))
	}
	e, cfg := rememberDrawnGameDecks(t, seed, seat0, seat1)
	find := func(p state.PlayerID, name string) state.ObjID {
		for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(z, p) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
					return id
				}
			}
		}
		t.Fatalf("precondition: %q not in seat %d's hand or library", name, p)
		return 0
	}
	b := &rememberDrawnBoard{e: e, cfg: cfg,
		shock: find(0, "Shock"), salve: find(0, "Healing Salve"), growth: find(0, "Giant Growth")}
	b.spell = find(0, spellCard.Faces[0].Name)
	if s.reflection {
		b.reflection = find(0, "Thought Reflection")
	}
	if s.almsCollector {
		b.spare = find(0, "Grizzly Bears")
		b.alms = find(1, "Alms Collector")
	}
	keep := map[state.ObjID]bool{b.spell: true, b.shock: true, b.salve: true, b.growth: true,
		b.reflection: true, b.spare: true}
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range append([]state.ObjID(nil), e.G.Zone(z, 0)...) {
			if !keep[id] {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZExile})
			}
		}
	}
	move := func(id state.ObjID, to state.Zone) {
		if o := e.G.Obj(id); o != nil && o.Zone != to {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: to})
		}
	}
	for _, id := range []state.ObjID{b.shock, b.salve, b.growth} {
		move(id, state.ZExile)
	}
	move(b.spell, state.ZHand)
	move(b.shock, state.ZHand)
	if s.almsCollector {
		move(b.spare, state.ZHand)
	}
	move(b.salve, state.ZLibrary)
	move(b.growth, state.ZLibrary) // appended after Salve: Salve is on top
	if s.reflection {
		move(b.reflection, state.ZBattlefield)
	}
	if s.almsCollector {
		move(b.alms, state.ZBattlefield)
	}
	// Precondition the scenario's counts: the spell and Shock leave one card
	// in hand at resolution (the batch pin adds the spare, leaving two), and
	// exactly the two-card library.
	wantHand := 2
	if s.almsCollector {
		wantHand = 3
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != wantHand {
		t.Fatalf("precondition: hand = %d, want %d", got, wantHand)
	}
	if lib := e.G.Zone(state.ZLibrary, 0); len(lib) != 2 || lib[0] != b.salve || lib[1] != b.growth {
		t.Fatalf("precondition: library = %v, want [Healing Salve Giant Growth]", lib)
	}
	if s.reflection {
		if o := e.G.Obj(b.reflection); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Thought Reflection not on the battlefield: %+v", o)
		}
	}
	if s.almsCollector {
		if o := e.G.Obj(b.alms); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Alms Collector not on seat 1's battlefield: %+v", o)
		}
		if len(e.G.Zone(state.ZLibrary, 1)) == 0 {
			t.Fatal("precondition: seat 1 needs a card to draw for Alms Collector")
		}
	}
	return b
}

// castSpell funds and casts the board's spell, leaving its resolution pending.
func (b *rememberDrawnBoard) castSpell(t *testing.T) {
	t.Helper()
	addMana(t, b.e, 0, "UUBB")
	d := b.e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("precondition: no priority decision to cast from: %+v", d)
	}
	cast := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == b.spell {
			cast = o.Index
		}
	}
	if cast < 0 {
		t.Fatalf("precondition: no cast option for the spell %d in %#v", b.spell, d.Options)
	}
	if err := b.e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{cast}}); err != nil {
		t.Fatal(err)
	}
}

// castAndResolve casts the spell and drives the resolution, answering the
// discard ask with pickObjs and requiring its count to be wantDiscards.
func (b *rememberDrawnBoard) castAndResolve(t *testing.T, wantDiscards int, pickObjs ...state.ObjID) {
	t.Helper()
	b.castSpell(t)
	sawDiscard := false
	for i := 0; i < 40; i++ {
		d := b.e.Pending()
		if d == nil {
			break
		}
		switch {
		case d.Kind == decision.KPriority:
			if o := b.e.G.Obj(b.spell); o == nil || o.Zone != state.ZStack {
				if !sawDiscard {
					t.Fatal("precondition: the discard ask was never posed after the draw resolved")
				}
				return
			}
			pass := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pass = o.Index
				}
			}
			if pass < 0 {
				t.Fatalf("no pass option in %#v", d.Options)
			}
			submitChoices(t, b.e, pass)
		case d.Kind == decision.KReplacement:
			if b.reflection == 0 {
				t.Fatalf("unexpected replacement choice: %+v", d)
			}
			pick := optionForObj(d, b.reflection)
			if pick < 0 {
				t.Fatalf("Thought Reflection missing from the draw competition: %+v", d.Options)
			}
			submitChoices(t, b.e, pick)
		case d.Kind == decision.KModes && d.ResumeKind == "discard":
			sawDiscard = true
			if d.Min != wantDiscards || d.Max != wantDiscards {
				t.Fatalf("discard ask bounds %d..%d, want exactly %d (Remembered$Amount)", d.Min, d.Max, wantDiscards)
			}
			if len(pickObjs) != wantDiscards {
				t.Fatalf("test wants %d discard picks, passed %d", wantDiscards, len(pickObjs))
			}
			var picks []int
			for _, want := range pickObjs {
				idx := optionForObj(d, want)
				if idx < 0 {
					t.Fatalf("discard ask does not offer card %d: %+v", want, d.Options)
				}
				picks = append(picks, idx)
			}
			submitChoices(t, b.e, picks...)
		default:
			t.Fatalf("unexpected decision %s %q: %+v", d.Kind, d.Prompt, d.Options)
		}
	}
	t.Fatal("resolution did not reach a stable priority")
}

// TestRememberDrawnAllReplacedCountsReplacementDraws is the reported defect:
// Thought Reflection replaces Ancient Excavation's one-card draw with two,
// and both cards must count as "drawn this way", so the discard asks for two.
func TestRememberDrawnAllReplacedCountsReplacementDraws(t *testing.T) {
	t.Parallel()
	b := setupRememberDrawnBoard(t, 9410, rememberDrawnSetup{reflection: true})
	b.castAndResolve(t, 2, b.shock, b.salve)
	for _, id := range []state.ObjID{b.shock, b.salve} {
		if !inZone(b.e, state.ZGraveyard, 0, id) {
			t.Fatalf("card %d is not in the graveyard; both cards drawn this way must be discarded", id)
		}
	}
	if !inZone(b.e, state.ZHand, 0, b.growth) {
		t.Fatalf("Giant Growth is not in hand after the discard")
	}
	if got := len(b.e.G.Zone(state.ZHand, 0)); got != 1 {
		t.Fatalf("hand size = %d, want 1 (only Giant Growth)", got)
	}
	replayCheck(t, b.e, b.cfg)
}

// TestRememberDrawnAllReplacedWithoutReplacement: with no Thought
// Reflection the same spell draws one card and asks for exactly one discard.
func TestRememberDrawnAllReplacedWithoutReplacement(t *testing.T) {
	t.Parallel()
	b := setupRememberDrawnBoard(t, 9411, rememberDrawnSetup{})
	b.castAndResolve(t, 1, b.shock)
	if !inZone(b.e, state.ZGraveyard, 0, b.shock) {
		t.Fatalf("Shock is not in the graveyard after the one-card discard")
	}
	if !inZone(b.e, state.ZHand, 0, b.salve) {
		t.Fatalf("Healing Salve is not in hand after being drawn")
	}
	if got := len(b.e.G.Zone(state.ZHand, 0)); got != 1 {
		t.Fatalf("hand size = %d, want 1 (Healing Salve)", got)
	}
	replayCheck(t, b.e, b.cfg)
}

// TestRememberDrawnTrueUnchanged: the True spelling under Thought
// Reflection still records only its own pre-draw top card, so the discard
// asks for one although two cards were drawn.
func TestRememberDrawnTrueUnchanged(t *testing.T) {
	t.Parallel()
	b := setupRememberDrawnBoard(t, 9412, rememberDrawnSetup{spellSrc: rememberDrawnTrueSpell, reflection: true})
	b.castAndResolve(t, 1, b.shock)
	if !inZone(b.e, state.ZGraveyard, 0, b.shock) {
		t.Fatalf("Shock is not in the graveyard after the one-card discard")
	}
	for _, id := range []state.ObjID{b.salve, b.growth} {
		if !inZone(b.e, state.ZHand, 0, id) {
			t.Fatalf("card %d is not in hand; True must have drawn two but remembered one", id)
		}
	}
	if got := len(b.e.G.Zone(state.ZHand, 0)); got != 2 {
		t.Fatalf("hand size = %d, want 2 (both replacement draws less the one discard)", got)
	}
	replayCheck(t, b.e, b.cfg)
}

// TestRememberDrawnAllReplacedCountsConsumedBatch pins the DrawBatchProposal
// arm: Alms Collector ("if an opponent would draw two or more cards, instead
// you and that player each draw a card") consumes Ancient Excavation's
// two-card batch as ONE event. Its body's draw for the drawing player is
// itself replaced by Thought Reflection ("draw two instead"), so the batch
// record must see BOTH cards that replacement moved into the drawing
// player's hand; the card Alms Collector's own controller drew is in the
// opponent's hand and must not be remembered. The discard asks for exactly
// two, so removing the batch record (draw.go's drawBatchConsumed) makes
// Remembered$Amount 0, which the discard's minimum of one exposes as an
// ask of one.
func TestRememberDrawnAllReplacedCountsConsumedBatch(t *testing.T) {
	t.Parallel()
	b := setupRememberDrawnBoard(t, 9413, rememberDrawnSetup{reflection: true, almsCollector: true})
	// Precondition: the cast leaves two cards in hand, so X (the hand count)
	// makes the resolution draw a two-card batch -- Alms Collector's Number$
	// GE2 window.
	if got := len(b.e.G.Zone(state.ZHand, 0)); got != 3 {
		t.Fatalf("precondition: pre-cast hand = %d, want 3 (spell + Shock + spare)", got)
	}
	hand1 := len(b.e.G.Zone(state.ZHand, 1))
	b.castAndResolve(t, 2, b.shock, b.salve)
	for _, id := range []state.ObjID{b.shock, b.salve} {
		if !inZone(b.e, state.ZGraveyard, 0, id) {
			t.Fatalf("card %d is not in the graveyard; both cards the batch's replacement drew must count", id)
		}
	}
	if !inZone(b.e, state.ZHand, 0, b.growth) {
		t.Fatalf("Giant Growth is not in hand: Thought Reflection drew both replacement cards")
	}
	if lib := b.e.G.Zone(state.ZLibrary, 0); len(lib) != 0 {
		t.Fatalf("seat 0 library = %d cards, want 0 (both drawn by the replacement body)", len(lib))
	}
	if got := len(b.e.G.Zone(state.ZHand, 0)); got != 2 {
		t.Fatalf("hand size = %d, want 2 (spare + Giant Growth)", got)
	}
	// The opponent's own draw is the opponent-hand control: Alms Collector
	// drew a card too, and it must not inflate Remembered$Amount (the ask
	// above was two, not three).
	if got := len(b.e.G.Zone(state.ZHand, 1)) - hand1; got != 1 {
		t.Fatalf("seat 1 hand delta = %d, want 1 (Alms Collector's controller draws)", got)
	}
	replayCheck(t, b.e, b.cfg)
}
