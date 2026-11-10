package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCantPreventDamageMindskinnerStillMills pins CR 615.12 for the
// "prevent that damage and <rider>" shape: under can't-prevent the damage is
// dealt AND the rider (the mill) still happens.
func TestCantPreventDamageMindskinnerStillMills(t *testing.T) {
	t.Parallel()
	reg := sharedCorpus(t)
	run := func(t *testing.T, punk bool) (life int32, milled int) {
		e := newSeats(t, 2)
		e.pending = nil
		if punk {
			onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Spider-Punk"))
		}
		onBoardCard(t, e, 0, mustCorpusCard(t, reg, "The Mindskinner"))
		src := onBoard(t, e, 0, "Name:Src\nTypes:Creature\nPT:2/2\nOracle:x\n")
		lib := len(e.G.Zone(state.ZLibrary, 1))
		if lib < 2 {
			t.Fatalf("precondition: seat 1 library has %d cards, want >= 2", lib)
		}
		if got := e.cantPreventDamage(src, 0); got != punk {
			t.Fatalf("precondition: cantPreventDamage = %v, want %v", got, punk)
		}
		e.damaging = src
		e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
		e.damaging = 0
		return int32(e.G.Players[1].Life), lib - len(e.G.Zone(state.ZLibrary, 1))
	}
	t.Run("under Spider-Punk", func(t *testing.T) {
		if life, milled := run(t, true); life != 18 || milled != 2 {
			t.Fatalf("life=%d milled=%d, want 18 and 2: damage dealt and rider kept", life, milled)
		}
	})
	t.Run("control without Spider-Punk", func(t *testing.T) {
		if life, milled := run(t, false); life != 20 || milled != 2 {
			t.Fatalf("life=%d milled=%d, want 20 and 2: damage prevented, rider kept", life, milled)
		}
	})
}

// TestCantPreventDamageIgnoresAbilitylessSource: Spider-Punk under Humility
// has no abilities (CR 613.1f), so its static no longer stops prevention.
// Humility strips every creature, so a protected creature would lose its
// protection too; Thunderstaff (an artifact, untouched by Humility) is the
// prevention that must work again.
func TestCantPreventDamageIgnoresAbilitylessSource(t *testing.T) {
	t.Parallel()
	reg := sharedCorpus(t)
	e := newSeats(t, 2)
	e.pending = nil
	punk := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Spider-Punk"))
	onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Humility"))
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Thunderstaff"))
	source := onBoard(t, e, 1, "Name:Attacker\nTypes:Creature\nPT:2/2\nOracle:x\n")
	if !e.printedAbilitiesGone(e.G.Obj(punk)) {
		t.Fatal("precondition: Humility did not strip Spider-Punk's abilities")
	}
	e.damaging, e.combatDamaging = source, true
	if e.cantPreventDamage(source, 0) {
		t.Fatal("precondition: damage is still unpreventable with Spider-Punk ability-less")
	}
	e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 2})
	e.damaging, e.combatDamaging = 0, false
	if got := e.G.Players[0].Life; got != 19 {
		t.Fatalf("life = %d, want 19: Thunderstaff prevents 1 once Spider-Punk has no abilities", got)
	}
}

// TestFogUnderCantPreventDamageDealsCombatDamage: CR 615.12, the resolved Fog
// prevents nothing the can't-prevent effect covers.
func TestFogUnderCantPreventDamageDealsCombatDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bear := card(t, bearSrc)
	e := corpusEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Fog"), lookup(t, reg, "Spider-Punk"), bear},
		[]*cards.Card{})
	moveByName(t, e, 0, "Bear", state.ZBattlefield)
	moveByName(t, e, 0, "Spider-Punk", state.ZBattlefield)
	for driveToTurn(t, e, 3, 0) {
	}
	moveByName(t, e, 0, "Fog", state.ZHand)
	addMana(t, e, 0, "G")
	fogID := findInHand(e, 0, "Fog")
	if fogID == 0 {
		t.Fatal("Fog not in seat 0's hand on turn 3")
	}
	castNamed(t, e, "Fog")
	passUntilStackEmpty(t, e, 30)
	if !e.fogActive() {
		t.Fatal("precondition: Fog resolved but no PreventCombatDamage effect is active")
	}
	e.combatDamaging = true
	cant := e.cantPreventDamage(firstCreature(t, e, 0), 0)
	e.combatDamaging = false
	if !cant {
		t.Fatal("precondition: Spider-Punk does not make combat damage unpreventable")
	}
	life := e.G.Players[1].Life
	dmgBefore := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.Damage && ev.Amount > 0 && ev.Obj == 0
	})
	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	atk := firstCreature(t, e, 0)
	power := e.combatDamageAmount(atk)
	if power <= 0 {
		t.Fatalf("precondition: attacker power %d", power)
	}
	submitAttackersOnly(t, e, atk)
	passAll(t, e, 300)
	if dmg := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.Damage && ev.Amount > 0 && ev.Obj == 0
	}); dmg <= dmgBefore {
		t.Fatal("no combat Damage event to the player under Fog + Spider-Punk")
	}
	if got := int32(life - e.G.Players[1].Life); got != int32(power) {
		t.Fatalf("seat 1 lost %d life, want %d", got, power)
	}
}
