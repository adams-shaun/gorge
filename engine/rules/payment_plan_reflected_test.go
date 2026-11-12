package rules

// The payment planner withholds AB$ ManaReflected sources (Exotic Orchard,
// Fellwar Stone) at its tier gate ("source:special_production") unless they
// have the one opponent-reflecting shape (pay.PaymentPlanReflectedShape), which
// is planned as a finite colour choice over the resolution's own candidates, so
// an auto-pay seat whose only mana for a spell is an Orchard still gets a
// PaymentAction plan for it.

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const reflectedRedProbe = "Name:Red Probe\nManaCost:R\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"

// reflectedBoard builds seat 0 with spellSrc in hand and, when source names a
// corpus card, that card on seat 0's battlefield; seat 1 controls a
// Mountain. It asserts the board the tests rely on: the opponent's land is
// on the battlefield and, apart from the source, seat 0 has no mana source
// the planner could use.
func reflectedBoard(t *testing.T, seed uint64, spellSrc, source string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e, _, spell := newFixtureDeck(t, seed, spellSrc)
	mountain := onBoard(t, e, 1, ppMountain)
	if o := e.G.Obj(mountain); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("precondition: opponent Mountain not on seat 1's battlefield")
	}
	var src state.ObjID
	if source != "" {
		src = onBoardCard(t, e, 0, corpusCardOrFatal(t, source))
		if o := e.G.Obj(src); o == nil || o.Zone != state.ZBattlefield || o.Tapped || o.Controller != 0 {
			t.Fatalf("precondition: %s not untapped on seat 0's battlefield", source)
		}
	}
	for _, u := range e.paymentPlanManaUnits(0) {
		if u.ID != src {
			t.Fatalf("precondition: seat 0 has another planner source %d", u.ID)
		}
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatal("precondition: seat 0's pool is not empty")
	}
	return e, spell, src
}

func TestPaymentPlanReflectedOrchardPlansAndExecutes(t *testing.T) {
	t.Parallel()
	for i, name := range []string{"Exotic Orchard", "Fellwar Stone"} {
		t.Run(name, func(t *testing.T) {
			e, spell, src := reflectedBoard(t, 9601+uint64(i), reflectedRedProbe, name)
			if tier, _, detail := pay.PaymentPlanAbilityTier(asPayer(e), 0, src, corpusCardOrFatal(t, name).Faces[0].Abilities[0]); tier != pay.TierNormal {
				t.Fatalf("%s tier = %v (%q), want normal", name, tier, detail)
			}
			if got := choiceColours(choiceAlts(t, e, src)); !slices.Equal(got, []string{"R"}) {
				t.Fatalf("%s alternatives = %v, want [R] (the opponent's Mountain)", name, got)
			}
			got := e.PlanCastPayment(0, paymentCast(spell))
			a, ok := choicePlanUses2(got, src)
			if !ok || got.Plan == nil || len(got.Plan.Activations) != 1 || a.Produces[state.ManaIndex('R')] != 1 {
				t.Fatalf("plan = %+v reason=%s, want %s -> R", got.Plan, got.Reason, name)
			}
			if err := e.ValidateCastPayment(0, paymentCast(spell), *got.Plan); err != nil {
				t.Fatalf("validate %s witness: %v", name, err)
			}
			d := paymentPlanReask(t, e)
			act := ppAction(t, d, spell)
			mark := len(e.L.Events)
			submitPaymentPlan(t, e, d, act)
			if nd := e.Pending(); nd == nil || nd.Kind != decision.KPriority {
				t.Fatalf("after the planned cast pending = %#v, want priority (no colour ask)", nd)
			}
			if !e.G.Obj(src).Tapped {
				t.Errorf("%s was not tapped by the executed plan", name)
			}
			// The counter carries the producer's provenance tag (Fellwar
			// Stone's "Artifact"); the colour is its last letter.
			if prod := producedManaSince(e, mark); len(prod) != 1 || !strings.HasSuffix(prod[0], "R") {
				t.Errorf("mana produced = %v, want one R", prod)
			}
			for _, ev := range e.L.Events[mark:] {
				if ev.Kind == events.Note && ev.Obj == src {
					t.Errorf("unexpected Note on %s: %q", name, ev.Text)
				}
			}
			if z := e.G.Obj(spell).Zone; z != state.ZStack {
				t.Errorf("spell zone after payment = %s, want stack", z)
			}
			if p := e.G.Players[0].Pool.Total(); p != 0 {
				t.Errorf("pool after payment = %d, want 0", p)
			}
		})
	}
}

