package rules

// cr-g125: an untapped RestrictValid$ mana source funds a planned cast or an
// autopaid activation exactly when its restriction admits that payment
// (CR 601.2g-h, 602.2b, 106.6). Every producer is a real corpus card; the
// payers are inline fixtures.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const greenToll = "Name:Green Toll\nManaCost:G\nTypes:Instant\nOracle:x\n"

// plannedOfferSetup builds a dotless engine with front at the top of seat
// 0's deck, moves the named cards onto the battlefield (a logged TurnChange
// clears their summoning sickness) and asserts the common precondition: they
// are untapped, and seat 0's pool is empty with no restricted batch.
func plannedOfferSetup(t *testing.T, reg *cards.Registry, front []*cards.Card, battlefield ...string) (*Engine, Config, map[string]state.ObjID) {
	t.Helper()
	e, cfg := dotlessEngine(t, reg, front...)
	ids := map[string]state.ObjID{}
	for _, name := range battlefield {
		ids[name] = searchMoveByName(t, e, name, state.ZBattlefield)
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 1})
	for _, name := range battlefield {
		if o := e.G.Obj(ids[name]); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: %s is not an untapped battlefield permanent: %+v", name, o)
		}
	}
	if pl := e.G.Players[0]; pl.Pool.Total() != 0 || len(pl.RestrictedMana) != 0 {
		t.Fatalf("precondition: pool %v restricted %v, want both empty", pl.Pool, pl.RestrictedMana)
	}
	return e, cfg, ids
}

func plannedOfferSources(a decision.PaymentAction) []state.ObjID {
	var out []state.ObjID
	for _, act := range a.Plans[0].Activations {
		out = append(out, act.Source)
	}
	slices.Sort(out)
	return out
}

