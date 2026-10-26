package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g141: a DB$ Play SA's ManaConversion$ rider ("mana of any type can be
// spent to cast that spell") scopes to the one spell the Play casts. Forge's
// PlayEffect copies the parameter onto the cast spell ability, so the
// conversion is per-cast provenance on pendingCast, parsed with the same
// grammar the board's stat:ManaConvert statics use.

// pmcMoveTo puts seat p's card named name into `to`, searching the zones a
// fixture seeds, and emits the move only when the card is not already there.
// (moveByName emits a no-op hand->hand move for an opening-hand card.)
func pmcMoveTo(t *testing.T, e *Engine, p state.PlayerID, name string, to state.Zone) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary, state.ZGraveyard} {
		for _, id := range e.G.Zone(z, p) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil || o.Face().Name != name {
				continue
			}
			if z != to {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: to})
			}
			return id
		}
	}
	t.Fatalf("card %q not found in seat %d's hand, library or graveyard", name, p)
	return 0
}

// pmcQuistisBoard is the brief's probe board: seat 0 holds Quistis Trepe,
// a Lightning Bolt is in seat 0's graveyard, turn 3 main 1, pool {U}{U}{U}{U}.
func pmcQuistisBoard(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	quistis := lookup(t, reg, "Quistis Trepe")
	bolt := lookup(t, reg, "Lightning Bolt")
	mountain := lookup(t, reg, "Mountain")
	// Two Bolts: the first is the played one, the second is the scoping
	// test's ordinary hand card.
	deck0 := []*cards.Card{quistis, bolt, bolt}
	for len(deck0) < 40 {
		deck0 = append(deck0, mountain)
	}
	deck1 := make([]*cards.Card, 40)
	for i := range deck1 {
		deck1[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck0, deck1}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	qID := pmcMoveTo(t, e, 0, "Quistis Trepe", state.ZHand)
	boltID := pmcMoveTo(t, e, 0, "Lightning Bolt", state.ZGraveyard)
	return e, cfg, qID, boltID
}

// pmcPlaySettled is the driver's stop: the played Bolt has reached exile, or
// the cast aborted (the shape a missing conversion rider produces, which the
// tests assert against with a clear message rather than driving on into the
// next turn's cleanup discard).
func pmcPlaySettled(e *Engine, boltID state.ObjID) bool {
	if hasNote(e, "cast aborted") {
		return true
	}
	o := e.G.Obj(boltID)
	return o != nil && o.Zone == state.ZExile && len(e.G.Stack) == 0
}

// pmcDrivePlayBolt casts Quistis, passes to resolve, targets boltID with the
// ETB trigger, answers the Play ask, and targets the opponent with the played
// Bolt. It stops once stop() holds at a priority decision, and reports
// whether the graveyard target ask and the Play ask were actually seen (so a
// dead Play path can never pass vacuously).
func pmcDrivePlayBolt(t *testing.T, e *Engine, boltID state.ObjID, stop func() bool) (sawGraveTarget, sawPlay bool) {
	t.Helper()
	for i := 0; i < 120; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while driving the Play (iteration %d)", i)
		}
		if d.Kind == decision.KPriority && stop() {
			return sawGraveTarget, sawPlay
		}
		switch {
		case d.Kind == decision.KPriority:
			submitPass(t, e)
		case d.Kind == decision.KTarget:
			if o := e.G.Obj(boltID); o != nil && o.Zone == state.ZGraveyard {
				// Quistis's ETB trigger: "cast target instant or sorcery
				// card from a graveyard".
				sawGraveTarget = true
				submitTarget(t, e, boltID)
			} else {
				// The played Bolt's own damage target.
				submitTargetTo(t, e, 1)
			}
		case d.Kind == decision.KModes && d.ResumeKind == "play":
			sawPlay = true
			idx := -1
			for _, o := range d.Options {
				if o.Obj == boltID {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("the Play ask offers no option for the Bolt: %+v", d.Options)
			}
			submitChoices(t, e, idx)
		default:
			t.Fatalf("unexpected decision %d: kind=%s resume=%q", i, d.Kind, d.ResumeKind)
		}
	}
	t.Fatalf("the Play did not settle within the decision budget")
	return false, false
}

