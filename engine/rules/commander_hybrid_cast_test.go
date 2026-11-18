package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCommanderHybridCastIsAShapeCastFromLands pins gorge's side of the
// fshadow pod-12900 divergence (class answer-missing|priority||action:cast):
// Kaust, Eyes of the Glade ({R/W}{G}) in the command zone with an untapped
// Forest and Scattered Groves (Forest Plains) and an empty pool. Forge offers
// "Cast Kaust" paid by tapping both lands; CR 107.4e lets the Groves' {W} pay
// {R/W} and CR 903.8 allows the command-zone cast, so the cast is legal.
//
// gorge offers it, by the route its design reserves for a hybrid cost: the
// payment planner declines a hybrid face for its shape (CR 601.2b announces
// it; pay.PlanCostDetail "cost:hybrid"), so no payment action names Kaust,
// and the decision's shape census (decision.PotentialShapeCasts) does. A seat
// then activates the lands for mana and casts from the pool. The divergence
// is the follower reading only Options and EnsurePaymentActions, not this
// census and route.
func TestCommanderHybridCastIsAShapeCastFromLands(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	kaust := corpusCommander(t, reg, "Kaust, Eyes of the Glade")
	e, cfg := colourIdentityGame(t, 12900, FormatCommander, kaust,
		nil, corpusCommander(t, reg, "Forest"), corpusCommander(t, reg, "Scattered Groves"))
	toMain1(t, e)
	cmd := e.G.Zone(state.ZCommand, 0)
	if len(cmd) != 1 || e.G.Obj(cmd[0]).Face().Name != "Kaust, Eyes of the Glade" {
		t.Fatalf("command zone = %v, want Kaust alone", cmd)
	}
	kid := cmd[0]
	forest := moveToBattlefieldByName(t, e, 0, "Forest")
	groves := moveToBattlefieldByName(t, e, 0, "Scattered Groves")
	e.G.Obj(groves).Tapped = false // it entered tapped; the pod's had untapped
	e.priorityRound()
	if p := e.G.Players[0].Pool; p.Total() != 0 {
		t.Fatalf("pool = %v, want empty: the lands alone must pay", p)
	}

	// No payment action: the planner declines the hybrid face for its shape.
	for _, a := range e.EnsurePaymentPlanAlternatives() {
		if a.Cast.Object == kid {
			t.Fatalf("payment action %+v names Kaust; the planner declines hybrid (cost:hybrid)", a)
		}
	}
	if got := e.PlanCastPayment(0, decision.PlannedCast{Object: kid, Origin: "command_zone"}); got.Plan != nil || got.Detail != "cost:hybrid" {
		t.Fatalf("Kaust plan verdict = %+v, want the cost:hybrid shape decline", got)
	}
	d := e.Pending()
	if !d.ShapeCastsKnown || !slices.Contains(d.PotentialShapeCasts, kid) {
		t.Fatalf("shape census known=%v %v, want Kaust (%d) in it", d.ShapeCastsKnown, d.PotentialShapeCasts, kid)
	}
	if hasCastOption(castOptions(t, e), kid) {
		t.Fatal("Kaust offered with an empty pool; the pool walk prices floating mana only")
	}

	// The route: tap the Forest for {G}, the Groves for {W}, cast from the pool.
	submitChoices(t, e, activateOption(t, e, forest))
	submitChoices(t, e, activateOption(t, e, groves))
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		submitChoices(t, e, manaOption(t, d, "W"))
	}
	if p := e.G.Players[0].Pool; p[state.MG] != 1 || p[state.MW] != 1 || p.Total() != 2 {
		t.Fatalf("pool = %v, want exactly {G}{W}", p)
	}
	opts := castOptions(t, e)
	if !hasCastOption(opts, kid) {
		t.Fatalf("Kaust not offered with {G}{W} floating: %+v", opts)
	}
	for _, o := range opts {
		if o.Obj == kid {
			submitChoices(t, e, o.Index)
			break
		}
	}
	if z := e.G.Obj(kid).Zone; z != state.ZStack {
		t.Fatalf("Kaust zone = %v after the cast, want the stack", z)
	}
	// The {R/W} face is chosen at payment; the pool funds W alone.
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 1 || d.Options[0].Kind != "pay_W" {
		t.Fatalf("pending after the cast = %+v, want the one-face hybrid choice Pay W", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	if p := e.G.Players[0].Pool; p.Total() != 0 {
		t.Fatalf("pool = %v after the cast, want empty ({W} paid {R/W}, {G} paid {G})", p)
	}
	commanderReplayCheck(t, e, cfg)
}