func TestRestrictedSourceFundsPlannedOffer(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)

	t.Run("ZigguratFundsCreatureCast", func(t *testing.T) {
		t.Parallel()
		e, cfg, ids := plannedOfferSetup(t, reg, []*cards.Card{searchCorpusCard(t, reg, "Ancient Ziggurat"),
			searchCorpusCard(t, reg, "Llanowar Elves")}, "Ancient Ziggurat")
		elves := searchMoveByName(t, e, "Llanowar Elves", state.ZHand)
		d := ppAsk(t, e)
		a := ppAction(t, d, elves)
		if len(a.Plans[0].Activations) != 1 || a.Plans[0].Activations[0].Source != ids["Ancient Ziggurat"] {
			t.Fatalf("plan = %+v, want Ancient Ziggurat alone", a.Plans[0])
		}
		ppSubmitPlan(t, e, d, a)
		if o := e.G.Obj(elves); o.Zone != state.ZStack {
			t.Fatalf("Llanowar Elves is in zone %v, want the stack", o.Zone)
		}
		if got := e.G.Players[0].RestrictedMana; len(got) != 0 || len(restrictedBatchesOf(e, 0, "Spell.Creature")) != 0 {
			t.Fatalf("restricted mana left after the cast: %+v", got)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("ZigguratRefusesNoncreatureCast", func(t *testing.T) {
		t.Parallel()
		e, cfg, _ := plannedOfferSetup(t, reg, []*cards.Card{searchCorpusCard(t, reg, "Ancient Ziggurat"),
			card(t, greenToll)}, "Ancient Ziggurat")
		toll := searchMoveByName(t, e, "Green Toll", state.ZHand)
		d := ppAsk(t, e)
		if ppHasAction(d, toll) {
			t.Fatalf("Green Toll has a payment action on a creature-only source: %+v", d.PaymentActions)
		}
		if got := e.PlanCastPayment(0, paymentCast(toll)); got.Plan != nil || got.Reason != "insufficient" {
			t.Fatalf("PlanCastPayment = %+v, want insufficient", got)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("ZigguratAndForestFundTwoDrop", func(t *testing.T) {
		t.Parallel()
		e, cfg, ids := plannedOfferSetup(t, reg, []*cards.Card{searchCorpusCard(t, reg, "Ancient Ziggurat"),
			searchCorpusCard(t, reg, "Grizzly Bears"), card(t, dotlessCharge2)}, "Ancient Ziggurat", "Forest")
		bears := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
		charge := searchMoveByName(t, e, "Splash Tithe", state.ZHand)
		d := ppAsk(t, e)
		if ppHasAction(d, charge) {
			t.Fatalf("the {2} instant has a payment action: it can use only Forest: %+v", d.PaymentActions)
		}
		a := ppAction(t, d, bears)
		want := []state.ObjID{ids["Ancient Ziggurat"], ids["Forest"]}
		slices.Sort(want)
		if got := plannedOfferSources(a); !slices.Equal(got, want) {
			t.Fatalf("plan sources = %v, want Ziggurat and Forest %v", got, want)
		}
		ppSubmitPlan(t, e, d, a)
		if o := e.G.Obj(bears); o.Zone != state.ZStack {
			t.Fatalf("Grizzly Bears is in zone %v, want the stack", o.Zone)
		}
		if len(e.G.Players[0].RestrictedMana) != 0 {
			t.Fatalf("restricted mana left after the cast: %+v", e.G.Players[0].RestrictedMana)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("HelperFundsArtifactCast", func(t *testing.T) {
		t.Parallel()
		e, cfg, ids := plannedOfferSetup(t, reg, []*cards.Card{searchCorpusCard(t, reg, "Hydraulic Helper"),
			searchCorpusCard(t, reg, "Bonesplitter"), card(t, dotlessCreatureU)}, "Hydraulic Helper")
		splitter := searchMoveByName(t, e, "Bonesplitter", state.ZHand)
		whelp := searchMoveByName(t, e, "Blue Whelp", state.ZHand)
		d := ppAsk(t, e)
		if ppHasAction(d, whelp) {
			t.Fatalf("the nonartifact {U} creature has a payment action: %+v", d.PaymentActions)
		}
		a := ppAction(t, d, splitter)
		if got := plannedOfferSources(a); !slices.Equal(got, []state.ObjID{ids["Hydraulic Helper"]}) {
			t.Fatalf("plan sources = %v, want Hydraulic Helper alone", got)
		}
		ppSubmitPlan(t, e, d, a)
		if o := e.G.Obj(splitter); o.Zone != state.ZStack {
			t.Fatalf("Bonesplitter is in zone %v, want the stack", o.Zone)
		}
		if len(e.G.Players[0].RestrictedMana) != 0 {
			t.Fatalf("restricted mana left after the cast: %+v", e.G.Players[0].RestrictedMana)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("PitAutomatonFundsActivation", func(t *testing.T) {
		t.Parallel()
		e, cfg, ids := plannedOfferSetup(t, reg, []*cards.Card{searchCorpusCard(t, reg, "Pit Automaton"),
			card(t, dotlessClerk), card(t, dotlessCharge1)}, "Pit Automaton", "Sunrise Clerk")
		charge := searchMoveByName(t, e, "One Toll", state.ZHand)
		d := ppAsk(t, e)
		if ppHasAction(d, charge) {
			t.Fatalf("the {1} instant has a payment action on an activated-only source: %+v", d.PaymentActions)
		}
		if findOption(d, "cast", charge) >= 0 {
			t.Fatalf("the {1} instant was offered on an activated-only source")
		}
		abIdx := findOption(d, "ability", ids["Sunrise Clerk"])
		if abIdx < 0 {
			t.Fatalf("the Clerk's {1} ability is not offered with Pit Automaton untapped: %+v", d.Options)
		}
		submitChoices(t, e, abIdx)
		passUntilStackEmpty(t, e, 30)
		if !e.G.Obj(ids["Pit Automaton"]).Tapped {
			t.Fatal("Pit Automaton was not tapped to pay the activation")
		}
		batches := restrictedBatchesOf(e, 0, "Activated")
		if len(batches) != 1 || restrictedBatchUnits(batches) != 1 {
			t.Fatalf("batches after the activation = %+v, want one 1-unit Activated batch", batches)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("MemoKeepsCastsApart", func(t *testing.T) {
		t.Parallel()
		// Llanowar Elves and Green Toll cost {G} each, so they share cost
		// and hand demand: only the payment's restriction admission tells
		// them apart. Both hand orders, so neither is the memo's first.
		for _, order := range [][]string{{"Llanowar Elves", "Green Toll"}, {"Green Toll", "Llanowar Elves"}} {
			e, _, _ := plannedOfferSetup(t, reg, []*cards.Card{searchCorpusCard(t, reg, "Ancient Ziggurat"),
				searchCorpusCard(t, reg, "Llanowar Elves"), card(t, greenToll)}, "Ancient Ziggurat")
			ids := map[string]state.ObjID{}
			for _, name := range order {
				ids[name] = searchMoveByName(t, e, name, state.ZHand)
			}
			d := ppAsk(t, e)
			if !ppHasAction(d, ids["Llanowar Elves"]) {
				t.Fatalf("order %v: Llanowar Elves has no payment action", order)
			}
			if ppHasAction(d, ids["Green Toll"]) {
				t.Fatalf("order %v: Green Toll has a payment action on a creature-only source", order)
			}
		}
	})
}
