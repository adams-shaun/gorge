// manifest_turnup_test.go — CR 701.40b/701.58b/708.6: a manifested or cloaked
// creature card may be turned face up for its printed mana cost as a special
// action (CR 116.2b). The offer rides the same priority walk and the same
// turnUpPay/TurnFaceUp path as the Morph-family action, so TurnFaceUp
// triggers and the face-down characteristic set behave identically. A
// manifested or cloaked card with Morph/Disguise also offers that keyword's
// procedure (CR 701.40c-d, 701.58c-d).
//
// The decks are compiled corpus cards only; the direct-event setups use the
// exact logged MoveZone the manifest/cloak primitives emit, the precedent
// cloak_test.go's cloakBear/cloakTop already set.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// manifestCard moves the named real corpus card from seat p's hand or library
// onto p's battlefield with the exact logged MoveZone the manifest primitive
// emits (Counter events.ManifestEntryCounter, Secret), returning its id.
func manifestCard(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, p) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, Player: p,
					From: z, To: state.ZBattlefield,
					Counter: events.ManifestEntryCounter, Secret: true})
				return id
			}
		}
	}
	t.Fatalf("seat %d has no %s in hand/library to manifest", p, name)
	return 0
}

// manifestTurnUpOptions returns the turn_face_up options the pending seat-0
// priority decision offers for id.
func manifestTurnUpOptions(t *testing.T, e *Engine, id state.ObjID) []decision.Option {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("no seat-0 priority decision pending: %+v", d)
	}
	var out []decision.Option
	for _, opt := range d.Options {
		if opt.Kind == "turn_face_up" && opt.Obj == id {
			out = append(out, opt)
		}
	}
	return out
}

// submitManifestTurnUp submits the turn_face_up option for id carrying mode,
// failing when no such offer exists.
func submitManifestTurnUp(t *testing.T, e *Engine, id state.ObjID, mode string) {
	t.Helper()
	for _, opt := range manifestTurnUpOptions(t, e, id) {
		if opt.Mode == mode {
			submitChoices(t, e, opt.Index)
			return
		}
	}
	t.Fatalf("no turn_face_up offer for %d with mode %q: %+v", id, mode, manifestTurnUpOptions(t, e, id))
}

// manifestDreadEngine deals a seatZeroStart game whose seat-0 opening hand is
// Unnatural Summons plus Forests and whose library top two are Grizzly Bears,
// so the real ManifestDread spell can manifest a known creature card.
func manifestDreadEngine(t *testing.T, reg *cards.Registry) (*Engine, Config) {
	t.Helper()
	forest := searchCorpusCard(t, reg, "Forest")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	summons := searchCorpusCard(t, reg, "Unnatural Summons")
	deck := []*cards.Card{summons}
	for len(deck) < 7 {
		deck = append(deck, forest)
	}
	deck = append(deck, bear, bear)
	for len(deck) < 40 {
		deck = append(deck, forest)
	}
	opp := make([]*cards.Card, 0, 40)
	for i := 0; i < 10; i++ {
		opp = append(opp, bear)
	}
	for len(opp) < 40 {
		opp = append(opp, forest)
	}
	cfg := seatZeroStart(Config{Seed: 7702, Names: []string{"dread", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// shapeManifestDreadLibrary makes the next ManifestDread window deterministic
// after genesis's shuffle: the spell is already in hand (the caller moved it),
// every non-bear library card is exiled, and the library is trimmed to two
// Grizzly Bears (moving hand bears in if the draw took them). Every step is a
// logged MoveZone, so replay rebuilds the same top two.
func shapeManifestDreadLibrary(t *testing.T, e *Engine) {
	t.Helper()
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || o.Face().Name != "Grizzly Bears" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZExile})
		}
	}
	bears := 0
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
			bears++
		}
	}
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...) {
		if bears >= 2 {
			break
		}
		o := e.G.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
			bears++
		}
	}
	if bears < 2 {
		t.Fatalf("test precondition: only %d Grizzly Bears available to shape the library", bears)
	}
	dreadTrimLibrary(t, e, 2)
	e.pending = nil
	e.priorityRound()
}

