package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestCumulativeWindowSubAbilityColourChoiceResumesWindow(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	charm := onBoard(t, e, 0, upkeepEnchantment)
	grotto := onBoard(t, e, 0, luckGrotto)
	e.emit(events.Event{Kind: events.CounterChange, Obj: grotto, Counter: "LUCK", Amount: 1})
	resolveUpkeepCumulative(t, e)
	d := cumulativeElectByHand(t, e)
	// Tap the grotto for mana inside the window.
	submitChoices(t, e, optionIndex(t, d, func(o decision.Option) bool { return o.Kind == "activate" && o.Obj == grotto }))
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || !strings.Contains(strings.ToLower(d.Prompt), "colo") {
		t.Fatalf("want the sub-ability's colour ask, got %+v", d)
	}
	submitChoices(t, e, optionIndex(t, d, func(o decision.Option) bool { return o.Label == "Add U" }))
	// The payment window resumed after the colour answer: the floated {U}
	// covers the elected pay, which settles with no second election.
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("paying the upkeep left %d mana in the pool", got)
	}
	if o := e.G.Obj(charm); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the upkeep was paid but the permanent left the battlefield")
	}
	if e.cumulative != nil {
		t.Fatal("the cumulative window never closed")
	}
}
