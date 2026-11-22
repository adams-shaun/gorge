package pay

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPayPlanLiteralManaFeeTier pins the literal-mana-fee classification
// (Painted Bluffs's "{1}, {T}: Add one mana of any color" class): the fee is
// the one mana part a last-resort step may carry beside its tap, a coloured
// fee, an X and any non-mana extra part stay outside it, and a plain cast
// shape does not read as a fee.
func TestPayPlanLiteralManaFeeTier(t *testing.T) {
	t.Parallel()
	fee := costvocab.Cost{Tap: true, Generic: 1}
	if n, ok := PaymentPlanManaFeeOf(fee); !ok || n != 1 {
		t.Fatalf("PaymentPlanManaFeeOf({1},{T}) = %d,%v; want 1,true", n, ok)
	}
	for name, c := range map[string]costvocab.Cost{
		"colored fee":  {Tap: true, Generic: 1, Colored: state.Mana{state.MU: 1}},
		"untapped fee": {Generic: 1},
		"x fee":        {Tap: true, Generic: 1, X: 1},
		"discard fee":  {Tap: true, Generic: 1, Discard: []costvocab.CostPart{{N: 1}}},
		"life fee":     {Tap: true, Generic: 1, Life: 1},
		"sac fee":      {Tap: true, Generic: 1, Sac: []costvocab.CostPart{{N: 1, Spec: "CARDNAME"}}},
		"pure tap":     {Tap: true},
		"no fee":       {Tap: true, Generic: 0},
	} {
		if n, ok := PaymentPlanManaFeeOf(c); ok {
			t.Fatalf("%s: PaymentPlanManaFeeOf admits a fee (%d)", name, n)
		}
	}
	// The shape tier classifies the fee ability last-resort with the fee as
	// its disclosed consequence, and a non-mana extra part still defers.
	ma := &cards.SA{Kind: "AB", API: "Mana", Params: map[string]string{"Produced": "Any"}}
	tier, c, detail, rider := PaymentPlanShapeTierOf(ma, fee)
	if tier != TierLastResort || c.Fee != 1 || detail != "source:last_resort" || rider {
		t.Fatalf("fee ability classified tier=%d consequence=%+v detail=%q rider=%v", tier, c, detail, rider)
	}
	discard := fee
	discard.Discard = []costvocab.CostPart{{N: 1}}
	tier, c, detail, rider = PaymentPlanShapeTierOf(ma, discard)
	if tier != TierDeferred || detail != "source:last_resort" || rider {
		t.Fatalf("fee+discard classified tier=%d consequence=%+v detail=%q rider=%v", tier, c, detail, rider)
	}
}

// feeSearchEnv is the search env the planner wires: settle with the ordinary
// solver (paymentSearchEnv's body without the engine).
func feeSearchEnv() Env {
	return Env{
		Settle: func(c Cost, pool state.Mana, life int32) (state.Mana, bool) {
			paid, ok := ResolveManaWith(c, pool, state.Mana{}, [8]state.Mana{}, life, false, PipRider{}, nil)
			return paid.Pool, ok
		},
	}
}

func feeAlt(source state.ObjID, m state.Mana, fee uint32) Alt {
	return Alt{
		Activation: decision.PaymentActivation{Source: source,
			Ability: decision.PaymentAbility{Kind: "printed", Index: 1}, Produces: ManaAmount(m)},
		Mana: m, Tier: TierLastResort, Consequence: Consequence{Fee: fee}, Flex: 1, FlexAll: 1,
	}
}

func plainAlt(source state.ObjID, m state.Mana) Alt {
	a := feeAlt(source, m, 0)
	a.Tier = TierNormal
	a.Consequence = Consequence{}
	return a
}

func colored(sym byte, n int32) state.Mana {
	var m state.Mana
	m[state.ManaIndex(sym)] = n
	return m
}

// TestPayPlanLiteralManaFeeSearch pins the fee-aware search: the fee is
// charged from the floating mana the branch accumulated BEFORE the
// production (CR 605.3b), the witness records the fee charge in the order
// the executor runs, and a fee its branch cannot fund is dropped.
func TestPayPlanLiteralManaFeeSearch(t *testing.T) {
	t.Parallel()
	// pool 0, one {U} producer and one {1},{T}: Add {B} fee unit, cost {B}:
	// the fee is charged from the producer's {U} before {B} joins it.
	choices := [][]Alt{
		{plainAlt(1, colored('U', 1))},
		{feeAlt(2, colored('B', 1), 1)},
	}
	cost := Cost{Colored: colored('B', 1)}
	ctx := NewRankContext(choices, [5]int{})
	s := Run(cost, state.Mana{}, 20, ctx, choices, Classes(choices), feeSearchEnv())
	if s.Best == nil {
		t.Fatalf("no plan for {B} over a {U} producer and a {1},{T}: Add {B} fee unit")
	}
	if got := len(s.Best.Activations); got != 2 {
		t.Fatalf("plan has %d activations, want 2", got)
	}
	if src := s.Best.Activations[0].Source; src != 1 {
		t.Fatalf("plan's first step is source %d, want the producer (1)", src)
	}
	if c := s.Best.Activations[1].Consequence; c == nil || c.Fee != 1 {
		t.Fatalf("plan's second step carries consequence %+v, want the fee", s.Best.Activations[1].Consequence)
	}
	if after := s.Best.PoolAfter; after != (decision.ManaAmount{}) {
		t.Fatalf("plan leaves %v floating, want exactly paid", after)
	}

	// The same fee unit with nothing to fund its fee: not a plan.
	s = Run(cost, state.Mana{}, 20, ctx, [][]Alt{{feeAlt(2, colored('B', 1), 1)}}, Classes([][]Alt{{feeAlt(2, colored('B', 1), 1)}}), feeSearchEnv())
	if s.Best != nil {
		t.Fatalf("unfunded fee planned anyway: %+v", s.Best)
	}

	// A fee whose only funding mana is the pip colour the cost needs: the
	// witness refuses the order (the {B} producer's unit must fund the pip,
	// not the fee), and the search falls back to the plan without the fee.
	prod := colored('B', 1)
	choices = [][]Alt{{plainAlt(1, prod)}, {feeAlt(2, colored('R', 1), 1)}}
	ctx = NewRankContext(choices, [5]int{})
	s = Run(cost, state.Mana{}, 20, ctx, choices, Classes(choices), feeSearchEnv())
	if s.Best == nil {
		t.Fatalf("no plan for {B} over a {B} producer beside an unfundable fee unit")
	}
	for _, a := range s.Best.Activations {
		if a.Source == 2 {
			t.Fatalf("plan spends the fee unit whose charge would eat the {B} pip: %+v", s.Best)
		}
	}
}
