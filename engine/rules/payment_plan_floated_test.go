package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	floatedSpell     = "Name:Floated Spell\nManaCost:1 R\nTypes:Creature Devil\nPT:1/1\nK:Flash\nA:SP$ PermanentCreature\nOracle:x\n"
	floatedDemon     = "Name:Floated Demon\nManaCost:1 R\nTypes:Creature Demon\nPT:1/1\nK:Flash\nA:SP$ PermanentCreature\nOracle:x\n"
	floatedRock      = "Name:Floated Rock\nManaCost:1\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Produced$ C | Amount$ 2\nOracle:x\n"
	floatedSnow      = "Name:Floated Snow Land\nTypes:Snow Land\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n"
	floatedDemonLand = "Name:Floated Demon Land\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ R | Amount$ 2 | RestrictValid$ Spell.Demon\nOracle:x\n"
	floatedMountain  = "Name:Floated Mountain\nTypes:Basic Land Mountain\nOracle:x\n"
	floatedSwamp     = "Name:Floated Swamp\nTypes:Basic Land Swamp\nOracle:x\n"
)

// floatFrom taps source for its mana through the ordinary activation.
func floatFrom(t *testing.T, e *Engine, source state.ObjID) {
	t.Helper()
	d := paymentPlanReask(t, e)
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == source {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
				t.Fatalf("float: %v", err)
			}
			return
		}
	}
	t.Fatalf("no mana activation of %d in %+v", source, d.Options)
}

// castPlanned submits the offered plan for spell and asserts it runs without
// reversing or falling back, leaving the spell on the stack.
func castPlanned(t *testing.T, e *Engine, spell state.ObjID) decision.PaymentPlan {
	t.Helper()
	d := paymentPlanReask(t, e)
	a := paymentPlanActionFor(t, d, spell)
	plan := a.Plans[0]
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Payment: &decision.PaymentSelection{ActionID: a.ID, Plan: plan}}); err != nil {
		t.Fatalf("Submit planned cast: %v", err)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Fatalf("spell zone = %s, pending %+v", z, e.Pending())
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.SpellCastReversed {
			t.Fatalf("the cast reversed: %+v", ev)
		}
	}
	if p := e.Pending(); p != nil && p.PaymentFallback != nil {
		t.Fatalf("the plan fell back: %+v", p.PaymentFallback)
	}
	return plan
}

// Floated artifact mana is ordinary mana of its colour to a plan: the {1} of
// the cast is paid from it, the {R} from the Mountain, and execution leaves
// exactly what the witness said.
func TestPlannedCastFromFloatedArtifactMana(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9201, floatedSpell)
	rock := onBoard(t, e, 0, floatedRock)
	mountain := onBoard(t, e, 0, floatedMountain)
	floatFrom(t, e, rock)
	if e.G.Players[0].ManaUnits() == ([7]state.Mana{}) {
		t.Fatal("precondition: the rock's mana carries no producer type")
	}
	plan := castPlanned(t, e, spell)
	if !e.G.Obj(mountain).Tapped {
		t.Fatal("the plan did not tap the Mountain")
	}
	got := e.G.Players[0].Pool
	if got[state.ManaIndex('C')] != 1 || plan.PoolAfter[state.ManaIndex('C')] != 1 {
		t.Fatalf("pool after = %v, witness %v, want one {C} left", got, plan.PoolAfter)
	}
}

// A snow unit pays as its colour, and the snow tally follows it.
func TestPlannedCastFromFloatedSnowMana(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9202, floatedSpell)
	snow := onBoard(t, e, 0, floatedSnow)
	onBoard(t, e, 0, floatedSwamp)
	floatFrom(t, e, snow)
	if e.G.Players[0].Snow.Total() != 1 {
		t.Fatal("precondition: no snow mana floats")
	}
	castPlanned(t, e, spell)
	if pl := &e.G.Players[0]; pl.Pool.Total() != 0 || pl.Snow.Total() != 0 {
		t.Fatalf("pool %v, snow %v, want both spent", pl.Pool, pl.Snow)
	}
}

// Persistent mana pays as its colour.
func TestPlannedCastFromPersistentMana(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9203, floatedSpell)
	onBoard(t, e, 0, floatedSwamp)
	pl := &e.G.Players[0]
	pl.Pool[state.ManaIndex('R')] = 1
	pl.PersistentMana[state.ManaIndex('R')] = 1
	castPlanned(t, e, spell)
	if pl.Pool.Total() != 0 || pl.PersistentMana.Total() != 0 {
		t.Fatalf("pool %v, persistent %v, want both spent", pl.Pool, pl.PersistentMana)
	}
}

