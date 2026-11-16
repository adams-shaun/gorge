package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// scratch probe: why does gorge withhold Spontaneous Combustion's plain cast
// on the lockstep game's board?
func probeBoard(t *testing.T, names []string) (*Engine, state.ObjID) {
	return probeBoardHand(t, "Spontaneous Combustion", names)
}

func probeBoardHand(t *testing.T, handCard string, names []string) (*Engine, state.ObjID) {
	t.Helper()
	sc := corpusAlternativeCard(t, handCard)
	e := handEngine(t, sc)
	spell := e.G.Zone(state.ZHand, 0)[0]
	for _, name := range names {
		o := e.G.AddObject(corpusAlternativeCard(t, name), 0)
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	return e, spell
}

func TestZZSCProbeDrawStep(t *testing.T) {
	e, spell := probeBoard(t, []string{"Swamp", "Swamp", "Swamp", "Island", "Karakas", "Henge of Ramos", "Mercadian Bazaar", "Crossroads Candleguide", "Famished Foragers"})
	e.G.Step = state.StepDraw
	var opts []string
	castOpt := false
	for _, o := range e.legalActions(0) {
		opts = append(opts, o.Kind+":"+o.Label)
		if o.Kind == "cast" && o.Obj == spell {
			castOpt = true
		}
	}
	t.Errorf("draw-step pool=%v options=%v castOption=%v", e.G.Players[0].Pool, opts, castOpt)
}

func TestZZSCProbeMinimalCast(t *testing.T) {
	// one Mountain on the battlefield, a {R} instant in hand, empty pool
	e, spell := probeBoardHand(t, "Necrosquito", []string{"Mountain"})
	var castOpt bool
	var opts []string
	for _, o := range e.legalActions(0) {
		opts = append(opts, o.Kind+":"+o.Label)
		if o.Kind == "cast" && o.Obj == spell {
			castOpt = true
		}
	}
	t.Errorf("pool=%v options=%v castOption=%v", e.G.Players[0].Pool, opts, castOpt)
}

func TestZZSCProbe(t *testing.T) {
	boards := [][]string{
		{"Swamp", "Swamp", "Swamp", "Island", "Karakas", "Henge of Ramos", "Mercadian Bazaar", "Crossroads Candleguide", "Famished Foragers"},
		{"Swamp", "Swamp", "Swamp", "Crossroads Candleguide", "Famished Foragers"},
		{"Swamp", "Swamp", "Swamp", "Henge of Ramos", "Famished Foragers"},
		{"Swamp", "Swamp", "Swamp", "Crossroads Candleguide"},
		{"Swamp", "Swamp", "Swamp", "Crossroads Candleguide", "Famished Foragers", "Island"},
	}
	for i, names := range boards {
		e, spell := probeBoard(t, names)
		pl := e.G.Players[0]
		var castOpt bool
		for _, o := range e.legalActions(0) {
			if o.Kind == "cast" && o.Obj == spell {
				castOpt = true
			}
		}
		t.Errorf("board %d (%v): pool=%+v restricted=%v castOption=%v", i, names, pl.Pool, pl.RestrictedMana, castOpt)
	}
}

// does the planner + potential bound witness the cast? (the autoPayOffer
// shape, applied to a cast)
func TestZZSCProbePlanner(t *testing.T) {
	e, spell := probeBoard(t, []string{"Swamp", "Swamp", "Swamp", "Island", "Karakas", "Henge of Ramos", "Mercadian Bazaar", "Crossroads Candleguide", "Famished Foragers"})
	e.priorityRound()
	if e.Pending() == nil {
		t.Fatalf("precondition: no priority ask posed")
	}
	f := e.G.Obj(spell).Face()
	base := pay.WithSpellAbilityExtras(f, e.castOfferBase(0, spell))
	bound := e.PotentialMana(0)
	pool := e.G.Players[0].Pool
	t.Errorf("PotentialMana=%v pool=%v bound==pool:%v", bound, pool, bound == pool)
	css := costStaticSource{e: e}
	statics := css.get()
	probe := base
	hypOK := e.offerCastableUsing(statics, 0, spell, &probe, spellScope(""), false, &bound)
	t.Errorf("offerCastableUsing(hyp=PotentialMana)=%v", hypOK)
	out := e.planCastPaymentMemo(0, decision.PlannedCast{Object: spell, Face: plannedCastFace(e.G.Obj(spell)), Origin: "hand"}, &css, &paymentCastCandidates{e: e, p: 0, priced: true})
	t.Errorf("planCastPaymentMemo: reason=%q plan=%+v", out.Reason, out.Plan)
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Obj == spell {
			t.Errorf("PotentialPaymentPlans: SC cast reason=%q plan=%+v scriptSteps=%d", pp.Reason, pp.Plan, len(pp.Script))
		}
	}
	// the search directly, with the walk's own option shape
	a := decision.PotentialAction{Kind: "cast", Obj: spell}
	for _, b := range []int{400, 1600, 6400} {
		st2, ok2 := e.PotentialPlayScript(0, a, b)
		if b == 1600 {
			for i, st := range st2 {
				t.Errorf("  step %d: kind=%v picks=%v", i, st.Kind, st.Picks)
			}
		}
		t.Errorf("PotentialPlayScript budget=%d ok=%v steps=%d", b, ok2, len(st2))
	}
	co := decision.Option{Kind: "cast", Obj: spell}
	script, ok := e.potentialPrefixScript(0, co, []state.ObjID{87, 89}, 320)
	t.Errorf("potentialPrefixScript: ok=%v steps=%d", ok, len(script))
	for i, st := range script {
		t.Errorf("  step %d: kind=%v picks=%v", i, st.Kind, st.Picks)
	}
	census := e.paymentPlanCensusOf(0, &bound)
	t.Errorf("census: complete=%v relaxable=%v fund=%d fees=%v gaps=%v relaxedN=%d paidN=%d bounded=%v boundN=%d boundPaidN=%d", census.complete, census.relaxable, census.fund, census.fees(), census.gaps, len(census.relaxed), len(census.paid), census.bounded, len(census.bound), len(census.boundPaid))
	var engProbe *Engine
	for _, g := range census.gaps {
		o := e.G.Obj(g)
		if o == nil {
			continue
		}
		t.Errorf("gap obj %d = %s", g, o.Face().Name)
		for _, ma := range e.appendAvailableManaAbilitiesGate(nil, nil, 0, g, true) {
			alts, fee, repeats, once, ok := e.paymentPlanRelaxedAlternatives(0, g, ma, &engProbe)
			t.Errorf("  relax %q: ok=%v fee=%d repeats=%d once=%v alts=%d", ma.ParamStr(cards.PKSpellDescription), ok, fee, repeats, once, len(alts))
			c2 := e.manaAbilityCC(0, g, ma).Cost
			t.Errorf("    cost: %+v", c2)
		}
	}
}