// TestPlayManaConversionQuistisCastsRedWithBlue is the brief's probe: the
// graveyard Lightning Bolt is cast with the leftover {U} (mana of any type
// can be spent), deals 3, and is exiled on resolution by ReplaceGraveyard$.
func TestPlayManaConversionQuistisCastsRedWithBlue(t *testing.T) {
	t.Parallel()
	e, cfg, qID, boltID := pmcQuistisBoard(t)
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("opponent starts at %d life, want 20", life)
	}
	if o := e.G.Obj(boltID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("the fixture Bolt is in %v, want the graveyard", o)
	}

	addMana(t, e, 0, "UUUU")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after funding: %+v, want priority", d)
	}
	idx := castOptionIdx(d, "cast", qID)
	if idx < 0 {
		t.Fatalf("Quistis is not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)

	sawGraveTarget, sawPlay := pmcDrivePlayBolt(t, e, boltID, func() bool {
		return pmcPlaySettled(e, boltID)
	})
	if !sawGraveTarget {
		t.Fatal("the ETB trigger never posed its graveyard target ask")
	}
	if !sawPlay {
		t.Fatal("the Play ask was never posed")
	}

	if hasNote(e, "cast aborted: cost no longer payable") {
		zone := state.Zone(0)
		if o := e.G.Obj(boltID); o != nil {
			zone = o.Zone
		}
		t.Fatalf("the converted cast aborted: cost no longer payable (the Bolt rests in %v, want exile)", zone)
	}
	if life := e.G.Players[1].Life; life != 17 {
		t.Fatalf("opponent is at %d life after the played Bolt, want 17", life)
	}
	if pool := e.G.Players[0].Pool.Total(); pool != 0 {
		t.Fatalf("mana pool after the converted payment = %d, want 0", pool)
	}
	if o := e.G.Obj(boltID); o == nil || o.Zone != state.ZExile {
		t.Fatalf("the played Bolt rests in %v, want exile", o)
	}
	replayCheck(t, e, cfg)
}

// TestPlayManaConversionIsScopedToThatCast: the rider belongs to the one
// spell the Play casts. A later ordinary Lightning Bolt in hand with only {U}
// available must not be offered; adding {R} offers it, proving the negative
// is about the colour and not an absent card.
func TestPlayManaConversionIsScopedToThatCast(t *testing.T) {
	t.Parallel()
	e, cfg, qID, boltID := pmcQuistisBoard(t)

	addMana(t, e, 0, "UUUU")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after funding: %+v, want priority", d)
	}
	idx := castOptionIdx(d, "cast", qID)
	if idx < 0 {
		t.Fatalf("Quistis is not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	sawGraveTarget, sawPlay := pmcDrivePlayBolt(t, e, boltID, func() bool {
		return pmcPlaySettled(e, boltID)
	})
	if !sawGraveTarget {
		t.Fatal("the ETB trigger never posed its graveyard target ask")
	}
	if !sawPlay {
		t.Fatal("the Play ask was never posed")
	}
	if pool := e.G.Players[0].Pool.Total(); pool != 0 {
		t.Fatalf("mana pool after the played Bolt = %d, want 0", pool)
	}

	// The second Bolt is still available to the ordinary hand-cast path.
	bolt2 := pmcMoveTo(t, e, 0, "Lightning Bolt", state.ZHand)
	addMana(t, e, 0, "U")
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after funding only {U}: %+v, want priority", d)
	}
	if o := e.G.Obj(bolt2); o == nil || o.Zone != state.ZHand {
		t.Fatalf("the second Bolt is in %v, want hand", o)
	}
	if pool := e.G.Players[0].Pool.Total(); pool != 1 {
		t.Fatalf("the pool holds %d mana, want exactly 1 blue", pool)
	}
	if idx := castOptionIdx(d, "cast", bolt2); idx >= 0 {
		t.Fatalf("a hand Bolt was offered with only {U} available: the Play rider outlived its cast (%+v)", d.Options[idx])
	}
	// Positive control: with {R} the same card becomes a legal hand cast.
	addMana(t, e, 0, "R")
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after funding a red mana: %+v, want priority", d)
	}
	if idx := castOptionIdx(d, "cast", bolt2); idx < 0 {
		t.Fatalf("the hand Bolt is not offered even with {R} available: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}

// TestPlayWithoutManaConversionStillNeedsRed is the rider-alone control: a
// fixture Play SA identical to Quistis's except that it carries no
// ManaConversion$ must still abort a red spell paid with only {U}.
func TestPlayWithoutManaConversionStillNeedsRed(t *testing.T) {
	t.Parallel()
	const enabler = `Name:TestPlayRecruiter
ManaCost:2 U
Types:Creature Human Wizard
PT:2/2
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigPlay | TriggerDescription$ When CARDNAME enters, you may cast target instant or sorcery card from a graveyard.
SVar:TrigPlay:DB$ Play | TgtZone$ Graveyard | ValidTgts$ Instant,Sorcery | ValidSA$ Spell | TgtPrompt$ Choose target instant or sorcery card from a graveyard | Optional$ True | ReplaceGraveyard$ Exile | AILogic$ ReplaySpell
Oracle:x
`
	const bolt = `Name:TestBolt
ManaCost:R
Types:Instant
A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3
Oracle:x
`
	e, cfg, enablerID := newFixtureDeck(t, 7, enabler, bolt)
	boltID := pmcMoveTo(t, e, 0, "TestBolt", state.ZGraveyard)
	addMana(t, e, 0, "UUUU")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after funding: %+v, want priority", d)
	}
	idx := castOptionIdx(d, "cast", enablerID)
	if idx < 0 {
		t.Fatalf("the fixture enabler is not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)

	sawGraveTarget, sawPlay := pmcDrivePlayBolt(t, e, boltID, func() bool {
		return hasNote(e, "cast aborted: cost no longer payable") && len(e.G.Stack) == 0
	})
	if !sawGraveTarget {
		t.Fatal("the ETB trigger never posed its graveyard target ask")
	}
	if !sawPlay {
		t.Fatal("the Play ask was never posed")
	}
	if !hasNote(e, "cast aborted: cost no longer payable") {
		t.Fatal("the unconverted red cast did not abort with the cost-unpayable Note")
	}
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("opponent is at %d life, want 20 (the cast must abort)", life)
	}
	if o := e.G.Obj(boltID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("the aborted Bolt rests in %v, want the graveyard", o)
	}
	replayCheck(t, e, cfg)
}
