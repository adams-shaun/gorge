package rules

// A trigger
// or replacement on source S affects only S's tier, a trigger/replacement on
// another object defers exactly the sources its filter can match, and only an
// unprovable global effect declines every plan of the affected player.
// Fixtures are authored IR or corpus cards read by name at test time; no
// Forge script text lives here.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/pay"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	interferenceBlueInstant = "Name:Blue Probe\nManaCost:U\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	interferenceOneInstant  = "Name:Generic Probe\nManaCost:1\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	interferenceIsland      = "Name:Island\nTypes:Basic Land Island\nOracle:x\n"
	interferenceSwamp       = "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"
)

// interferencePlanSources lists the planned sources of an outcome's witness.
func interferencePlanSources(got PaymentPlanOutcome) []state.ObjID {
	if got.Plan == nil {
		return nil
	}
	return paymentPlanSources(*got.Plan)
}

// interferenceTier classifies every mana ability src's face prints and
// returns the first classification, failing when the face prints none.
func interferenceTier(t *testing.T, e *Engine, src state.ObjID) (pay.Tier, pay.Consequence, string) {
	t.Helper()
	o := e.G.Obj(src)
	abilities := o.Face().ManaAbilities()
	if len(abilities) == 0 {
		// Basic lands carry their intrinsic ability only through the window.
		for _, u := range e.paymentPlanManaUnits(o.Controller) {
			if u.ID == src {
				for _, alt := range u.Alts {
					abilities = append(abilities, alt.Ma)
				}
			}
		}
	}
	if len(abilities) == 0 {
		t.Fatalf("precondition: %s has no mana ability", o.Face().Name)
	}
	return pay.PaymentPlanAbilityTier(asPayer(e), o.Controller, src, abilities[0])
}

// An opponent's permanent whose tap/mana/untap text can only reach its own
// sources -- Mana Vault's doesn't-untap, City of Brass's self-tap damage,
// Claustrophobia's enchanted-creature doesn't-untap -- and an opponent's face
// that neither grants sunburst (Engineered Explosives prints its own) nor
// taxes an untargeted spell (Syr Elenora's ValidTarget$ RaiseCost) leaves
// seat 0's Island plan for an untargeted {U} instant exactly as it was.
func TestPaymentPlanInterferenceOpponentPermanentsLeaveIslandPlan(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Mana Vault", "City of Brass", "Claustrophobia", "Engineered Explosives", "Syr Elenora, the Discerning", "Grim Monolith"} {
		t.Run(name, func(t *testing.T) {
			e, _, spell := newFixtureDeck(t, 9901, interferenceBlueInstant)
			island := onBoard(t, e, 0, interferenceIsland)
			before := e.PlanCastPayment(0, paymentCast(spell))
			if before.Plan == nil {
				t.Fatalf("control: no Island plan without %s: %+v", name, before)
			}
			onBoardCard(t, e, 1, corpusCard(t, name))
			after := e.PlanCastPayment(0, paymentCast(spell))
			if after.Plan == nil {
				t.Fatalf("opponent's %s disables the Island plan: %+v", name, after)
			}
			if !reflect.DeepEqual(*before.Plan, *after.Plan) {
				t.Fatalf("opponent's %s changed the plan:\n before %+v\n after  %+v", name, *before.Plan, *after.Plan)
			}
			if got := interferencePlanSources(after); len(got) != 1 || got[0] != island {
				t.Fatalf("plan sources = %v, want [Island %d]", got, island)
			}
			if e.paymentPlanManaInterference() {
				t.Fatalf("opponent's %s reads as a global mana effect", name)
			}
		})
	}
}

