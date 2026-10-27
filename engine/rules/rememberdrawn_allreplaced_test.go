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
// Thought Reflection (when requested) is on the battlefield.
type rememberDrawnBoard struct {
	e          *Engine
	cfg        Config
	spell      state.ObjID
	shock      state.ObjID
	salve      state.ObjID
	growth     state.ObjID
	reflection state.ObjID
}

// rememberDrawnGame builds a two-seat game from seat 0's deck and stops at
// turn 1's first main, where every board below is set up.
func rememberDrawnGame(t *testing.T, seed uint64, seat0 []*cards.Card) (*Engine, Config) {
	t.Helper()
	deck := append([]*cards.Card(nil), seat0...)
	deck = append(deck, mountainDeck(t, 40-len(deck))...)
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// setupRememberDrawnBoard builds the board with logged moves only, so
// replayCheck can rebuild it. Every other seat-0 hand/library card is exiled
// to make the hand and library counts exact.
func setupRememberDrawnBoard(t *testing.T, seed uint64, spellSrc string, withReflection bool) *rememberDrawnBoard {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	spellCard := mustCorpusCard(t, reg, "Ancient Excavation")
	if spellSrc != "" {
		spellCard = card(t, spellSrc)
	}
	seat0 := []*cards.Card{spellCard,
		mustCorpusCard(t, reg, "Shock"),
		mustCorpusCard(t, reg, "Healing Salve"),
		mustCorpusCard(t, reg, "Giant Growth")}
	if withReflection {
		seat0 = append(seat0, mustCorpusCard(t, reg, "Thought Reflection"))
	}
	e, cfg := rememberDrawnGame(t, seed, seat0)
	find := func(name string) state.ObjID {
		for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(z, 0) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
					return id
				}
			}
		}
		t.Fatalf("precondition: %q not in seat 0's hand or library", name)
		return 0
	}
	b := &rememberDrawnBoard{e: e, cfg: cfg,
		shock: find("Shock"), salve: find("Healing Salve"), growth: find("Giant Growth")}
	b.spell = find(spellCard.Faces[0].Name)
	if withReflection {
		b.reflection = find("Thought Reflection")
	}
	keep := map[state.ObjID]bool{b.spell: true, b.shock: true, b.salve: true, b.growth: true, b.reflection: true}
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
	move(b.salve, state.ZLibrary)
	move(b.growth, state.ZLibrary) // appended after Salve: Salve is on top
	if withReflection {
		move(b.reflection, state.ZBattlefield)
	}
	// Precondition the scenario's counts: two cards in hand (the spell and
	// Shock, leaving one at resolution) and exactly the two-card library.
	if got := len(e.G.Zone(state.ZHand, 0)); got != 2 {
		t.Fatalf("precondition: hand = %d, want 2 (spell + Shock)", got)
	}
	if lib := e.G.Zone(state.ZLibrary, 0); len(lib) != 2 || lib[0] != b.salve || lib[1] != b.growth {
		t.Fatalf("precondition: library = %v, want [Healing Salve Giant Growth]", lib)
	}
	if withReflection {
		if o := e.G.Obj(b.reflection); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Thought Reflection not on the battlefield: %+v", o)
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
	b := setupRememberDrawnBoard(t, 9410, "", true)
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
	b := setupRememberDrawnBoard(t, 9411, "", false)
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
	b := setupRememberDrawnBoard(t, 9412, rememberDrawnTrueSpell, true)
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
