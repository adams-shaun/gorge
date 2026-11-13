package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGrantedDelayedSpellCastResolvesOnGrantor: Way of the Cryomancer grants
// every planeswalker its controller has "[-3]: When you next cast an instant
// or sorcery spell this turn, copy that spell." The granted activation's
// Ctx.Source is the grantee (the Jace token), but its Execute$ body
// (EffTrigCopy) lives on the grantor's face. The registration must carry the
// grantor so the fire-time SVar lookup resolves the body and its table there,
// instead of dropping the registration because the grantee has no EffTrigCopy.
func TestGrantedDelayedSpellCastResolvesOnGrantor(t *testing.T) {
	t.Parallel()
	e, cfg, tok := grantedDelayedSpellCastGame(t)
	activateGrantedDelayedSpellCast(t, e, tok)
	assertGrantedDelayedRegistration(t, e, tok)

	ensureInHand(t, e, 0, "Opt")
	addMana(t, e, 0, "U")
	castNamed(t, e, "Opt")
	// The delayed trigger is minted on the cast and resolves on the next
	// priority round; the original Opt is still on the stack beside its copy.
	passUntilCopy(t, e)
	if got := len(e.G.Stack); got != 2 {
		t.Fatalf("stack size = %d, want the cast and its copy (stack %v)", got, e.G.Stack)
	}
	if got := copiesOnStack(e); got != 1 {
		t.Fatalf("stack has %d copied spells, want exactly 1 (stack %v)", got, e.G.Stack)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("the one-shot registration survived its firing: %+v", e.G.Delayed)
	}
	drainGrantedStack(t, e, 60)
	replayCheck(t, e, cfg)
}

// TestGrantedDelayedSpellCastOneShotAndExpires: the granted registration is
// one-shot (a second instant the same turn gets no copy) and ThisTurn$ True
// still expires it at the turn change.
func TestGrantedDelayedSpellCastOneShotAndExpires(t *testing.T) {
	t.Parallel()
	t.Run("second cast same turn", func(t *testing.T) {
		e, cfg, tok := grantedDelayedSpellCastGame(t)
		activateGrantedDelayedSpellCast(t, e, tok)
		assertGrantedDelayedRegistration(t, e, tok)

		ensureInHand(t, e, 0, "Opt")
		addMana(t, e, 0, "U")
		castNamed(t, e, "Opt")
		passUntilCopy(t, e)
		drainGrantedStack(t, e, 60)

		ensureInHand(t, e, 0, "Opt")
		addMana(t, e, 0, "U")
		castNamed(t, e, "Opt")
		if got := len(e.G.Stack); got != 1 {
			t.Errorf("second cast put %d objects on the stack, want 1 (no second copy)", got)
		}
		if got := delayedPushCount(e); got != 1 {
			t.Errorf("delayed pushes after the second cast = %d, want 1 (only the first cast)", got)
		}
		drainGrantedStack(t, e, 60)
		replayCheck(t, e, cfg)
	})
	t.Run("next turn", func(t *testing.T) {
		e, cfg, tok := grantedDelayedSpellCastGame(t)
		activateGrantedDelayedSpellCast(t, e, tok)
		assertGrantedDelayedRegistration(t, e, tok)

		passUntilTurn(t, e, 3)
		if len(e.G.Delayed) != 0 {
			t.Fatalf("the one-shot registration survived the turn change: %+v", e.G.Delayed)
		}
		ensureInHand(t, e, 0, "Opt")
		addMana(t, e, 0, "U")
		castNamed(t, e, "Opt")
		if got := len(e.G.Stack); got != 1 {
			t.Errorf("next-turn cast put %d objects on the stack, want 1 (ThisTurn$ expired)", got)
		}
		if got := delayedPushCount(e); got != 0 {
			t.Errorf("delayed pushes on the next turn = %d, want 0", got)
		}
		drainGrantedStack(t, e, 60)
		replayCheck(t, e, cfg)
	})
}

