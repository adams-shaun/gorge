package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// exiledWithHovership is Unidentified Hovership's post-ff076d4432 script: the
// leave trigger's ManifestDread no longer reads the remembered set the ETB
// exiled with (RememberChanged$ True + DefinedPlayer$ RememberedOwner +
// Cleanup ClearRemembered$), it names the owner through Forge's
// HasCardsInExile player property instead. The test fixtures are inline; the
// corpus .cards still carries the old form at the pinned FORGE_REF.
const exiledWithHovership = `Name:Unidentified Hovership
ManaCost:1 W W
Types:Artifact Vehicle
PT:2/2
K:Flying
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigChangeZone | TriggerDescription$ enters: exile up to one creature of toughness 5 or less.
SVar:TrigChangeZone:DB$ ChangeZone | TargetMin$ 0 | TargetMax$ 1 | ValidTgts$ Creature.toughnessLE5 | TgtPrompt$ Pick a creature of toughness 5 or less. | Origin$ Battlefield | Destination$ Exile
T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ Card.Self | Execute$ TrigDreadOwner | TriggerDescription$ leaves: the exiled card's owner manifests dread.
SVar:TrigDreadOwner:DB$ ManifestDread | DefinedPlayer$ Player.HasCardsInExile_Card.ExiledWithSource_GE1
K:Crew:1
Oracle:Flying\nWhen Unidentified Hovership enters, exile up to one target creature with toughness 5 or less.\nWhen Unidentified Hovership leaves the battlefield, the exiled card's owner manifests dread.\nCrew 1
`

// exiledWithEngine deals seat 0 the new-form Hovership and Shatter (plus
// basics); stageBear puts a Grizzly Bears on seat 1's battlefield for the ETB
// to exile. Without it there is no creature anywhere and the ETB exiles
// nothing.
func exiledWithEngine(t *testing.T, reg *cards.Registry, stageBear bool) (*Engine, Config, state.ObjID) {
	t.Helper()
	c, diags := cards.ParseBytes("exiled_with_hovership.txt", []byte(exiledWithHovership))
	if len(diags) != 0 {
		t.Fatalf("parse inline Hovership: %v", diags)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatalf("link inline Hovership: %v", d)
	}
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	plains := searchCorpusCard(t, reg, "Plains")
	mountain := searchCorpusCard(t, reg, "Mountain")
	deck := []*cards.Card{c, searchCorpusCard(t, reg, "Shatter")}
	for len(deck) < 40 {
		deck = append(deck, plains, mountain)
	}
	opp := []*cards.Card{searchCorpusCard(t, reg, "Grizzly Bears")}
	for len(opp) < 40 {
		opp = append(opp, mountain)
	}
	cfg := seatZeroStart(Config{Seed: 9212, Names: []string{"hovership", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	bearID := state.ObjID(0)
	if stageBear {
		for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(z, 1) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" && bearID == 0 {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZBattlefield})
					e.pending = nil
					bearID = id
				}
			}
		}
		if bearID == 0 {
			t.Fatal("no Grizzly Bears in seat 1's hand/library to stage")
		}
		e.priorityRound()
	}
	return e, cfg, bearID
}

// exiledWithEnterAndShatter casts the new-form Hovership (answering its ETB
// with the bear when one is staged), resolves the ETB, destroys the Hovership
// with Shatter and drives until the leave trigger has resolved or asks
// something. It asserts the exile-provenance precondition (the exiled bear
// carries the ExiledWith link to the Hovership) so "nobody manifested" is
// meaningful.
func exiledWithEnterAndShatter(t *testing.T, e *Engine, bearID state.ObjID) (state.ObjID, *decision.Decision) {
	t.Helper()
	hov := searchMoveByName(t, e, "Unidentified Hovership", state.ZHand)
	addMana(t, e, 0, "WWW")
	castByID(t, e, hov)
	d := passUntilResolved(t, e, 20)
	if d != nil && d.Kind == decision.KTarget {
		if bearID == 0 {
			t.Fatalf("ETB target ask with no creature on the battlefield: %+v", d)
		}
		idx := -1
		for _, o := range d.Options {
			if o.Obj == bearID {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("staged bear %d not offered to the ETB exile: %+v", bearID, d.Options)
		}
		submitChoices(t, e, idx)
	} else if bearID != 0 {
		t.Fatalf("after Hovership entered: %+v, want the ETB exile target ask", d)
	}
	if d = passUntilResolved(t, e, 20); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("while the ETB resolved: %+v, want priority", d)
	}
	hovObj := e.G.Obj(hov)
	if hovObj == nil || hovObj.Zone != state.ZBattlefield {
		t.Fatalf("Hovership = %+v, want battlefield", hovObj)
	}
	if bearID != 0 {
		bear := e.G.Obj(bearID)
		if bear == nil || bear.Zone != state.ZExile {
			t.Fatalf("exiled bear = %+v, want exile", bear)
		}
		if bear.ExiledWith != hov {
			t.Fatalf("precondition: exiled bear's ExiledWith = %d, want the Hovership %d", bear.ExiledWith, hov)
		}
	}

	shatter := searchMoveByName(t, e, "Shatter", state.ZHand)
	addMana(t, e, 0, "RRR")
	castByID(t, e, shatter)
	if d = e.Pending(); d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after casting Shatter: %+v, want the target ask", d)
	}
	tIdx := -1
	for _, o := range d.Options {
		if o.Obj == hov {
			tIdx = o.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("Hovership %d not offered to Shatter: %+v", hov, d.Options)
	}
	submitChoices(t, e, tIdx)
	d = passUntilResolved(t, e, 20)
	if o := e.G.Obj(hov); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("destroyed Hovership = %+v, want graveyard", o)
	}
	if d != nil && d.Kind == decision.KPriority {
		return hov, nil
	}
	return hov, d
}

