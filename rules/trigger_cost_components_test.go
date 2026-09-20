package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The trigcost2 pins: the triggered-cost window's pay arm settles the
// Sac/Exile/Discard cost components for real (the walk the mandatory arm
// already runs), so a "you may pay <cost>. If you do, ..." trigger whose
// cost carries a choice-bearing component is genuinely payable instead of
// either silently skipping the component on a Draw-bearing cost or offering
// decline-only on a component-only one. All fixtures are inline synthetic
// scripts (no Forge text committed); the cost specs are the real carriers'
// verbatim shapes. None of the named real carriers (Ambergris Citadel Agent,
// Springbloom Druid, Sanctum of Ugin) is in any legacy golden deck.

// discardCostLooter is the Ambergris-shape fixture: an ETB trigger whose
// body carries `Cost$ Discard<1/Hand> Draw<2/You>` verbatim (the SILENT-SKIP
// carrier's exact cost) behind a one-card body draw.
const discardCostLooter = "Name:Ambergris Looter\nManaCost:2 U\nTypes:Creature Human Wizard\nPT:1/1\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCost | TriggerDescription$ When CARDNAME enters, you may discard your hand. If you do, draw two cards.\n" +
	"SVar:TrigCost:AB$ Draw | Cost$ Discard<1/Hand> Draw<2/You> | NumCards$ 1\n" +
	"Oracle:x\n"

// etbCostWindow places src in hand with extra hand cards, emits its ETB move
// (firing the ChangesZone trigger) and returns the pending trigger-cost
// window ask.
func etbCostWindow(t *testing.T, e *Engine, script string, handExtra ...*cards.Card) *decision.Decision {
	t.Helper()
	for _, c := range handExtra {
		o := e.G.AddObject(c, 0)
		o.Zone = state.ZHand
		e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), o.ID))
	}
	src := e.G.AddObject(card(t, script), 0)
	src.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), src.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the trigger-cost window ask, got %+v", d)
	}
	return d
}

// windowPayDecline returns the pay and decline option indices of a pending
// trigger-cost window ask.
func windowPayDecline(t *testing.T, d *decision.Decision) (pay, decline int) {
	t.Helper()
	pay, decline = -1, -1
	for _, o := range d.Options {
		switch o.Kind {
		case "trigger_cost_pay":
			pay = o.Index
		case "trigger_cost_decline":
			decline = o.Index
		}
	}
	if decline < 0 {
		t.Fatalf("no trigger-cost window options: %+v", d.Options)
	}
	return pay, decline
}

// TestAmbergrisStyleDiscardDrawCostPays pins the silent-skip class end to
// end: paying the `Cost$ Discard<1/Hand> Draw<2/You>` body drains the whole
// hand (the Hand spec pays every hand card), emits the cost-discard events
// BEFORE any draw, draws the cost's two cards plus the body's one, and the
// body runs. The decline moves nothing.
func TestAmbergrisStyleDiscardDrawCostPays(t *testing.T) {
	e := handEngine(t, card(t, discardCostJunk), card(t, discardCostJunk))

	d := etbCostWindow(t, e, discardCostLooter)
	if got := len(e.G.Zone(state.ZHand, 0)); got != 2 {
		t.Fatalf("fixture hand = %d cards, want the 2 junk cards", got)
	}
	pay, _ := windowPayDecline(t, d)
	if pay < 0 {
		t.Fatalf("the Discard<1/Hand> Draw<2/You> cost was not offered as payable: %+v", d.Options)
	}

	libBefore := len(e.G.Zone(state.ZLibrary, 0))
	mark := len(e.L.Events)
	submitChoices(t, e, pay)
	passUntilStackEmpty(t, e, 20)

	// 2 discarded, then 3 drawn (the cost's two plus the body's one).
	if got := len(e.G.Zone(state.ZHand, 0)); got != 3 {
		t.Fatalf("hand after pay = %d, want 3 (the drawn cards; the whole hand was discarded first)", got)
	}
	for _, z := range e.G.Zone(state.ZGraveyard, 0) {
		if o := e.G.Obj(z); o != nil && o.Face() != nil && o.Face().Name == "Discard Fodder" && o.Zone != state.ZGraveyard {
			t.Fatalf("a junk hand card did not reach the graveyard")
		}
	}
	discards, draws := 0, 0
	lastDiscard, firstDraw := -1, len(e.L.Events)
	for i, ev := range e.L.Events[mark:] {
		if events.IsDiscardCost(ev) {
			discards++
			lastDiscard = i
		}
		if ev.Kind == events.Draw && ev.Player == 0 {
			draws++
			if i < firstDraw {
				firstDraw = i
			}
		}
	}
	if discards != 2 {
		t.Fatalf("discard-cost events = %d, want 2 (the whole hand)", discards)
	}
	if draws != 3 {
		t.Fatalf("draw events = %d, want 3 (the cost's two plus the body's one)", draws)
	}
	if lastDiscard >= firstDraw {
		t.Fatal("a draw event was recorded before the cost's discards were paid")
	}
	if got := libBefore - len(e.G.Zone(state.ZLibrary, 0)); got != 3 {
		t.Fatalf("library shrank by %d cards, want 3", got)
	}
	for _, z := range e.G.Zone(state.ZGraveyard, 0) {
		if o := e.G.Obj(z); o != nil && o.Face() != nil && o.Face().Name == "Ambergris Looter" {
			t.Fatalf("the source was discarded by its own cost; only the hand pays")
		}
	}
}

