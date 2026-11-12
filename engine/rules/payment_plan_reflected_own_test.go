package rules

// The planner plans
// AB$ ManaReflected sources that reflect the payer's OWN permanents
// (Reflecting Pool, Gond Gate), read colours ("Is": Mox Amber), and the
// Defined.* sets no step can shrink (Chrome Mox's imprint). An own reflected
// object a planned step could remove (a self-sacrificing land) is dropped
// from the census so the planner never offers a plan that degrades.

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	reflGreenProbe = "Name:Green Probe\nManaCost:G\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"
	reflWhiteProbe = "Name:White Probe\nManaCost:W\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"
	reflBlueProbe  = "Name:Blue Probe\nManaCost:U\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"
	reflGenProbe   = "Name:Generic Probe\nManaCost:1\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"
	reflForest     = "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"
	reflPlains     = "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n"
	reflGate       = "Name:Test Gate\nTypes:Land Gate\nA:AB$ Mana | Cost$ T | Produced$ W | SpellDescription$ Add {W}.\nOracle:x\n"
	reflBlueLegend = "Name:Blue Legend\nManaCost:2 U\nTypes:Legendary Creature Wizard\nPT:2/2\nOracle:x\n"
	reflBlueCard   = "Name:Blue Imprint\nManaCost:U\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"
	// reflSacLand is a land whose only mana ability sacrifices it: a planned
	// step that removes a reflected object.
	reflSacLand = "Name:Sac Land\nTypes:Land\nA:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ R | SpellDescription$ Add {R}.\nOracle:x\n"
)

// reflOwnBoard builds seat 0 with spellSrc in hand, the src corpus card on
// the battlefield, and each own inline card in `own` on the battlefield
// TAPPED (a tapped permanent is not a mana source but is still reflected).
// It asserts the precondition every test relies on: src is the only mana
// source the planner could use.
func reflOwnBoard(t *testing.T, seed uint64, spellSrc, srcName string, own ...string) (*Engine, state.ObjID, state.ObjID, []state.ObjID) {
	t.Helper()
	e, _, spell := newFixtureDeck(t, seed, spellSrc)
	var ids []state.ObjID
	for _, s := range own {
		id := onBoard(t, e, 0, s)
		e.G.Obj(id).Tapped = true
		ids = append(ids, id)
	}
	src := onBoardCard(t, e, 0, corpusCardOrFatal(t, srcName))
	if o := e.G.Obj(src); o == nil || o.Zone != state.ZBattlefield || o.Tapped || o.Controller != 0 {
		t.Fatalf("precondition: %s not untapped on seat 0's battlefield", srcName)
	}
	for _, u := range e.paymentPlanManaUnits(0) {
		if u.ID != src {
			t.Fatalf("precondition: seat 0 has another planner source %d", u.ID)
		}
	}
	return e, spell, src, ids
}

// reflPlanAndRun plans the spell, requires exactly src's one step producing
// colour, validates it and executes it through the engine without a colour
// ask.
func reflPlanAndRun(t *testing.T, e *Engine, spell, src state.ObjID, colour string) {
	t.Helper()
	got := e.PlanCastPayment(0, paymentCast(spell))
	a, ok := choicePlanUses2(got, src)
	if !ok || got.Plan == nil || len(got.Plan.Activations) != 1 || a.Produces[state.ManaIndex(colour[0])] != 1 {
		t.Fatalf("plan = %+v reason=%s, want the source -> %s", got.Plan, got.Reason, colour)
	}
	if err := e.ValidateCastPayment(0, paymentCast(spell), *got.Plan); err != nil {
		t.Fatalf("validate witness: %v", err)
	}
	d := paymentPlanReask(t, e)
	act := ppAction(t, d, spell)
	mark := len(e.L.Events)
	submitPaymentPlan(t, e, d, act)
	if nd := e.Pending(); nd == nil || nd.Kind != decision.KPriority {
		t.Fatalf("after the planned cast pending = %#v, want priority (no colour ask)", nd)
	}
	if !e.G.Obj(src).Tapped {
		t.Error("source not tapped by the executed plan")
	}
	if prod := producedManaSince(e, mark); len(prod) != 1 || !strings.HasSuffix(prod[0], colour) {
		t.Errorf("mana produced = %v, want one %s", prod, colour)
	}
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Note && ev.Obj == src {
			t.Errorf("unexpected Note on the source: %q", ev.Text)
		}
	}
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Errorf("spell zone after payment = %s, want stack", z)
	}
}

