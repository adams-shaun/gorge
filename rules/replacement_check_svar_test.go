// The K:.../R:... replacement CheckSVar$ gates whose faces define a real
// SVar:X body, pinned on real corpus carriers in both directions after the
// replacementCheckValue reorder: the face's SVar:X body is evaluated through
// the count machinery BEFORE the announced-X fallback, so the switch's
// Count$Party / Count$Valid Permanent.YouCtrl$Colors cases and the
// PlayerCountOpponents$/Count$ValidHand heads wake, while a body that reads
// the announced X (banefire's Count$xPaid) lands on the same value either
// way. The event-name side of the same unit: "DrawCards" (Forge's spelling
// of the draw replacement event; Quantum Riddler) matches the engine's
// events.Draw, and its ReplaceCount$Number/Plus.1 body reads the per-card
// draw's match-view amount of 1.
//
// The entries drive the real replacement machinery over real corpus cards;
// authored filler permanents (party members, colour bearers, draw fodder)
// are inline fixtures, never committed Forge text.

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// addFillerPermanent adds an authored permanent to a seat's battlefield with
// an optional Types/Colors line, logged with a MoveZone so trigger machinery
// sees a real entry.
func addFillerPermanent(t *testing.T, e *Engine, p state.PlayerID, face string) state.ObjID {
	t.Helper()
	o := e.G.AddObject(card(t, face+"\nOracle:x\n"), p)
	o.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, p, append(e.G.Zone(state.ZBattlefield, p), o.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZHand, To: state.ZBattlefield})
	return o.ID
}

func drainQueuedTriggers(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 50; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		if len(e.G.Stack) == 0 {
			return
		}
		e.resolveTop()
	}
	t.Fatalf("trigger drain did not settle: %d pending, %d on the stack", len(e.pendingTriggers), len(e.G.Stack))
}

// Walking Dream's `R:Event$ Untap | ... | CheckSVar$ X | SVarCompare$ GE2`
// with `SVar:X:PlayerCountOpponents$HighestValid Creature.YouCtrl`: at two
// or more opposing creatures the untap is prevented; below two it untaps.
func TestWalkingDreamStaysTappedAtTwoOpposingCreatures(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Walking Dream"))
	dream := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: dream, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Tap, Obj: dream})
	addFillerPermanent(t, e, 1, "Name:Bean\nManaCost:G\nTypes:Creature\nPT:1/1")
	addFillerPermanent(t, e, 1, "Name:Pea\nManaCost:G\nTypes:Creature\nPT:1/1")
	e.G.Step = state.StepUntap
	e.G.Active = 0
	e.emit(events.Event{Kind: events.Untap, Obj: dream})
	if !e.G.Obj(dream).Tapped {
		t.Fatal("Walking Dream untapped with two opposing creatures: the CheckSVar$ X gate (GE2 over PlayerCountOpponents$HighestValid) must block the untap")
	}
}

func TestWalkingDreamUntapsBelowTwoOpposingCreatures(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Walking Dream"))
	dream := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: dream, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Tap, Obj: dream})
	addFillerPermanent(t, e, 1, "Name:Bean\nManaCost:G\nTypes:Creature\nPT:1/1")
	e.G.Step = state.StepUntap
	e.G.Active = 0
	e.emit(events.Event{Kind: events.Untap, Obj: dream})
	if e.G.Obj(dream).Tapped {
		t.Fatal("Walking Dream stayed tapped with one opposing creature: the GE2 gate must not hold at 1")
	}
}

// Multiclass Baldric's `R:Event$ DamageDone | Prevent$ True |
// ValidTarget$ Creature.EquippedBy | CheckSVar$ X | SVarCompare$ EQ4` with
// `SVar:X:Count$Party` (the switch case the reorder wakes): a full party
// (one of each of the four roles) prevents all damage to the equipped
// creature; an incomplete party does not.
func TestMulticlassBaldricPreventsDamageAtAFullParty(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Multiclass Baldric"))
	baldric := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: baldric, From: state.ZHand, To: state.ZBattlefield})
	bearer := addFillerPermanent(t, e, 0, "Name:Bearer\nManaCost:G\nTypes:Creature Warrior\nPT:2/2")
	e.emit(events.Event{Kind: events.Attach, Obj: baldric, IDs: []state.ObjID{bearer}})
	for _, role := range []string{"Cleric", "Rogue", "Wizard"} {
		addFillerPermanent(t, e, 0, "Name:"+role+"\nManaCost:G\nTypes:Creature "+role+"\nPT:1/1")
	}
	e.emit(events.Event{Kind: events.Damage, Obj: bearer, Amount: 3})
	if got := e.G.Obj(bearer).Damage; got != 0 {
		t.Fatalf("equipped bearer marked %d damage at a full party, want 0 (the Count$Party EQ4 gate must prevent)", got)
	}
}

func TestMulticlassBaldricDamageLandsWithoutTheFullParty(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Multiclass Baldric"))
	baldric := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: baldric, From: state.ZHand, To: state.ZBattlefield})
	bearer := addFillerPermanent(t, e, 0, "Name:Bearer\nManaCost:G\nTypes:Creature Warrior\nPT:2/2")
	e.emit(events.Event{Kind: events.Attach, Obj: baldric, IDs: []state.ObjID{bearer}})
	e.emit(events.Event{Kind: events.Damage, Obj: bearer, Amount: 3})
	if got := e.G.Obj(bearer).Damage; got != 3 {
		t.Fatalf("equipped bearer marked %d damage with an incomplete party, want 3 (EQ4 must not hold at 2 roles)", got)
	}
}

