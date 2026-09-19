package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// These tests pin task copyp1: the registered DB$ CopyPermanent primitive, on
// real corpus cards, end to end. Every one asserts there is no
// "unimplemented API CopyPermanent" note anywhere in the log, and that the
// whole game replays byte-identically from its log.
func noUnimplementedCopyPermanent(t *testing.T, e *Engine) {
	t.Helper()
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented API") {
			t.Fatalf("log carries an unimplemented-API note: %q", ev.Text)
		}
	}
}

// findTokenCopyOf returns the battlefield token whose face is card's and
// whose id is none of the skips (the original, a previously found copy), or
// fails.
func findTokenCopyOf(t *testing.T, e *Engine, card *cards.Card, skip ...state.ObjID) state.ObjID {
	t.Helper()
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if !o.IsToken || o.Card != card {
			continue
		}
		gone := false
		for _, s := range skip {
			if o.ID == s {
				gone = true
				break
			}
		}
		if gone {
			continue
		}
		return o.ID
	}
	t.Fatalf("no token copy of %q on the battlefield", card.Faces[0].Name)
	return 0
}

// exiledTo asserts the object left the battlefield through an exile move and
// now rests in the CR 704.5d token tombstone (a token that leaves the
// battlefield ceases to exist, so the zone after the exile move is ZCeased).
func exiledTo(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	if got := e.G.Obj(id).Zone; got == state.ZBattlefield || got == state.ZStack {
		t.Fatalf("copy %d is still on the battlefield, want exiled", id)
	}
	for _, ev := range e.L.Events {
		if (ev.Kind == events.MoveZone && ev.Obj == id && ev.To == state.ZExile) ||
			ev.Kind == events.MyriadCleanup {
			return
		}
	}
	t.Fatalf("copy %d never moved to exile", id)
}

// TestFlamerushRiderCopiesTheOtherAttackerExiledAtEndOfCombat is the
// ticket's headline carrier end to end: attack with Flamerush Rider plus one
// other creature, answer the trigger's target ask with the other attacker,
// and the copy exists -- tapped, attacking the same defender, printed
// characteristics of the copied bear -- and is exiled when the combat ends.
func TestFlamerushRiderCopiesTheOtherAttackerExiledAtEndOfCombat(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	rider, ok := reg.Lookup("Flamerush Rider")
	if !ok {
		t.Fatal("missing corpus card Flamerush Rider")
	}
	bear, ok := reg.Lookup("Runeclaw Bear")
	if !ok {
		t.Fatal("missing corpus card Runeclaw Bear")
	}
	deck := append(mountainDeck(t, 40), rider, bear)
	e := New(seatZeroStart(Config{Seed: 42, Names: []string{"attacker", "bystander", "defender"},
		Decks: [][]*cards.Card{deck, deck, deck}}))
	e.Advance()
	id := crAbortMove(t, e, 0, "Flamerush Rider", state.ZBattlefield)
	mine := crAbortMove(t, e, 0, "Runeclaw Bear", state.ZBattlefield)

	e.pending = nil
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("want attackers decision, got %+v", d)
	}
	var picks []int
	for _, o := range d.Options {
		if o.Player == 2 && (o.Obj == id || o.Obj == mine) {
			picks = append(picks, o.Index)
		}
	}
	if len(picks) != 2 {
		t.Fatalf("attack-against-seat-2 options for both attackers not offered: %+v", d.Options)
	}
	crAbortAnswer(t, e, "attack declaration", picks...)

	// Flamerush's trigger fires and its Execute sub (DB$ CopyPermanent |
	// ValidTgts$ Creature.attacking+Other) poses the target ask at
	// placement, exactly the Master-of-Diversion precedent.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("want target choice at placement, got %+v", d)
	}
	if len(d.Options) != 1 || d.Options[0].Obj != mine {
		t.Fatalf("target options = %+v, want only the other attacking bear", d.Options)
	}
	crAbortAnswer(t, e, "Flamerush copy target", 0)
	passUntilStackEmpty(t, e, 20)

	// The copy: a tapped, attacking battlefield token with the bear's
	// printed characteristics, controller 0, flagged for end-of-combat
	// exile -- and no unimplemented-API note anywhere.
	cid := findTokenCopyOf(t, e, bear, mine)
	o := e.G.Obj(cid)
	if !o.Tapped || !o.IsAttacking || o.Controller != 0 || o.Zone != state.ZBattlefield {
		t.Fatalf("Flamerush copy: tapped=%v attacking=%v controller=%d zone=%s",
			o.Tapped, o.IsAttacking, o.Controller, o.Zone)
	}
	if o.Attacking != 2 {
		t.Fatalf("Flamerush copy attacks %d, want seat 2", o.Attacking)
	}
	noUnimplementedCopyPermanent(t, e)

	// Leaving the end-of-combat step exiles the copy (the IsMyriad
	// MyriadCleanup semantics the AtEOT$ ExileCombat bit rides); the CR
	// 704.5d tombstone is the token's final resting state.
	e.pending = nil
	e.setStep(state.StepEndCombat)
	e.pending = nil
	e.setStep(state.StepEnd)
	exiledTo(t, e, cid)
	if got := e.G.Obj(mine).Zone; got != state.ZBattlefield {
		t.Fatalf("the copied bear itself moved: %s", got)
	}
	noUnimplementedCopyPermanent(t, e)
}

