package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const payPlanFeeIsland = "Name:Island\nTypes:Basic Land Island\nOracle:x\n"

// payPlanFeeBluffsPlay puts Painted Bluffs and Cave of Temptation (each
// "{T}: Add {C}. {1}, {T}: Add one mana of any color."), Stensia Bloodhall
// ("{3}{B}{R}, {T}: deals 2 damage") and plains Islands on p0's battlefield,
// with the pool empty, and returns Bloodhall's ping potential play (false:
// the potential walk does not list it). The ping's {B}/{R} has no printed
// source: only the two any-color abilities make it, and each charges its {1}
// literal mana fee from the floating mana before its production resolves
// (CR 605.3a, 601.2b).
func payPlanFeeBluffsPlay(t *testing.T, seed uint64, islands, prefloat int) (PotentialPlan, bool) {
	t.Helper()
	e, _, _ := newFixtureDeck(t, seed, ppFiller)
	e.paidManaCensus = true
	onBoardCard(t, e, 0, corpusCard(t, "Painted Bluffs"))
	onBoardCard(t, e, 0, corpusCard(t, "Cave of Temptation"))
	bloodhall := onBoardCard(t, e, 0, corpusCard(t, "Stensia Bloodhall"))
	for i := 0; i < islands; i++ {
		onBoard(t, e, 0, payPlanFeeIsland)
	}
	ppPriority(t, e)
	if prefloat > 0 {
		// Float prefloat mana from plain islands first, as the recorded game's
		// seat had: the fees are charged from the floating mana (CR 605.3b).
		for i := 0; i < prefloat; i++ {
			ppChoose(t, e, func(o decision.Option) bool { return o.Kind == "activate" && strings.Contains(o.Label, "Island") })
			ppPriority(t, e)
		}
		if got := e.G.Players[0].Pool.Total(); got != int32(prefloat) {
			t.Fatalf("precondition: floated %d mana, want %d", got, prefloat)
		}
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == optAbility && state.ObjID(pp.Action.Obj) == bloodhall {
			return pp, true
		}
	}
	return PotentialPlan{}, false
}

// TestPayPlanLiteralManaFeeTier pins what prices the literal-mana-fee mana
// ability class (Painted Bluffs' "{1},{T}: Add one mana of any color"): the
// payment plan's tier ladder defers it (engine/rules/pay/planshape.go
// PaymentPlanShapeTierOf: any mana part in the activation cost), so the
// potential planner's prefix script is the pricer -- it plays the fee
// abilities on engine clones, charging each {1} from the floating mana
// before the production resolves, exactly what the kernel's filter search
// prices (dirac/oracle gorge.go prScript; the kernel's actScriptPays).
//
// Two of the five islands are tapped for mana first (the recorded game's
// seat had floating mana): the two {1} fees are charged from it, the two
// productions float {B}/{R}, and the three remaining plain taps pay the
// {3}: the play is priced, its script the fee activations with a colour ask
// paying the production. Four islands leave one tap short of the {3}: the
// play is not priced.
func TestPayPlanLiteralManaFeeTier(t *testing.T) {
	t.Parallel()
	t.Run("the fee class is priced, its fee charged before the production", func(t *testing.T) {
		pp, ok := payPlanFeeBluffsPlay(t, 9801, 5, 2)
		if !ok {
			t.Fatal("the ping is not a potential play")
		}
		if pp.Reason != "" {
			t.Fatalf("five islands: reason=%q detail=%q, want a priced play", pp.Reason, pp.Detail)
		}
		if len(pp.Script) == 0 {
			t.Fatalf("five islands: no prefix script (plan=%+v), want the fee abilities' script", pp.Plan)
		}
		activated, coloured := 0, false
		for _, s := range pp.Script {
			for _, k := range s.Picks {
				if k.Kind == "activate" {
					activated++
				}
				if strings.Contains("WUBRG", k.ManaSymbol) {
					coloured = true
				}
			}
		}
		if activated < 2 || !coloured {
			t.Fatalf("script %d steps: %d mana activations, coloured ask %t, want both fee abilities activated and a production colour paid", len(pp.Script), activated, coloured)
		}
	})
	t.Run("a fee the pool cannot fund drops the play", func(t *testing.T) {
		pp, ok := payPlanFeeBluffsPlay(t, 9802, 4, 2)
		if ok && pp.Reason == "" {
			t.Fatalf("two islands fund both fees but leave nothing for the {3}: priced anyway (%+v, script %d steps)", pp.Plan, len(pp.Script))
		}
	})
}