// dreadTriggerResolved reports whether a ManifestDread instance whose source
// is hov resolved off the stack -- the precondition that makes "nothing was
// manifested" meaningful.
func dreadTriggerResolved(e *Engine, hov state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind != events.Resolve {
			continue
		}
		if o := e.G.Obj(ev.Obj); o != nil && o.Source == hov && o.Ability != nil && o.Ability.API == "ManifestDread" {
			return true
		}
	}
	return false
}

// TestExiledWithOwnerManifestsDread is the positive half: the ETB exiled seat
// 1's bear, so `DefinedPlayer$ Player.HasCardsInExile_Card.ExiledWithSource_GE1`
// names seat 1 -- the only seat whose exile holds a card exiled with the
// Hovership -- and the leave trigger asks seat 1, not the Hovership's
// controller, to manifest dread.
func TestExiledWithOwnerManifestsDread(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg, bear := exiledWithEngine(t, reg, true)
	lib1 := len(e.G.Zone(state.ZLibrary, 1))
	hov, d := exiledWithEnterAndShatter(t, e, bear)
	if !dreadTriggerResolved(e, hov) {
		t.Fatal("precondition: the Hovership's leave trigger never resolved")
	}
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "manifest_dread" {
		t.Fatalf("after the leave trigger: %+v, want seat 1's manifest-dread choice", d)
	}
	if d.Player != 1 {
		t.Fatalf("manifest dread asked player %d, want 1 (the exiled bear's owner)", d.Player)
	}
	// Answering the ask manifests the top card of seat 1's library face down
	// and puts the other card into its owner's graveyard (CR 701.62a).
	submitChoices(t, e, 0)
	if d = passUntilResolved(t, e, 20); d != nil && d.Kind != decision.KPriority {
		t.Fatalf("after the manifest-dread ask: %+v, want priority", d)
	}
	var facedown []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 1) {
		if o := e.G.Obj(id); o != nil && o.FaceDown {
			facedown = append(facedown, id)
		}
	}
	if len(facedown) != 1 {
		t.Fatalf("seat 1 face-down permanents %v, want the one manifested card", facedown)
	}
	if o := e.G.Obj(facedown[0]); o == nil || o.Owner != 1 {
		t.Fatalf("manifested card = %+v, want owner 1", o)
	}
	if n := len(e.G.Zone(state.ZGraveyard, 1)); n != 1 {
		t.Fatalf("seat 1 graveyard %d cards, want the window's other card", n)
	}
	if n := len(e.G.Zone(state.ZLibrary, 1)); n != lib1-2 {
		t.Fatalf("seat 1 library %d cards, want %d (the window's two cards left it)", n, lib1-2)
	}
	replayCheck(t, e, cfg)
}

// TestExiledWithOwnerNothingExiledManifestsNothing pins the empty arm: the
// ETB exiled nothing, so no seat's exile holds a card exiled with the
// Hovership, `GE1` matches no seat and nobody manifests dread -- never the
// Hovership's controller.
func TestExiledWithOwnerNothingExiledManifestsNothing(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg, _ := exiledWithEngine(t, reg, false)
	searchMoveByName(t, e, "Unidentified Hovership", state.ZHand)
	searchMoveByName(t, e, "Shatter", state.ZHand)
	lib0 := len(e.G.Zone(state.ZLibrary, 0))
	lib1 := len(e.G.Zone(state.ZLibrary, 1))
	hov, d := exiledWithEnterAndShatter(t, e, 0)
	if d != nil {
		t.Fatalf("leave trigger asked %+v (player %d); nothing was exiled, so nobody manifests dread", d.Kind, d.Player)
	}
	if !dreadTriggerResolved(e, hov) {
		t.Fatal("precondition: the Hovership's leave trigger never resolved")
	}
	if n := len(e.G.Zone(state.ZLibrary, 0)); n != lib0 {
		t.Fatalf("seat 0 library %d -> %d: the Hovership's controller manifested dread", lib0, n)
	}
	if n := len(e.G.Zone(state.ZLibrary, 1)); n != lib1 {
		t.Fatalf("seat 1 library %d -> %d: nobody was exiled, so nobody manifests dread", lib1, n)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented API ManifestDread") {
			t.Fatalf("ManifestDread degraded to a note: %q", ev.Text)
		}
	}
	replayCheck(t, e, cfg)
}
