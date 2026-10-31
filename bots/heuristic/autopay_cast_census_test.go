package heuristic

// Regressions for botmanual (2026-10-14): why the auto-pay bot still answered
// priority with manual mana taps while payment plans were on offer. Measured
// over a 420-game constructed repo-deck auto-pay mirror and a 100-game network
// bot vs sb-tactical sample, the classes these tests pin were:
//
//   - the tap gate aiming at a card the engine already knew was not castable
//     this window (absent from the offer builder's cast census:
//     decision.PotentialCasts) or PROVED unpayable
//     (decision.PotentialUnpayableCasts), then passing or casting a planned
//     card with the floated mana -- two of every three manual tap runs;
//   - a graveyard card's flashback cast, which had no plan, so the bot tapped
//     by hand -- and, pricing the card at its PRINTED cost, often stopped
//     short of the flashback cost and floated the mana away.

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// censusBoard: five Forests, a planned 2/2 (42) and an unplanned 5/5 (50)
// the tap gate prefers.
func censusBoard() (botpolicy.Board, decision.Decision) {
	brd := botpolicy.Board{IsMain: true, FirstMain: true, MyTurn: true,
		Cards: botpolicy.TableOf(map[state.ObjID]botpolicy.Card{
			1: forestCard(), 2: forestCard(), 3: forestCard(), 4: forestCard(), 5: forestCard(),
			42: {Creature: true, Power: 2, Toughness: 2, CMC: 2, Castable: true, ManaCost: "1 G"},
			50: {Creature: true, Power: 5, Toughness: 5, CMC: 5, Castable: true, ManaCost: "3 G G"},
		}),
	}
	d := decision.Decision{Seq: 7, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1}
	for _, id := range []state.ObjID{1, 2, 3, 4, 5} {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "activate", Obj: id})
	}
	d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "pass"})
	return brd, d
}

// TestAutoPayCensusExcludesUncastableIntent: the 5/5 is the tap gate's intent
// and has no plan, so without the census the bot taps toward it (the old
// behaviour, asserted as the precondition). The engine's census says the 5/5
// has no cast this window at all (no legal target, a restriction ...): the
// bot must take the 2/2's plan instead of floating mana.
func TestAutoPayCensusExcludesUncastableIntent(t *testing.T) {
	brd, d := censusBoard()
	d.PaymentActions = []decision.PaymentAction{paymentAction("pay", 42)}

	before := NewBot(19).EnableAutoPayMana().decide(brd, &d)
	if len(before.Choices) != 1 || d.Options[before.Choices[0]].Kind != "activate" {
		t.Fatalf("precondition: without the census the bot should tap toward the 5/5, got %+v", before)
	}

	d.PotentialCastsKnown, d.PotentialCasts = true, []state.ObjID{42}
	in := NewBot(19).EnableAutoPayMana().decide(brd, &d)
	if in.Payment == nil || in.Payment.ActionID != "pay" {
		t.Fatalf("intent = %+v, want the 2/2's plan: the 5/5 is not castable this window", in)
	}
}

// TestAutoPayCensusExcludesProvedUnpayable: the 5/5 is castable this window
// in principle (listed in the census) but the planner PROVED no assignment of
// the seat's sources pays it; tapping toward it can only float mana.
func TestAutoPayCensusExcludesProvedUnpayable(t *testing.T) {
	brd, d := censusBoard()
	d.PaymentActions = []decision.PaymentAction{paymentAction("pay", 42)}
	d.PotentialCastsKnown, d.PotentialCasts = true, []state.ObjID{42, 50}
	d.PotentialUnpayableCasts = []state.ObjID{50}
	in := NewBot(19).EnableAutoPayMana().decide(brd, &d)
	if in.Payment == nil || in.Payment.ActionID != "pay" {
		t.Fatalf("intent = %+v, want the 2/2's plan: the 5/5 is proved unpayable", in)
	}
	// Listed and not proved unpayable, the 5/5 is still the manual intent.
	d.PotentialUnpayableCasts = nil
	in = NewBot(19).EnableAutoPayMana().decide(brd, &d)
	if len(in.Choices) != 1 || d.Options[in.Choices[0]].Kind != "activate" {
		t.Fatalf("intent = %+v, want a manual tap toward the castable unplanned 5/5", in)
	}
}

