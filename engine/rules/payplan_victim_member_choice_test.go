package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two mana creatures of one class beside an other-sacrifice altar: the
// planner's search picks HOW MANY of a class to tap, and the victim check
// must not refuse the cast because the first member choice takes the only
// legal victim as a source (CR 601.2g-h).

const (
	memberChoiceSpellSrc = "Name:MemberChoice Spell\nManaCost:2 G\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"
	memberChoiceDorkSrc  = "Name:MemberChoice Dork\nTypes:Creature\nPT:1/1\nA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ x\nOracle:x\n"
	memberChoiceKeptSrc  = "Name:MemberChoice Kept\nTypes:Creature\nPT:1/1\nA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ x\n" +
		"S:Mode$ CantSacrifice | ValidCard$ Card.Self | Description$ x\nOracle:x\n"
)

// memberChoiceBoard puts the altar and the two dorks (first, then second, so
// first has the lower id) on seat 0's battlefield, both able to tap.
func memberChoiceBoard(t *testing.T, seed uint64, firstSrc, secondSrc string) (e *Engine, spell, altar, first, second state.ObjID) {
	t.Helper()
	e, _, spell = newFixtureDeck(t, seed, memberChoiceSpellSrc)
	altar = onBoard(t, e, 0, otherSacAltarSrc)
	first = onBoard(t, e, 0, firstSrc)
	second = onBoard(t, e, 0, secondSrc)
	e.G.Obj(first).SummonSick = false
	e.G.Obj(second).SummonSick = false
	if !(first < second) {
		t.Fatalf("precondition: creature ids %d, %d, want the first one lower", first, second)
	}
	return e, spell, altar, first, second
}

// memberChoiceSteps splits a witness into the creature step's source and the
// altar step's victim, failing unless it is exactly those two steps.
func memberChoiceSteps(t *testing.T, plan decision.PaymentPlan, altar state.ObjID) (source, victim state.ObjID) {
	t.Helper()
	if len(plan.Activations) != 2 {
		t.Fatalf("witness %+v, want the altar and one creature", plan.Activations)
	}
	for _, a := range plan.Activations {
		if a.Source != altar {
			source = a.Source
			continue
		}
		if len(a.Sacrifices) != 1 {
			t.Fatalf("altar step victims = %v, want exactly one", a.Sacrifices)
		}
		victim = a.Sacrifices[0]
	}
	if source == 0 || victim == 0 {
		t.Fatalf("witness %+v, want one altar step and one creature step", plan.Activations)
	}
	return source, victim
}

func TestPaymentPlanVictimGateMemberChoice(t *testing.T) {
	t.Parallel()

	// Control: with both creatures legal victims the planner taps the
	// lower-id one -- the member choice the restricted board cannot fund.
	ctl, ctlSpell, ctlAltar, ctlFirst, ctlSecond := memberChoiceBoard(t, 4731, memberChoiceDorkSrc, memberChoiceDorkSrc)
	source, victim := memberChoiceSteps(t, otherSacAction(t, ctl, ctlSpell).Plans[0], ctlAltar)
	if source != ctlFirst || victim != ctlSecond {
		t.Fatalf("control: source %d victim %d, want source %d victim %d", source, victim, ctlFirst, ctlSecond)
	}

	// The second creature can't be sacrificed: only tapping IT and
	// sacrificing the first pays {2}{G}.
	e, spell, altar, first, second := memberChoiceBoard(t, 4732, memberChoiceDorkSrc, memberChoiceKeptSrc)
	if e.sacrificeBlockedForCost(first, pay.CostCauseActivated) || !e.sacrificeBlockedForCost(second, pay.CostCauseActivated) {
		t.Fatalf("precondition: want only creature %d sacrifice-blocked, not %d", second, first)
	}
	units := pay.PaymentPlanQueryUnits(asPayer(e), 0)
	choices := make([][]pay.Alt, len(units))
	for i, u := range units {
		choices[i] = pay.PaymentPlanQueryAlternatives(asPayer(e), u)
	}
	if !slices.ContainsFunc(pay.Classes(choices), func(c pay.Class) bool { return len(c.Members) == 2 }) {
		t.Fatalf("precondition: the two creatures are not one class: %+v", choices)
	}

	a := otherSacAction(t, e, spell)
	source, victim = memberChoiceSteps(t, a.Plans[0], altar)
	if source != second || victim != first {
		t.Fatalf("source %d victim %d, want source %d victim %d", source, victim, second, first)
	}

	// Accept: the validator admits the plan and the cast resolves.
	submitPayment(t, e, a)
	for i := 0; i < 20 && e.G.Obj(spell).Zone == state.ZStack; i++ {
		d := e.Pending()
		switch {
		case d == nil:
			e.Advance()
		case d.Kind == decision.KPriority:
			passPriorityOnce(t, e)
		default:
			t.Fatalf("unexpected ask while the planned cast resolves: %+v", d)
		}
	}
	if z := e.G.Obj(spell).Zone; z != state.ZGraveyard {
		t.Fatalf("spell zone = %s, want graveyard (cast paid and resolved)", z)
	}
	if z := e.G.Obj(first).Zone; z != state.ZGraveyard {
		t.Fatalf("victim zone = %s, want graveyard", z)
	}
	if o := e.G.Obj(second); o.Zone != state.ZBattlefield || !o.Tapped {
		t.Fatalf("mana creature zone = %s tapped = %v, want tapped on the battlefield", o.Zone, o.Tapped)
	}
}
