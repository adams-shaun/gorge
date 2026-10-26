package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Effect-delivered Event$ Moved replacements whose body sends the card to the
// library (Kylox's Voltstrider, CR 614.1a) or makes it enter tapped
// (Lightstall Inquisitor, CR 614.12) register live instead of the loud Note.

// emrReplacementNotes reports whether the log carries the registration gate's
// loud "continuous replacement unimplemented" Note.
func emrReplacementNotes(e *Engine) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "continuous replacement unimplemented") {
			return true
		}
	}
	return false
}

// emrMovedRepls counts the live Effect-created Moved replacements.
func emrMovedRepls(e *Engine) int {
	n := 0
	for _, ce := range e.active() {
		if ce.ReplacementEvent == "Moved" {
			n++
		}
	}
	return n
}

// emrFace returns the face carrying the card's DBEffect SVar. A modal
// double-faced card keeps it on its alternate face (Kaldring).
func emrFace(t *testing.T, name string) *cards.Face {
	t.Helper()
	c := corpusAlternativeCard(t, name)
	for _, f := range c.Faces {
		if cards.ResolveSVar(f.SVars, "DBEffect") != nil {
			return f
		}
	}
	t.Fatalf("%s has no DBEffect SVar", name)
	return nil
}

// emrRegister resolves the card's DBEffect with remembered as the captured set
// -- the recipe in the ticket's Workspace facts.
func emrRegister(e *Engine, f *cards.Face, src state.ObjID, remembered ...state.ObjID) {
	var rem []state.Target
	for _, id := range remembered {
		rem = append(rem, state.Target{Obj: id})
	}
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0, SVars: f.SVars, Remembered: rem},
		cards.ResolveSVar(f.SVars, "DBEffect"))
	e.pending = nil
}

// emrEngine builds a corpus game (seat 0 deck led by names0, seat 1 by
// names1, Plains padding) at seat 0's first main phase, so every fixture move
// is a logged event and replayCheck can rebuild the game.
func emrEngine(t *testing.T, names0, names1 []string) (*Engine, Config) {
	t.Helper()
	reg := searchTestRegistry(t)
	plains := searchCorpusCard(t, reg, "Plains")
	mk := func(names []string) []*cards.Card {
		var d []*cards.Card
		for _, n := range names {
			d = append(d, searchCorpusCard(t, reg, n))
		}
		for len(d) < 40 {
			d = append(d, plains)
		}
		return d
	}
	cfg := seatZeroStart(Config{Seed: 7402, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mk(names0), mk(names1)}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// emrCastResolve funds and casts the spell, then drains the stack.
func emrCastResolve(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "R")
	lbCast(t, e, id, 0)
	lbResolve(t, e)
}

// emrKyloxEngine puts Kylox's Voltstrider on seat 0's battlefield with two
// Shocks in hand, registers its real DBEffect remembering the first Shock,
// and returns both Shocks.
func emrKyloxEngine(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	f := emrFace(t, "Kylox's Voltstrider")
	e, cfg := emrEngine(t, []string{"Kylox's Voltstrider", "Shock", "Shock"}, nil)
	src := searchMoveByName(t, e, "Kylox's Voltstrider", state.ZBattlefield)

	var shocks []state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range append([]state.ObjID(nil), e.G.Zone(z, 0)...) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil || o.Face().Name != "Shock" {
				continue
			}
			if z != state.ZHand {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZHand})
			}
			shocks = append(shocks, id)
		}
	}
	if len(shocks) < 2 {
		t.Fatalf("fixture: %d Shock in hand, want 2", len(shocks))
	}
	emrRegister(e, f, src, shocks[0])
	e.priorityRound()
	return e, cfg, shocks[0], shocks[1]
}