// Manabarbs' TapsForMana trigger (ValidCard$ Land, no Activator$) matches a
// tap of ANY player's land, so it defers every land -- seat 0's included --
// by name, while a non-land source of seat 0 still funds the plan: the
// trigger's filter, not its presence, decides.
func TestPaymentPlanInterferenceManabarbsDefersOnlyWhatItMatches(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9902, interferenceBlueInstant)
	island := onBoard(t, e, 0, interferenceIsland)
	onBoardCard(t, e, 1, corpusCard(t, "Manabarbs"))
	got := e.PlanCastPayment(0, paymentCast(spell))
	if got.Plan != nil || got.Reason != "insufficient" || !strings.Contains(got.Detail, "Manabarbs") {
		t.Fatalf("Island under Manabarbs = %+v, want insufficient naming Manabarbs", got)
	}
	if tier, _, detail := interferenceTier(t, e, island); tier != pay.TierDeferred || !strings.Contains(detail, "Manabarbs") {
		t.Fatalf("Island tier = %d %q, want deferred naming Manabarbs", tier, detail)
	}
	rock := onBoard(t, e, 0, "Name:Blue Rock\nManaCost:2\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Produced$ U | SpellDescription$ Add {U}.\nOracle:x\n")
	got = e.PlanCastPayment(0, paymentCast(spell))
	if srcs := interferencePlanSources(got); len(srcs) != 1 || srcs[0] != rock {
		t.Fatalf("plan under Manabarbs = %+v (sources %v), want the artifact %d only", got, srcs, rock)
	}
	if e.paymentPlanManaInterference() {
		t.Fatal("Manabarbs reads as a global mana effect")
	}
}

// Seat 0's own Wild Growth on Island A: its TapsForMana ValidCard$
// Card.AttachedBy matches only A, and its body is a fixed `Add {G}` to the
// land's controller, so A is priced as {U}{G} while the untouched Island B
// stays {U}.
func TestPaymentPlanInterferenceWildGrowthPricesOnlyEnchantedLand(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9903, interferenceBlueInstant)
	islandA := onBoard(t, e, 0, interferenceIsland)
	islandB := onBoard(t, e, 0, interferenceIsland)
	growth := onBoardCard(t, e, 0, corpusCard(t, "Wild Growth"))
	e.G.Obj(growth).AttachedTo = islandA
	if o := e.G.Obj(growth); o.Zone != state.ZBattlefield || o.Face().Triggers[0].Mode != "TapsForMana" {
		t.Fatalf("precondition: Wild Growth zone %v triggers %+v", o.Zone, o.Face().Triggers)
	}
	if tier, _, detail := interferenceTier(t, e, islandA); tier != pay.TierNormal {
		t.Fatalf("enchanted Island tier = %d %q, want normal (priced)", tier, detail)
	}
	if got := interferenceAltMana(t, e, islandA); got != (state.Mana{state.MU: 1, state.MG: 1}) {
		t.Fatalf("enchanted Island alternative = %v, want {U}{G}", got)
	}
	if got := interferenceAltMana(t, e, islandB); got != (state.Mana{state.MU: 1}) {
		t.Fatalf("untouched Island alternative = %v, want {U}", got)
	}
	// With B tapped, A alone funds the {U} plan, disclosing {U}{G}.
	e.G.Obj(islandB).Tapped = true
	got := e.PlanCastPayment(0, paymentCast(spell))
	if srcs := interferencePlanSources(got); len(srcs) != 1 || srcs[0] != islandA {
		t.Fatalf("only the enchanted Island left = %+v (sources %v), want Island A %d", got, srcs, islandA)
	}
}

