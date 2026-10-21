package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCopyTokenFiresTokenCreated pins CR 111.1/707.2: a token created by a
// copy effect (DB$ CopyPermanent) is still a token being created, so Akim's
// "whenever you create one or more tokens for the first time each turn"
// trigger (Mode$ TokenCreatedOnce) fires for it. The copy mints through
// events.CopyToken and enters with a separate MoveZone, not the TokenCreate
// event the matcher used to require, so before the fix the Bird was never
// created.
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
