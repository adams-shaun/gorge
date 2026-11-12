package rules

// A deterministic
// additional-mana `TapsForMana | Static$ True` trigger (Forsaken Monument,
// Badgermole Cub, Crypt Ghast, Wild Growth) is folded into the planned
// alternative's production instead of deferring the source. The shapes the
// fold does not prove keep deferring.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const tapManaDork = "Name:Colorless Dork\nTypes:Artifact Creature Construct\nPT:1/1\nA:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\nOracle:x\n"

func tapManaTrigger(body string, extra string) string {
	return "Name:Tap Probe\nTypes:Enchantment\nT:Mode$ TapsForMana | ValidCard$ Land | Activator$ You | Execute$ TrigMana" + extra +
		"\nSVar:TrigMana:" + body + "\nOracle:x\n"
}

func tapManaTriggerPrecondition(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	o := e.G.Obj(id)
	has := false
	for _, tr := range o.Face().Triggers {
		has = has || tr.Mode == "TapsForMana"
	}
	if o.Zone != state.ZBattlefield || !has {
		t.Fatalf("precondition: %s zone %v has no TapsForMana trigger", o.Face().Name, o.Zone)
	}
}

func TestPaymentPlanTapManaFoldMonumentPricesSolRingNotForest(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9911, interferenceOneInstant)
	ring := lrCorpus(t, e, "Sol Ring")
	forest := onBoard(t, e, 0, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n")
	tapManaTriggerPrecondition(t, e, lrCorpus(t, e, "Forsaken Monument"))
	if got := interferenceAltMana(t, e, ring); got != (state.Mana{state.MC: 3}) {
		t.Fatalf("Sol Ring under Forsaken Monument = %v, want {C}x3", got)
	}
	if tier, _, detail := interferenceTier(t, e, ring); tier != pay.TierNormal {
		t.Fatalf("Sol Ring tier = %d %q, want normal", tier, detail)
	}
	if got := interferenceAltMana(t, e, forest); got != (state.Mana{state.MG: 1}) {
		t.Fatalf("Forest under Forsaken Monument = %v, want {G} (Produced$ C does not fire)", got)
	}
}

func TestPaymentPlanTapManaFoldStacksMonumentAndBadgermole(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9912, interferenceOneInstant)
	dork := onBoard(t, e, 0, tapManaDork)
	e.G.Obj(dork).SummonSick = false
	tapManaTriggerPrecondition(t, e, lrCorpus(t, e, "Forsaken Monument"))
	if got := interferenceAltMana(t, e, dork); got != (state.Mana{state.MC: 2}) {
		t.Fatalf("dork under Monument = %v, want {C}{C}", got)
	}
	tapManaTriggerPrecondition(t, e, lrCorpus(t, e, "Badgermole Cub"))
	if got := interferenceAltMana(t, e, dork); got != (state.Mana{state.MC: 2, state.MG: 1}) {
		t.Fatalf("dork under Monument + Badgermole Cub = %v, want {C}{C}{G}", got)
	}
}

// The folded plan survives validation and execution: Sol Ring under Forsaken
// Monument pays a {2} instant on its own, the extra {C} floats in the pool
// and no production_changed fallback (a manual window) interrupts the cast.
func TestPaymentPlanTapManaFoldEndToEnd(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9913, lrSpell("2"))
	ring := lrCorpus(t, e, "Sol Ring")
	tapManaTriggerPrecondition(t, e, lrCorpus(t, e, "Forsaken Monument"))
	got := e.PlanCastPayment(0, paymentCast(spell))
	if got.Plan == nil || len(got.Plan.Activations) != 1 || got.Plan.Activations[0].Source != ring ||
		got.Plan.Activations[0].Produces != (decision.ManaAmount{state.MC: 3}) {
		t.Fatalf("plan = %+v, want Sol Ring alone producing {C}x3", got)
	}
	if err := e.ValidateCastPayment(0, paymentCast(spell), *got.Plan); err != nil {
		t.Fatalf("the bot's own plan fails validation: %v", err)
	}
	_, mark := lrSubmit(t, e, spell)
	lrOnlyPriorityAsked(t, e, mark)
	if !e.G.Obj(ring).Tapped {
		t.Fatal("Sol Ring was not tapped by the plan")
	}
	if e.G.Obj(spell).Zone != state.ZStack {
		t.Fatalf("spell zone = %v, want the stack", e.G.Obj(spell).Zone)
	}
	if pool := e.G.Players[0].Pool; pool != (state.Mana{state.MC: 1}) {
		t.Fatalf("pool after paying = %v, want the one floating {C}", pool)
	}
	adds := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.ManaAdd && ev.Player == 0 && ev.Amount > 0 {
			adds += int(ev.Amount)
		}
	}
	if adds != 3 {
		t.Fatalf("ManaAdd total = %d, want 3 (Sol Ring {C}{C} + Monument {C})", adds)
	}
}