// Quantum Riddler's `R:Event$ DrawCards | ... | CheckSVar$ X |
// SVarCompare$ LE1` with `SVar:X:Count$ValidHand Card.YouOwn` and the
// ReplaceWith$ DrawPlusOne body (NumCards$ ReplaceCount$Number/Plus.1): at
// one or fewer cards in hand a draw draws one extra; past that it is
// ordinary.
func TestQuantumRiddlerDrawsAnExtraCardAtOneCardInHand(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Quantum Riddler"))
	qr := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: qr, From: state.ZHand, To: state.ZBattlefield})
	drainQueuedTriggers(t, e)
	// The ETB draw happened while the hand was EMPTY (0 <= 1), so it took
	// the +1 itself; move one card back (with its real zone field -- the
	// zone-slice shuffle alone leaves o.Zone stale and the tested draw's
	// Apply would no-op the library removal) so the tested draw reads
	// exactly 1 card in hand.
	hand := e.G.Zone(state.ZHand, 0)
	e.G.Obj(hand[0]).Zone = state.ZLibrary
	e.G.SetZone(state.ZHand, 0, hand[1:])
	e.G.SetZone(state.ZLibrary, 0, append([]state.ObjID{hand[0]}, e.G.Zone(state.ZLibrary, 0)...))
	lib := e.G.Zone(state.ZLibrary, 0)
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: lib[0], From: state.ZLibrary, To: state.ZHand, Secret: true})
	if got := len(e.G.Zone(state.ZHand, 0)); got != 3 {
		t.Fatalf("hand went to %d after a draw at 1 card, want 3 (LE1 holds: the DrawPlusOne body draws 2)", got)
	}
}

func TestQuantumRiddlerOrdinaryDrawPastOneCardInHand(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Quantum Riddler"))
	qr := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: qr, From: state.ZHand, To: state.ZBattlefield})
	drainQueuedTriggers(t, e)
	for _, name := range []string{"Filler A", "Filler B"} {
		o := e.G.AddObject(card(t, "Name:"+name+"\nTypes:Creature\nPT:1/1\n"), 0)
		o.Zone = state.ZHand
		e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), o.ID))
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != 4 {
		t.Fatalf("fixture hand before the tested draw = %d, want 4 (the ETB draw's own +1, plus two fillers)", got)
	}
	lib := e.G.Zone(state.ZLibrary, 0)
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: lib[0], From: state.ZLibrary, To: state.ZHand, Secret: true})
	if got := len(e.G.Zone(state.ZHand, 0)); got != 5 {
		t.Fatalf("hand went to %d after a draw at 4 cards, want 5 (LE1 must not hold: an ordinary draw)", got)
	}
}

// Spirit of Resistance's `R:Event$ DamageDone | Prevent$ True |
// ValidTarget$ You | CheckSVar$ X | SVarCompare$ EQ5` with
// `SVar:X:Count$Valid Permanent.YouCtrl$Colors` (the switch case the reorder
// wakes): all five colours on your battlefield prevent all damage to you;
// four do not.
func TestSpiritOfResistancePreventsAtAllFiveColours(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Spirit of Resistance"))
	spirit := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: spirit, From: state.ZHand, To: state.ZBattlefield})
	for _, c := range []string{"U", "B", "R", "G", "W"} {
		addFillerPermanent(t, e, 0, "Name:Bearer"+c+"\nManaCost:"+c+"\nTypes:Creature\nPT:1/1")
	}
	before := e.G.Players[0].Life
	e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 3})
	if got := e.G.Players[0].Life; got != before {
		t.Fatalf("seat 0 at %d life after 3 damage at five colours, want %d (the five-colour gate must prevent)", got, before)
	}
}

func TestSpiritOfResistanceDamageLandsAtFourColours(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Spirit of Resistance"))
	spirit := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: spirit, From: state.ZHand, To: state.ZBattlefield})
	for _, c := range []string{"U", "B", "R"} {
		addFillerPermanent(t, e, 0, "Name:Bearer"+c+"\nManaCost:"+c+"\nTypes:Creature\nPT:1/1")
	}
	before := e.G.Players[0].Life
	e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 3})
	if got := e.G.Players[0].Life; got != before-3 {
		t.Fatalf("seat 0 at %d life after 3 damage at four colours, want %d (EQ5 must not hold)", got, before-3)
	}
}

// Banefire's `R:Event$ Counter | ValidCard$ Card.Self | CheckSVar$ X |
// SVarCompare$ GE5` with `SVar:X:Count$xPaid`: the reordered gate must land
// on the same announced X it read through the shortcut before -- a body that
// READS the paid X is unaffected by the reorder.
func TestBanefireCounterGateStillReadsThePaidX(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Banefire"))
	id := e.G.Zone(state.ZHand, 0)[0]
	o := e.G.Obj(id)
	o.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 0, append(e.G.Zone(state.ZStack, 0), id))
	o.X = 5
	if got := e.replacementCheckValue(id, "X"); got != 5 {
		t.Fatalf("banefire's CheckSVar$ X gate read %d, want the paid X's 5 (Count$xPaid unchanged by the reorder)", got)
	}
}
