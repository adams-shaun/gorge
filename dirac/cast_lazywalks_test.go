package dirac

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// The nested-walk shadow: scriptOffers builds a nested priority walk only when
// its candidate loop can read it (script_gen.go). These tests hold the lazy
// path to the eager one it replaced (scriptOffersEager) in two ways: the
// shadow hook compares every call's options, spent cap, searched count and
// State against the eager twin run on a fork, and the decision stream of
// whole games played lazily must equal the one played eagerly.

// edhStream plays n transitions of BenchmarkTransitionEDH4's loop (pods,
// seeds, policy, restarts on game over and on a kernel panic) under cfg and
// returns one hash per transition of the Decision posed and the pick.
func edhStream(t *testing.T, pods []edhPod, cfg Config, n int) []uint64 {
	t.Helper()
	var s State
	var d Decision
	g := 0
	var pol RandomPolicy
	start := func() {
		pods[g%len(pods)].initCfg(&s, uint64(g)+1, cfg)
		pol = NewRandomPolicy(uint64(g) + 1)
	}
	out := make([]uint64, 0, n)
	segment := func() {
		defer func() {
			if r := recover(); r != nil {
				if m, ok := r.(string); ok && strings.HasPrefix(m, "dirac: scriptOffers lazy") {
					t.Fatalf("transition %d (game %d): %s%s", len(out), g, m, shadowDiff())
				}
				out = append(out, 0xdead) // the kernel panic ends the game, as the bench's segment does
				g++
				start()
			}
		}()
		for len(out) < n {
			s.StepInto(&d)
			if d.Kind == DkNone {
				g++
				start()
				s.StepInto(&d)
			}
			a := pol.Pick(&d)
			out = append(out, decisionHash(&d, a))
			s.Apply(a)
		}
	}
	start()
	for len(out) < n {
		segment()
	}
	return out
}

func decisionHash(d *Decision, a Action) uint64 {
	h := uint64(14695981039346656037)
	mix := func(v uint64) {
		h ^= v
		h *= 1099511628211
	}
	mix(uint64(d.Kind))
	mix(uint64(d.Player))
	mix(uint64(d.N))
	for i := 0; i < int(d.N); i++ {
		mix(uint64(d.Options[i]))
	}
	mix(uint64(a))
	return h
}

// shadowDiff names the stateCore fields the last shadow mismatch differs in.
func shadowDiff() string {
	a, b := &shadowMismatch.lazy, &shadowMismatch.eager
	var names []string
	seen := map[string]bool{}
	av := reflect.ValueOf(&a.stateCore).Elem()
	bv := reflect.ValueOf(&b.stateCore).Elem()
	var walk func(x, y reflect.Value, path string)
	walk = func(x, y reflect.Value, path string) {
		switch x.Kind() {
		case reflect.Struct:
			for i := 0; i < x.NumField(); i++ {
				walk(x.Field(i), y.Field(i), path+"."+x.Type().Field(i).Name)
			}
		case reflect.Array:
			for i := 0; i < x.Len(); i++ {
				walk(x.Index(i), y.Index(i), fmt.Sprintf("%s[%d]", path, i))
			}
		default:
			if x.CanInt() && x.Int() != y.Int() || x.CanUint() && x.Uint() != y.Uint() || x.Kind() == reflect.Bool && x.Bool() != y.Bool() {
				if !seen[path] && len(names) < 20 {
					seen[path] = true
					names = append(names, path)
				}
			}
		}
	}
	walk(av, bv, "")
	if a.perms != b.perms {
		names = append(names, "perms")
	}
	return fmt.Sprintf(" (stateCore fields differing: %v)", names)
}

// shadowMismatch is the last shadow difference, for the report: the lazy
// State and the eager fork's, copied when the shadow is about to panic.
var shadowMismatch struct {
	lazy, eager State
}

// shadowCalls counts the scriptOffers calls the shadow compared, and
// shadowPlays those that searched a play.
var shadowCalls, shadowPlays int

func eagerScriptOffers(s *State, k *prep, d, capSpent *Decision, p uint8) int {
	return s.scriptOffersEager(k, d, capSpent, p)
}

// withNestedHooks sets the hook: shadow compares each call with the eager
// twin, eager plays the twin in scriptOffers' place.
func withNestedHooks(t *testing.T, shadow, eager bool) {
	t.Helper()
	switch {
	case shadow:
		nestedWalkHook = shadowScriptOffers
	case eager:
		nestedWalkHook = eagerScriptOffers
	default:
		nestedWalkHook = nil
	}
	shadowCalls, shadowPlays = 0, 0
	t.Cleanup(func() { nestedWalkHook = nil })
}

// nestedShadowN is the transitions each EDH shadow test plays per run.
const nestedShadowN = 60000