// A restricted batch the cast may not spend neither pays it nor removes the
// offer: another source does, and the batch still floats afterwards.
func TestPlannedCastLeavesARestrictedBatchItMayNotSpend(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9204, floatedSpell)
	land := onBoard(t, e, 0, floatedDemonLand)
	onBoard(t, e, 0, floatedMountain)
	second := onBoard(t, e, 0, floatedMountain)
	floatFrom(t, e, land)
	castPlanned(t, e, spell)
	if !e.G.Obj(second).Tapped {
		t.Fatal("the plan did not pay with both Mountains")
	}
	if got := e.G.Players[0].Pool[state.ManaIndex('R')]; got != 2 {
		t.Fatalf("restricted mana left = %d, want the whole batch", got)
	}
}

// With only the restricted batch floating and no other source, the cast it
// may not spend is not offered.
func TestRestrictedBatchAloneOffersNoInadmissibleCast(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9205, floatedSpell)
	land := onBoard(t, e, 0, floatedDemonLand)
	floatFrom(t, e, land)
	d := paymentPlanReask(t, e)
	for _, a := range d.PaymentActions {
		if a.Cast.Object == spell {
			t.Fatalf("offered %+v from mana it may not spend", a)
		}
	}
}

// A cast the batch admits is paid from it.
func TestPlannedCastSpendsARestrictedBatchItAdmits(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9206, floatedDemon)
	land := onBoard(t, e, 0, floatedDemonLand)
	floatFrom(t, e, land)
	castPlanned(t, e, spell)
	if pl := &e.G.Players[0]; pl.Pool.Total() != 0 || len(pl.RestrictedMana) != 0 {
		t.Fatalf("pool %v, batches %+v, want the batch spent", pl.Pool, pl.RestrictedMana)
	}
}

// Two copies of a cost in one query scope differ only in what the batch admits;
// the plan memo must not serve one's plan to the other.
func TestPlanMemoSeparatesRestrictedAdmission(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9207, floatedSpell, floatedDemon)
	demon := moveSeededToHand(t, e, 0, "Floated Demon")
	land := onBoard(t, e, 0, floatedDemonLand)
	floatFrom(t, e, land)
	d := paymentPlanReask(t, e)
	offered := map[state.ObjID]bool{}
	for _, a := range d.PaymentActions {
		offered[a.Cast.Object] = true
	}
	if !offered[demon] || offered[spell] {
		t.Fatalf("offers = %v, want the Demon (%d) and not the other spell (%d)", offered, demon, spell)
	}
}

// With Auto-pay off the seat answers the same row through announce.
func TestAnnounceFromFloatedArtifactMana(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9208, floatedSpell)
	rock := onBoard(t, e, 0, floatedRock)
	mountain := onBoard(t, e, 0, floatedMountain)
	floatFrom(t, e, rock)
	paymentPlanReask(t, e)
	w := announce(t, e, spell)
	o := apOption(t, w, "mana", mountain, "")
	if err := e.Submit(decision.Intent{Seq: w.Seq, Player: w.Player, Choices: []int{o.Index}}); err != nil {
		t.Fatalf("Submit mana: %v", err)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Fatalf("spell zone = %s, pending %+v", z, e.Pending())
	}
}

// Floated mana an activation may not spend does not fund its offer (CR 106.6):
// the Demon-only batch leaves the {3} one short until a third land is untapped.
func TestRestrictedBatchFundsNoAbilityOffer(t *testing.T) {
	t.Parallel()
	const relic = "Name:Floated Relic\nTypes:Artifact\nA:AB$ GainLife | Cost$ 3 | LifeAmount$ 1\nOracle:x\n"
	offered := func(e *Engine, obj state.ObjID) bool {
		t.Helper()
		_, ok := autopayAbilityOffer(paymentPlanReask(t, e), obj)
		return ok
	}
	e, _, _ := newFixtureDeck(t, 9208, floatedSpell)
	e.paidManaCensus = true
	src := onBoard(t, e, 0, relic)
	floatFrom(t, e, onBoard(t, e, 0, floatedDemonLand))
	onBoard(t, e, 0, floatedSwamp)
	onBoard(t, e, 0, floatedMountain)
	if offered(e, src) {
		t.Fatalf("two lands and a Demon-only batch offered {3}: %+v", e.Pending().Options)
	}
	onBoard(t, e, 0, floatedSwamp)
	if !offered(e, src) {
		t.Fatalf("three lands did not offer {3}: %+v", e.Pending().Options)
	}
}