// Crypt Ghast's TapsForMana is ValidCard$ Swamp | Activator$ You with a fixed
// `Add {B}`: seat 0's Swamp is priced {B}{B} (the Island stays {U}), and an
// opponent's Crypt Ghast leaves seat 0's Swamp at {B}.
func TestPaymentPlanInterferenceCryptGhastScopesByActivator(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9904, interferenceOneInstant)
	swamp := onBoard(t, e, 0, interferenceSwamp)
	island := onBoard(t, e, 0, interferenceIsland)
	ghast := onBoardCard(t, e, 0, corpusCard(t, "Crypt Ghast"))
	if o := e.G.Obj(ghast); o.Zone != state.ZBattlefield || o.Controller != 0 || o.Face().Triggers[0].Mode != "TapsForMana" {
		t.Fatalf("precondition: Crypt Ghast zone %v controller %d", o.Zone, o.Controller)
	}
	if tier, _, detail := interferenceTier(t, e, swamp); tier != pay.TierNormal {
		t.Fatalf("own Swamp under own Crypt Ghast tier = %d %q, want normal", tier, detail)
	}
	if got := interferenceAltMana(t, e, swamp); got != (state.Mana{state.MB: 2}) {
		t.Fatalf("Swamp under Crypt Ghast = %v, want {B}{B}", got)
	}
	if got := interferenceAltMana(t, e, island); got != (state.Mana{state.MU: 1}) {
		t.Fatalf("Island under Crypt Ghast = %v, want {U}", got)
	}
	if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan == nil {
		t.Fatalf("no plan under Crypt Ghast: %+v", got)
	}
	// Hand the Ghast to the opponent: "you" is now seat 1.
	e.G.Obj(ghast).Controller = 1
	e.G.SetZone(state.ZBattlefield, 0, interferenceRemoveID(e.G.Zone(state.ZBattlefield, 0), ghast))
	e.G.SetZone(state.ZBattlefield, 1, append(e.G.Zone(state.ZBattlefield, 1), ghast))
	e.staticEpoch, e.activeEpoch = -1, -1
	if tier, _, detail := interferenceTier(t, e, swamp); tier != pay.TierNormal {
		t.Fatalf("own Swamp under opponent's Crypt Ghast tier = %d %q, want normal", tier, detail)
	}
	if got := interferenceAltMana(t, e, swamp); got != (state.Mana{state.MB: 1}) {
		t.Fatalf("Swamp under opponent's Crypt Ghast = %v, want {B}", got)
	}
}

// interferenceAltMana is the Mana of src's single planner alternative.
func interferenceAltMana(t *testing.T, e *Engine, src state.ObjID) state.Mana {
	t.Helper()
	o := e.G.Obj(src)
	for _, u := range e.paymentPlanManaUnits(o.Controller) {
		if u.ID != src {
			continue
		}
		alts := pay.PaymentPlanUnitAlternatives(asPayer(e), u)
		if len(alts) != 1 {
			t.Fatalf("%s has %d planner alternatives, want 1", o.Face().Name, len(alts))
		}
		if alts[0].Tier != pay.TierNormal {
			t.Fatalf("%s alternative tier = %d", o.Face().Name, alts[0].Tier)
		}
		return alts[0].Mana
	}
	t.Fatalf("%s is not a mana unit of seat %d", o.Face().Name, o.Controller)
	return state.Mana{}
}

func interferenceRemoveID(ids []state.ObjID, id state.ObjID) []state.ObjID {
	out := make([]state.ObjID, 0, len(ids))
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
}

// Seat 0's own City of Brass and Mana Vault are last resort -- damage:1 and
// no_untap, through the classifier -- so they never appear in a plan while
// another seat-0 source still funds one, and alone they fund a plan that
// discloses the cheaper consequence (City's damage 1 = 3 < Vault's 25).
func TestPaymentPlanInterferenceOwnCityAndVaultAreLastResort(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9905, interferenceOneInstant)
	city := onBoardCard(t, e, 0, corpusCard(t, "City of Brass"))
	vault := onBoardCard(t, e, 0, corpusCard(t, "Mana Vault"))
	e.G.Obj(vault).SummonSick = false
	tier, c, detail := interferenceTier(t, e, city)
	if tier != pay.TierLastResort || c != (pay.Consequence{Damage: 1}) || detail != "source:last_resort" {
		t.Fatalf("City of Brass = tier %d %+v %q, want last resort damage:1", tier, c, detail)
	}
	tier, c, detail = interferenceTier(t, e, vault)
	if tier != pay.TierLastResort || c != (pay.Consequence{NoUntap: true}) || detail != "source:last_resort" {
		t.Fatalf("Mana Vault = tier %d %+v %q, want last resort no_untap", tier, c, detail)
	}
	if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan == nil || len(got.Plan.Activations) != 1 ||
		got.Plan.Activations[0].Source != city || got.Plan.Activations[0].Consequence == nil ||
		*got.Plan.Activations[0].Consequence != (decision.PaymentConsequence{Damage: 1}) {
		t.Fatalf("plan from last-resort sources only = %+v, want City of Brass disclosing damage:1", got.Plan)
	}
	island := onBoard(t, e, 0, interferenceIsland)
	got := e.PlanCastPayment(0, paymentCast(spell))
	if srcs := interferencePlanSources(got); len(srcs) != 1 || srcs[0] != island {
		t.Fatalf("plan = %+v (sources %v), want the Island %d only", got, srcs, island)
	}
	if e.paymentPlanManaInterference() {
		t.Fatal("own City of Brass / Mana Vault read as a global mana effect")
	}
}

