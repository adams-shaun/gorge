package azmcts

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// strandCards: a {1}{U} creature, and a {B} sorcery that must target a
// creature of yours that entered this turn and makes each opponent lose 20
// life. Over an Island, a Swamp and a rock tapping for {C}{C}, the planner
// pays {1}{U} with the Island and the Swamp (no surplus beats the rock's
// spare {C}) and strands the sorcery, whose target exists only once the
// creature has resolved; paying with the Island and the rock keeps the Swamp
// for it, and the game is won this turn.
func strandCards(t *testing.T) (creature, finisher, island, swamp, rock *cards.Card) {
	t.Helper()
	creature = payWindowCard(t, "Name:Strand Scout\nManaCost:1 U\nTypes:Creature Human Scout\nPT:1/1\nOracle:x\n")
	finisher = payWindowCard(t, "Name:Strand Finale\nManaCost:B\nTypes:Sorcery\nA:SP$ Pump | ValidTgts$ Creature.YouCtrl+ThisTurnEntered | TgtPrompt$ Pick a newcomer | NumAtt$ +1 | SubAbility$ FinaleDrain | SpellDescription$ A newcomer grows; your foes falter.\nSVar:FinaleDrain:DB$ LoseLife | Defined$ Player.Opponent | LifeAmount$ 20\nOracle:x\n")
	island = payWindowCard(t, "Name:Island\nTypes:Basic Land Island\nOracle:x\n")
	swamp = payWindowCard(t, "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n")
	rock = payWindowCard(t, "Name:Strand Rock\nManaCost:0\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Produced$ C | Amount$ 2 | SpellDescription$ Two colourless.\nOracle:x\n")
	return
}

// strandRoot plays seat 0 (the three sources, scouts and finales) against a
// deck of Islands with the auto-pay bot until seat 0 holds main-phase priority
// with a scout and a finale in hand, no creature, and exactly the Island,
// the Swamp and the rock untapped: the first window a scout is castable.
func strandRoot(t *testing.T) (*rules.Engine, *cards.Card) {
	t.Helper()
	creature, finisher, island, swamp, rock := strandCards(t)
	d0 := []*cards.Card{island, swamp, rock}
	for i := 0; i < 12; i++ {
		d0 = append(d0, creature, finisher)
	}
	var d1 []*cards.Card
	for i := 0; i < 27; i++ {
		d1 = append(d1, island)
	}
	holds := func(g *state.Game, c *cards.Card) bool {
		for _, id := range g.Zone(state.ZHand, 0) {
			if g.Obj(id).Card == c {
				return true
			}
		}
		return false
	}
	ready := func(e *rules.Engine, d *decision.Decision) bool {
		g := e.G
		if d.Player != 0 || d.Kind != decision.KPriority || g.Active != 0 || len(g.Stack) != 0 ||
			(g.Step != state.StepMain1 && g.Step != state.StepMain2) || !holds(g, creature) || !holds(g, finisher) {
			return false
		}
		bf := g.Zone(state.ZBattlefield, 0)
		if len(bf) != 3 {
			return false
		}
		for _, id := range bf {
			if o := g.Obj(id); o.Tapped || (o.Card != island && o.Card != swamp && o.Card != rock) {
				return false
			}
		}
		return true
	}
	for seed := uint64(1); seed < 300; seed++ {
		e := rules.New(rules.Config{Seed: seed, Names: []string{"a", "b"}, Decks: [][]*cards.Card{d0, d1}})
		e.Advance()
		bots := [2]*heuristic.Bot{heuristic.NewBot(seed).EnableAutoPayMana(), heuristic.NewBot(seed + 1).EnableAutoPayMana()}
		board := botpolicy.NewBoard(2)
		for steps := 0; steps < 3000 && !e.G.Over; steps++ {
			d := e.Pending()
			if ready(e, d) {
				return e, creature
			}
			if d.Kind == decision.KPriority {
				e.EnsurePaymentActions()
				d = e.Pending()
			}
			in, err := bots[d.Player].DecideBoard(context.Background(), botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), *d)
			if err != nil {
				t.Fatal(err)
			}
			if err := e.Submit(in); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Fatal("no stranding window reached")
	return nil, nil
}

// The planner's own plan for the scout strands the finale; the search, given
// the alternative plans, casts the scout with the Island and the rock and
// keeps the Swamp. Without them it has no candidate that does.
func TestAZSearchFindsThePlanThatKeepsTheFollowUp(t *testing.T) {
	e, creature := strandRoot(t)
	d := e.Pending()
	e.EnsurePaymentActions()
	var scout decision.PaymentAction
	for _, a := range d.PaymentActions {
		if e.G.Obj(a.Cast.Object).Card == creature {
			scout = a
		}
	}
	if len(scout.Plans) != 1 {
		t.Fatalf("scout action %+v, want one ordinary plan", scout)
	}
	tapsSwamp := func(plan decision.PaymentPlan) bool {
		for _, s := range plan.Activations {
			if e.G.Obj(s.Source).Card.Faces[0].Name == "Swamp" {
				return true
			}
		}
		return false
	}
	if !tapsSwamp(scout.Plans[0]) {
		t.Fatalf("plan 0 %+v keeps the Swamp: no strand to find", scout.Plans[0])
	}
	search := func(plans int) Result {
		t.Helper()
		obs := searchprobe.NewCollector(d.Player)
		src, err := newTestClairvoyant(e, obs)
		if err != nil {
			t.Fatal(err)
		}
		opts := DefaultOptions()
		opts.Sims, opts.AutoPayment, opts.PaymentPlans = 64, true, plans
		res, err := Search(context.Background(), Root{Engine: e, Decision: d, Observer: obs, NoBot: true}, src, nil, opts)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	alternative := func(res Result) bool {
		for _, c := range res.Candidates {
			if c.Payment != nil && c.Payment.ActionID == scout.ID && !tapsSwamp(c.Payment.Plan) {
				return true
			}
		}
		return false
	}
	if res := search(0); alternative(res) {
		t.Fatalf("plan 0 only: a candidate keeps the Swamp: %+v", res.Candidates)
	}
	res := search(3)
	if !alternative(res) {
		t.Fatalf("no candidate pays the scout without the Swamp: %+v", res.Candidates)
	}
	in := res.Intent
	if in.Payment == nil || in.Payment.ActionID != scout.ID || tapsSwamp(in.Payment.Plan) {
		t.Fatalf("search answered %+v, want the scout paid with the Island and the rock", in)
	}
	if err := e.Submit(in); err != nil {
		t.Fatalf("the real engine rejects the searched plan: %v", err)
	}
}