// TestMoltenEchoesCopiesEnteringCreatureExilesAtNextEndStep pins the
// TriggeredCardLKICopy source and the AtEOT$ Exile delayed registration: a
// nontoken Bear entering under a Molten Echoes whose chosen type is Bear
// creates a token copy of it, which the next end step's delayed trigger
// exiles. The PumpKeywords$ Haste rider is modification-family (out of
// scope): the copy mints WITHOUT haste and the one per-call note names it.
func TestMoltenEchoesCopiesEnteringCreatureExilesAtNextEndStep(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Molten Echoes")
	molten := searchMoveByName(t, e, "Molten Echoes", state.ZBattlefield)

	// The "as this enters, choose a creature type" ask is the known cast-time
	// etbAsk approximation: a directly seated Molten Echoes never poses it.
	// Record the choice the same way the answered ask's Choose event does.
	e.emit(events.Event{Kind: events.Choose, Obj: molten, Counter: "type", Text: "Bear"})

	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	e.putTriggersOnStack()
	passUntilStackEmpty(t, e, 20)

	bearCard := e.G.Obj(bear).Card
	cid := findTokenCopyOf(t, e, bearCard, bear)
	o := e.G.Obj(cid)
	if o.Controller != 0 || o.Zone != state.ZBattlefield || o.Tapped || o.IsAttacking {
		t.Fatalf("Molten Echoes copy: controller=%d zone=%s tapped=%v attacking=%v",
			o.Controller, o.Zone, o.Tapped, o.IsAttacking)
	}
	// The PumpKeywords$ Haste rider is modification-family (out of scope):
	// the copy mints with the bear's own printed keyword set -- no Haste
	// grant reached it -- and the skipped note named the family.
	if o.Face().HasKeyword("Haste") {
		t.Fatal("the skipped PumpKeywords$ Haste rider reached the copy anyway")
	}
	noted := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "PumpKeywords$") {
			noted = true
		}
	}
	if !noted {
		t.Fatal("the skipped PumpKeywords$ rider was not named by the per-call note")
	}
	noUnimplementedCopyPermanent(t, e)

	// AtEOT$ Exile: the delayed registration fires at the beginning of the
	// next end step and exiles the copy.
	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	passUntilStackEmpty(t, e, 20)
	exiledTo(t, e, cid)
	if got := e.G.Obj(bear).Zone; got != state.ZBattlefield {
		t.Fatalf("the copied bear itself moved: %s", got)
	}
	noUnimplementedCopyPermanent(t, e)
	replayCheck(t, e, cfg)
}

// TestGrowingRanksPopulatesTheCreatureTokenYouControl pins the Populate$
// arm: at the beginning of its controller's upkeep, Growing Ranks copies a
// creature token you control. With exactly one eligible token the
// deterministic candidate IS the answer, so no stand-in note is emitted.
func TestGrowingRanksPopulatesTheCreatureTokenYouControl(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Growing Ranks")
	searchMoveByName(t, e, "Growing Ranks", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)

	// Seed one creature token to populate from: a battlefield token copy of
	// the bear (the CardToken mint encore uses).
	want := e.G.NextID
	e.emit(events.Event{Kind: events.CardToken, Obj: bear, Player: 0})
	tok := e.G.Obj(want)
	if tok == nil || tok.Zone != state.ZBattlefield || !tok.IsToken {
		t.Fatalf("seeded token: %+v", tok)
	}

	// Growing Ranks' upkeep trigger: populate, no target ask, one copy of
	// the only eligible creature token. Seat 0's next upkeep is two global
	// turns away (seat 1's whole turn passes in between).
	driveToStep(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	passUntilStackEmpty(t, e, 20)
	bearCard := e.G.Obj(bear).Card
	cid := findTokenCopyOf(t, e, bearCard, bear, want)
	if cid == want {
		t.Fatal("populate copied the seed token onto itself instead of minting a new token")
	}
	if got := e.G.Obj(cid).Controller; got != 0 {
		t.Fatalf("populated token controller = %d, want 0", got)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "Populate$") {
			t.Fatalf("no stand-in note is owed with exactly one eligible token: %q", ev.Text)
		}
	}
	noUnimplementedCopyPermanent(t, e)
	replayCheck(t, e, cfg)
}