// Shapes the fold does not prove keep deferring, naming the object.
func TestPaymentPlanTapManaFoldKeepsDeferring(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, body, extra, other string }{
		{"non-literal amount", "DB$ Mana | Produced$ G | Amount$ X", "\nSVar:X:Count$Valid Land.YouCtrl", ""},
		{"colour choice", "DB$ Mana | Produced$ Any | Amount$ 1", "", ""},
		{"not a mana ability", "DB$ Mana | Produced$ G | Amount$ 1", "", "nonstatic"},
		{"produce-mana replacement", "DB$ Mana | Produced$ G | Amount$ 1", "", "replacement"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, _, _ := newFixtureDeck(t, 9914, interferenceOneInstant)
			island := onBoard(t, e, 0, interferenceIsland)
			src := tapManaTrigger(c.body, " | Static$ True"+c.extra)
			if c.other == "nonstatic" {
				src = tapManaTrigger(c.body, c.extra)
			}
			probe := onBoard(t, e, 0, src)
			tapManaTriggerPrecondition(t, e, probe)
			if c.other == "replacement" {
				onBoardCard(t, e, 0, corpusCard(t, "Mana Reflection"))
			}
			tier, _, detail := interferenceTier(t, e, island)
			if tier != pay.TierDeferred || !strings.Contains(detail, "source:interference:") {
				t.Fatalf("Island tier = %d %q, want deferred by interference", tier, detail)
			}
		})
	}
}

// A trigger on the opponent's permanent adds mana for the opponent: seat 0's
// land is not priced with it, it stays deferred.
func TestPaymentPlanTapManaFoldOtherPlayersManaDefers(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9915, interferenceOneInstant)
	island := onBoard(t, e, 0, interferenceIsland)
	// No Activator$: matches seat 0's tap; the mana is the carrier's controller's.
	probe := onBoard(t, e, 1, "Name:Tap Probe\nTypes:Enchantment\nT:Mode$ TapsForMana | ValidCard$ Land | Execute$ TrigMana | Static$ True\nSVar:TrigMana:DB$ Mana | Produced$ G | Amount$ 1\nOracle:x\n")
	tapManaTriggerPrecondition(t, e, probe)
	tier, _, detail := interferenceTier(t, e, island)
	if tier != pay.TierDeferred || !strings.Contains(detail, "Tap Probe") {
		t.Fatalf("Island tier = %d %q, want deferred naming Tap Probe", tier, detail)
	}
}

// A layer-1 copy of a TapsForMana carrier is a carrier (CR 707.2, 605.4a): a
// Blank Bear copying Forsaken Monument folds its trigger into the dork's plan.
func TestPaymentPlanTapManaFoldCopiedCarrier(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9913, interferenceOneInstant)
	dork := onBoard(t, e, 0, tapManaDork)
	e.G.Obj(dork).SummonSick = false
	if got := interferenceAltMana(t, e, dork); got != (state.Mana{state.MC: 1}) {
		t.Fatalf("precondition: dork with no carrier = %v, want {C}", got)
	}
	bear := onBoard(t, e, 0, blankBearSrc)
	model := onBoardCard(t, e, 1, corpusCard(t, "Forsaken Monument"))
	if got := interferenceAltMana(t, e, dork); got != (state.Mana{state.MC: 1}) {
		t.Fatalf("precondition: dork before the copy = %v, want {C}", got)
	}
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: bear, IDs: []state.ObjID{model}})
	tapManaTriggerPrecondition(t, e, bear)
	if o := e.G.Obj(bear); o.CopyFace == nil {
		t.Fatal("precondition: the bear has no copy face")
	}
	if got := interferenceAltMana(t, e, dork); got != (state.Mana{state.MC: 2}) {
		t.Fatalf("dork under a copy of Forsaken Monument = %v, want {C}{C}", got)
	}
}