// decompose board 0's refusal: mana feasibility vs non-mana castability.
func TestZZSCProbeDecompose(t *testing.T) {
	e, spell := probeBoard(t, []string{"Swamp", "Swamp", "Swamp", "Island", "Karakas", "Henge of Ramos", "Mercadian Bazaar", "Crossroads Candleguide", "Famished Foragers"})
	e.priorityRound()
	if e.Pending() == nil {
		t.Fatalf("precondition: no priority ask posed")
	}
	f := e.G.Obj(spell).Face()
	base := pay.WithSpellAbilityExtras(f, e.castOfferBase(0, spell))
	if len(base.Sac) != 1 {
		t.Fatalf("precondition: want one Sac part, got %+v", base.Sac)
	}
	fmt.Printf("NOTE: sac candidates=%d\n", len(pay.SacrificeCostCandidates(asPayer(e), 0, spell, base.Sac[0], false)))
	if cold := e.offerCastable(0, spell, base, spellScope(""), false); cold {
		fmt.Printf("offerCastable COLD=TRUE\n")
	} else {
		fmt.Printf("offerCastable COLD=FALSE\n")
	}
	statics := e.collectCostStatics()
	mayApply := false
	mods := e.costModifiersCompose(statics, 0, spell, spellScope(""), nil, false, 0, &mayApply)
	tax := e.commanderTaxAmount(0, spell)
	scope := spellScope("")
	feas := e.manaFeasiblePricedP(0, offerPayment(spell, false, scope), &base, &mods, tax, 0, nil)
	fmt.Printf("manaFeasiblePricedP=%v\n", feas)
	var composed Cost
	e.composedOfferCostInto(&composed, 0, spell, &base, &mods, scope)
	fmt.Printf("composed cost: %+v\n", composed)
	nm := pay.NonManaCastableP(asPayer(e), 0, spell, &composed, false, tapCostSAKind(scope.Ab))
	fmt.Printf("NonManaCastableP=%v\n", nm)
	if got := e.offerCastable(0, spell, base, spellScope(""), false); got {
		fmt.Printf("offerCastable=TRUE (no refusal on this board)\n")
	} else {
		fmt.Printf("offerCastable=FALSE\n")
	}
	t.Errorf("PROBE: sac candidates=%d feas=%v nm=%v", len(pay.SacrificeCostCandidates(asPayer(e), 0, spell, base.Sac[0], false)), feas, nm)
}
