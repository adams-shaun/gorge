package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// untapDenialGrants counts the registered continuous effects that are
// Untap-event replacements (the DontUntap grants this file studies).
func untapDenialGrants(e *Engine, controller state.PlayerID) []ContinuousEffect {
	var out []ContinuousEffect
	for _, ce := range e.continuous {
		if ce.ReplacementEvent == "Untap" && ce.Controller == controller {
			out = append(out, ce)
		}
	}
	return out
}

// activateVecsManaCombo submits Vec's priority "activate" option and, on the
// mana-ability choice ask it poses (two {T} abilities share the option),
// picks the Produced$ Combo G W one -- the flattened "Add {G}"/"Add {W}"
// options are the ones carrying a ManaSymbol -- whose SubAbility registers
// the stay-tapped Effect.
func activateVecsManaCombo(t *testing.T, e *Engine, vec state.ObjID) {
	t.Helper()
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision to activate Vec from: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == vec {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("Vec's mana activation not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	for i := 0; i < 8; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		choice := -1
		for _, o := range d.Options {
			if o.Kind == "mana" && o.ManaSymbol != "" {
				choice = o.Index
			}
		}
		if choice < 0 {
			t.Fatalf("no Combo colour option on the mana-ability ask: %+v", d.Options)
		}
		submitChoices(t, e, choice)
	}
	passUntilStackEmpty(t, e, 50)
}

func TestVecTownshipsStaysTappedThroughOneUntapStep(t *testing.T) {
	t.Parallel()
	// Real corpus card: activating {T}: Add {G} or {W} registers an Effect
	// whose replacement (DontUntap, ValidCard$ Card.IsRemembered,
	// ValidStepTurnToController$ Player.Activator, Duration$
	// UntilTheEndOfYourNextUntap) keeps VEC tapped through the activator's
	// ONE next untap step, then releases it.
	e := handEngine(t, corpusCard(t, "Vec Townships"))
	vec := moveByName(t, e, 0, "Vec Townships", state.ZBattlefield)
	other := onBoard(t, e, 0, "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n")
	e.emit(events.Event{Kind: events.Tap, Obj: other})

	activateVecsManaCombo(t, e, vec)
	passPriority(t, e)

	// Precondition: the {T} cost tapped Vec, and the Effect registered the
	// seat-0 Untap replacement this test is about.
	if !e.G.Obj(vec).Tapped {
		t.Fatal("precondition: activating should have tapped Vec Townships")
	}
	if n := len(untapDenialGrants(e, 0)); n != 1 {
		t.Fatalf("precondition: want exactly 1 seat-0 Untap replacement, got %d", n)
	}

	// An opponent's untap step is not the grant's step: everything about
	// seat 0's board is untouched by it. (The untap step itself is eventless,
	// so the first observable stop is the following upkeep.)
	driveToStepAny(t, e, 2, 1, state.StepUpkeep)
	if !e.G.Obj(vec).Tapped || !e.G.Obj(other).Tapped {
		t.Fatal("seat 1's untap step must not touch seat 0's permanents")
	}
	if n := len(untapDenialGrants(e, 0)); n != 1 {
		t.Fatalf("grant must survive an opponent's untap step, have %d", n)
	}

	// The controller's next untap step: the replacement fires, so Vec stays
	// tapped while the other land untaps -- and the grant is gone once the
	// step ends, even though it did drop no untap (Vec was the only tapped
	// card it could have named).
	driveToStepAny(t, e, 3, 0, state.StepUpkeep)
	if e.G.Obj(other).Tapped {
		t.Error("the ordinary land must untap in its controller's untap step")
	}
	if !e.G.Obj(vec).Tapped {
		t.Error("Vec Townships must stay tapped through its controller's next untap step")
	}
	if n := len(untapDenialGrants(e, 0)); n != 0 {
		t.Fatalf("the grant must be dropped at the end of that untap step, %d left", n)
	}

	// One step later the grant's lifetime is over: Vec untaps again.
	driveToStepAny(t, e, 5, 0, state.StepUpkeep)
	if e.G.Obj(vec).Tapped {
		t.Error("Vec Townships must untap on the untap step after the grant expired")
	}
}

func TestKefnetsLastWordLandsSkipOneUntap(t *testing.T) {
	t.Parallel()
	// Real corpus card: the sorcery's SubAbility Effect grants
	// "Lands you control don't untap during your next untap step"
	// (ValidCard$ Land.YouCtrl, ValidStepTurnToController$ Player.Activator,
	// Duration$ UntilTheEndOfYourNextUntap). The caster's lands must survive
	// the casting turn's OWN cleanup (a plain UntilEOT grant would die there)
	// and skip exactly one untap step.
	e := handEngine(t, corpusCard(t, "Kefnet's Last Word"))
	kef := e.G.Obj(e.G.Zone(state.ZHand, 0)[0]).ID
	bear := onBoard(t, e, 1, "Name:Grizzly Bears\nTypes:Creature Bear\nOracle:x\nPT:2/2\n")
	landA := onBoard(t, e, 0, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	landB := onBoard(t, e, 0, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	e.emit(events.Event{Kind: events.Tap, Obj: landA})
	e.emit(events.Event{Kind: events.Tap, Obj: landB})

	addMana(t, e, 0, "CCUUU")
	submitChoices(t, e, castOptionFor(t, e, kef).Index)
	answerKTarget(t, e, bear)
	passUntilStackEmpty(t, e, 50)

	if n := len(untapDenialGrants(e, 0)); n != 1 {
		t.Fatalf("precondition: want exactly 1 seat-0 Untap replacement, got %d", n)
	}
	for _, g := range untapDenialGrants(e, 0) {
		if g.UntilEOT || g.UntilTurn != 0 || g.Permanent {
			t.Fatalf("grant lifetime fields wrong: %+v", g)
		}
	}

	// Drive through the casting turn's cleanup and seat 1's untap step (the
	// untap step itself is eventless, so the first observable stop is the
	// following upkeep): the grant must survive both -- UntilEOT=false keeps
	// it at cleanup, the controller match keeps it through an opponent's
	// step.
	driveToStepAny(t, e, 2, 1, state.StepUpkeep)
	if n := len(untapDenialGrants(e, 0)); n != 1 {
		t.Fatalf("grant must survive the casting turn's cleanup and the opponent's untap step, have %d", n)
	}

	// Seat 0's next untap step has run by the following upkeep: both lands
	// stayed tapped, and the grant died with that step.
	driveToStepAny(t, e, 3, 0, state.StepUpkeep)
	if !e.G.Obj(landA).Tapped || !e.G.Obj(landB).Tapped {
		t.Error("the caster's lands must not untap during the caster's next untap step")
	}
	// ... and the grant dies with that step.
	driveToStepAny(t, e, 3, 0, state.StepUpkeep)
	if n := len(untapDenialGrants(e, 0)); n != 0 {
		t.Fatalf("the grant must be dropped at the end of that untap step, %d left", n)
	}

	// The NEXT untap step untaps the lands again.
	driveToStepAny(t, e, 5, 0, state.StepUpkeep)
	if e.G.Obj(landA).Tapped || e.G.Obj(landB).Tapped {
		t.Error("the lands must untap on the untap step after the grant expired")
	}
}

// TestReplUntapStepControllerSpecs pins the ValidStepTurnToController$
// grammar of the ReplUntap matcher directly: You reads the untapped card's
// own controller, Player.Activator and Player.IsRemembered read the
// registering effect's controller and captured player set, the untap-step
// gate applies to every spelling, and an unknown spec fails closed.
func TestReplUntapStepControllerSpecs(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	bear := onBoard(t, e, 0, "Name:Grizzly Bears\nTypes:Creature Bear\nOracle:x\nPT:2/2\n")
	src := onBoard(t, e, 0, "Name:Fixture Source\nTypes:Enchantment\nOracle:x\n")
	ev := events.Event{Kind: events.Untap, Obj: bear}

	match := func(params map[string]string, active, controller state.PlayerID, step state.Step,
		remembered []state.ObjID, rememberedPlayers []state.PlayerID) bool {
		t.Helper()
		r := cards.Repl{Event: "Untap", Params: params}
		e.G.Step = step
		e.G.Active = active
		return e.replacementMatchesEffectCreatedBy(r, src, ev, remembered, rememberedPlayers, controller)
	}

	inUntap := state.StepUntap
	main1 := state.StepMain1

	// You: the untapped card's own controller's untap step (the Basalt
	// Monolith shape, unchanged).
	if !match(map[string]string{"ValidStepTurnToController": "You"}, 0, 0, inUntap, nil, nil) {
		t.Error("You must match the untapped card's controller's untap step")
	}
	if match(map[string]string{"ValidStepTurnToController": "You"}, 1, 0, inUntap, nil, nil) {
		t.Error("You must not match another player's untap step")
	}
	if match(map[string]string{"ValidStepTurnToController": "You"}, 0, 0, main1, nil, nil) {
		t.Error("no spelling may match outside the untap step")
	}

	// Player.Activator: the registering effect's controller (you).
	spec := map[string]string{"ValidStepTurnToController": "Player.Activator"}
	if !match(spec, 0, 0, inUntap, nil, nil) {
		t.Error("Player.Activator must match the effect controller's untap step")
	}
	if match(spec, 1, 0, inUntap, nil, nil) {
		t.Error("Player.Activator must not match another player's untap step")
	}

	// Player.IsRemembered: the effect's captured player set.
	spec = map[string]string{"ValidStepTurnToController": "Player.IsRemembered"}
	if !match(spec, 1, 0, inUntap, nil, []state.PlayerID{1}) {
		t.Error("Player.IsRemembered must match a remembered active player")
	}
	if match(spec, 0, 0, inUntap, nil, []state.PlayerID{1}) {
		t.Error("Player.IsRemembered must not match an unremembered active player")
	}

	// Unknown spec fails closed.
	if match(map[string]string{"ValidStepTurnToController": "Player.Opponent"}, 0, 0, inUntap, nil, nil) {
		t.Error("an unknown ValidStepTurnToController$ spec must fail closed")
	}

	// ValidCard$ Card.IsRemembered reads the registration's captured objects.
	spec = map[string]string{"ValidStepTurnToController": "Player.Activator", "ValidCard": "Card.IsRemembered"}
	if !match(spec, 0, 0, inUntap, []state.ObjID{bear}, nil) {
		t.Error("Card.IsRemembered must match a remembered untapped object")
	}
	if match(spec, 0, 0, inUntap, nil, nil) {
		t.Error("Card.IsRemembered must not match an unremembered untapped object")
	}
}