func TestPaymentPlanReflectedOwnPoolAndGatePlanAndExecute(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, src, spell, colour string
		own                      []string
	}{
		{"Reflecting Pool", "Reflecting Pool", reflGreenProbe, "G", []string{reflForest}},
		{"Gond Gate", "Gond Gate", reflWhiteProbe, "W", []string{reflGate}},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e, spell, src, own := reflOwnBoard(t, 9701+uint64(i), c.spell, c.src, c.own...)
			if o := e.G.Obj(own[0]); o.Zone != state.ZBattlefield || o.Controller != 0 {
				t.Fatalf("precondition: reflected object not on seat 0's battlefield")
			}
			// Gond Gate also has its own {T}: Add {C}, so only require that
			// the reflected colour is offered (and that it is the only
			// coloured alternative).
			var cols []string
			for _, col := range choiceColours(choiceAlts(t, e, src)) {
				if col != "C" {
					cols = append(cols, col)
				}
			}
			if !slices.Equal(cols, []string{c.colour}) {
				t.Fatalf("coloured alternatives = %v, want [%s]", cols, c.colour)
			}
			reflPlanAndRun(t, e, spell, src, c.colour)
		})
	}
}

func TestPaymentPlanReflectedMoxAmberReadsLegendaryColour(t *testing.T) {
	t.Parallel()
	e, spell, src, _ := reflOwnBoard(t, 9703, reflBlueProbe, "Mox Amber")
	if got := choiceAlts(t, e, src); len(got) != 0 {
		t.Fatalf("precondition: Mox Amber offers %v with no legendary permanent", got)
	}
	if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan != nil {
		t.Fatalf("Mox Amber with no legend planned %+v", got.Plan)
	}
	legend := onBoard(t, e, 0, reflBlueLegend)
	if o := e.G.Obj(legend); o.Zone != state.ZBattlefield {
		t.Fatal("precondition: legend not on the battlefield")
	}
	if got := choiceColours(choiceAlts(t, e, src)); !slices.Equal(got, []string{"U"}) {
		t.Fatalf("alternatives = %v, want [U]", got)
	}
	reflPlanAndRun(t, e, spell, src, "U")
}

func TestPaymentPlanReflectedChromeMoxImprint(t *testing.T) {
	t.Parallel()
	e, spell, src, _ := reflOwnBoard(t, 9704, reflBlueProbe, "Chrome Mox")
	if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan != nil {
		t.Fatalf("un-imprinted Chrome Mox planned %+v", got.Plan)
	}
	imp := e.G.AddObject(card(t, reflBlueCard), 0)
	imp.Zone = state.ZExile
	e.emit(events.Event{Kind: events.Imprint, Obj: src, IDs: []state.ObjID{imp.ID}})
	if o := e.G.Obj(src); len(o.Imprinted) != 1 || e.G.Obj(o.Imprinted[0]).Zone != state.ZExile {
		t.Fatalf("precondition: imprint not recorded in exile: %+v", o.Imprinted)
	}
	reflPlanAndRun(t, e, spell, src, "U")
}

