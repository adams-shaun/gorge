// The Effect-delivered may-play grant's CheckSVar$/SVarCompare$ gate
// (task cr-g135): a `DB$ Effect` static line whose STPlay carries
// `CheckSVar$ X` ("for as long as those cards remain exiled, you may play
// them if you control a Kavu/Wizard" -- CR 611.2a/b) must register AND be
// gated on the live board: the permission holds exactly while its condition
// does (CR 601.3), evaluated through the same shared statics evaluator the
// printed route's mayPlayConditionGateHolds runs. Both carriers here are
// real corpus cards; no Forge text is committed.

package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// effectMayPlayEngine is adventureCorpusEngine's shape narrowed for this
// file: seat 0's 40-card deck leads with the named corpus carrier (followed
// by any extras, then Islands), so replayFromLog's genesis reconstruction
// contains every card the test later moves; the carrier is bridged into
// seat 0's hand with a logged MoveZone. Returns the engine, the config for
// the closing replayCheck, and the carrier's hand id.
func effectMayPlayEngine(t *testing.T, name string, extras ...*cards.Card) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	carrier := searchCorpusCard(t, reg, name)
	island := searchCorpusCard(t, reg, "Island")
	deck := append([]*cards.Card{carrier}, extras...)
	for len(deck) < 40 {
		deck = append(deck, island)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = island
	}
	cfg := seatZeroStart(Config{Seed: 7401, Names: []string{"cr135", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	id := searchMoveByName(t, e, name, state.ZHand)
	return e, cfg, id
}

// effectMayPlayGateGrant returns the active may-play grant carrying a
// CheckSVar$ gate, or nil.
func effectMayPlayGateGrant(e *Engine) *ContinuousEffect {
	for i := range e.continuous {
		ce := &e.continuous[i]
		if ce.MayPlay && ce.MayPlayCheckSVar != "" {
			return ce
		}
	}
	return nil
}

// TestEffectMayPlayCheckSVarGateFlameshape pins the gate end to end on
// Flameshape (Gandalf, Goblins' Bane's Adventure face): its STPlay grant
// permits the two exiled cards only while you control a Wizard
// (SVar:X:Count$Valid Wizard.YouCtrl, printed on the resolving Adventure
// face). With no Wizard both cards are unplayable and unoffered; with one
// on the battlefield both are granted and the priority offer names an
// exiled card.
func TestEffectMayPlayCheckSVarGateFlameshape(t *testing.T) {
	t.Parallel()
	wiz := card(t, "Name:Test Wizard\nManaCost:1U\nTypes:Creature Human Wizard\nPT:1/1\nOracle:x\n")
	e, cfg, flame := effectMayPlayEngine(t, "Gandalf, Goblins' Bane", wiz)
	addMana(t, e, 0, "RR")
	submitOption(t, e, "", "Cast Flameshape")
	passUntilStackEmpty(t, e, 60)

	if o := e.G.Obj(flame); o.Zone != state.ZExile {
		t.Fatalf("precondition: Flameshape resolved to %s, want exile", o.Zone)
	}
	dug := map[state.ObjID]bool{}
	for _, id := range e.G.Zone(state.ZExile, 0) {
		if id != flame {
			dug[id] = true
		}
	}
	if len(dug) != 2 {
		t.Fatalf("precondition: want the two dug cards exiled, got %d", len(dug))
	}
	grant := effectMayPlayGateGrant(e)
	if grant == nil {
		t.Fatal("precondition: no CheckSVar-gated may-play grant registered")
	}
	if grant.SVars == nil || grant.SVars["X"] != "Count$Valid Wizard.YouCtrl" {
		t.Fatalf("precondition: grant carries no resolving-face SVar table: %+v", grant.SVars)
	}

	// No Wizard: the gate denies both cards, and the priority offer names
	// neither.
	for id := range dug {
		if e.mayPlayEffectGrantsCast(0, e.G.Obj(id)) {
			t.Fatalf("no Wizard on the battlefield: exiled card %d granted", id)
		}
	}
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected a priority decision, got %+v", d)
	}
	for i := range d.Options {
		if dug[d.Options[i].Obj] {
			t.Fatalf("no Wizard: priority options name an exiled card: %+v", d.Options)
		}
	}

	// Put the Wizard (the deck's second card) onto the battlefield: both
	// cards become playable and the offer names one.
	wizID := searchMoveByName(t, e, "Test Wizard", state.ZBattlefield)
	if w := e.G.Obj(wizID); w.Zone != state.ZBattlefield {
		t.Fatalf("precondition: wizard at %s, want battlefield", w.Zone)
	}
	e.priorityRound()
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected a priority decision after the Wizard, got %+v", d)
	}

	for id := range dug {
		if !e.mayPlayEffectGrantsCast(0, e.G.Obj(id)) {
			t.Fatalf("Wizard on the battlefield: exiled card %d still not granted", id)
		}
	}
	named := false
	for i := range d.Options {
		if dug[d.Options[i].Obj] {
			named = true
		}
	}
	if !named {
		t.Fatalf("Wizard on the battlefield: no priority option names an exiled card: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}

// TestEffectMayPlayCheckSVarPossibilityTechnician pins the probe board:
// Possibility Technician's ETB trigger exiles the top card of the library
// and delivers the STPlay grant; with the fix the log carries no
// `unimplemented (STPlay)` Note and the exiled card is covered while the
// Technician (a Kavu) is on the battlefield (SVar:X:Count$Valid
// Kavu.YouCtrl reads 1).
func TestEffectMayPlayCheckSVarPossibilityTechnician(t *testing.T) {
	t.Parallel()
	e, cfg, tech := effectMayPlayEngine(t, "Possibility Technician")
	placeOnBattlefield(t, e, tech)
	passUntilStackEmpty(t, e, 60)

	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented (STPlay)") {
			t.Fatalf("STPlay line still loud-unimplemented: %s", ev.Text)
		}
	}
	exiled := e.G.Zone(state.ZExile, 0)
	if len(exiled) != 1 {
		t.Fatalf("precondition: want the ETB trigger's one exiled card, got %d", len(exiled))
	}
	if grant := effectMayPlayGateGrant(e); grant == nil {
		t.Fatal("precondition: no CheckSVar-gated may-play grant registered")
	}
	if !e.mayPlayEffectGrantsCast(0, e.G.Obj(exiled[0])) {
		t.Fatalf("Kavu on the battlefield: exiled card %d not covered", exiled[0])
	}
	replayCheck(t, e, cfg)
}
