package rules

// chooseplayer-subability-riders: ChoosePlayer's chained riders
// (ChooseSubAbility$/CantChooseSubAbility$) and the per-player MustAttack$
// ChosenPlayer requirement they bind, pinned on the real corpus carrier
// Territorial Hellkite (a Temur Roar deck card).
//
// The card's begin-combat trigger chooses an opponent at random that the
// Hellkite did not attack during the last combat
// (Choices$ Player.Opponent+!IsRemembered | Random$ True). A successful
// choice runs ChooseSubAbility$ DBPump, which registers an Effect-delivered
// `Mode$ MustAttack | ValidCreature$ Card.EffectSource | MustAttack$
// ChosenPlayer` for the combat; a choice with no candidate runs
// CantChooseSubAbility$ DBTap, tapping the dragon. The requirement is read at
// the declare-attackers step through the shared requiredAttackDefender seam
// (rules/combat.go), which filters the offered (attacker, defender) pairs to
// the named player and marks the creature required.
//
// Every fixture mutation goes through e.emit, so every game replays
// byte-identically.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// hellkiteEngine builds a three-seat game (seat 0 is the active player
// controlling the real corpus Territorial Hellkite, seats 1 and 2 are its
// opponents) parked at seat 0's Main1, ready for the test to cross into
// BeginCombat. Mountain decks fill every seat so the fixture's only
// non-land permanent is the Hellkite.
func hellkiteEngine(t *testing.T, extras int) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	m, ok := reg.Lookup("Mountain")
	if !ok {
		t.Fatal("corpus fixture: Mountain missing")
	}
	fill := func(n int) []*cards.Card {
		out := make([]*cards.Card, n)
		for i := range out {
			out[i] = m
		}
		return out
	}
	hk := lookup(t, reg, "Territorial Hellkite")
	if hk == nil {
		t.Fatal("corpus fixture: Territorial Hellkite missing")
	}
	cfg := seatZeroStart(Config{Seed: 716, Names: []string{"a", "b", "c"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{hk}, fill(40)...)[:40],
			fill(40),
			fill(40),
		}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	id := moveByName(t, e, 0, "Territorial Hellkite", state.ZBattlefield)
	return e, id
}

// crossIntoBeginCombat emits the StepChange for seat 0's BeginCombat and
// drains the trigger's resolution, so the ChoosePlayer rider has run by the
// time it returns. It uses the real Phase trigger machinery (the card's
// `T:Mode$ Phase | Phase$ BeginCombat` line), not a hand-resolved SVar.
func crossIntoBeginCombat(t *testing.T, e *Engine) {
	t.Helper()
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepBeginCombat})
	// Place the queued Phase trigger on the stack (the direct StepChange the
	// phase-fixture tests use queues rather than places), answer any trigger
	// order, then resolve it: the ChoosePlayer rider runs as the trigger
	// resolves.
	e.putTriggersOnStack()
	answerTriggerOrders(t, e)
	passUntilStackEmpty(t, e, 20)
}

// TestTerritorialHellkiteChoosesAndBindsAttackDefender is the brief's leaf:
// with two un-remembered opponents the random pick is a real choice (one of
// seats 1/2), the chosen player is recorded on the source, and the
// declare-attackers offer list contains ONLY that defender for the dragon,
// with the dragon marked Required.
func TestTerritorialHellkiteChoosesAndBindsAttackDefender(t *testing.T) {
	e, hk := hellkiteEngine(t, 0)

	// Precondition: the dragon is on the battlefield under seat 0's control
	// and the board starts with NO chosen player and NO remembered opponents,
	// so the Choices$ filter has both opponents eligible.
	o := e.G.Obj(hk)
	if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Hellkite not on seat 0's battlefield: %+v", o)
	}
	if len(o.Chosen) != 0 || len(o.Remembered) != 0 {
		t.Fatalf("precondition: dragon already has chosen/remembered state: chosen=%v remembered=%v", o.Chosen, o.Remembered)
	}

	crossIntoBeginCombat(t, e)

	// The rider registered the requirement and the random pick recorded
	// exactly one opponent as chosen.
	o = e.G.Obj(hk)
	if len(o.Chosen) != 1 || !o.Chosen[0].IsPlayer || (o.Chosen[0].Player != 1 && o.Chosen[0].Player != 2) {
		t.Fatalf("after the begin-combat trigger, chosen = %+v, want exactly one opponent seat", o.Chosen)
	}
	chosen := o.Chosen[0].Player
	if !e.requiredAttackDefenderMatches(hk, chosen) {
		t.Fatalf("no MustAttack requirement binds the dragon to chosen player %d", chosen)
	}

	// Drive into the declare-attackers step and inspect the offer list.
	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected a KAttackers decision, got %+v", d)
	}
	sawDragon := false
	for _, opt := range d.Options {
		if opt.Obj != hk {
			continue
		}
		sawDragon = true
		if opt.Player != chosen {
			t.Fatalf("dragon offered against seat %d; the requirement binds it to seat %d", opt.Player, chosen)
		}
		if !opt.Required {
			t.Fatalf("dragon option against the chosen player is not marked Required")
		}
	}
	if !sawDragon {
		t.Fatalf("dragon has no offered attack pair at all; options=%+v", d.Options)
	}
	// The declaration must be accepted against the chosen player.
	submitAttackersOnly(t, e, hk)
}

// TestTerritorialHellkiteNoCandidateTaps is the "If you can't choose an
// opponent this way, tap CARDNAME" arm: with every opponent already in the
// dragon's remembered set (the set the Choices$ !IsRemembered filter
// excludes), CantChooseSubAbility$ DBTap runs, the dragon taps, and no
// requirement is registered.
func TestTerritorialHellkiteNoCandidateTaps(t *testing.T) {
	e, hk := hellkiteEngine(t, 0)

	// Precondition: the dragon is on the battlefield and both opponents are
	// already remembered, so the Choices$ filter admits nobody.
	o := e.G.Obj(hk)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Hellkite not on the battlefield")
	}
	e.emit(events.Event{Kind: events.Choose, Obj: hk, Counter: "remembered",
		IDs: []state.ObjID{state.PlayerRef(1), state.PlayerRef(2)}})
	if got := len(e.G.Obj(hk).Remembered); got != 2 {
		t.Fatalf("precondition: remembered set = %d players, want 2", got)
	}
	if e.G.Obj(hk).Tapped {
		t.Fatal("precondition: dragon already tapped")
	}

	crossIntoBeginCombat(t, e)

	if !e.G.Obj(hk).Tapped {
		t.Fatal("with no eligible opponent the dragon should have tapped (CantChooseSubAbility$)")
	}
	if len(e.G.Obj(hk).Chosen) != 0 {
		t.Fatalf("no-candidate arm recorded a chosen player: %+v", e.G.Obj(hk).Chosen)
	}
	// No per-player requirement may survive the failed choice.
	if _, ok := e.requiredAttackDefender(hk); ok {
		t.Fatal("a failed choice still registered a MustAttack requirement")
	}
}

// requiredAttackDefenderMatches reports whether id is required to attack
// exactly defender (a small test-side read of the engine's own resolver).
func (e *Engine) requiredAttackDefenderMatches(id state.ObjID, defender state.PlayerID) bool {
	p, ok := e.requiredAttackDefender(id)
	return ok && p == defender
}