// Mana Reflection (ProduceMana | ValidActivator$ You | ValidCard$ Permanent)
// defers every source its controller could tap -- seat 0 has no plan, and the
// diagnostic names it -- while seat 1's sources are outside its filter.
func TestPaymentPlanInterferenceManaReflectionDefersControllerSources(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9906, interferenceBlueInstant)
	island := onBoard(t, e, 0, interferenceIsland)
	onBoardCard(t, e, 0, corpusCard(t, "Mana Reflection"))
	got := e.PlanCastPayment(0, paymentCast(spell))
	if got.Plan != nil || got.Reason != "insufficient" || !strings.Contains(got.Detail, "Mana Reflection") {
		t.Fatalf("seat 0 under own Mana Reflection = %+v, want insufficient naming Mana Reflection", got)
	}
	if tier, _, detail := interferenceTier(t, e, island); tier != pay.TierDeferred || !strings.Contains(detail, "Mana Reflection") {
		t.Fatalf("seat 0 Island tier = %d %q, want deferred naming Mana Reflection", tier, detail)
	}
	oppIsland := onBoard(t, e, 1, interferenceIsland)
	oppSpell := onHand(t, e, 1, interferenceBlueInstant)
	opp := e.PlanCastPayment(1, paymentCast(oppSpell))
	if srcs := interferencePlanSources(opp); len(srcs) != 1 || srcs[0] != oppIsland {
		t.Fatalf("seat 1 plan under seat 0's Mana Reflection = %+v (sources %v), want its Island %d", opp, srcs, oppIsland)
	}
	if e.paymentPlanManaInterference() {
		t.Fatal("a printed, scoped ProduceMana replacement reads as a global mana effect")
	}
}

// A ManaConvert static reaching the payer (the authored Celestial Dawn
// shape) is the one printed global effect: the ordinary solver does not
// apply the conversion, so no plan is offered and the reason names it.
func TestPaymentPlanInterferenceCelestialDawnIsGlobal(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9907, "Name:Green Instant Test\nManaCost:G\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n")
	onBoard(t, e, 0, "Name:Dawn Test\nTypes:Enchantment\nS:Mode$ ManaConvert | ValidPlayer$ You | ManaConversion$ White->AnyColor nonWhite<-C | Description$ Fixture: white as any colour, other mana only as colorless.\nOracle:x\n")
	elf := onBoard(t, e, 0, "Name:Elf Test\nTypes:Creature Elf\nPT:1/1\nA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Add G.\nOracle:x\n")
	e.G.Obj(elf).SummonSick = false
	got := e.PlanCastPayment(0, paymentCast(spell))
	if got.Plan != nil || got.Reason != "unsupported" || !strings.HasPrefix(got.Detail, "global_mana_effect") {
		t.Fatalf("plan under a payer-reaching ManaConvert = %+v, want unsupported global_mana_effect", got)
	}
	if got.Detail != "global_mana_effect:Dawn Test" {
		t.Fatalf("detail = %q, want it to name Dawn Test", got.Detail)
	}
	if ok, detail := e.paymentPlanGlobalManaEffect(0, spell); !ok || detail != got.Detail {
		t.Fatalf("paymentPlanGlobalManaEffect(0) = %v %q", ok, detail)
	}
	// ValidPlayer$ You: the opponent's payments are untouched.
	if ok, _ := e.paymentPlanGlobalManaEffect(1, 0); ok {
		t.Fatal("seat 0's ManaConvert reaches seat 1's payments")
	}
}