// TestGrantedDelayedSpellCastSurvivesGrantorLeaving: CR 603.7 makes the
// registration independent of its source once created. Way of the Cryomancer
// leaving the battlefield before the cast must not drop the promise; its face
// still owns EffTrigCopy in the graveyard.
func TestGrantedDelayedSpellCastSurvivesGrantorLeaving(t *testing.T) {
	t.Parallel()
	e, cfg, tok := grantedDelayedSpellCastGame(t)
	activateGrantedDelayedSpellCast(t, e, tok)
	dt := assertGrantedDelayedRegistration(t, e, tok)

	way := findNamedInZone(e, 0, state.ZBattlefield, "Way of the Cryomancer")
	if way == 0 {
		t.Fatal("precondition: Way of the Cryomancer is not on the battlefield")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: way, From: state.ZBattlefield, To: state.ZGraveyard})
	if findNamedInZone(e, 0, state.ZBattlefield, "Way of the Cryomancer") != 0 {
		t.Fatal("precondition: Way of the Cryomancer is still on the battlefield")
	}
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].ID != dt.ID {
		t.Fatalf("the registration left with its grantor: %+v", e.G.Delayed)
	}

	ensureInHand(t, e, 0, "Opt")
	addMana(t, e, 0, "U")
	castNamed(t, e, "Opt")
	passUntilCopy(t, e)
	if got := copiesOnStack(e); got != 1 {
		t.Fatalf("stack has %d copied spells after the grantor left, want 1 (stack %v)", got, e.G.Stack)
	}
	drainGrantedStack(t, e, 60)
	replayCheck(t, e, cfg)
}

// TestGrantedDelayedSpellCastOpponentDoesNotCopy: ValidActivatingPlayer$ You
// keeps the registration from firing on the opponent's instant.
func TestGrantedDelayedSpellCastOpponentDoesNotCopy(t *testing.T) {
	t.Parallel()
	e, cfg, tok := grantedDelayedSpellCastGame(t)
	activateGrantedDelayedSpellCast(t, e, tok)
	assertGrantedDelayedRegistration(t, e, tok)

	// Hand priority to seat 1, then cast its instant.
	ensureInHand(t, e, 1, "Opt")
	passPriorityOnce(t, e)
	if d := e.Pending(); d == nil || d.Player != 1 {
		t.Fatalf("precondition: seat 1 does not hold priority: %+v", d)
	}
	addMana(t, e, 1, "U")
	castNamed(t, e, "Opt")
	if got := len(e.G.Stack); got != 1 {
		t.Errorf("the opponent's cast put %d objects on the stack, want 1 (no copy)", got)
	}
	if got := delayedPushCount(e); got != 0 {
		t.Errorf("delayed pushes on the opponent's cast = %d, want 0", got)
	}
	if len(e.G.Delayed) != 1 {
		t.Errorf("the opponent's cast consumed the registration: %+v", e.G.Delayed)
	}
	drainGrantedStack(t, e, 60)
	replayCheck(t, e, cfg)
}

// grantedDelayedSpellCastGame casts Way of the Cryomancer, whose ETB creates
// (or grows) a Jace token, and returns the engine and that token.
func grantedDelayedSpellCastGame(t *testing.T) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{
		lookup(t, reg, "Way of the Cryomancer"),
		lookup(t, reg, "Opt"),
		lookup(t, reg, "Opt"),
	}, []*cards.Card{lookup(t, reg, "Opt")})
	ensureInHand(t, e, 0, "Way of the Cryomancer")
	addMana(t, e, 0, "UUU")
	castNamed(t, e, "Way of the Cryomancer")
	drainGrantedStack(t, e, 60)
	toks := jaceTokens(e, 0)
	if len(toks) != 1 {
		t.Fatalf("precondition: Way of the Cryomancer made %d Jace tokens, want 1", len(toks))
	}
	if got := e.G.Obj(toks[0]).Counter("LOYALTY"); got < 3 {
		t.Fatalf("precondition: Jace token loyalty = %d, want at least 3 for the [-3]", got)
	}
	return e, cfg, toks[0]
}