// A Reflecting Pool beside a Wastes: the planner offers coloured
// alternatives only, so the Pool has none here (the {C} stays out of the
// reflected counts and the PotentialMana bound alike) and the Wastes pays.
func TestPaymentPlanReflectedColourlessIsLeftToRealSources(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9705, reflGenProbe)
	wastes := onBoardCard(t, e, 0, corpusCardOrFatal(t, "Wastes"))
	poolCard := corpusCardOrFatal(t, "Reflecting Pool")
	pool := onBoardCard(t, e, 0, poolCard)
	if o := e.G.Obj(wastes); o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatal("precondition: Wastes not untapped on the battlefield")
	}
	// The Pool's shape must be admitted, or this test would pass with the
	// whole own-reflection feature unregistered.
	if ma := poolCard.Faces[0].Abilities[0]; !pay.PaymentPlanReflectedShape(ma) {
		t.Fatal("precondition: Reflecting Pool shape not admitted (own-reflection absent)")
	}
	if got := choiceAlts(t, e, pool); len(got) != 0 {
		t.Fatalf("Pool alternatives = %v, want none (colourless reflects stay out)", got)
	}
	got := e.PlanCastPayment(0, paymentCast(spell))
	if _, ok := choicePlanUses2(got, wastes); !ok || got.Plan == nil || len(got.Plan.Activations) != 1 {
		t.Fatalf("plan = %+v reason=%s, want the Wastes' one step", got.Plan, got.Reason)
	}
	// Wastes tapped: the Pool reflects only {C}, which the planner leaves
	// to a manual payment rather than planning.
	e.G.Obj(wastes).Tapped = true
	if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan != nil {
		t.Fatalf("Pool reflecting only {C} planned %+v", got.Plan)
	}
}

func TestPaymentPlanReflectedHorizonOfProgressIsLastResortWithLife(t *testing.T) {
	t.Parallel()
	e, spell, src, _ := reflOwnBoard(t, 9706, reflGreenProbe, "Horizon of Progress", reflForest)
	ma := corpusCardOrFatal(t, "Horizon of Progress").Faces[0].Abilities[0]
	tier, c, detail := pay.PaymentPlanAbilityTier(asPayer(e), 0, src, ma)
	if tier != pay.TierLastResort || c != (pay.Consequence{Life: 1}) {
		t.Fatalf("Horizon tier = %v consequence = %+v (%q), want last resort {Life:1}", tier, c, detail)
	}
	got := e.PlanCastPayment(0, paymentCast(spell))
	if _, ok := choicePlanUses2(got, src); !ok {
		t.Fatalf("plan = %+v reason=%s, want the Horizon's step", got.Plan, got.Reason)
	}
}

// Removal safety: the Pool's only red source is a land whose mana ability
// sacrifices it, which a planned step could remove before the Pool's step
// runs. The census drops it, so no plan relies on the Pool for {R}; a
// non-sacrificing Mountain restores the plan.
func TestPaymentPlanReflectedDropsSelfSacrificingObjects(t *testing.T) {
	t.Parallel()
	e, spell, pool, own := reflOwnBoard(t, 9707, reflRedProbeOwn, "Reflecting Pool", reflSacLand)
	sacMa := e.G.Obj(own[0]).Face().ManaAbilities()[0]
	if tier, c, _ := pay.PaymentPlanAbilityTier(asPayer(e), 0, own[0], sacMa); tier == pay.TierDeferred || !c.Sacrifice {
		t.Fatalf("precondition: the sac land classifies %v %+v, want a plannable sacrifice", tier, c)
	}
	if got := choiceAlts(t, e, pool); len(got) != 0 {
		t.Fatalf("Pool alternatives = %v, want none", got)
	}
	if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan != nil {
		t.Fatalf("plan relied on a removable reflected land: %+v", got.Plan)
	}
	if b := e.PotentialMana(0); b[state.ManaIndex('R')] != 0 {
		t.Fatalf("PotentialMana R = %d, want 0 (bound agrees with the planner)", b[state.ManaIndex('R')])
	}
	mtn := onBoard(t, e, 0, ppMountain)
	e.G.Obj(mtn).Tapped = true
	if got := choiceColours(choiceAlts(t, e, pool)); !slices.Equal(got, []string{"R"}) {
		t.Fatalf("with a Mountain the Pool offers %v, want [R]", got)
	}
	reflPlanAndRun(t, e, spell, pool, "R")
}

