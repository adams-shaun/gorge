package rules

// The continuous recheck gate (rules/layers.go's continuousGateHolds): a
// Mode$ Continuous static carrying IsPresent$/IsPresent2$ or
// CheckSVar$/SVarCompare$ grants only while its gate holds, re-evaluated once
// per emitted event (the staticContinuous memo's epoch key) so board movement
// turns the grant on and off. Every fixture below embeds the real Forge
// script line(s) of the corpus card it probes (never a committed .txt -- the
// licensing rule), except the synthetic Human the Overseer's gate counts.

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

const angelicOverseerSrc = "Name:Angelic Overseer\nManaCost:3 W W\nTypes:Creature Angel\nPT:5/3\nK:Flying\n" +
	"S:Mode$ Continuous | Affected$ Card.Self | AddKeyword$ Hexproof & Indestructible | IsPresent$ Human.YouCtrl | Description$ As long as you control a Human, CARDNAME has hexproof and indestructible.\n" +
	"Oracle:As long as you control a Human, Angelic Overseer has hexproof and indestructible.\n"

const auriokSteelshaperSrc = "Name:Auriok Steelshaper\nManaCost:1 W\nTypes:Creature Human Soldier\nPT:1/1\n" +
	"S:Mode$ ReduceCost | ValidCard$ Card | ValidSpell$ Activated.Equip | Activator$ You | Amount$ 1 | Description$ Equip costs you pay cost {1} less.\n" +
	"S:Mode$ Continuous | Affected$ Creature.Soldier+YouCtrl,Creature.Knight+YouCtrl | AddPower$ 1 | AddToughness$ 1 | IsPresent$ Card.Self+equipped | Description$ As long as CARDNAME is equipped, each creature you control that's a Soldier or a Knight gets +1/+1.\n" +
	"Oracle:Equip costs you pay cost {1} less.\n"

const kiyomaroSrc = "Name:Kiyomaro, First to Stand\nManaCost:3 W W\nTypes:Legendary Creature Spirit\nPT:*/*\n" +
	"S:Mode$ Continuous | Affected$ Card.Self | AddKeyword$ Vigilance | CheckSVar$ X | SVarCompare$ GE4 | Description$ As long as you have four or more cards in hand, NICKNAME has vigilance.\n" +
	"SVar:X:Count$ValidHand Card.YouOwn\n" +
	"Oracle:As long as you have four or more cards in hand, Kiyomaro has vigilance.\n"