// activateGrantedDelayedSpellCast activates the token's granted [-3] and
// resolves it, leaving the delayed SpellCast registration live.
func activateGrantedDelayedSpellCast(t *testing.T, e *Engine, tok state.ObjID) {
	t.Helper()
	opt, ok := findAbilityOptionByLabel(e, tok, "When you next cast")
	if !ok {
		t.Fatalf("the Jace token's granted [-3] is not offered: %+v", e.Pending())
	}
	submitChoices(t, e, opt.Index)
	drainGrantedStack(t, e, 60)
	if got := e.G.Obj(tok).Counter("LOYALTY"); got != 2 {
		t.Fatalf("Jace token loyalty after the [-3] = %d, want 2", got)
	}
}

// assertGrantedDelayedRegistration checks the live registration's shape and,
// crucially, the precondition the fix depends on: the grantee's face has no
// EffTrigCopy while the grantor's does.
func assertGrantedDelayedRegistration(t *testing.T, e *Engine, tok state.ObjID) state.DelayedTrigger {
	t.Helper()
	if len(e.G.Delayed) != 1 {
		t.Fatalf("live registrations = %+v, want exactly the granted SpellCast one", e.G.Delayed)
	}
	dt := e.G.Delayed[0]
	if dt.Source != tok || dt.Execute != "EffTrigCopy" || dt.EventMode != "SpellCast" {
		t.Fatalf("registration = %+v, want Source %d Execute EffTrigCopy Mode SpellCast", dt, tok)
	}
	if dt.Grantor == 0 {
		t.Fatalf("registration carries no grantor: %+v", dt)
	}
	grantor := e.G.Obj(dt.Grantor)
	if grantor == nil || grantor.Face() == nil || grantor.Face().Name != "Way of the Cryomancer" {
		t.Fatalf("registration grantor = %+v, want Way of the Cryomancer", grantor)
	}
	if events.ResolveSVarAcrossFaces(e.G.Obj(dt.Source), dt.Execute) != nil {
		t.Fatalf("precondition: the grantee's face unexpectedly has %s; the bug needs it absent", dt.Execute)
	}
	if events.ResolveSVarAcrossFaces(grantor, dt.Execute) == nil {
		t.Fatalf("precondition: the grantor's face has no %s", dt.Execute)
	}
	return dt
}

// passUntilCopy passes priority (never answering a non-priority ask) until a
// copied spell is on the stack, and fails if it never appears.
func passUntilCopy(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 20; i++ {
		if copiesOnStack(e) > 0 {
			return
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while waiting for the copy")
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision while waiting for the copy: %+v", d)
		}
		passPriorityOnce(t, e)
	}
	t.Fatalf("no copied spell reached the stack (stack %v)", e.G.Stack)
}

// delayedPushCount counts the DelayedPush events in the engine's whole log.
func delayedPushCount(e *Engine) int {
	return countEvents(e, func(ev events.Event) bool { return ev.Kind == events.DelayedPush })
}

// drainGrantedStack resolves the stack, taking the offered order for a Scry
// arrange ask (the only mid-resolution decision Opt poses) and passing every
// priority decision.
func drainGrantedStack(t *testing.T, e *Engine, limit int) {
	t.Helper()
	for i := 0; i < limit && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the stack (depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KArrange {
			choices := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				choices = append(choices, o.Index)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
				t.Fatalf("submit scry order: %v", err)
			}
			continue
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("non-priority decision %+v while draining the stack", d)
		}
		passPriorityOnce(t, e)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("the stack never emptied (depth %d)", len(e.G.Stack))
	}
}
