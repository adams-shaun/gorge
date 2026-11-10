package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// U.S.S. Enterprise-C-shaped: one trigger of each mode, each paying
// `Discard<1/Hand>` and drawing `Discarded$Amount` cards.
const enterpriseEtb = "Name:Enterprise Etb\nManaCost:3 U\nTypes:Creature Human\nPT:1/1\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | OptionalDecider$ You | Execute$ PayHand | TriggerDescription$ When it enters, you may discard your hand. If you do, draw a card for each card discarded this way.\n" +
	"SVar:PayHand:AB$ Draw | Cost$ Discard<1/Hand> | NumCards$ X\n" +
	"SVar:X:Discarded$Amount\n" +
	"Oracle:x\n"

const enterpriseDies = "Name:Enterprise Dies\nManaCost:3 U\nTypes:Creature Human\nPT:1/1\n" +
	"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Secondary$ True | OptionalDecider$ You | Execute$ PayHand | TriggerDescription$ When it dies, you may discard your hand. If you do, draw a card for each card discarded this way.\n" +
	"SVar:PayHand:AB$ Draw | Cost$ Discard<1/Hand> | NumCards$ X\n" +
	"SVar:X:Discarded$Amount\n" +
	"Oracle:x\n"

func countSeat0DrawsSince(e *Engine, mark int) int {
	n := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Draw && ev.Player == 0 {
			n++
		}
	}
	return n
}

// payAndCountDiscardDraws pays the pending window, settles the stack and
// checks the paid hand became graveyard cards and the same number of draws.
func payAndCountDiscardDraws(t *testing.T, e *Engine, d *decision.Decision, want int) {
	t.Helper()
	pay, _ := windowPayDecline(t, d)
	if pay < 0 {
		t.Fatalf("the cost was not offered as payable: %+v", d.Options)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != want {
		t.Fatalf("hand before the pay = %d, want %d", got, want)
	}
	if got := len(e.G.Zone(state.ZLibrary, 0)); got < want {
		t.Fatalf("library = %d, want at least %d so the draws can happen", got, want)
	}
	grave := len(e.G.Zone(state.ZGraveyard, 0))
	mark := len(e.L.Events)
	submitChoices(t, e, pay)
	passUntilStackEmpty(t, e, 20)
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != grave+want {
		t.Fatalf("graveyard = %d, want %d (the whole hand discarded)", got, grave+want)
	}
	if got := countSeat0DrawsSince(e, mark); got != want {
		t.Fatalf("draw events = %d, want %d (one per discarded card)", got, want)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != want {
		t.Fatalf("hand after = %d, want %d", got, want)
	}
}

func TestTriggerCostDiscardHandAmountDrawsThatMany(t *testing.T) {
	t.Parallel()
	junk := card(t, discardCostJunk)
	e := handEngine(t, junk, junk, junk, junk)
	d := etbCostWindow(t, e, enterpriseEtb)
	payAndCountDiscardDraws(t, e, d, 4)
}

func TestTriggerCostDiscardHandAmountDiesTrigger(t *testing.T) {
	t.Parallel()
	junk := card(t, discardCostJunk)
	e := handEngine(t, junk, junk, junk, junk)
	src := e.G.AddObject(card(t, enterpriseDies), 0)
	src.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), src.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the dies trigger-cost window ask, got %+v", d)
	}
	payAndCountDiscardDraws(t, e, d, 4)
}

func TestTriggerCostDiscardCardManaCostReadsThePaidCard(t *testing.T) {
	t.Parallel()
	const script = "Name:Pricey Discarder\nManaCost:3 U\nTypes:Creature Human\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | OptionalDecider$ You | Execute$ PayHand | TriggerDescription$ When it enters, you may discard a card. If you do, draw cards equal to its mana value.\n" +
		"SVar:PayHand:AB$ Draw | Cost$ Discard<1/Card> | NumCards$ X\n" +
		"SVar:X:Discarded$CardManaCost\n" +
		"Oracle:x\n"
	const pricey = "Name:Pricey Fodder\nManaCost:3\nTypes:Artifact\nOracle:x\n"
	junk := card(t, discardCostJunk)
	big := card(t, pricey)
	e := handEngine(t, junk, big, junk)

	d := etbCostWindow(t, e, script)
	pay, _ := windowPayDecline(t, d)
	if pay < 0 {
		t.Fatalf("the cost was not offered as payable: %+v", d.Options)
	}
	submitChoices(t, e, pay)
	pick := e.Pending()
	if pick == nil || pick.Kind != decision.KChoose || len(pick.Options) != 3 || pick.Options[0].Kind != "discard" {
		t.Fatalf("expected the 3-way discard pick, got %+v", pick)
	}
	chosen := -1
	var chosenMV, otherMV int32
	for _, o := range pick.Options {
		ob := e.G.Obj(o.Obj)
		if ob == nil {
			continue
		}
		if ob.Card == big {
			chosen, chosenMV = o.Index, state.ManaValue(e.G, ob)
		} else {
			otherMV = state.ManaValue(e.G, ob)
		}
	}
	if chosenMV != 3 || otherMV != 1 {
		t.Fatalf("mana values chosen=%d other=%d, want 3 and 1 (they must differ)", chosenMV, otherMV)
	}
	if chosen < 0 {
		t.Fatal("the MV-3 card is not among the discard options")
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	mark := len(e.L.Events)
	submitChoices(t, e, chosen)
	passUntilStackEmpty(t, e, 20)
	if got := countSeat0DrawsSince(e, mark); got != 3 {
		t.Fatalf("draw events = %d, want 3 (the discarded card's mana value)", got)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore-1+3 {
		t.Fatalf("hand after = %d, want %d", got, handBefore-1+3)
	}
}