// TestContinuousIsPresentGateTurnsTheGrantOnAndOff drives Angelic Overseer's
// real IsPresent$ gate -- Human.YouCtrl -- across a full off/on/off cycle:
// the hexproof/indestructible grant is absent with no Human on the
// battlefield, present once one enters, and absent again the moment it
// leaves, all on the same engine with no re-registration.
func TestContinuousIsPresentGateTurnsTheGrantOnAndOff(t *testing.T) {
	e, _, overseer := newFixtureDeck(t, 61, angelicOverseerSrc, "Name:Human\nManaCost:W\nTypes:Creature Human Soldier\nPT:1/1\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: overseer, From: state.ZHand, To: state.ZBattlefield})
	if e.HasKeyword(overseer, "Hexproof") || e.HasKeyword(overseer, "Indestructible") {
		t.Fatalf("grant applied with no Human on the battlefield (hexproof %v indestructible %v)",
			e.HasKeyword(overseer, "Hexproof"), e.HasKeyword(overseer, "Indestructible"))
	}
	human := putCreature(t, e, 0, "Name:Human\nManaCost:W\nTypes:Creature Human Soldier\nPT:1/1\nOracle:x\n")
	if !e.HasKeyword(overseer, "Hexproof") || !e.HasKeyword(overseer, "Indestructible") {
		t.Fatalf("grant absent with a Human on the battlefield (hexproof %v indestructible %v)",
			e.HasKeyword(overseer, "Hexproof"), e.HasKeyword(overseer, "Indestructible"))
	}
	// The Human leaves: the gate re-evaluates on the next emitted event and
	// the grant turns back off -- a static recheck, not a registration.
	e.emit(events.Event{Kind: events.MoveZone, Obj: human, From: state.ZBattlefield, To: state.ZGraveyard})
	if e.HasKeyword(overseer, "Hexproof") || e.HasKeyword(overseer, "Indestructible") {
		t.Fatalf("grant still applied after the Human left (hexproof %v indestructible %v)",
			e.HasKeyword(overseer, "Hexproof"), e.HasKeyword(overseer, "Indestructible"))
	}
	// An OPPONENT's Human never satisfies YouCtrl. (onBoard is eventless, so
	// the Tap emit below is the epoch mover that forces the statics re-scan.)
	oh := onBoard(t, e, 1, "Name:Other Human\nManaCost:W\nTypes:Creature Human Soldier\nPT:1/1\nOracle:x\n")
	e.emit(events.Event{Kind: events.Tap, Obj: oh})
	if e.HasKeyword(overseer, "Hexproof") || e.HasKeyword(overseer, "Indestructible") {
		t.Fatal("grant applied for an opponent's Human -- the YouCtrl qualifier was ignored")
	}
}

// TestContinuousIsPresentGateEquippedPredicate drives Auriok Steelshaper's
// real IsPresent$ Card.Self+equipped through the genuine equip flow: the
// +1/+1 to Soldiers and Knights is absent while nothing is attached, present
// once the Equipment attaches, and absent again after the Equipment leaves.
func TestContinuousIsPresentGateEquippedPredicate(t *testing.T) {
	sword := "Name:Gate Sword\nManaCost:3\nTypes:Artifact Equipment\nK:Equip:2\nOracle:x\n"
	e, cfg, sw := newFixtureDeck(t, 61, sword, auriokSteelshaperSrc)
	auriok := putCreature(t, e, 0, auriokSteelshaperSrc)
	if got := e.Power(auriok); got != 1 {
		t.Fatalf("Auriok power = %d, want 1 (unequipped: the IsPresent$ gate withholds the pump)", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: sw, From: state.ZHand, To: state.ZBattlefield})
	addMana(t, e, 0, "CC")
	e.Advance()
	opt := abilityOption(t, e, sw, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || len(d.Options) != 1 || d.Options[0].Obj != auriok {
		t.Fatalf("equip target %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(sw).AttachedTo != auriok {
		t.Fatalf("sword attached to %d, want %d", e.G.Obj(sw).AttachedTo, auriok)
	}
	if got := e.Power(auriok); got != 2 {
		t.Fatalf("equipped Auriok power = %d, want 2 (1 base + 1 pump; Auriok is itself a Soldier)", got)
	}
	// The Equipment leaves: the gate fails again and the pump is gone.
	e.emit(events.Event{Kind: events.MoveZone, Obj: sw, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := e.Power(auriok); got != 1 {
		t.Fatalf("Auriok power after the Equipment left = %d, want 1 (gate re-checked)", got)
	}
	replayCheck(t, e, cfg)
}

// TestContinuousCheckSVarGateTurnsTheGrantOnAndOff drives Kiyomaro, First to
// Stand's real CheckSVar$/SVarCompare$ gate -- SVar X = Count$ValidHand
// Card.YouOwn, SVarCompare$ GE4: vigilance is present with four or more
// cards in hand and absent below the threshold, re-checked as the hand
// shrinks.
func TestContinuousCheckSVarGateTurnsTheGrantOnAndOff(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	kiyoCard, ok := reg.Lookup("Kiyomaro, First to Stand")
	if !ok {
		t.Fatal("corpus has no Kiyomaro, First to Stand")
	}
	e := layerEngine(t)
	kiyo := onBoardCard(t, e, 0, kiyoCard)
	hand := e.G.Zone(state.ZHand, 0)
	if len(hand) < 4 {
		t.Fatalf("fixture opening hand = %d cards, want at least 4 for the GE4 gate", len(hand))
	}
	if !e.HasKeyword(kiyo, "Vigilance") {
		t.Fatalf("vigilance absent with %d cards in hand (the GE4 gate should hold)", len(hand))
	}
	// Shrink the hand below four: the gate re-evaluates and the grant turns
	// off -- the same engine, no re-registration.
	for _, id := range append([]state.ObjID(nil), hand[:len(hand)-3]...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != 3 {
		t.Fatalf("hand after the moves = %d cards, want 3", got)
	}
	if e.HasKeyword(kiyo, "Vigilance") {
		t.Fatal("vigilance still applied with 3 cards in hand -- the CheckSVar$ gate did not re-check")
	}
}