// TestAmbergrisStyleDiscardDrawCostDeclineMovesNothing pins the decline arm:
// declining the pay election discards nothing and draws nothing.
func TestAmbergrisStyleDiscardDrawCostDeclineMovesNothing(t *testing.T) {
	e := handEngine(t, card(t, discardCostJunk), card(t, discardCostJunk))

	d := etbCostWindow(t, e, discardCostLooter)
	_, decline := windowPayDecline(t, d)
	mark := len(e.L.Events)
	submitChoices(t, e, decline)
	passUntilStackEmpty(t, e, 20)

	if got := len(e.G.Zone(state.ZHand, 0)); got != 2 {
		t.Fatalf("hand after decline = %d, want the untouched 2", got)
	}
	for _, ev := range e.L.Events[mark:] {
		if events.IsDiscard(ev) || ev.Kind == events.Draw {
			t.Fatalf("a declined cost still moved a card: %+v", ev)
		}
	}
}

// TestTriggerCostEmptyHandDeclinesNoHalfPayment pins the gate's empty-hand
// half: with an empty hand the Hand-spec discard cannot pay, so the window
// offers DECLINE ONLY -- the cost's draw half is never paid on an unpaid
// component.
func TestTriggerCostEmptyHandDeclinesNoHalfPayment(t *testing.T) {
	e := handEngine(t)

	d := etbCostWindow(t, e, discardCostLooter)
	pay, decline := windowPayDecline(t, d)
	if pay >= 0 {
		t.Fatalf("an empty-hand Discard<1/Hand> cost was offered as payable: %+v", d.Options)
	}
	mark := len(e.L.Events)
	submitChoices(t, e, decline)
	passUntilStackEmpty(t, e, 20)

	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Draw || events.IsDiscard(ev) {
			t.Fatalf("the unpayable cost still moved a card: %+v", ev)
		}
	}
}

// TestTriggerCostEmptyHandOrdinarySpecDeclines is the ordinary-spec twin: a
// `Cost$ Discard<1/Card>` body with no hand card is decline-only too.
func TestTriggerCostEmptyHandOrdinarySpecDeclines(t *testing.T) {
	const script = "Name:Ordinary Discarder\nManaCost:1 B\nTypes:Creature Zombie\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCost | TriggerDescription$ When CARDNAME enters, you may discard a card. If you do, draw a card.\n" +
		"SVar:TrigCost:AB$ Draw | Cost$ Discard<1/Card> | NumCards$ 1\n" +
		"Oracle:x\n"
	e := handEngine(t)

	d := etbCostWindow(t, e, script)
	pay, decline := windowPayDecline(t, d)
	if pay >= 0 {
		t.Fatalf("an empty-hand Discard<1/Card> cost was offered as payable: %+v", d.Options)
	}
	mark := len(e.L.Events)
	submitChoices(t, e, decline)
	passUntilStackEmpty(t, e, 20)
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Draw {
			t.Fatalf("the unpayable cost still drew: %+v", ev)
		}
	}
}

