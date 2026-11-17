package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 701.35a: until the detaining player's next turn, a detained permanent
// can't attack or block and its activated abilities can't be activated.
const detainVictimSrc = "Name:Detainee\nManaCost:1\nTypes:Creature Human\nPT:2/2\n" +
	"A:AB$ GainLife | Cost$ 1 | Defined$ You | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\nOracle:x\n"

const detainAttackerSrc = "Name:Detain Aggressor\nManaCost:1\nTypes:Creature Soldier\nPT:1/1\nOracle:x\n"

// hasAttackOffer reports whether the active player's attack-offer list (the
// exact enumeration askAttackers turns into KAttackers options) names id.
func hasAttackOffer(e *Engine, id state.ObjID) bool {
	for _, of := range e.attackOffers() {
		if of.id == id {
			return true
		}
	}
	return false
}

func TestDetainStopsAttackBlockAndAbilitiesUntilYourNextTurn(t *testing.T) {
	t.Parallel()
	victimCard := card(t, detainVictimSrc)
	attackerCard := card(t, detainAttackerSrc)
	e, cfg := cbaEngine(t, 7718,
		[]*cards.Card{corpusAlternativeCard(t, "Azorius Arrester"), attackerCard},
		[]*cards.Card{victimCard})

	// Establish the board first: the victim (seat 1) and the seat-0 attacker
	// the block check will ask about. Both enter through logged MoveZone
	// events so replayCheck can rebuild them.
	victim := moveByName(t, e, 1, "Detainee", state.ZBattlefield)
	attacker := moveByName(t, e, 0, "Detain Aggressor", state.ZBattlefield)
	vo := e.G.Obj(victim)
	if vo == nil || vo.Zone != state.ZBattlefield {
		t.Fatalf("precondition: victim zone = %+v, want battlefield", vo)
	}
	if vo.Face() == nil || len(vo.Face().Abilities) == 0 {
		t.Fatal("precondition: victim has no activated ability to restrict")
	}

	// Azorius Arrester enters and its real ETB trigger detains the victim.
	arrester := moveByName(t, e, 0, "Azorius Arrester", state.ZBattlefield)
	if o := e.G.Obj(arrester); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Azorius Arrester zone = %+v, want battlefield", e.G.Obj(arrester))
	}
	e.pending = nil
	e.priorityRound()
	d := passUntilNonPriority(t, e, 60)
	if d.Kind != decision.KTarget {
		t.Fatalf("Azorius Arrester's detain trigger asked %+v, want a target decision", d)
	}
	target := -1
	for _, o := range d.Options {
		if o.Obj == victim {
			target = o.Index
		}
	}
	if target < 0 {
		t.Fatalf("victim absent from the detain targets: %+v", d.Options)
	}
	submitChoices(t, e, target)
	passUntilStackEmpty(t, e, 60)

	// The victim is now detained: during the opponent's turn it can't attack
	// or block and its activated ability is not offered.
	drainThrough(t, e, e.G.Turn+1, 1, state.StepMain1)
	addMana(t, e, 1, "1")
	if hasAttackOffer(e, victim) {
		t.Fatalf("detained creature is offered as an attacker: %+v", e.attackOffers())
	}
	if ab := cbaAbility(t, e, victim); !e.abilityRestricted(1, victim, ab) {
		t.Fatal("detained creature's activated ability is not restricted")
	}
	if hasAbilityOptionFor(e.legalActions(1), victim) {
		t.Fatalf("detained creature's ability was offered: %+v", e.legalActions(1))
	}
	// CanBlock is the same legality gate askBlockers filters its options with
	// (combat_shims_test.go forwards to combat.CanBlock). The declaration is
	// logged, so replay still sees it.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{attacker}})
	if e.canBlock(victim, attacker) {
		t.Fatal("detained creature can block during the opponent's turn")
	}

	// At the start of the detainer's next turn all three come back: the
	// duration is UntilYourNextTurn, not the end of the current turn.
	drainThrough(t, e, e.G.Turn+1, 0, state.StepMain1)
	// AttackBlocked is the gate attackOffers filters pairs with; the victim's
	// controller is not the active player here, so the pair read is direct.
	if combat.AttackBlocked(asBoard(e), victim, 0, 0) {
		t.Fatal("detained creature still can't attack after the detainer's next turn began")
	}
	addMana(t, e, 1, "1")
	if ab := cbaAbility(t, e, victim); e.abilityRestricted(1, victim, ab) {
		t.Fatal("detained creature's activated ability is still restricted after the next turn began")
	}
	if !hasAbilityOptionFor(e.legalActions(1), victim) {
		t.Fatalf("detained creature's ability was not offered after the next turn began: %+v", e.legalActions(1))
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{attacker}})
	if !e.canBlock(victim, attacker) {
		t.Fatal("detained creature still can't block after the detainer's next turn began")
	}
	replayCheck(t, e, cfg)
}

func TestDetainCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Azorius Arrester", "Inaction Injunction", "Lavinia of the Tenth"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}