// TestPaymentPlanReflectedNeedsTheSource is the precondition pair: the same
// board without the Orchard offers no plan, and the Orchard cannot pay a
// colour no opponent land produces.
func TestPaymentPlanReflectedNeedsTheSource(t *testing.T) {
	t.Parallel()
	e, spell, _ := reflectedBoard(t, 9611, reflectedRedProbe, "")
	if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan != nil {
		t.Fatalf("no-source board planned %+v", got.Plan)
	}
	if d := paymentPlanReask(t, e); ppHasAction(d, spell) {
		t.Fatal("no-source board posed a PaymentAction for the spell")
	}
	blue := "Name:Blue Probe\nManaCost:U\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"
	e, spell, src := reflectedBoard(t, 9612, blue, "Exotic Orchard")
	if len(choiceAlts(t, e, src)) == 0 {
		t.Fatal("precondition: the Orchard has no planner alternative at all")
	}
	if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan != nil {
		t.Fatalf("Orchard reflecting only red planned a {U} spell: %+v", got.Plan)
	}
}

// TestPaymentPlanReflectedBotAnswerValidates runs the auto-pay bot's own
// answer shape (engine/seat/bot.go: the action's first plan, cloned) through the
// validator and the engine on the board where the reflected shape binds, so
// the planner cannot offer a witness its validator rejects (the livelock the
// one-home rule guards).
func TestPaymentPlanReflectedBotAnswerValidates(t *testing.T) {
	t.Parallel()
	e, spell, src := reflectedBoard(t, 9614, reflectedRedProbe, "Exotic Orchard")
	d := paymentPlanReask(t, e)
	act := ppAction(t, d, spell)
	plan := decision.ClonePaymentPlan(act.Plans[0])
	if len(plan.Activations) != 1 || plan.Activations[0].Source != src {
		t.Fatalf("bot plan = %+v, want the Orchard's one step", plan)
	}
	if err := e.ValidateCastPayment(0, act.Cast, plan); err != nil {
		t.Fatalf("validator rejects the bot's plan: %v", err)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Payment: &decision.PaymentSelection{ActionID: act.ID, Plan: plan}}); err != nil {
		t.Fatalf("engine rejects the bot's plan: %v", err)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Fatalf("spell zone after the bot's plan = %s, want stack", z)
	}
}

// TestPaymentPlanReflectedChoosesAmongSeveralColours: with the opponent's
// lands producing W, R and G, the Orchard is a real three-way choice; the
// planned step names R and the executor resolves it without posing the
// reflected colour ask.
func TestPaymentPlanReflectedChoosesAmongSeveralColours(t *testing.T) {
	t.Parallel()
	e, spell, src := reflectedBoard(t, 9615, reflectedRedProbe, "Exotic Orchard")
	onBoardCard(t, e, 1, corpusCardOrFatal(t, "Selesnya Guildgate"))
	if got := choiceColours(choiceAlts(t, e, src)); !slices.Equal(got, []string{"W", "R", "G"}) {
		t.Fatalf("precondition: Orchard alternatives = %v, want [W R G]", got)
	}
	d := paymentPlanReask(t, e)
	act := ppAction(t, d, spell)
	mark := len(e.L.Events)
	submitPaymentPlan(t, e, d, act)
	if nd := e.Pending(); nd == nil || nd.Kind != decision.KPriority {
		t.Fatalf("after the planned cast pending = %#v, want priority (no colour ask)", nd)
	}
	if prod := producedManaSince(e, mark); !slices.Equal(prod, []string{"R"}) {
		t.Errorf("mana produced = %v, want [R]", prod)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Errorf("spell zone after payment = %s, want stack", z)
	}
}
