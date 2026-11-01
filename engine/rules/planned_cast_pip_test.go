package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A planned cast's witness prices a Phyrexian pip as neither mana nor life, and the
// answer settles it (CR 107.4f). Paying the pip with mana outdates the witness
// before any step ran, and the cast is still paid without the manual window.
func TestPlannedCastPaysAPhyrexianPipWithMana(t *testing.T) {
	t.Parallel()
	const (
		imp   = "Name:Test Imp\nManaCost:2 BP\nTypes:Creature Imp\nPT:1/1\nOracle:x\n"
		swamp = "Name:Test Swamp\nTypes:Land Swamp\nA:AB$ Mana | Cost$ T | Produced$ B | SpellDescription$ Black.\nOracle:x\n"
	)
	for _, tc := range []struct {
		kind string
		taps int
	}{{"pay_B", 3}, {"pay_life", 2}} {
		e, _, spell := newFixtureDeck(t, 9700, imp)
		toMain1(t, e)
		e.paidManaCensus = true
		var lands []state.ObjID
		for range 3 {
			lands = append(lands, onBoard(t, e, 0, swamp))
		}
		d := ppPriority(t, e)
		e.EnsurePaymentActions()
		ppSubmitPlan(t, e, d, ppAction(t, e.Pending(), spell))
		ask := e.Pending()
		if ask == nil || ask.Kind != decision.KChoose {
			t.Fatalf("%s: pending %+v, want the pip ask", tc.kind, ask)
		}
		idx := -1
		for _, o := range ask.Options {
			if o.Kind == tc.kind {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("%s: not offered: %+v", tc.kind, ask.Options)
		}
		submitChoices(t, e, idx)
		if d := e.Pending(); d == nil || d.PaymentFallback != nil || d.Kind != decision.KPriority {
			t.Fatalf("%s: pending %+v, want the spell paid without a fallback", tc.kind, d)
		}
		tapped := 0
		for _, id := range lands {
			if e.G.Obj(id).Tapped {
				tapped++
			}
		}
		if tapped != tc.taps || len(e.G.Stack) != 1 {
			t.Errorf("%s: %d lands tapped, stack %v; want %d and the spell", tc.kind, tapped, e.G.Stack, tc.taps)
		}
	}
}

// A keyword granted only to a spell cast from exile isn't there when the plan is
// offered, so the seat's own convoke answer outdates the witness (CR 702.51a). The
// cast is still paid without the manual window.
func TestPlannedCastSurvivesAConvokeAnswer(t *testing.T) {
	t.Parallel()
	const (
		patron = "Name:Test Patron\nTypes:Creature Human\nPT:1/1\n" +
			"S:Mode$ Continuous | Affected$ Card.YouOwn | AffectedZone$ Exile | MayPlay$ True | Description$ Play exiled.\n" +
			"S:Mode$ Continuous | Affected$ Card.YouCtrl+wasCastFromExile | AffectedZone$ Stack | AddKeyword$ Convoke | Description$ Convoke.\nOracle:x\n"
		curse = "Name:Test Curse\nManaCost:2 R\nTypes:Enchantment\nOracle:x\n"
	)
	e, _, spell := newFixtureDeck(t, 9700, curse)
	toMain1(t, e)
	e.paidManaCensus = true
	helper := onBoard(t, e, 0, patron)
	e.G.Obj(helper).SummonSick = false
	var lands []state.ObjID
	for range 3 {
		lands = append(lands, onBoard(t, e, 0, apMountain))
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: spell, From: state.ZHand, To: state.ZExile})
	d := ppPriority(t, e)
	e.EnsurePaymentActions()
	ppSubmitPlan(t, e, d, ppAction(t, e.Pending(), spell))
	ask := e.Pending()
	if ask == nil || ask.Kind != decision.KChoose || ask.Source != spell {
		t.Fatalf("pending %+v, want the convoke ask", ask)
	}
	idx := -1
	for _, o := range ask.Options {
		if o.Kind == "convoke_generic" && o.Obj == helper {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no generic convoke option: %+v", ask.Options)
	}
	submitChoices(t, e, idx)
	if d := e.Pending(); d == nil || d.PaymentFallback != nil || d.Kind != decision.KPriority {
		t.Fatalf("pending %+v, want the spell paid without a fallback", d)
	}
	tapped := 0
	for _, id := range lands {
		if e.G.Obj(id).Tapped {
			tapped++
		}
	}
	if tapped != 2 || !e.G.Obj(helper).Tapped || len(e.G.Stack) != 1 {
		t.Errorf("%d lands tapped, helper tapped %v, stack %v; want 2, true and the spell", tapped, e.G.Obj(helper).Tapped, e.G.Stack)
	}
}
