package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// boundBoard puts board on seat 0's battlefield and hand in seat 0's hand at
// its first main phase of turn 3, with every battlefield permanent untapped
// and past summoning sickness, and returns the engine.
func boundBoard(t *testing.T, board, hand []string) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for _, n := range board {
		deck = append(deck, lookup(t, reg, n))
	}
	for _, n := range hand {
		deck = append(deck, lookup(t, reg, n))
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	for _, n := range board {
		moveToBattlefield(t, e, findAndMoveToHand(t, e, 0, n))
	}
	for _, n := range hand {
		findAndMoveToHand(t, e, 0, n)
	}
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		e.G.Obj(id).SummonSick = false
	}
	return e
}

// boundHandID returns the id of the named card in seat 0's hand.
func boundHandID(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("%s is not in seat 0's hand", name)
	return 0
}

// boundPlayOption returns the walk's plain-cast option for id.
func boundPlayOption(t *testing.T, e *Engine, id state.ObjID) decision.Option {
	t.Helper()
	_, opts := e.potentialWalkOf(0, true)
	for _, o := range opts {
		if o.Kind == "cast" && o.Obj == id && o.Mode == "" && o.AltCostIndex == 0 {
			return o
		}
	}
	t.Fatalf("%s has no plain-cast option in the potential walk", e.G.Obj(id).Face().Name)
	return decision.Option{}
}

// boundManaAbility returns id's activatable mana ability.
func boundManaAbility(t *testing.T, e *Engine, id state.ObjID) *cards.SA {
	t.Helper()
	for _, ma := range e.appendAvailableManaAbilitiesGate(nil, nil, 0, id, true) {
		if ma.API == "Mana" {
			return ma
		}
	}
	t.Fatalf("%s has no activatable mana ability", e.G.Obj(id).Face().Name)
	return nil
}

// boundRelaxedRepeats returns the relaxed unit count paymentPlanRelaxedAlternatives
// derives for id's mana ability.
func boundRelaxedRepeats(t *testing.T, e *Engine, id state.ObjID) (int32, bool) {
	t.Helper()
	var probe *Engine
	alts, fee, repeats, once, ok := e.paymentPlanRelaxedAlternatives(0, id, boundManaAbility(t, e, id), &probe)
	if !ok || len(alts) == 0 || fee != 0 {
		t.Fatalf("relaxation of %s: alts %d fee %d ok %v, want a free unit", e.G.Obj(id).Face().Name, len(alts), fee, ok)
	}
	return repeats, once
}

// boundVerdict is potentialPlans' verdict closure for one play: the planner's
// outcome with the census's own units.
func boundVerdict(e *Engine, o decision.Option) func() PaymentPlanOutcome {
	share := castPlanShare{statics: costStaticSource{e: e}, candidates: paymentCastCandidates{e: e, p: 0, priced: true}}
	return func() PaymentPlanOutcome {
		got, _ := e.potentialPlayVerdictShared(0, o, &share)
		return got
	}
}

// boundPoisonHypSpares leaves e's hypothetical-clone pool holding one spare
// whose event and object arrays are too small for the live game, so
// hypClone (hypclone.go) must replace them if it takes any clone at all.
func boundPoisonHypSpares(e *Engine) {
	pl := e.hypPool()
	pl.spares = append(pl.spares[:0], Spare{})
}

// boundHypClones reports how many of the pool's spares a hypothetical clone
// grew from the poisoned zero capacity.
func boundHypClones(e *Engine) int {
	n := 0
	for _, sp := range e.hypPool().spares {
		if cap(sp.events) > 0 || cap(sp.objs) > 0 {
			n++
		}
	}
	return n
}

// boundCensusGapNames returns the names of the sources in the census's gaps.
func boundCensusGapNames(t *testing.T, e *Engine, c paymentPlanCensus) map[string]bool {
	t.Helper()
	names := make(map[string]bool, len(c.gaps))
	for _, id := range c.gaps {
		names[e.G.Obj(id).Face().Name] = true
	}
	return names
}

