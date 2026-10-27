package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 114.4 + CR 603.2: an Effect's Mode$ Phase trigger (an emblem's "at the
// beginning of combat on your turn", Full Throttle's "at the beginning of
// each combat this turn") fires at EVERY occurrence of its step until the
// Effect ends, not once. Its registration carries |EF (EffectRepeat), so the
// DelayedPush that fires it does not consume it; only OneOff$ True, the
// Effect's Duration, or the body's own self-exile retires it.

const effectPhaseUpkeepSrc = "Name:Emblem Maker\nManaCost:G\nTypes:Instant\n" +
	"A:SP$ Effect | Duration$ Permanent | Name$ Emblem Test | Triggers$ TU\n" +
	"SVar:TU:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Command | Execute$ TG%s\n" +
	"SVar:TG:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"

// effectPhaseUpkeepEngine casts the fixture on turn 1 and returns the engine
// with the Effect registered (precondition-checked) and seat 0 at 20 life.
func effectPhaseUpkeepEngine(t *testing.T, seed uint64, oneOff bool) (*Engine, Config) {
	t.Helper()
	extra := ""
	if oneOff {
		extra = " | OneOff$ True"
	}
	src := fmt.Sprintf(effectPhaseUpkeepSrc, extra)
	e, cfg, id := newFixtureDeck(t, seed, src)
	addMana(t, e, 0, "G")
	castByID(t, e, id)
	fdnCensusDrain(t, e)
	if e.G.Obj(id).Zone != state.ZGraveyard {
		t.Fatalf("precondition: the spell resolved (zone %s)", e.G.Obj(id).Zone)
	}
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].Phase != state.StepUpkeep || e.G.Delayed[0].EventMode != "" {
		t.Fatalf("precondition: one Upkeep Phase registration, got %+v", e.G.Delayed)
	}
	if e.G.Delayed[0].EffectRepeat == oneOff {
		t.Fatalf("EffectRepeat = %v for oneOff=%v", e.G.Delayed[0].EffectRepeat, oneOff)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("precondition: seat 0 life = %d, want 20", got)
	}
	return e, cfg
}

func TestEffectPhaseTriggerRecursEachUpkeep(t *testing.T) {
	t.Parallel()
	e, cfg := effectPhaseUpkeepEngine(t, 205, false)
	// Registered during turn 1, so the first firing is seat 0's turn-3 upkeep.
	passUntilTurn(t, e, 4)
	if got := e.G.Players[0].Life; got != 21 {
		t.Fatalf("life after the turn-3 upkeep = %d, want 21", got)
	}
	passUntilTurn(t, e, 6)
	if got := e.G.Players[0].Life; got != 22 {
		t.Fatalf("life after two own upkeeps = %d, want 22", got)
	}
	passUntilTurn(t, e, 8)
	if got := e.G.Players[0].Life; got != 23 {
		t.Fatalf("life after three own upkeeps = %d, want 23", got)
	}
	if len(e.G.Delayed) != 1 || !e.G.Delayed[0].EffectRepeat {
		t.Fatalf("the emblem registration must persist: %+v", e.G.Delayed)
	}
	replayCheck(t, e, cfg)
}

func TestEffectPhaseOneOffStillOneShot(t *testing.T) {
	t.Parallel()
	e, cfg := effectPhaseUpkeepEngine(t, 205, true)
	passUntilTurn(t, e, 8)
	if got := e.G.Players[0].Life; got != 21 {
		t.Fatalf("OneOff$ True fired %d times, want once (life 21, got %d)", got-20, got)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("the one-shot registration must be consumed: %+v", e.G.Delayed)
	}
	replayCheck(t, e, cfg)
}

// phaseRecurEngine is a two-seat game whose seat-0 and seat-1 decks lead with
// the named corpus cards; seat 0 starts and the engine sits at its Main 1.
func phaseRecurEngine(t *testing.T, names0, names1 []string) (*Engine, Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	deck := func(names []string) []*cards.Card {
		var d []*cards.Card
		for _, n := range names {
			d = append(d, mustCorpusCard(t, reg, n))
		}
		return append(d, mountainDeck(t, 40-len(d))...)
	}
	cfg := seatZeroStart(Config{Seed: 77, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{deck(names0), deck(names1)}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// resolvedAPI counts Resolve events of abilities with the given API.
func resolvedAPI(e *Engine, api string) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind != events.Resolve {
			continue
		}
		if o := e.G.Obj(ev.Obj); o != nil && o.Ability != nil && o.Ability.API == api {
			n++
		}
	}
	return n
}

func TestTezzeretCruelCaptainEmblemFiresEveryCombat(t *testing.T) {
	t.Parallel()
	e, cfg := phaseRecurEngine(t, []string{"Tezzeret, Cruel Captain", "Sol Ring"}, nil)
	tez := moveByName(t, e, 0, "Tezzeret, Cruel Captain", state.ZBattlefield)
	ring := moveByName(t, e, 0, "Sol Ring", state.ZBattlefield)
	f := e.G.Obj(tez).Face()
	var ult *cards.SA
	for _, sa := range f.Abilities {
		if sa.ParamStr(cards.PKName) != "" {
			ult = sa
		}
	}
	if ult == nil {
		t.Fatal("no emblem ability on Tezzeret")
	}
	ctx := effects.NewCtx(tez, 0, effects.CtxInit{SVars: f.SVars})
	effects.Resolve(e, &ctx, ult)
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].Phase != state.StepBeginCombat || !e.G.Delayed[0].EffectRepeat {
		t.Fatalf("precondition: one recurring BeginCombat registration, got %+v", e.G.Delayed)
	}
	if n := e.G.Obj(ring).Counter("P1P1"); n != 0 {
		t.Fatalf("precondition: Sol Ring starts with %d counters", n)
	}
	// Turn 1 (seat 0): the first combat, then turn 2 (seat 1): none, then turn 3.
	passUntilTurn(t, e, 2)
	if n := e.G.Obj(ring).Counter("P1P1"); n != 3 {
		t.Fatalf("counters after seat 0's first combat = %d, want 3", n)
	}
	passUntilTurn(t, e, 3)
	if n := e.G.Obj(ring).Counter("P1P1"); n != 3 {
		t.Fatalf("counters after seat 1's turn = %d, want 3 (the emblem is yours only)", n)
	}
	passUntilTurn(t, e, 4)
	if n := e.G.Obj(ring).Counter("P1P1"); n != 6 {
		t.Fatalf("counters after seat 0's second combat = %d, want 6", n)
	}
	replayCheck(t, e, cfg)
}