// TestAutoPayCensusNoPlanWindowDoesNotFloat: a window with no plan at all
// whose only unpayable card the census excludes must not tap (the no-plan
// branch's AnyCastableNow reads the same census).
func TestAutoPayCensusNoPlanWindowDoesNotFloat(t *testing.T) {
	brd, d := censusBoard()
	brd.Cards.Delete(42)
	before := NewBot(19).EnableAutoPayMana().decide(brd, &d)
	if len(before.Choices) != 1 || d.Options[before.Choices[0]].Kind != "activate" {
		t.Fatalf("precondition: without the census the no-plan window taps, got %+v", before)
	}
	d.PotentialCastsKnown, d.PotentialCasts = true, nil
	in := NewBot(19).EnableAutoPayMana().decide(brd, &d)
	if len(in.Choices) != 1 || d.Options[in.Choices[0]].Kind != "pass" {
		t.Fatalf("intent = %+v, want pass: nothing castable, so no tap", in)
	}
}

// flashbackBoard: three Islands and a graveyard flashback card (printed
// {1}{U}, flashback {2}{U}) whose flashback cast has a plan.
func flashbackBoard() (botpolicy.Board, decision.Decision) {
	brd := botpolicy.Board{IsMain: true, FirstMain: true, MyTurn: true,
		Cards: botpolicy.TableOf(map[state.ObjID]botpolicy.Card{
			1: islandCard(), 2: islandCard(), 3: islandCard(),
			70: {CMC: 2, Castable: true, ManaCost: "1 U", InstantSpeed: true},
		}),
	}
	d := decision.Decision{Seq: 9, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1}
	for _, id := range []state.ObjID{1, 2, 3} {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "activate", Obj: id})
	}
	d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "pass"})
	fb := paymentAction("fb", 70)
	fb.Cast.Origin = decision.OriginFlashback
	fb.Label = "Cast Flashback Test (flashback)"
	d.PaymentActions = []decision.PaymentAction{fb}
	d.PotentialCastsKnown, d.PotentialCasts = true, []state.ObjID{70}
	return brd, d
}

// TestAutoPayCastsFlashbackThroughPlan: with no floating mana the flashback
// cast is not a legacy option; its plan-only candidate (Mode "flashback")
// must win over a manual tap.
func TestAutoPayCastsFlashbackThroughPlan(t *testing.T) {
	brd, d := flashbackBoard()
	in := NewBot(19).EnableAutoPayMana().decide(brd, &d)
	if in.Payment == nil || in.Payment.ActionID != "fb" {
		t.Fatalf("intent = %+v, want the flashback plan", in)
	}
}

// TestAutoPayFlashbackLegacyOptionSubmitsPlan: when the floating pool already
// offers the flashback cast as a legacy option, picking it submits the
// flashback-origin plan, and a further-instance flashback (Option.Cost set)
// is never substituted by it.
func TestAutoPayFlashbackLegacyOptionSubmitsPlan(t *testing.T) {
	brd, d := flashbackBoard()
	d.Options = append([]decision.Option{{Kind: "cast", Obj: 70, Mode: "flashback", Label: "Cast Flashback Test (flashback)"}}, d.Options...)
	for i := range d.Options {
		d.Options[i].Index = i
	}
	in := NewBot(19).EnableAutoPayMana().decide(brd, &d)
	if in.Payment == nil || in.Payment.ActionID != "fb" {
		t.Fatalf("intent = %+v, want the flashback plan for the legacy flashback pick", in)
	}

	d.Options[0].Cost = "U U U" // a further flashback instance: its own cost, not the plan's
	d.PaymentActions = nil
	in = NewBot(19).EnableAutoPayMana().decide(brd, &d)
	if in.Payment != nil {
		t.Fatalf("intent = %+v, a further flashback instance must never submit a plan", in)
	}
}