func TestEffectReplacementMoveToLibraryBottom(t *testing.T) {
	t.Parallel()
	e, cfg, a, _ := emrKyloxEngine(t)
	// Precondition: the gate registered exactly one live Moved replacement,
	// and the remembered spell starts in hand (where the cast reads it).
	if n := emrMovedRepls(e); n != 1 {
		t.Fatalf("live Moved registrations = %d, want 1", n)
	}
	if z := e.G.Obj(a).Zone; z != state.ZHand {
		t.Fatalf("precondition: remembered spell in %s, want hand", z)
	}
	emrCastResolve(t, e, a)
	lib := e.G.Zone(state.ZLibrary, 0)
	if z := e.G.Obj(a).Zone; z != state.ZLibrary || len(lib) == 0 || lib[len(lib)-1] != a {
		t.Fatalf("spell in %v (bottom of library %v), want the bottom of the library", z, len(lib) > 0 && lib[len(lib)-1] == a)
	}
	if emrReplacementNotes(e) {
		t.Fatal("loud 'continuous replacement unimplemented' Note survived")
	}
	replayCheck(t, e, cfg)
}

func TestEffectReplacementMoveToLibraryOtherSpellUnaffected(t *testing.T) {
	t.Parallel()
	e, cfg, _, b := emrKyloxEngine(t)
	if n := emrMovedRepls(e); n != 1 {
		t.Fatalf("precondition: live Moved registrations = %d, want 1", n)
	}
	if z := e.G.Obj(b).Zone; z != state.ZHand {
		t.Fatalf("precondition: unremembered spell in %s, want hand", z)
	}
	emrCastResolve(t, e, b)
	if z := e.G.Obj(b).Zone; z != state.ZGraveyard {
		t.Fatalf("unremembered spell in %v, want graveyard", z)
	}
	replayCheck(t, e, cfg)
}