// Damping Sphere gates on the size of the production, so a land with a dynamic
// Amount$ is measured: Gaea's Cradle tapping for two would make {C}, not {G}{G}.
func TestPaymentPlanInterferenceSizesADynamicProduction(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9909, interferenceOneInstant)
	cradle := onBoard(t, e, 0, "Name:Cradle Test\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ G | Amount$ X | SpellDescription$ Add G.\nSVar:X:Count$Valid Creature.YouCtrl\nOracle:x\n")
	onBoard(t, e, 0, "Name:Bear Test\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	got := e.PlanCastPayment(0, paymentCast(spell))
	if got.Plan == nil || len(got.Plan.Activations) != 1 || got.Plan.Activations[0].Source != cradle {
		t.Fatalf("plan with one creature = %+v, want the land taps for one", got)
	}
	onBoard(t, e, 0, "Name:Bear Test\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	onBoard(t, e, 1, "Name:Sphere Test\nTypes:Artifact\nR:Event$ ProduceMana | ActiveZones$ Battlefield | ValidCard$ Land | ManaAmount$ GE2 | ReplaceWith$ AsC\nSVar:AsC:DB$ ReplaceMana | ReplaceMana$ C\nOracle:x\n")
	if got = e.PlanCastPayment(0, paymentCast(spell)); got.Plan != nil {
		t.Fatalf("plan with two creatures = %+v, want the replaced production deferred", got)
	}
}

// An effect-created ProduceMana replacement WITH a matchable filter is scoped
// by the real matcher: ValidCard$ Card.Self on Island A defers A only, and a
// ValidActivator$ You effect of seat 1 leaves seat 0 alone. (The unscoped
// form is TestPaymentPlanDeclinesEffectCreatedProduceManaReplacement's global
// decline.)
func TestPaymentPlanInterferenceScopedEffectCreatedReplacementDefersItsCard(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9908, interferenceBlueInstant)
	islandA := onBoard(t, e, 0, interferenceIsland)
	islandB := onBoard(t, e, 0, interferenceIsland)
	e.AddContinuous(state.ContinuousEffect{Source: islandA, Controller: 0,
		ReplacementEvent: "ProduceMana", ReplacementBody: "DB$ ReplaceMana | ReplaceAmount$ 2",
		ReplacementParams: map[string]string{"ValidCard": "Card.Self"}})
	e.AddContinuous(state.ContinuousEffect{Source: islandB, Controller: 1,
		ReplacementEvent: "ProduceMana", ReplacementBody: "DB$ ReplaceMana | ReplaceAmount$ 2",
		ReplacementParams: map[string]string{"ValidActivator": "You"}})
	if ok, detail := e.paymentPlanGlobalManaEffect(0, spell); ok {
		t.Fatalf("a scoped effect-created replacement is global: %q", detail)
	}
	got := e.PlanCastPayment(0, paymentCast(spell))
	if srcs := interferencePlanSources(got); len(srcs) != 1 || srcs[0] != islandB {
		t.Fatalf("plan = %+v (sources %v), want Island B %d only", got, srcs, islandB)
	}
}

// The unscoped effect-created replacement still declines every plan, now
// with the global diagnostic.
func TestPaymentPlanInterferenceUnscopedEffectCreatedReplacementIsGlobal(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9909, interferenceBlueInstant)
	source := onBoard(t, e, 0, interferenceIsland)
	e.AddContinuous(state.ContinuousEffect{Source: source, Controller: 0,
		ReplacementEvent: "ProduceMana", ReplacementBody: "DB$ ReplaceMana | ReplaceAmount$ 2"})
	got := e.PlanCastPayment(0, paymentCast(spell))
	if got.Plan != nil || got.Reason != "unsupported" || got.Detail != "global_mana_effect:Island" {
		t.Fatalf("plan under an unscoped effect-created replacement = %+v, want unsupported global_mana_effect:Island", got)
	}
	if !e.paymentPlanManaInterference() {
		t.Fatal("zero-argument wrapper misses the unscoped effect-created replacement")
	}
}

// A targeted spell under an opponent's Syr Elenora still declines: its cost
// depends on the target chosen at CR 601.2c. An untargeted one does not
// (TestPaymentPlanInterferenceOpponentPermanentsLeaveIslandPlan).
func TestPaymentPlanInterferenceTargetedSpellUnderSyrElenoraDeclines(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9910, "Name:Blue Pump\nManaCost:U\nTypes:Instant\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +1 | SpellDescription$ x\nOracle:x\n")
	onBoard(t, e, 0, interferenceIsland)
	onBoard(t, e, 0, interferenceIsland)
	onBoard(t, e, 0, interferenceIsland)
	onBoardCard(t, e, 1, corpusCard(t, "Syr Elenora, the Discerning"))
	got := e.PlanCastPayment(0, paymentCast(spell))
	if got.Plan != nil || got.Reason != "unsupported" || got.Detail != "shape:target_dependent_cost" {
		t.Fatalf("targeted spell under Syr Elenora = %+v, want unsupported shape:target_dependent_cost", got)
	}
}