// TestTriggerCostDiscardChoiceAsksAndSettles pins the multi-candidate
// Discard component: with more eligible cards than the count the window
// poses a real exact-N KChoose (the chooseTriggeredMandatory walk), the
// picked cards go to the graveyard as cost discards, the unpicked card
// stays, and the body runs.
func TestTriggerCostDiscardChoiceAsksAndSettles(t *testing.T) {
	const script = "Name:Choosy Discarder\nManaCost:1 B\nTypes:Creature Zombie\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCost | TriggerDescription$ When CARDNAME enters, you may discard two cards. If you do, draw a card.\n" +
		"SVar:TrigCost:AB$ Draw | Cost$ Discard<2/Card> | NumCards$ 1\n" +
		"Oracle:x\n"
	junk := card(t, discardCostJunk)
	e := handEngine(t, junk, junk, junk)

	d := etbCostWindow(t, e, script)
	pay, _ := windowPayDecline(t, d)
	submitChoices(t, e, pay)

	pick := e.Pending()
	if pick == nil || pick.Kind != decision.KChoose || pick.Min != 2 || pick.Max != 2 {
		t.Fatalf("expected the exact-2 discard pick, got %+v", pick)
	}
	if len(pick.Options) != 3 || pick.Options[0].Kind != "discard" {
		t.Fatalf("discard pick options = %+v, want 3 discard-kind options", pick.Options)
	}
	staying := pick.Options[2].Obj
	mark := len(e.L.Events)
	submitChoices(t, e, pick.Options[0].Index, pick.Options[1].Index)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(staying); o == nil || o.Zone != state.ZHand {
		t.Fatalf("the unpicked card must stay in hand, got %v", o)
	}
	// 2 discarded, then the body's one drawn.
	if got := len(e.G.Zone(state.ZHand, 0)); got != 2 {
		t.Fatalf("hand after pay = %d, want 2 (the unpicked card plus the body's draw)", got)
	}
	discards := 0
	for _, ev := range e.L.Events[mark:] {
		if events.IsDiscardCost(ev) {
			discards++
		}
	}
	if discards != 2 {
		t.Fatalf("discard-cost events = %d, want 2", discards)
	}
}

// TestTriggerCostSacComponentPays pins the Sac-component loot shape
// (Springbloom Druid's `Cost$ Sac<1/Land>`): with exactly one eligible land
// the settle needs no ask, the land is sacrificed by a real events.Sacrifice
// before the body runs, and the body draws.
func TestTriggerCostSacComponentPays(t *testing.T) {
	const script = "Name:Ramp Elf\nManaCost:1 G\nTypes:Creature Elf Druid\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCost | TriggerDescription$ When CARDNAME enters, you may sacrifice a land. If you do, draw a card.\n" +
		"SVar:TrigCost:AB$ Draw | Cost$ Sac<1/Land> | NumCards$ 1\n" +
		"Oracle:x\n"
	const landScript = "Name:Test Land\nTypes:Land\nOracle:x\n"
	e := handEngine(t)
	land := onBoard(t, e, 0, landScript)

	d := etbCostWindow(t, e, script)
	pay, _ := windowPayDecline(t, d)
	if pay < 0 {
		t.Fatalf("the Sac<1/Land> cost was not offered as payable: %+v", d.Options)
	}
	mark := len(e.L.Events)
	submitChoices(t, e, pay)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(land); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("the paying land zone = %v, want the graveyard", o)
	}
	sawSac := false
	for _, ev := range e.L.Events[mark:] {
		if events.IsSacrifice(ev) && ev.Obj == land {
			sawSac = true
		}
	}
	if !sawSac {
		t.Fatal("no events.Sacrifice for the paying land")
	}
	draws := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Draw && ev.Player == 0 {
			draws++
		}
	}
	if draws != 1 {
		t.Fatalf("draw events = %d, want 1 (the paid body)", draws)
	}
}

// TestTriggerCostSacComponentMultiCandidateAsks pins the Sac pick path: with
// two eligible lands the walk poses a real exact-1 KChoose and only the
// picked land leaves.
func TestTriggerCostSacComponentMultiCandidateAsks(t *testing.T) {
	const script = "Name:Ramp Elf\nManaCost:1 G\nTypes:Creature Elf Druid\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCost | TriggerDescription$ When CARDNAME enters, you may sacrifice a land. If you do, draw a card.\n" +
		"SVar:TrigCost:AB$ Draw | Cost$ Sac<1/Land> | NumCards$ 1\n" +
		"Oracle:x\n"
	const landScript = "Name:Test Land\nTypes:Land\nOracle:x\n"
	e := handEngine(t)
	landA := onBoard(t, e, 0, landScript)
	landB := onBoard(t, e, 0, landScript)

	d := etbCostWindow(t, e, script)
	pay, _ := windowPayDecline(t, d)
	submitChoices(t, e, pay)

	pick := e.Pending()
	if pick == nil || pick.Kind != decision.KChoose || pick.Min != 1 || pick.Max != 1 {
		t.Fatalf("expected the exact-1 sacrifice pick, got %+v", pick)
	}
	if len(pick.Options) != 2 || pick.Options[0].Kind != "sacrifice" {
		t.Fatalf("sacrifice pick options = %+v, want 2 sacrifice-kind options", pick.Options)
	}
	pickIdx := -1
	if pick.Options[0].Obj == landA {
		pickIdx = 0
	}
	if pickIdx < 0 {
		t.Fatalf("the pick does not offer the first land: %+v", pick.Options)
	}
	submitChoices(t, e, pickIdx)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(landA); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("the picked land zone = %v, want the graveyard", o)
	}
	if o := e.G.Obj(landB); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the unpicked land must stay on the battlefield")
	}
}

