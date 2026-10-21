package effects_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// An inline Masticore trigger body: no Forge script is included in the fixture.
const sacrificeUnlessDiscardSrc = "Name:Discard Warden\nManaCost:4\n" +
	"Types:Artifact Creature Warden\nPT:4/4\n" +
	"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigUpkeep\n" +
	"SVar:TrigUpkeep:DB$ Sacrifice | UnlessPayer$ You | UnlessCost$ Discard<1/Card>\n" +
	"Oracle:At the beginning of your upkeep, sacrifice CARDNAME unless you discard a card.\n"

// sacrificeUnlessDiscardGame stages the Warden on seat 0's battlefield at its
// upkeep with hand cards in seat 0's hand. Staging is used so the test starts
// exactly at the beginning-of-upkeep trigger, with no turn driving.
func sacrificeUnlessDiscardGame(t *testing.T, hand int) (*rules.Engine, state.ObjID, []state.ObjID) {
	t.Helper()
	return sacrificeUnlessDiscardGameSrc(t, sacrificeUnlessDiscardSrc, hand)
}

// sacrificeUnlessDiscardGameSrc is sacrificeUnlessDiscardGame over a given
// inline card script, so a test can vary the UnlessCost$ spec.
func sacrificeUnlessDiscardGameSrc(t *testing.T, src string, hand int) (*rules.Engine, state.ObjID, []state.ObjID) {
	t.Helper()
	warden, diags := cards.ParseBytes("inline-discard-warden.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("fixture parse: %v", diags)
	}
	warden.Link()
	bear, diags := cards.ParseBytes("inline-bear.txt", []byte("Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatalf("bear parse: %v", diags)
	}
	bear.Link()
	mountain, diags := cards.ParseBytes("inline-mountain.txt", []byte("Name:Test Mountain\nTypes:Basic Land Mountain\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatalf("mountain parse: %v", diags)
	}
	mountain.Link()
	library := make([]*cards.Card, 10)
	for i := range library {
		library[i] = mountain
	}
	handCards := make([]*cards.Card, hand)
	for i := range handCards {
		handCards[i] = bear
	}
	e, ids, err := rules.NewStaged(rules.Config{Seed: 7, Names: []string{"a", "b"}}, rules.Stage{
		Turn: 3, Active: 0, Starting: 0, Step: state.StepUpkeep, Enter: rules.StageBeginStep,
		Players: []rules.StagedPlayer{
			{Life: 20, Library: library, Hand: handCards},
			{Life: 20, Library: library},
		},
		Permanents: []rules.StagedPermanent{{Card: warden, Owner: 0, Controller: 0}},
	})
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	permanent := ids.Permanents[0]
	if o := e.G.Obj(permanent); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Warden zone = %v, want battlefield", o.Zone)
	}
	if got := len(ids.Hand[0]); got != hand {
		t.Fatalf("precondition: hand = %d cards, want %d", got, hand)
	}
	return e, permanent, ids.Hand[0]
}

// drainUntilUnlessPay passes priority until the upkeep trigger resolves into
// the shared unless-pay ask.
func drainUntilUnlessPay(t *testing.T, e *rules.Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 50; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KModes {
			if d.ResumeKind != "unless_pay" {
				t.Fatalf("unless ask resume kind = %q, want unless_pay", d.ResumeKind)
			}
			return d
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision kind %v while seeking the unless-pay ask", d.Kind)
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		if pass < 0 {
			t.Fatalf("priority decision has no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pass}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	t.Fatal("no unless-pay ask posed for the upkeep trigger")
	return nil
}

func unlessModeOption(d *decision.Decision, mode string) (int, bool) {
	for _, o := range d.Options {
		if o.Mode == mode {
			return o.Index, true
		}
	}
	return 0, false
}

// TestSacrificeUnlessDiscardPays is the Sacrifice/Discard<1/Card> contract:
// with a card in hand the shared unless gate offers the pay branch, paying
// discards the card and keeps the permanent; declining sacrifices it; and an
// empty hand offers only the decline.
func TestSacrificeUnlessDiscardPays(t *testing.T) {
	t.Run("pay discards a card and keeps the permanent", func(t *testing.T) {
		e, permanent, hand := sacrificeUnlessDiscardGame(t, 1)
		ask := drainUntilUnlessPay(t, e)
		if len(ask.Options) != 2 {
			t.Fatalf("options = %d, want pay and decline with a card in hand", len(ask.Options))
		}
		pay, ok := unlessModeOption(ask, decision.ModeUnlessPay)
		if !ok {
			t.Fatalf("no pay option: %+v", ask.Options)
		}
		if err := e.Submit(decision.Intent{Seq: ask.Seq, Player: ask.Player, Choices: []int{pay}}); err != nil {
			t.Fatalf("submit pay: %v", err)
		}
		if z := e.G.Obj(hand[0]).Zone; z != state.ZGraveyard {
			t.Fatalf("discarded card zone = %v, want graveyard", z)
		}
		if z := e.G.Obj(permanent).Zone; z != state.ZBattlefield {
			t.Fatalf("Warden zone = %v, want battlefield after the discard paid", z)
		}
	})

	t.Run("decline sacrifices the permanent", func(t *testing.T) {
		e, permanent, hand := sacrificeUnlessDiscardGame(t, 1)
		ask := drainUntilUnlessPay(t, e)
		decline, ok := unlessModeOption(ask, decision.ModeUnlessDecline)
		if !ok {
			t.Fatalf("no decline option: %+v", ask.Options)
		}
		if err := e.Submit(decision.Intent{Seq: ask.Seq, Player: ask.Player, Choices: []int{decline}}); err != nil {
			t.Fatalf("submit decline: %v", err)
		}
		if z := e.G.Obj(permanent).Zone; z != state.ZGraveyard {
			t.Fatalf("Warden zone = %v, want graveyard after the decline", z)
		}
		if z := e.G.Obj(hand[0]).Zone; z != state.ZHand {
			t.Fatalf("hand card zone = %v, want hand after the decline", z)
		}
	})

	t.Run("empty hand offers no pay", func(t *testing.T) {
		e, _, _ := sacrificeUnlessDiscardGame(t, 0)
		ask := drainUntilUnlessPay(t, e)
		if len(ask.Options) != 1 {
			t.Fatalf("options = %d, want only the decline with an empty hand", len(ask.Options))
		}
		if _, ok := unlessModeOption(ask, decision.ModeUnlessPay); ok {
			t.Fatalf("pay offered with no card to discard: %+v", ask.Options)
		}
		if _, ok := unlessModeOption(ask, decision.ModeUnlessDecline); !ok {
			t.Fatalf("no decline option: %+v", ask.Options)
		}
	})
}
