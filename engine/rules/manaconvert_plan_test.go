package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestManaConvertPlansCreatureFromSources: Vizier of the Menagerie lets its
// controller spend mana as though it were mana of any type to cast creature
// spells (CR 609.4b). With only untapped Mountains, the payment planner must
// fund Wary Thespian's {1}{G} from them (its pips priced as generic, as under a
// may-play MayPlayIgnoreType$ grant), and the planned cast must settle: the
// Thespian reaches the stack paid by two red mana. Grow from the Ashes, a
// green sorcery, gets no plan.
func TestManaConvertPlansCreatureFromSources(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	l := func(n string) *cards.Card { return manaConvertCard(t, reg, n) }
	e := corpusEngine(t, reg, []*cards.Card{l("Vizier of the Menagerie"), l("Wary Thespian"),
		l("Mountain"), l("Mountain"), l("Grow from the Ashes")}, nil)
	moveByName(t, e, 0, "Vizier of the Menagerie", state.ZBattlefield)
	m1 := moveByName(t, e, 0, "Mountain", state.ZBattlefield)
	m2 := moveByName(t, e, 0, "Mountain", state.ZBattlefield)
	thespian := moveByName(t, e, 0, "Wary Thespian", state.ZHand)
	grow := moveByName(t, e, 0, "Grow from the Ashes", state.ZHand)
	e.askPriority(0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("precondition: no priority decision for seat 0: %+v", d)
	}
	if e.G.Players[0].Pool.Total() != 0 || e.G.Obj(m1).Tapped || e.G.Obj(m2).Tapped {
		t.Fatal("precondition: the pool is not empty or a Mountain is tapped")
	}
	if c := asEval(e).Conv(0, thespian, pay.SASpell); c == nil || !c.WildC {
		t.Fatalf("precondition: Vizier's conversion does not reach the Thespian: %+v", c)
	}
	var action *decision.PaymentAction
	for i, a := range e.EnsurePaymentActions() {
		switch a.Cast.Object {
		case grow:
			t.Fatalf("Grow from the Ashes (a sorcery) is planned under a creature-only conversion: %s", a.Label)
		case thespian:
			if len(a.Plans) > 0 {
				action = &d.PaymentActions[i]
			}
		}
	}
	if action == nil {
		t.Fatalf("no planned cast of Wary Thespian from two Mountains under Vizier: %+v", d.PaymentActions)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
		Payment: &decision.PaymentSelection{ActionID: action.ID, Plan: action.Plans[0]}}); err != nil {
		t.Fatalf("submit the planned cast: %v", err)
	}
	if o := e.G.Obj(thespian); o == nil || o.Zone != state.ZStack {
		t.Fatalf("Wary Thespian is not on the stack after its planned payment: %+v (pending %+v)", o, e.Pending())
	}
	if !e.G.Obj(m1).Tapped || !e.G.Obj(m2).Tapped || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("the payment did not tap both Mountains and spend their mana: pool %+v", e.G.Players[0].Pool)
	}
}