func TestFullThrottleUntapsAtEachCombat(t *testing.T) {
	t.Parallel()
	e, cfg := phaseRecurEngine(t, []string{"Full Throttle"}, nil)
	id := moveByName(t, e, 0, "Full Throttle", state.ZHand)
	addMana(t, e, 0, "RRRRRR")
	castByID(t, e, id)
	fdnCensusDrain(t, e)
	if e.G.Obj(id).Zone != state.ZGraveyard {
		t.Fatalf("precondition: Full Throttle resolved (zone %s)", e.G.Obj(id).Zone)
	}
	passUntilTurn(t, e, 2)
	if n := countStepChanges(e, state.StepBeginCombat); n != 3 {
		t.Fatalf("precondition: beginning-of-combat steps on the throttled turn = %d, want 3", n)
	}
	if n := resolvedAPI(e, "UntapAll"); n != 3 {
		t.Fatalf("UntapAll resolutions on the throttled turn = %d, want 3 (one per combat)", n)
	}
	passUntilTurn(t, e, 4)
	if n := resolvedAPI(e, "UntapAll"); n != 3 {
		t.Fatalf("UntapAll resolutions after later turns = %d, want 3", n)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("the this-turn registration must be gone: %+v", e.G.Delayed)
	}
	replayCheck(t, e, cfg)
}

// countStepChanges counts StepChange events into step.
func countStepChanges(e *Engine, step state.Step) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.StepChange && ev.Step == step {
			n++
		}
	}
	return n
}

// passUntilStep is passUntilTurn stopping at the first decision of the given
// turn at the given step.
func passUntilStep(t *testing.T, e *Engine, turn int32, step state.Step) {
	t.Helper()
	for i := 0; i < 4000 && !e.G.Over && !(e.G.Turn == turn && e.G.Step == step); i++ {
		if answerIfDiscard(t, e) {
			continue
		}
		d := e.Pending()
		if d == nil {
			continue
		}
		if d.Kind == decision.KPriority {
			passOnce(t, e)
			continue
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if e.G.Turn != turn || e.G.Step != step {
		t.Fatalf("did not reach turn %d %s (at turn %d %s)", turn, step, e.G.Turn, e.G.Step)
	}
}

func TestSizzlingSoloistMustAttackStaysOneShot(t *testing.T) {
	t.Parallel()
	e, cfg := phaseRecurEngine(t,
		[]string{"Sizzling Soloist", "Savannah Lions", "Elite Vanguard"}, []string{"Grizzly Bears"})
	moveByName(t, e, 0, "Sizzling Soloist", state.ZBattlefield)
	bears := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	for _, n := range []string{"Savannah Lions", "Elite Vanguard"} {
		id := moveByName(t, e, 0, n, state.ZHand)
		addMana(t, e, 0, "W")
		castByID(t, e, id)
		fdnCensusDrain(t, e)
		if e.G.Obj(id).Zone != state.ZBattlefield {
			t.Fatalf("precondition: %s resolved (zone %s)", n, e.G.Obj(id).Zone)
		}
	}
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].Phase != state.StepBeginCombat || !e.G.Delayed[0].EffectRepeat {
		t.Fatalf("precondition: the second resolution registered the Effect's Phase trigger, got %+v", e.G.Delayed)
	}
	// Seat 1's next combat (turn 2) runs the body once. The body's own
	// self-exile (Host.EndEffectSource) ends the Effect before any attacker is
	// declared -- asserted at the declare-attackers ask, before the forced
	// attack can kill the remembered creature and retire the registration by
	// another route -- so seat 1's combat on turn 4 does nothing.
	passUntilStep(t, e, 2, state.StepDeclareAttackers)
	if n := resolvedAPI(e, "Animate"); n != 1 {
		t.Fatalf("MustAttack body resolutions at turn 2's declare attackers = %d, want 1", n)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("the Effect must be gone once its body ran: %+v", e.G.Delayed)
	}
	if e.G.Obj(bears).Zone != state.ZBattlefield {
		t.Fatalf("precondition: the remembered creature is still on the battlefield (%s)", e.G.Obj(bears).Zone)
	}
	passUntilTurn(t, e, 5)
	if n := resolvedAPI(e, "Animate"); n != 1 {
		t.Fatalf("MustAttack body resolutions after turn 4 = %d, want 1", n)
	}
	replayCheck(t, e, cfg)
}