// TestManifestedCreatureTurnsFaceUpForManaCost is the primary pin: a
// manifested Grizzly Bears is offered turn_face_up for {1}{G}, paying it turns
// the card face up with its printed characteristics and mints no stack object.
func TestManifestedCreatureTurnsFaceUpForManaCost(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Grizzly Bears")
	id := manifestCard(t, e, 0, "Grizzly Bears")
	addMana(t, e, 0, "CG")

	opts := manifestTurnUpOptions(t, e, id)
	if len(opts) != 1 || opts[0].Mode != "manifest" {
		t.Fatalf("manifested bear offers = %+v, want one mode manifest", opts)
	}
	submitManifestTurnUp(t, e, id, "manifest")

	o := e.G.Obj(id)
	if o.FaceDown || o.Manifested {
		t.Fatalf("after turn-up FaceDown=%v Manifested=%v, want both false", o.FaceDown, o.Manifested)
	}
	der := e.Derived(id)
	if der.Power != 2 || der.Toughness != 2 || len(der.Types) != 2 ||
		der.Types[0] != "Creature" || der.Types[1] != "Bear" {
		t.Fatalf("turned-up bear derived=%+v, want printed 2/2 Creature Bear", der)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.PutOnStack && ev.Obj == id {
			t.Fatalf("turn-face-up used the stack: %+v", ev)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %d after the special action, want 0", len(e.G.Stack))
	}
	replayCheck(t, e, cfg)
}

// TestManifestedNoncreatureNeverTurnsUp pins CR 701.40b/701.40g: a manifested
// instant and a manifested land are not offered the mana-cost procedure.
func TestManifestedNoncreatureNeverTurnsUp(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Lightning Bolt")
	bolt := manifestCard(t, e, 0, "Lightning Bolt")
	forest := manifestCard(t, e, 0, "Forest")
	addMana(t, e, 0, "CCCC")

	for _, id := range []state.ObjID{bolt, forest} {
		if opts := manifestTurnUpOptions(t, e, id); len(opts) != 0 {
			t.Fatalf("non-creature %s offers turn_face_up: %+v", e.G.Obj(id).Face().Name, opts)
		}
	}
	replayCheck(t, e, cfg)
}

// TestCloakedCreatureTurnsFaceUp pins CR 701.58b through the cloak marker: the
// cloaked bear turns up for its mana cost, and the cloak ward is gone once it
// is face up.
func TestCloakedCreatureTurnsFaceUp(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Grizzly Bears")
	id := cloakBear(t, e, 0)
	addMana(t, e, 0, "CG")

	opts := manifestTurnUpOptions(t, e, id)
	if len(opts) != 1 || opts[0].Mode != "cloak" {
		t.Fatalf("cloaked bear offers = %+v, want one mode cloak", opts)
	}
	submitManifestTurnUp(t, e, id, "cloak")

	o := e.G.Obj(id)
	if o.FaceDown || o.Cloaked {
		t.Fatalf("after turn-up FaceDown=%v Cloaked=%v, want both false", o.FaceDown, o.Cloaked)
	}
	for _, k := range e.Derived(id).Keywords {
		if k == "Ward:2" {
			t.Fatalf("turned-up cloaked bear still has Ward:2: %v", e.Derived(id).Keywords)
		}
	}
	replayCheck(t, e, cfg)
}

// TestManifestDreadCreatureTurnsUp drives the real Unnatural Summons spell:
// its ManifestDread ask manifests a known Grizzly Bears, which is then turned
// face up for {1}{G}.
func TestManifestDreadCreatureTurnsUp(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestDreadEngine(t, reg)

	id := searchMoveByName(t, e, "Unnatural Summons", state.ZHand)
	shapeManifestDreadLibrary(t, e)
	addMana(t, e, 0, "CUG")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority before casting Unnatural Summons: %+v", d)
	}
	cast := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == id {
			cast = opt.Index
		}
	}
	if cast < 0 {
		t.Fatalf("no cast option for Unnatural Summons: %+v", d.Options)
	}
	submitChoices(t, e, cast)

	d = passUntilAskKind(t, e, decision.KChoose, 20)
	if d.ResumeKind != "manifest_dread" {
		t.Fatalf("after casting Unnatural Summons: %+v, want the manifest-dread ask", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	passUntilStackEmpty(t, e, 20)

	var manifested state.ObjID
	for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(oid)
		if o != nil && o.FaceDown && o.Manifested && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
			manifested = oid
		}
	}
	if manifested == 0 {
		t.Fatal("ManifestDread did not leave a manifested Grizzly Bears on the battlefield")
	}
	addMana(t, e, 0, "CG")
	submitManifestTurnUp(t, e, manifested, "manifest")
	if o := e.G.Obj(manifested); o.FaceDown || o.Manifested {
		t.Fatalf("ManifestDread bear after turn-up FaceDown=%v Manifested=%v, want both false", o.FaceDown, o.Manifested)
	}
	replayCheck(t, e, cfg)
}