// A face that GRANTS sunburst to a spell (Solar Array's Animate Keywords$
// Sunburst) still withholds the plan; merely printing Sunburst does not
// (Engineered Explosives, above).
func TestPaymentPlanInterferenceSunburstGrantStillDeclines(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9911, interferenceBlueInstant)
	onBoard(t, e, 0, interferenceIsland)
	onBoardCard(t, e, 0, corpusCard(t, "Solar Array"))
	got := e.PlanCastPayment(0, paymentCast(spell))
	if got.Plan != nil || got.Detail != "shape:mana_spent_reader" {
		t.Fatalf("plan beside a sunburst grant = %+v, want shape:mana_spent_reader", got)
	}
	if !pay.FaceGrantsSunburstForPlan(corpusCard(t, "Solar Array").Faces[0]) {
		t.Fatal("Solar Array is not recognised as a sunburst grant")
	}
	if pay.FaceGrantsSunburstForPlan(corpusCard(t, "Engineered Explosives").Faces[0]) {
		t.Fatal("Engineered Explosives' own K:Sunburst read as a grant")
	}
}

// The executor re-reads every remaining step's source through the same
// classifier (paymentPlanStepReady -> paymentPlanUnitAlternatives), so the
// corpus's unconditional Contamination arriving between the offer and the
// payment window defers the planned Mountains before any is tapped: nothing
// is activated and the manual window names the plan with source_changed (no
// V1 ability is left under the step's identity).
func TestPaymentPlanInterferenceArrivingAfterOfferStopsBeforeTapping(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9912, paymentPlanBlast)
	m1 := onBoard(t, e, 0, paymentPlanMountain)
	m2 := onBoard(t, e, 0, paymentPlanMountain)
	d := paymentPlanReask(t, e)
	a := paymentPlanActionFor(t, d, spell)
	submitPaymentPlan(t, e, d, a)
	onBoardReadyCard(t, e, 1, corpusCard(t, "Contamination"))
	start := len(e.L.Events)
	submitChoices(t, e, 0) // the target
	if n := paymentPlanTapsSince(e, start, m1) + paymentPlanTapsSince(e, start, m2); n != 0 {
		t.Fatalf("executor tapped %d planned sources a live Contamination defers", n)
	}
	nd := e.Pending()
	if nd == nil || nd.PaymentFallback == nil || nd.PaymentFallback.Reason != paymentFallbackSourceChanged || nd.PaymentFallback.PlanID != a.Plans[0].ID {
		t.Fatalf("pending = %s, want the manual window with source_changed for plan %s", paymentPlanPendingSummary(nd), a.Plans[0].ID)
	}
}

// A purely WIDENING ManaConvert static (Mycosynth Lattice: "players may spend
// mana as though it were mana of any color") cannot make a plan the ordinary
// solver priced unpayable, so it is not a global plan-blocker: the Island
// plan is offered, submits, and settles with no fallback. A RESTRICTING one
// (the corpus Celestial Dawn: other mana only as colorless) stays global, and
// with both on the battlefield the diagnostic names the restricting one.
func TestPaymentPlanInterferenceWideningManaConvertIsNotGlobal(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9913, interferenceBlueInstant)
	island := onBoard(t, e, 0, interferenceIsland)
	onBoardCard(t, e, 1, corpusCard(t, "Mycosynth Lattice"))
	if asEval(e).Conv(0, spell, pay.SASpell) == nil {
		t.Fatal("precondition: Mycosynth Lattice does not reach seat 0's payment")
	}
	if ok, detail := e.paymentPlanGlobalManaEffect(0, spell); ok {
		t.Fatalf("Mycosynth Lattice is a global mana effect: %q", detail)
	}
	d := paymentPlanReask(t, e)
	a := paymentPlanActionFor(t, d, spell)
	if srcs := paymentPlanSources(a.Plans[0]); len(srcs) != 1 || srcs[0] != island {
		t.Fatalf("plan under Mycosynth Lattice = %+v, want the Island %d", a.Plans[0], island)
	}
	submitPaymentPlan(t, e, d, a)
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Fatalf("spell zone after the planned cast = %s, want stack (pending %s)", z, paymentPlanPendingSummary(e.Pending()))
	}
	if !e.G.Obj(island).Tapped || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("Island tapped=%v pool=%v, want the Island to have paid exactly", e.G.Obj(island).Tapped, e.G.Players[0].Pool)
	}
	if nd := e.Pending(); nd != nil && nd.PaymentFallback != nil {
		t.Fatalf("planned cast under Mycosynth Lattice fell back: %s", paymentPlanPendingSummary(nd))
	}
}