func testNestedWalksEDH(t *testing.T, cfg Config) {
	pods := edhPods(t, 16)
	withNestedHooks(t, true, false)
	lazy := edhStream(t, pods, cfg, nestedShadowN) // every call checked against the eager twin
	t.Logf("%d scriptOffers calls shadowed, %d searched a play", shadowCalls, shadowPlays)
	if shadowPlays == 0 {
		t.Fatal("the shadow compared no call that searched a play")
	}
	withNestedHooks(t, false, true)
	eager := edhStream(t, pods, cfg, nestedShadowN)
	for i := range eager {
		if lazy[i] != eager[i] {
			t.Fatalf("transition %d: the lazy and eager nested walks pose different decisions", i)
		}
	}
}

// TestNestedWalksShadowEDH4: the EDH bench pods under Config{Mulligans: 2}.
func TestNestedWalksShadowEDH4(t *testing.T) {
	testNestedWalksEDH(t, Config{Mulligans: 2})
}

// TestNestedWalksShadowEDH4Autopay: the same pods on LockstepConfig's
// auto-pay surface, where scriptOffers' real walk runs.
func TestNestedWalksShadowEDH4Autopay(t *testing.T) {
	testNestedWalksEDH(t, edhAutopayConfig())
}

// scriptOffersEager is scriptOffers as it ran before its nested walks were
// made conditional on a reader (script_gen.go): every walk built where the
// candidate loop might read it. Keep it byte for byte the old body, so the
// shadow compares the lazy path with the code it replaced.
func (s *State) scriptOffersEager(k *prep, d *Decision, capSpent *Decision, p uint8) (searched int) {
	restr, anyR := s.scriptRestr(k, p), s.scriptAnyR(k, p)
	tapsDef := s.scriptTapsDefer(k, p)
	mix := s.mixBoard(k, p)
	filt := s.scriptOfferFilt(k, p, mix)
	tx := s.scriptTapXModelled(k, p).or(s.scriptCtrModelled(k, p))
	dh := k.gen.anyDiscHand && s.dhBoard(k, p) // Lion's Eye Diamonds the only gaps (mana_discardhand.go)
	ca := k.gen.anyComboAny && s.caBoard(k, p) // a Combo Any source among the gaps (mana_comboany.go)
	caM := s.caGapMask(k, p, ca)
	// a Combo Any source beside a filter (the filter search's otherComboAny
	// step): gorge's census is as unrelaxable as with the source alone
	caAny := s.caOffersBoard(k, p, ca)
	anyDiff := s.anyDifferentGaps(k, p) // a Firemind Vessel among the gaps (mana_distinctcolours.go)
	life := s.lifeGapMask(k, p)         // a life-only mana ability among the gaps (mana_lifegap.go)
	// raBoard: a live reflected Amount$ source is the board's only modelled
	// census gap (no filter, no other gap kind beside it). gorge's census is
	// unrelaxable there (mana_reflamt.go reflAmtBoundPlay), so the candidates
	// are the any-mana walk and the bound arm admits them (reflAmtBoundPlay).
	raBoard := !ca && !dh && !filt.any() && !mix && !restr.any() && !anyR.any() &&
		!tapsDef.any() && !s.scriptCtr(k, p).any() && !tx.any() && !anyDiff.any() && !life.any() &&
		s.reflAmtLive(k, p)
	if !dh && !ca && !s.caBesideGrant(k, p) && !s.scriptGaps(k, p).or(restr).or(anyR).or(tapsDef).or(filt).or(s.scriptCtr(k, p)).or(tx).or(anyDiff).or(life).any() {
		return
	}
	// With only restricted gaps (Giada, a Courtyard) gorge's census is
	// relaxable (each has a fixed relaxed alternative; a Tender's has
	// none): a play the relaxed proof cannot pay, the gaps walk's, is
	// insufficient and no search, so it takes none of the searches
	// (potential_plan.go paymentPlanRelaxProof), and the candidates are
	// the gaps walk's alone (one walk, not two).
	// A live Baylen makes the census unrelaxable (gorge paymentPlanRelaxedAlternatives,
	// no once-only cost): every walk play is searched; so is every play beside a live
	// Krark-Clan Ironworks when the potential bound is open. On a closed bound the
	// Ironworks' plays are the gaps walk's and those gorge's PotentialMana bound
	// pays on its own (sacAny; tapXSacAnyBound; sacPotPays). A closed-bound
	// sacrifice-any one whose cost sacrifices dedicated tokens (Jungle Patrol's
	// "Sacrifice a token named Wood: Add {R}") is the Baylen shape, not this one:
	// once is false there too, gorge runs no bounded proof, and every
	// census-incomplete play takes a search slot (sacRepeatLive). One that
	// sacrifices a permanent type (Krark-Clan Ironworks' artifact) is not: its
	// fodder can be a mana source the bound already prices.
	sacAny, sacOpen := s.tapXSacAnyBound(k, p)
	sacRepeat, twoLive := s.sacRepeatBoard(k, p, sacOpen)
	relax := s.scriptRelax(k, p, filt, caAny && !s.caRelaxable(k, p, ca) || twoLive || sacAny || raBoard)
	// A live repeatable fee-only filter (Nomadic Elf) has no relaxed or bounded
	// proof in gorge either: it searches every insufficient play, and the gaps
	// walk (one conversion per filter) cannot say which a many-times-activated
	// one pays (mana_filterrepeat.go).
	repeatLive := s.filtRepeatIn(k, filt)
	var any, gaps Decision
	s.plainGapsOptions(k, &gaps, filt)
	s.mergeBareGaps(k, &gaps, filt)
	src := &gaps
	if s.scriptAnyWalk(k, p, filt, relax, caAny || twoLive) {
		s.plainOptions(k, &any, availAny)
		src = &any
		if sacAny {
			s.unionGapsWalk(&any, &gaps)
		}
	}
	// the insufficient plays in gorge's walk order (potentialWalkOf: the
	// hand walk, the permission and graveyard casts, then the battlefield's
	// activations in entry order): the kernel's walk lists activations in
	// slot order
	var cand [MaxOptions]Action
	var key [MaxOptions]uint64
	nc := 0
	wide := !mix && !tapsDef.any() && !ca && s.wideGapsOnly(k, p, restr, anyR) // (gorge's relaxed proof reads a wide source against each play; a Taps-deferred gap is none of them)
	// The drop of a play no wide source may fund is the relaxed proof's
	// (potentialRestrictionAdmits): an unrelaxable census (a live
	// two-permanent source, Supportive Parents) runs no proof, so gorge
	// searches every play the walk lists, a wide source's mana counted.
	mixWide := mix && relax && s.scriptWide(k, p).any()
	var real Decision // the plays the printed sources pay, plain casts the autopay surface drops included
	if s.cfg&cfgGorgeAutopay != 0 {
		s.plainOptions(k, &real, availWalk)
	}
	var noWide, noWideNF Decision
	if mixWide {
		s.plainOptions(k, &noWide, availGaps|availNoWide) // (gorge's potential walk counts a wide source only for what its restriction admits)
		// The held-out walk is the fee-charged gaps walk, but the relaxed
		// proof accepts any subset of the paid gaps, the empty filter subset
		// included (availNoFilt): a play that subset pays with the wide
		// source's mana held out is not proven insufficient (tail-42 pair
		// 49: Subterranean Hangar's storage counters paid Liliana's Contract
		// beside the School of the Unseen's {2},{T} fee and Gwenna's
		// creature-only mana, the cast unoffered).
		s.plainOptions(k, &noWideNF, availGaps|availNoWide|availNoFilt)
	}
	skip := offerSkip{
		d: d, capSpent: capSpent, gaps: &gaps,
		sacAny: sacAny, sacRepeat: sacRepeat,
		wide: wide, anyR: anyR, mixWide: mixWide, noWide: &noWide, noWideNF: &noWideNF,
		anyWalk: src == &any, reflFilt: s.reflFiltBoard(k, p, ca, filt),
	}
	for i := 0; i < int(src.N); i++ {
		a := src.Options[i]
		if s.scriptOfferDrops(k, p, a, &skip) {
			continue
		}
		if s.pricedByPrintedSources(k, &real, a) {
			continue // priced by the printed sources: gorge's planner finds it sufficient and runs no prefix search for it
		}
		if plainCastMode(a) && s.plainCastUnsearched(k, p, a) {
			continue // gorge runs no prefix search for it: the printed sources pay it but the witness eats a target (witness_unreachable, mana_sacsource.go), or the shape is additional_cost and the composition is its only witness, an insufficient one dropped (potentialPlayVerdictShared)
		}
		if cd := &k.t.Cards[s.objs[Obj(a.Source())].card]; isCastKind(a.Kind()) && (s.spellTgtDep(k, p, s.objs[Obj(a.Source())].card, Obj(a.Source())) || cd.HasModeCost && !modalPricedAtPrinted(a) && !s.modalFloorFree(k, p, s.objs[Obj(a.Source())].card)) {
			continue // gorge searches no prefix for a target-priced cost, nor for a modal one whose cheapest mode still costs mana (a may-play or flashback cast is priced at its printed cost: potentialPlayVerdictShared's mayplay arm, and potentialModeCastPlan for a "flashback" mode)
		}
		if isCastKind(a.Kind()) && a.Mode()&castMayPlay == 0 && k.gen.anyOptCost && optCostCard(k, s.objs[Obj(a.Source())].card) {
			continue // nor for an OptionalCost card: the planner declines it (shape:optional_cost, cost_optional.go); a may-play cast it composes (potentialModeCastPlan)
		}
		if a.Kind() == AkCast && a.Mode()&3 == 0 && a.Mode()&castMayPlay == 0 && k.t.Cards[s.objs[Obj(a.Source())].card].HasMultikicker {
			continue // nor for a Multikicker card's plain cast: gorge's planner declines the face (payment_plan.go "shape:optional_cost"), so a plain cast only a census gap pays is not offered (Gnarlid Pack beside Jaspera Sentinel); the multikicked cast keeps its own offer gate (legal_walk_hand.go phMultikicker)
		}
		if s.scriptShapeSkip(k, p, a) {
			continue // nor for an exile, forage, return, blight or evidence part (shape:additional_cost; objPartCast), nor a contribution (shape:contribution); a forage-or-pay either-or (Feed the Cycle) stays with gorge's offer gate, which searches the filter script for its paid part (payment_plan.go altAdd)
		}
		kk := s.walkKey(a, p, i)
		j := nc
		for ; j > 0 && key[j-1] > kk; j-- {
			cand[j], key[j] = cand[j-1], key[j-1]
		}
		cand[j], key[j] = a, kk
		nc++
	}
	var lifeWalk, ownWalk Decision
	lifeBuilt, ownBuilt := false, false
	boundLive := filtDeferBoundLive(sacAny, filt, tapsDef)
	rt := scriptRoute{
		dh: dh, ca: ca, caM: caM, filt: filt, mix: mix,
		tapsDef: tapsDef, restr: restr, anyR: anyR, anyDiff: anyDiff,
		tx: tx, life: life, gaps: &gaps,
		twoLive: twoLive, repeatLive: repeatLive,
	}
	for i := 0; i < nc && searched < potentialScriptPlays; i++ { // the first potentialScriptPlays: searched
		a := cand[i]
		if !s.gapsOrBoundPlay(k, p, boundLive, &gaps, a) && !twoLive && !repeatLive && !((caAny || raBoard) && s.reflAmtBoundPlay(k, p, a)) && !s.ownTapWalkPlay(k, p, a, &ownWalk, &ownBuilt) {
			if !relax {
				searched++
			}
			continue
		}
		searched++
		if s.autopayDropsPlainCast(a) && !s.lifePlanned(k, p, life, &lifeWalk, &lifeBuilt, a) && !s.phyHalfPlanned(k, p, a) {
			continue // a plain cast: dropped by the autopay surface unless gorge's planner prices it with a life-only source (mana_lifeplan.go)
		}
		if s.scriptRouteDrop(k, &rt, a, p) {
			continue
		}
		d.add(a)
	}
	return searched
}

