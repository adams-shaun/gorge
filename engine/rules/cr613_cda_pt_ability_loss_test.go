package rules

// CR 613.4a / 613.1f: a characteristic-defining ability that defines power
// and toughness is an ability like any other. A "loses all abilities" effect
// removes it in layer 6, before layer 7a would apply it, so the creature's
// P/T is the printed value (a printed * reads 0). Forge
// (StaticAbility.shouldApplyContinuousAbility: the host must still carry the
// static at layer 7a) and XMage (ContinuousEffects.apply: 7a only for an
// abilityActive ability) agree. A CDA still functions in every zone the
// removal does not reach (CR 604.3).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cdaMill moves n cards from the top of seat p's library to its graveyard.
func cdaMill(t *testing.T, e *Engine, p state.PlayerID, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		lib := e.G.Zone(state.ZLibrary, p)
		if len(lib) == 0 {
			t.Fatalf("seat %d library is empty", p)
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: lib[len(lib)-1], From: state.ZLibrary, To: state.ZGraveyard})
	}
	e.pending = nil
	e.Advance()
}

// cdaPT reads o's P/T through both derivation paths: the scalar walk
// (combat, SBA) and the full Derived record (filters, the view).
func cdaPT(e *Engine, id state.ObjID) (p, t, dp, dt int32) {
	d := e.Derived(id)
	return e.Power(id), e.Toughness(id), d.Power, d.Toughness
}

// TestCR613_4aCDAPTLostUnderDressDown: Tarmogoyf (*/1+*) over a graveyard
// holding only lands is 1/2. Under Dress Down ("Creatures lose all
// abilities") its CDA is gone in layer 6, so it is its printed 0/1; when
// Dress Down leaves, the CDA is back.
func TestCR613_4aCDAPTLostUnderDressDown(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Tarmogoyf"), lookup(t, reg, "Dress Down")}, nil)
	goyf := moveCorpusCard(t, e, "Tarmogoyf", 0, state.ZBattlefield)
	cdaMill(t, e, 1, 2)
	if p, tt, dp, dt := cdaPT(e, goyf); p != 1 || tt != 2 || dp != 1 || dt != 2 {
		t.Fatalf("precondition: Tarmogoyf over a land-only graveyard is %d/%d (Derived %d/%d), want 1/2", p, tt, dp, dt)
	}
	dress := moveCorpusCard(t, e, "Dress Down", 0, state.ZBattlefield)
	if o := e.G.Obj(goyf); o.Zone != state.ZBattlefield {
		t.Fatalf("CR 208.2a seq %d: Tarmogoyf under Dress Down is in %v, want the battlefield (printed */1+* without its CDA is 0/1)", len(e.L.Events), o.Zone)
	}
	if p, tt, dp, dt := cdaPT(e, goyf); p != 0 || tt != 1 || dp != 0 || dt != 1 {
		t.Errorf("CR 613.4a seq %d: Tarmogoyf under Dress Down is %d/%d (Derived %d/%d), want its printed 0/1 (the CDA is lost in layer 6)",
			len(e.L.Events), p, tt, dp, dt)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: dress, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.Advance()
	// Dress Down in the graveyard adds the enchantment type: land + enchantment.
	if p, tt, dp, dt := cdaPT(e, goyf); p != 2 || tt != 3 || dp != 2 || dt != 3 {
		t.Errorf("CR 613.4a seq %d: Tarmogoyf once Dress Down left is %d/%d (Derived %d/%d), want 2/3 (the CDA is back)",
			len(e.L.Events), p, tt, dp, dt)
	}
	replayCheck(t, e, cfg)
}

// TestCR613_4aCDAPTLossKillsConsumingAberration: Consuming Aberration (*/*,
// a Foundations card) is 3/3 over three cards in the opponent's graveyard.
// Dress Down removes its CDA, so it is 0/0 and dies to the CR 704.5f
// state-based action.
func TestCR613_4aCDAPTLossKillsConsumingAberration(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Consuming Aberration"), lookup(t, reg, "Dress Down")}, nil)
	// Mill first: an Aberration entering over an empty graveyard is 0/0.
	cdaMill(t, e, 1, 3)
	ab := moveCorpusCard(t, e, "Consuming Aberration", 0, state.ZBattlefield)
	if o := e.G.Obj(ab); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Consuming Aberration is in %v, want the battlefield", o.Zone)
	}
	if p, tt, _, _ := cdaPT(e, ab); p != 3 || tt != 3 {
		t.Fatalf("precondition: Consuming Aberration over three opposing graveyard cards is %d/%d, want 3/3", p, tt)
	}
	moveCorpusCard(t, e, "Dress Down", 0, state.ZBattlefield)
	if o := e.G.Obj(ab); o.Zone != state.ZGraveyard {
		p, tt, _, _ := cdaPT(e, ab)
		t.Errorf("CR 613.4a/704.5f seq %d: Consuming Aberration under Dress Down is in %v at %d/%d, want the graveyard (0/0 without its CDA)",
			len(e.L.Events), o.Zone, p, tt)
	}
	replayCheck(t, e, cfg)
}

// TestCR613_4aCDAPTGuards pins what stays right: Dress Down's removal reaches
// only creatures on the battlefield, so a Tarmogoyf in hand keeps its CDA
// (CR 604.3); and Witness Protection's base 1/1 (layer 7b) still applies to
// an enchanted Consuming Aberration.
func TestCR613_4aCDAPTGuards(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	t.Run("Tarmogoyf in hand under Dress Down", func(t *testing.T) {
		e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Tarmogoyf"), lookup(t, reg, "Dress Down")}, nil)
		goyf := moveCorpusCard(t, e, "Tarmogoyf", 0, state.ZHand)
		cdaMill(t, e, 1, 2)
		moveCorpusCard(t, e, "Dress Down", 0, state.ZBattlefield)
		if p, tt, dp, dt := cdaPT(e, goyf); p != 1 || tt != 2 || dp != 1 || dt != 2 {
			t.Errorf("CR 604.3 seq %d: Tarmogoyf in hand under Dress Down is %d/%d (Derived %d/%d), want 1/2", len(e.L.Events), p, tt, dp, dt)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("Witness Protection on Consuming Aberration", func(t *testing.T) {
		e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Consuming Aberration"), lookup(t, reg, "Witness Protection")}, nil)
		cdaMill(t, e, 1, 3)
		ab := moveCorpusCard(t, e, "Consuming Aberration", 0, state.ZBattlefield)
		moveCorpusCard(t, e, "Witness Protection", 0, state.ZHand)
		lossAttach(t, e, "Witness Protection", 0, ab)
		if p, tt, dp, dt := cdaPT(e, ab); p != 1 || tt != 1 || dp != 1 || dt != 1 {
			t.Errorf("CR 613.4b seq %d: Consuming Aberration under Witness Protection is %d/%d (Derived %d/%d), want 1/1", len(e.L.Events), p, tt, dp, dt)
		}
		replayCheck(t, e, cfg)
	})
}