func TestPaymentPlanInterferenceRestrictingManaConvertIsGlobal(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9914, interferenceBlueInstant)
	onBoard(t, e, 0, interferenceIsland)
	onBoardCard(t, e, 0, corpusCard(t, "Mycosynth Lattice"))
	onBoardCard(t, e, 0, corpusCard(t, "Celestial Dawn"))
	got := e.PlanCastPayment(0, paymentCast(spell))
	if got.Plan != nil || got.Reason != "unsupported" || got.Detail != "global_mana_effect:Celestial Dawn" {
		t.Fatalf("plan under Celestial Dawn + Mycosynth Lattice = %+v, want unsupported global_mana_effect:Celestial Dawn", got)
	}
	// Celestial Dawn's ValidPlayer$ You: seat 1 sees only the Lattice.
	if ok, detail := e.paymentPlanGlobalManaEffect(1, 0); ok {
		t.Fatalf("seat 0's Celestial Dawn reaches seat 1: %q", detail)
	}
}

// An effect-created Taps/TapsForMana registration -- a resolved Bubbling
// Muck's "until end of turn, whenever a player taps a Swamp for mana, that
// player adds an additional {B}", which lives as a state.Game.Delayed
// EffectRepeat entry, not on any face or static -- defers exactly the sources
// its matcher sees: the Swamp, not the Island. Before the delayed walk the
// planner priced the Swamp at {B} while its tap added {B}{B} (cardfuzz mirror
// seed 14886721532440673633, wrong_production).
func TestPaymentPlanInterferenceDelayedTapTriggerDefersItsSources(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9915, interferenceOneInstant)
	swamp := onBoard(t, e, 0, interferenceSwamp)
	island := onBoard(t, e, 0, interferenceIsland)
	if tier, _, _ := interferenceTier(t, e, swamp); tier != pay.TierNormal {
		t.Fatalf("control: the Swamp is tier %v before Bubbling Muck", tier)
	}

	muck := e.G.AddObject(corpusCard(t, "Bubbling Muck"), 0)
	muck.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), muck.ID))
	e.staticEpoch, e.activeEpoch = -1, -1
	muckID := muck.ID
	addMana(t, e, 0, "B")
	submitChoices(t, e, castOptionFor(t, e, muckID).Index)
	passUntilStackEmpty(t, e, 40)
	registered := false
	for _, dt := range e.G.Delayed {
		if dt.EffectRepeat && dt.EventMode == "TapsForMana" {
			registered = true
		}
	}
	if !registered {
		t.Fatalf("precondition: Bubbling Muck registered no TapsForMana effect trigger: %+v", e.G.Delayed)
	}

	tier, _, detail := interferenceTier(t, e, swamp)
	if tier != pay.TierDeferred || detail != "source:interference:Bubbling Muck" {
		t.Fatalf("Swamp under Bubbling Muck = tier %v %q, want deferred by Bubbling Muck", tier, detail)
	}
	if tier, _, detail := interferenceTier(t, e, island); tier != pay.TierNormal {
		t.Fatalf("Island under Bubbling Muck = tier %v %q, want normal", tier, detail)
	}
	got := e.PlanCastPayment(0, paymentCast(spell))
	if srcs := interferencePlanSources(got); len(srcs) != 1 || srcs[0] != island {
		t.Fatalf("plan under Bubbling Muck = %+v (sources %v), want the Island %d only", got, srcs, island)
	}
}
