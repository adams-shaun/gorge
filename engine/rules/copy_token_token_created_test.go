package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCopyTokenFiresTokenCreated pins CR 111.1/707.2: a token created by a
// copy effect (DB$ CopyPermanent) is still a token being created, so Akim's
// "whenever you create one or more tokens for the first time each turn"
// trigger (Mode$ TokenCreatedOnce) fires for it. The copy mints through
// events.CopyToken and enters with a separate MoveZone, not a TokenCreate
// event, so a matcher requiring one would never create the Bird.
func TestCopyTokenFiresTokenCreated(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	akim := lookup(t, reg, "Akim, the Soaring Wind")
	spell := lookup(t, reg, "Electroduplicate")
	bear := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{akim, spell, bear}, nil)

	akimID := moveByName(t, e, 0, "Akim, the Soaring Wind", state.ZBattlefield)
	bearID := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	spellID := moveByName(t, e, 0, "Electroduplicate", state.ZBattlefield)
	for _, id := range []state.ObjID{akimID, bearID, spellID} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: object %d is not on the battlefield", id)
		}
	}
	sa := e.G.Obj(spellID).Face().SpellAbility()
	if sa == nil || sa.API != "CopyPermanent" {
		t.Fatalf("precondition: Electroduplicate's CopyPermanent ability missing: %+v", sa)
	}
	if n := countTokensNamedOnSeat(t, e, 0, "Bird Token"); n != 0 {
		t.Fatalf("precondition: seat 0 already has %d Bird Tokens", n)
	}

	effects.Resolve(e, &effects.Ctx{Source: spellID, Controller: 0,
		Targets: []state.Target{{Obj: bearID}}, TargetsOffered: true,
		SVars: e.G.Obj(spellID).Face().SVars}, sa)

	copyID := findTokenCopyOf(t, e, e.G.Obj(bearID).Card, bearID)
	if o := e.G.Obj(copyID); o == nil || !o.IsToken {
		t.Fatalf("precondition: the copy %d is not a token", copyID)
	}

	e.putTriggersOnStack()
	answerTriggerOrders(t, e)
	passUntilStackEmpty(t, e, 20)

	if got := countTokensNamedOnSeat(t, e, 0, "Bird Token"); got != 1 {
		t.Fatalf("after a CopyPermanent token, Bird Tokens = %d, want 1 (TokenCreatedOnce did not fire)", got)
	}
	replayCheck(t, e, cfg)
}

// TestIdolOfOblivionCountsADeadCopyTokenThisTurn pins CR 111.1 / 700.14 /
// 603.10a at the card level: Idol of Oblivion's "created a token this turn"
// gate (Count$ThisTurnEntered_Battlefield_Card.tokenCreated+YouCtrl) must
// stay open after the token COPY it saw enter has died, exactly as it does
// for a plain token that died. The copy is a CopyPermanent mint (IsCopy) that
// then goes to the graveyard and ceases; the battlefield entry still proves
// it was a permanent there, so CR 707.10h's ceased-copy rejection (for a
// spell copy parked off the stack) must not apply.
func TestIdolOfOblivionCountsADeadCopyTokenThisTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	idolCard := lookup(t, reg, "Idol of Oblivion")
	spellCard := lookup(t, reg, "Electroduplicate")
	bearCard := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{idolCard, spellCard, bearCard}, nil)

	idol := moveByName(t, e, 0, "Idol of Oblivion", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	spell := moveByName(t, e, 0, "Electroduplicate", state.ZBattlefield)
	for _, id := range []state.ObjID{idol, bear, spell} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: object %d is not on the battlefield", id)
		}
	}
	sa := e.G.Obj(spell).Face().SpellAbility()
	if sa == nil || sa.API != "CopyPermanent" {
		t.Fatalf("precondition: Electroduplicate's CopyPermanent ability missing: %+v", sa)
	}
	addMana(t, e, 0, "")
	if hasAbilityAction(e.legalActions(0), idol, 0) {
		t.Fatal("precondition: Idol's draw ability was offered before the copy token entered")
	}

	effects.Resolve(e, &effects.Ctx{Source: spell, Controller: 0,
		Targets: []state.Target{{Obj: bear}}, TargetsOffered: true,
		SVars: e.G.Obj(spell).Face().SVars}, sa)
	copyID := findTokenCopyOf(t, e, e.G.Obj(bear).Card, bear)
	copyObj := e.G.Obj(copyID)
	if copyObj == nil || !copyObj.IsToken || !copyObj.IsCopy || copyObj.Zone != state.ZBattlefield {
		t.Fatalf("precondition: copy token = %+v, want a token copy on the battlefield", copyObj)
	}
	if !hasAbilityAction(e.legalActions(0), idol, 0) {
		t.Fatal("Idol's draw ability was withheld after the copy token entered")
	}

	// The copy dies and then ceases: the object is off the battlefield with
	// IsCopy still set, exactly the shape the ceased-copy rejection dropped.
	e.emit(events.Event{Kind: events.MoveZone, Obj: copyID, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := e.G.Obj(copyID).Zone; got != state.ZGraveyard {
		t.Fatalf("precondition: copy's zone after the move = %v, want the graveyard", got)
	}
	e.emit(events.Cease(copyID, state.ZGraveyard))
	copyObj = e.G.Obj(copyID)
	if copyObj.Zone != state.ZCeased {
		t.Fatalf("precondition: copy's zone after Cease = %v, want ZCeased", copyObj.Zone)
	}
	if !copyObj.IsCopy {
		t.Fatal("precondition: the copy lost IsCopy, so the ceased-copy rejection cannot be the cause")
	}
	if !hasAbilityAction(e.legalActions(0), idol, 0) {
		t.Fatal("Idol's draw withheld after the copy token died this turn")
	}
	replayCheck(t, e, cfg)
}