// emrLightstall builds the board through the real ETB trigger: Inquisitor on
// seat 0's battlefield, seat 1 exiling one of its lands to the hidden ask, and
// the trigger's own DBEffect registering the Moved replacement. It returns the
// exiled land and another land still in seat 1's hand.
func emrLightstall(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	e, cfg := emrEngine(t, []string{"Lightstall Inquisitor"}, []string{"Forest", "Forest", "Forest"})
	searchMoveByName(t, e, "Lightstall Inquisitor", state.ZBattlefield)
	d := passToAsk(t, e, 40)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "hand_move" {
		t.Fatalf("Lightstall Inquisitor did not pose the hidden hand-exile ask: %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	passUntilStackEmpty(t, e, 40)
	ex := e.G.Zone(state.ZExile, 1)
	if len(ex) != 1 {
		t.Fatalf("opponent exile count = %d, want 1", len(ex))
	}
	land := ex[0]
	var other state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 1) {
		if id != land {
			other = id
			break
		}
	}
	if e.G.Obj(land).Zone != state.ZExile || other == 0 {
		t.Fatalf("fixture: land in %v, other %d", e.G.Obj(land).Zone, other)
	}
	return e, cfg, land, other
}

// emrSeat1Main drives to seat 1's first main phase with priority.
func emrSeat1Main(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	driveToStepAll(t, e, e.G.Turn+1, 1, state.StepMain1)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 1 {
		t.Fatalf("expected seat 1's priority, got %+v", d)
	}
	return d
}

func emrPlayLand(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	d := emrSeat1Main(t, e)
	for _, o := range d.Options {
		if o.Kind == "play_land" && o.Obj == id {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no play_land option for %d: %+v", id, d.Options)
}

func TestEffectReplacementEntersTappedLightstall(t *testing.T) {
	t.Parallel()
	e, cfg, land, _ := emrLightstall(t)
	// Precondition: the real DBEffect registered the Moved replacement, and
	// the remembered land waits in exile (the zone its may-play grant reads).
	if n := emrMovedRepls(e); n != 1 {
		t.Fatalf("live Moved registrations = %d, want 1", n)
	}
	if z := e.G.Obj(land).Zone; z != state.ZExile {
		t.Fatalf("precondition: remembered land in %s, want exile", z)
	}
	emrPlayLand(t, e, land)
	o := e.G.Obj(land)
	if o.Zone != state.ZBattlefield || !o.Tapped {
		t.Fatalf("exiled land: zone %v tapped %v, want tapped on the battlefield", o.Zone, o.Tapped)
	}
	if emrReplacementNotes(e) {
		t.Fatal("loud Note survived")
	}
	replayCheck(t, e, cfg)

	// A land played from hand (a separate game with the same registration) is
	// not played through Inquisitor's permission and enters untapped.
	e2, cfg2, land2, other := emrLightstall(t)
	if land2 == other {
		t.Fatalf("fixture: exiled and hand land are the same object %d", land2)
	}
	emrPlayLand(t, e2, other)
	if oo := e2.G.Obj(other); oo.Zone != state.ZBattlefield || oo.Tapped {
		t.Fatalf("hand land: zone %v tapped %v, want untapped on the battlefield", oo.Zone, oo.Tapped)
	}
	replayCheck(t, e2, cfg2)
}

func TestEffectReplacementEntersTappedNeedsThePlay(t *testing.T) {
	t.Parallel()
	e, cfg, land, _ := emrLightstall(t)
	if n := emrMovedRepls(e); n != 1 {
		t.Fatalf("precondition: live Moved registrations = %d, want 1", n)
	}
	if z := e.G.Obj(land).Zone; z != state.ZExile {
		t.Fatalf("precondition: remembered land in %s, want exile", z)
	}
	// A plain MoveZone is not a land play: etbLandPlay is disarmed, so the
	// LandAbility.MayPlaySource ValidCause cannot hold and the entry is not
	// updated to tapped.
	e.emit(events.Event{Kind: events.MoveZone, Obj: land, From: state.ZExile, To: state.ZBattlefield})
	o := e.G.Obj(land)
	if o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("land put onto the battlefield (not played): zone %v tapped %v, want untapped", o.Zone, o.Tapped)
	}
	replayCheck(t, e, cfg)
}

func TestEffectReplacementUnreadValidCauseStaysLoud(t *testing.T) {
	t.Parallel()
	c := corpusAlternativeCard(t, "Jorn, God of Winter")
	// Jorn/Kaldring's Effect is the activated ability's inline body, so build
	// its DBEffect head over the card's real ETBCreat/DBTap/DBExile SVars.
	kaldring := c.Faces[1]
	f := &cards.Face{SVars: map[string]string{
		"DBEffect": "DB$ Effect | ReplacementEffects$ ETBCreat",
	}}
	for _, name := range []string{"ETBCreat", "DBTap", "DBExile"} {
		f.SVars[name] = kaldring.SVars[name]
	}
	e := mayPlayBase(t)
	src := e.G.AddObject(c, 0)
	src.Zone = state.ZBattlefield
	emrRegister(e, f, src.ID)
	if !emrReplacementNotes(e) {
		t.Fatal("Jorn's ValidCause$ SpellAbility.MayPlaySource line lost its loud Note")
	}
	if n := emrMovedRepls(e); n != 0 {
		t.Fatalf("Moved registrations = %d, want 0", n)
	}
}

// Due Respect's ValidCard$ Permanent does not match an entering object, so
// registering it live would be silently wrong; it keeps the loud Note.
func TestEffectReplacementPermanentValidCardStaysLoud(t *testing.T) {
	t.Parallel()
	f := &cards.Face{SVars: map[string]string{
		"DBEffect":  "DB$ Effect | ReplacementEffects$ RepETB",
		"RepETB":    "Event$ Moved | ValidCard$ Permanent | Destination$ Battlefield | ReplaceWith$ ETBTapped | ReplacementResult$ Updated | Description$ Permanents enter tapped this turn.",
		"ETBTapped": "DB$ Tap | ETB$ True | Defined$ ReplacedCard",
	}}
	e := mayPlayBase(t)
	src := e.G.AddObject(corpusAlternativeCard(t, "Jorn, God of Winter"), 0)
	src.Zone = state.ZBattlefield
	emrRegister(e, f, src.ID)
	if !emrReplacementNotes(e) {
		t.Fatal("ValidCard$ Permanent enters-tapped line lost its loud Note")
	}
	if n := emrMovedRepls(e); n != 0 {
		t.Fatalf("Moved registrations = %d, want 0", n)
	}
}