const reflRedProbeOwn = reflectedRedProbe

// CR 106.7: two lone Reflecting Pools reflect nothing.
func TestPaymentPlanReflectedTwoPoolsReflectNothing(t *testing.T) {
	t.Parallel()
	e, spell, _, _ := reflOwnBoard(t, 9708, reflGreenProbe, "Reflecting Pool")
	poolCard := corpusCardOrFatal(t, "Reflecting Pool")
	p2 := onBoardCard(t, e, 0, poolCard)
	if o := e.G.Obj(p2); o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatal("precondition: second Pool not untapped on the battlefield")
	}
	// The walk must run and see both Pools, or this test would pass with the
	// whole own-reflection feature unregistered.
	if ma := poolCard.Faces[0].Abilities[0]; !pay.PaymentPlanReflectedShape(ma) {
		t.Fatal("precondition: Reflecting Pool shape not admitted (own-reflection absent)")
	}
	if cols := effects.ManaReflectedCandidates(e, effects.NewCtxPtr(p2, 0, effects.CtxInit{}), poolCard.Faces[0].Abilities[0]); len(cols) != 0 {
		t.Fatalf("two Pools reflected %v, want none (the walk ran and reflected nothing)", cols)
	}
	if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan != nil {
		t.Fatalf("two Pools planned %+v", got.Plan)
	}
	b := e.PotentialMana(0)
	for _, c := range "WUBRG" {
		if n := b[state.ManaIndex(byte(c))]; n != 0 {
			t.Errorf("PotentialMana %c = %d, want 0 (CR 106.7)", c, n)
		}
	}
}

func TestPaymentPlanReflectedOwnBotAnswerValidates(t *testing.T) {
	t.Parallel()
	e, spell, src, _ := reflOwnBoard(t, 9709, reflGreenProbe, "Reflecting Pool", reflForest)
	d := paymentPlanReask(t, e)
	act := ppAction(t, d, spell)
	plan := decision.ClonePaymentPlan(act.Plans[0])
	if len(plan.Activations) != 1 || plan.Activations[0].Source != src {
		t.Fatalf("bot plan = %+v, want the Pool's one step", plan)
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

// The shape predicate over the corpus's real ManaReflected lines.
func TestPaymentPlanReflectedShapeOverCorpus(t *testing.T) {
	t.Parallel()
	want := map[string]bool{
		"Exotic Orchard": true, "Fellwar Stone": true, "Quirion Explorer": true, "Sylvok Explorer": true,
		"Reflecting Pool": true, "Cactus Preserve": true, "Naga Vitalist": true, "Horizon of Progress": true,
		"Harvester Druid": true, "Gond Gate": true, "Plaza of Harmony": true, "Star Compass": true,
		"Mox Amber": true, "Plaza of Heroes": true, "Meteor Crater": true, "Chrome Mox": true,
		"Pit of Offerings": true, "Corrupted Grafstone": true, "The Grey Havens": true,
		"Incubation Druid": false, "Benthic Explorers": false, "Squandered Resources": false,
	}
	for name, ok := range want {
		var got, seen bool
		for _, ab := range corpusCardOrFatal(t, name).Faces[0].Abilities {
			if ab.APIKind() == cards.APIManaReflected {
				seen = true
				got = pay.PaymentPlanReflectedShape(ab)
			}
		}
		if !seen {
			t.Errorf("%s: no ManaReflected ability", name)
		} else if got != ok {
			t.Errorf("%s: shape = %v, want %v", name, got, ok)
		}
	}
}