// TestTriggerCostUnpayableSacComponentDeclinesOnly pins the gate's
// component-payability half: a Sac component with no eligible candidate on
// the battlefield keeps the decline-only ask (the cost's body never runs,
// nothing is charged).
func TestTriggerCostUnpayableSacComponentDeclinesOnly(t *testing.T) {
	const script = "Name:Ramp Elf\nManaCost:1 G\nTypes:Creature Elf Druid\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCost | TriggerDescription$ When CARDNAME enters, you may sacrifice a land. If you do, draw a card.\n" +
		"SVar:TrigCost:AB$ Draw | Cost$ Sac<1/Land> | NumCards$ 1\n" +
		"Oracle:x\n"
	e := handEngine(t)

	d := etbCostWindow(t, e, script)
	pay, decline := windowPayDecline(t, d)
	if pay >= 0 {
		t.Fatalf("an unpayable Sac<1/Land> cost was offered as payable: %+v", d.Options)
	}
	mark := len(e.L.Events)
	submitChoices(t, e, decline)
	passUntilStackEmpty(t, e, 20)
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Draw || events.IsSacrifice(ev) {
			t.Fatalf("the unpayable cost still moved: %+v", ev)
		}
	}
}

// TestAmbergrisXCountsThePaidDiscard is the secondary-scope pin, on the REAL
// carrier's exact body and SVar verbatim (`AB$ DamageAll | Cost$
// Discard<1/Hand> Draw<2/You> | NumDmg$ X | ValidPlayers$ Opponent` with
// `SVar:X:PlayerCountPropertyYou$CardsDiscardedThisTurn`): the cost's
// discards are paid BEFORE the body resolves, so the body's X counts them --
// the opponent takes one damage per discarded card. Without the count head
// the X degraded to 0 and the body dealt nothing.
func TestAmbergrisXCountsThePaidDiscard(t *testing.T) {
	const script = "Name:Ambergris Citadel Agent\nManaCost:2 U\nTypes:Creature Human Wizard\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDamage | TriggerDescription$ When CARDNAME enters, you may discard your hand. If you do, it deals damage to each opponent equal to the cards discarded this turn.\n" +
		"SVar:TrigDamage:AB$ DamageAll | Cost$ Discard<1/Hand> Draw<2/You> | NumDmg$ X | ValidPlayers$ Opponent\n" +
		"SVar:X:PlayerCountPropertyYou$CardsDiscardedThisTurn\n" +
		"Oracle:x\n"
	junk := card(t, discardCostJunk)
	e := handEngine(t, junk, junk)

	d := etbCostWindow(t, e, script)
	pay, _ := windowPayDecline(t, d)
	if pay < 0 {
		t.Fatalf("the cost was not offered as payable: %+v", d.Options)
	}
	oppLife := e.G.Players[1].Life
	mark := len(e.L.Events)
	submitChoices(t, e, pay)
	passUntilStackEmpty(t, e, 20)

	if got := e.CardsDiscardedThisTurn(0); got != 2 {
		t.Fatalf("CardsDiscardedThisTurn(0) = %d, want 2 (the paid cost discards count)", got)
	}
	if got := e.G.Players[1].Life; got != oppLife-2 {
		t.Fatalf("opponent life = %d, want %d (X = the paid discards)", got, oppLife-2)
	}
	// 2 discarded, then the cost's two drawn.
	if got := len(e.G.Zone(state.ZHand, 0)); got != 2 {
		t.Fatalf("hand after pay = %d, want 2 (the drawn cards)", got)
	}
	draws := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Draw && ev.Player == 0 {
			draws++
		}
	}
	if draws != 2 {
		t.Fatalf("draw events = %d, want 2 (the cost's draw half)", draws)
	}
}