// shadowScriptOffers runs scriptOffersEager on a fork of the same State and
// panics unless the two leave the same options in d in the same order, the
// same spent cap, the same searched count and the same game State
// (shadowSameState).
func shadowScriptOffers(s *State, k *prep, d, capSpent *Decision, p uint8) int {
	f := s.fork()
	dE := *d
	var cE Decision
	cp := (*Decision)(nil)
	if capSpent != nil {
		cE, cp = *capSpent, &cE
	}
	shadowCalls++
	nE := f.scriptOffersEager(k, &dE, cp, p)
	n := s.scriptOffers(k, d, capSpent, p) // its forks sit above f in the pool
	why := ""
	switch {
	case n != nE:
		why = "searched"
	case !sameOptionList(d, &dE):
		why = "options"
	case capSpent != nil && !sameOptionList(capSpent, &cE):
		why = "capSpent"
	case !shadowSameState(s, f):
		why = "state"
	}
	if nE > 0 {
		shadowPlays++
	}
	if why != "" {
		shadowMismatch.lazy.CopyFrom(s)
		shadowMismatch.eager.CopyFrom(f)
	}
	s.unfork()
	if why != "" {
		panic("dirac: scriptOffers lazy walks differ from the eager path: " + why)
	}
	return n
}

func sameOptionList(a, b *Decision) bool {
	if a.N != b.N {
		return false
	}
	for i := 0; i < int(a.N); i++ {
		if a.Options[i] != b.Options[i] {
			return false
		}
	}
	return true
}

// shadowSameState compares the lazy State with the eager fork's, the
// filtBoundPays memo aside: a walk the lazy path skipped may have left a
// different entry there, and the memo answers only for its full key
// (mana_filter.go fbpMemo).
func shadowSameState(s, f *State) bool {
	m := s.g.fbp
	s.g.fbp = f.g.fbp
	same := s.SameGame(f)
	s.g.fbp = m
	return same
}