// TestManifestedMorphOffersBothCosts pins CR 701.40c: a manifested Nantuko
// Vigilante (Morph {1}{G}, printed {3}{G}) offers both procedures, and the
// morph one is payable as the keyword cost.
func TestManifestedMorphOffersBothCosts(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Nantuko Vigilante")
	id := manifestCard(t, e, 0, "Nantuko Vigilante")
	addMana(t, e, 0, "CCCCG")

	modes := map[string]bool{}
	for _, opt := range manifestTurnUpOptions(t, e, id) {
		modes[opt.Mode] = true
	}
	if !modes["manifest"] || !modes["morph"] {
		t.Fatalf("manifested Morph offers modes %v, want manifest and morph", modes)
	}
	submitManifestTurnUp(t, e, id, "morph")
	if o := e.G.Obj(id); o.FaceDown || o.Manifested {
		t.Fatalf("after the morph turn-up FaceDown=%v Manifested=%v, want both false", o.FaceDown, o.Manifested)
	}
	replayCheck(t, e, cfg)
}

// TestManifestTurnUpFiresTurnFaceUpTrigger pins CR 708.6's trigger: manifesting
// Belltoll Dragon and turning it face up fires its TurnFaceUp ability, which
// puts a +1/+1 counter on each other Dragon its controller controls.
func TestManifestTurnUpFiresTurnFaceUpTrigger(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Belltoll Dragon", "Shivan Dragon")
	dragon := searchMoveByName(t, e, "Shivan Dragon", state.ZHand)
	if dragon == 0 {
		t.Fatal("test precondition: Shivan Dragon must be in seat 0's hand")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: dragon, From: state.ZHand, To: state.ZBattlefield})
	id := manifestCard(t, e, 0, "Belltoll Dragon")
	addMana(t, e, 0, "CCCCCU")

	submitManifestTurnUp(t, e, id, "manifest")
	passUntilStackEmpty(t, e, 20)

	if got := counterCount(e.G.Obj(dragon), "P1P1"); got != 1 {
		t.Fatalf("other Dragon +1/+1 counters = %d, want 1 (Belltoll's TurnFaceUp trigger)", got)
	}
	replayCheck(t, e, cfg)
}

// TestManifestedPayloadFaceDownEntryNeverTurnsUp pins the marker boundary the
// review broke: Secret is not manifest-only. A generic ChangeZone FaceDown$
// True entry from a hand or library is Secret too (Tezzeret, Cruel Machinist;
// Cybership's Dig) and may carry a payload marker, and a plain bare marker is
// the generic face-down encoding. Neither is a manifest, so neither grants the
// CR 701.40b turn-up even when the hidden card is a creature.
func TestManifestedPayloadFaceDownEntryNeverTurnsUp(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Grizzly Bears", "Grizzly Bears")
	var bears []state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
				bears = append(bears, id)
			}
		}
	}
	if len(bears) < 2 {
		t.Fatalf("test precondition: only %d Grizzly Bears available", len(bears))
	}
	// Tezzeret/Cybership shape: a payload-bearing generic face-down entry.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bears[0], Player: 0,
		From: state.ZHand, To: state.ZBattlefield,
		Counter: events.FaceDownEntryCounterFor("Artifact & Creature", 5, 5, true), Secret: true})
	// Plain generic face-down entry: the bare marker, also Secret from hand.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bears[1], Player: 0,
		From: state.ZHand, To: state.ZBattlefield,
		Counter: events.FaceDownEntryCounter, Secret: true})
	addMana(t, e, 0, "CCGG")
	for _, id := range bears[:2] {
		o := e.G.Obj(id)
		if o == nil || !o.FaceDown {
			t.Fatalf("precondition: %d is not face down: %+v", id, o)
		}
		if o.Manifested || o.Cloaked {
			t.Fatalf("generic face-down entry %d folded Manifested=%v Cloaked=%v, want both false", id, o.Manifested, o.Cloaked)
		}
		if opts := manifestTurnUpOptions(t, e, id); len(opts) != 0 {
			t.Fatalf("generic face-down entry %d offers turn_face_up: %+v", id, opts)
		}
	}
	replayCheck(t, e, cfg)
}