// TestPrefixSearchBoundSkipsAnUnpayableTapXPlay covers the bounded
// necessary-condition proof that a census-incomplete play cannot be paid:
// Supportive Parents ("Tap two untapped creatures you control: Add one mana
// of any color") is bounded once (two creatures, K = 1), Interdimensional Web
// Watch's mana is refused for a spell cast from hand by its RestrictValid$,
// and Robotics Mastery costs 4U, so the bounded plan can reach at most one
// mana from the Island, one from Parents and none from Web Watch. The prefix
// search on that board cannot find a script either, so the proof may skip it:
// the search slot is still counted (dirac mirrors the slot accounting) and
// the verdict stays unsupported/census_incomplete.
func TestPrefixSearchBoundSkipsAnUnpayableTapXPlay(t *testing.T) {
	t.Parallel()
	e := boundBoard(t,
		[]string{"Supportive Parents", "Grizzly Bears", "Interdimensional Web Watch", "Island"},
		[]string{"Robotics Mastery"})
	robotics := boundHandID(t, e, "Robotics Mastery")
	parents := findBattlefieldCard(t, e, "Supportive Parents")
	findBattlefieldCard(t, e, "Interdimensional Web Watch")

	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || census.relaxable || !census.bounded {
		t.Fatalf("census: complete %v relaxable %v bounded %v, want incomplete, unrelaxable and bounded",
			census.complete, census.relaxable, census.bounded)
	}
	gaps := boundCensusGapNames(t, e, census)
	if !gaps["Supportive Parents"] || !gaps["Interdimensional Web Watch"] {
		t.Fatalf("census gaps %v, want Supportive Parents and Interdimensional Web Watch", gaps)
	}
	// The repeatable source is bounded by its untapped candidates, not
	// relaxed: two creatures pay for one activation, and the existing
	// relaxed proof must not run on it.
	repeats, once := boundRelaxedRepeats(t, e, parents)
	if once || repeats != 1 {
		t.Fatalf("Supportive Parents: repeats %d once %v, want a bounded repeatable source with K=1", repeats, once)
	}

	option := boundPlayOption(t, e, robotics)
	if got := e.paymentPlanRelaxProof(census.boundedProof(), boundVerdict(e, option)); got.Reason != "insufficient" {
		t.Fatalf("bounded relax proof: reason %q detail %q, want insufficient (Island 1 + Parents 1)", got.Reason, got.Detail)
	}

	// No clone may run: the poisoned spare's arrays stay empty only when the
	// skip stopped potentialPrefixScript.
	boundPoisonHypSpares(e)
	plans := e.PotentialPaymentPlans(0)
	if clones := boundHypClones(e); clones != 0 {
		t.Fatalf("PotentialPaymentPlans took %d hypothetical clones, want 0: the bounded proof should skip the prefix search", clones)
	}
	var plan *PotentialPlan
	for i := range plans {
		if plans[i].Action.Kind == "cast" && plans[i].Action.Obj == robotics {
			plan = &plans[i]
			break
		}
	}
	if plan == nil {
		t.Fatal("Robotics Mastery is not among the potential plays")
	}
	if plan.Reason != "unsupported" || plan.Detail != "census_incomplete" {
		t.Fatalf("Robotics Mastery: reason %q detail %q, want unsupported/census_incomplete", plan.Reason, plan.Detail)
	}
	if plan.Script != nil {
		t.Fatalf("Robotics Mastery: %d script steps, want none", len(plan.Script))
	}

	// The bound agrees with the search: run it directly on the same play and
	// board, and it finds no script.
	hyp = e.PotentialMana(0)
	census = e.paymentPlanCensusOf(0, &hyp)
	if _, ok := e.potentialPrefixScript(0, option, census.gaps, census.fees()); ok {
		t.Fatalf("potentialPrefixScript found a script on a play the bounded proof calls unpayable")
	}
}

// TestPrefixSearchBoundKeepsAPayableTapXPlay guards the bound against
// undercounting a repeatable source. Supportive Parents beside three Grizzly
// Bears has four untapped creatures, so K = 2 (the source may tap itself),
// and three Islands plus those two Parents activations pay Robotics Mastery's
// 4U. The bound must not call the play unpayable, and the prefix search must
// still find the script with two Parents activations.
func TestPrefixSearchBoundKeepsAPayableTapXPlay(t *testing.T) {
	t.Parallel()
	e := boundBoard(t,
		[]string{"Supportive Parents", "Grizzly Bears", "Grizzly Bears", "Grizzly Bears", "Island", "Island", "Island"},
		[]string{"Robotics Mastery"})
	robotics := boundHandID(t, e, "Robotics Mastery")
	parents := findBattlefieldCard(t, e, "Supportive Parents")

	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || census.relaxable || !census.bounded {
		t.Fatalf("census: complete %v relaxable %v bounded %v, want incomplete, unrelaxable and bounded",
			census.complete, census.relaxable, census.bounded)
	}
	if gaps := boundCensusGapNames(t, e, census); !gaps["Supportive Parents"] || len(gaps) != 1 {
		t.Fatalf("census gaps %v, want only Supportive Parents", gaps)
	}
	// Four untapped creatures are two activations of the two-creature cost:
	// an undercount here would make the bound call the play unpayable.
	repeats, once := boundRelaxedRepeats(t, e, parents)
	if once || repeats != 2 {
		t.Fatalf("Supportive Parents: repeats %d once %v, want a bounded repeatable source with K=2", repeats, once)
	}

	option := boundPlayOption(t, e, robotics)
	if got := e.paymentPlanRelaxProof(census.boundedProof(), boundVerdict(e, option)); got.Reason == "insufficient" {
		t.Fatalf("bounded relax proof calls Robotics Mastery unpayable: %q %q", got.Reason, got.Detail)
	}

	var plan *PotentialPlan
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "cast" && pp.Action.Obj == robotics {
			plan = &pp
			break
		}
	}
	if plan == nil {
		t.Fatal("Robotics Mastery is not among the potential plays")
	}
	if plan.Reason != "" || len(plan.Script) == 0 {
		t.Fatalf("Robotics Mastery: reason %q detail %q, %d script steps, want a scripted payment", plan.Reason, plan.Detail, len(plan.Script))
	}
	activations := 0
	for _, st := range plan.Script {
		for _, pk := range st.Picks {
			if pk.Kind == "activate" && pk.Obj == parents {
				activations++
			}
		}
	}
	if activations != 2 {
		t.Fatalf("the script activates Supportive Parents %d times, want 2: %+v", activations, plan.Script)
	}
}
