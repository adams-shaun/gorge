package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The second cut's free-source scan (feeOnlyFreeTapColour) must count only
// sources the swapped-line domination actually covers: an activation that is
// forbidden (Cursed Totem), summoning sick, or life-costing is not a free
// bare tap, and dropping the fee-only gap's answer for its colour would
// withhold the only line that fills the pip (the kernel's mirror fails open
// on the life cost for the same reason, dirac's filtFreeTapColour). The
// board is the class board's mana shape: the Refractor is the gap, Orzhova's
// own tap makes {C} alone and cannot fund (the play's {T} taps it, CR
// 602.2b), the Mountains, Forests and Island make only {R}, {G} and {U}, and
// the Swamp is the free {B} the second cut drops the Refractor's B answer
// for. The {W} pip is therefore fillable by the Refractor and by the tested
// source and by nothing else, so the found script's W is the assertion.

// feeOnlyUnusableFreeBoard is the class mana board with extra, named
// permanents; it returns Orzhova and the Refractor.
func feeOnlyUnusableFreeBoard(t *testing.T, extra ...string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	board := []string{"Mountain", "Mountain", "Mountain", "Forest", "Forest", "Forest",
		"Island", "Swamp", "Orzhova, the Church of Deals", "Energy Refractor"}
	board = append(board, extra...)
	e := boundBoard(t, board, nil)
	var orz, refractor state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Orzhova, the Church of Deals":
			orz = id
		case "Energy Refractor":
			refractor = id
		}
	}
	if orz == 0 || refractor == 0 {
		t.Fatal("precondition: Orzhova and the Refractor are not on the battlefield")
	}
	return e, orz, refractor
}

// feeOnlyOrzActivation returns the potential walk's Orzhova activation option.
func feeOnlyOrzActivation(t *testing.T, e *Engine, orz state.ObjID) decision.Option {
	t.Helper()
	_, opts := e.potentialWalkOf(0, true)
	for _, o := range opts {
		if o.Kind == "ability" && o.Obj == orz {
			return o
		}
	}
	t.Fatal("precondition: the potential walk has no Orzhova activation option")
	return decision.Option{}
}

// feeOnlySourceMakes asserts source's printed mana abilities include one that
// makes colour -- the produced read the scan itself uses (the printed list,
// not the availability gate: the point is what the scan WOULD see were the
// usability check skipped).
func feeOnlySourceMakes(t *testing.T, e *Engine, source state.ObjID, colour int) {
	t.Helper()
	for _, ma := range e.G.Obj(source).Face().ManaAbilities() {
		var made state.Mana
		e.addPotentialManaOf(&made, ma)
		if made[colour] > 0 {
			return
		}
	}
	t.Fatalf("precondition: no printed mana ability of obj %d makes colour %d", source, colour)
}

// TestFeeOnlyFreeTapScanSkipsForbiddenSources: Birds of Paradise's bare-tap
// ability is the board's only free W maker, and Cursed Totem forbids every
// creature's activated abilities -- the scan must fail open, the Refractor's
// W answer stays in the ask, and the plain prefix search prices the
// activation with the Refractor's W (the Swamp's B the cut drops). Were the
// scan to count the forbidden Birds, the prune would drop the W answer too,
// no colour the Refractor can make would fill a pip, and the play would go
// unpriced.
func TestFeeOnlyFreeTapScanSkipsForbiddenSources(t *testing.T) {
	t.Parallel()
	e, orz, refractor := feeOnlyUnusableFreeBoard(t, "Birds of Paradise", "Cursed Totem")
	birds := mustObj(t, e, "Birds of Paradise")
	o := feeOnlyOrzActivation(t, e, orz)
	feeOnlySourceMakes(t, e, birds, state.MW)
	feeOnlySourceMakes(t, e, birds, state.MB)
	sr0 := e.newScriptSearch(0, decision.PotentialAction{Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode}, o.AltCostIndex, potentialScriptNodes)
	if sr0 == nil {
		t.Fatal("precondition: no script search")
	}
	if sr0.feeOnlyFreeTapColour(e, refractor, state.MW) {
		t.Fatal("the scan counted the forbidden Birds of Paradise as a free bare tap")
	}
	if !sr0.feeOnlyFreeTapColour(e, refractor, state.MB) {
		t.Fatal("precondition: the Swamp is the free B source the scan must see")
	}
	script, ok := e.potentialPrefixScript(0, o, feeOnlyCensusGaps(t, e), feeOnlyCensusFees(t, e))
	if !ok || len(script) == 0 {
		t.Fatalf("the activation was not priced with the forbidden Birds on the board (ok=%v)", ok)
	}
	for _, st := range script {
		for _, pick := range st.Picks {
			if pick.Obj == refractor && pick.Kind == "mana" {
				if pick.ManaSymbol != "W" {
					t.Fatalf("the found script fills a pip through the Refractor with %s, want W: %v", pick.ManaSymbol, script)
				}
				return
			}
		}
	}
	t.Fatalf("the found script does not fill the W pip through the Refractor: %v", script)
}

// TestFeeOnlyFreeTapScanSkipsSummonSickSources: the same board without the
// Totem, with the Birds left summoning sick: the scan must not count it.
// Healthy the Birds are the free source the cut reads (the precondition the
// prune's swapped line stands on). Helper-level only: the PotentialMana bound
// still counts the sick Birds, so the play is offered outright here and no
// prefix script exists for the search to change.
func TestFeeOnlyFreeTapScanSkipsSummonSickSources(t *testing.T) {
	t.Parallel()
	e, orz, refractor := feeOnlyUnusableFreeBoard(t, "Birds of Paradise")
	birds := mustObj(t, e, "Birds of Paradise")
	o := feeOnlyOrzActivation(t, e, orz)
	feeOnlySourceMakes(t, e, birds, state.MW)
	sr0 := e.newScriptSearch(0, decision.PotentialAction{Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode}, o.AltCostIndex, potentialScriptNodes)
	if sr0 == nil {
		t.Fatal("precondition: no script search")
	}
	e.G.Obj(birds).SummonSick = true
	if sr0.feeOnlyFreeTapColour(e, refractor, state.MW) {
		t.Fatal("the scan counted a summoning-sick Birds of Paradise as a free bare tap")
	}
	e.G.Obj(birds).SummonSick = false
	if !sr0.feeOnlyFreeTapColour(e, refractor, state.MW) {
		t.Fatal("precondition: a healthy Birds of Paradise is the free W source the scan must see")
	}
}

// TestFeeOnlyFreeTapScanSkipsLifeCostSources: Mana Confluence's "{T}, Pay 1
// life: Add one mana of any color" is an untapped, unhindered source that can
// make W, but its activation costs life the dominated branch saved, so the
// swapped line's endpoint is not the same and the scan must refuse it (the
// kernel's mirror fails open on the same class, pcard.painLife). Helper-level
// only: the PotentialMana bound counts the paid-life source too, so the play
// is offered outright here and no prefix script exists for the search to
// change.
func TestFeeOnlyFreeTapScanSkipsLifeCostSources(t *testing.T) {
	t.Parallel()
	e, orz, refractor := feeOnlyUnusableFreeBoard(t, "Mana Confluence")
	conf := mustObj(t, e, "Mana Confluence")
	o := feeOnlyOrzActivation(t, e, orz)
	var madeW bool
	for _, ma := range e.appendAvailableManaAbilitiesGate(nil, nil, 0, conf, true) {
		cc := e.manaAbilityCC(0, conf, ma)
		if cc.Cost.Life == 0 {
			continue
		}
		var made state.Mana
		e.addPotentialManaOf(&made, ma)
		if made[state.MW] > 0 {
			madeW = true
		}
	}
	if !madeW {
		t.Fatal("precondition: Mana Confluence's paid ability does not produce W")
	}
	if e.G.Players[0].Life < 1 {
		t.Fatal("precondition: the payer is at 0 life")
	}
	sr0 := e.newScriptSearch(0, decision.PotentialAction{Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode}, o.AltCostIndex, potentialScriptNodes)
	if sr0 == nil {
		t.Fatal("precondition: no script search")
	}
	if sr0.feeOnlyFreeTapColour(e, refractor, state.MW) {
		t.Fatal("the scan counted Mana Confluence's paid-life ability as a free bare tap")
	}
	if !sr0.feeOnlyFreeTapColour(e, refractor, state.MB) {
		t.Fatal("precondition: the Swamp is the free B source the scan must see")
	}
}

// feeOnlyCensusGaps and feeOnlyCensusFees are the class board's census route
// (the Refractor is the gap the searches price).
func feeOnlyCensusGaps(t *testing.T, e *Engine) []state.ObjID {
	t.Helper()
	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || len(census.gaps) == 0 {
		t.Fatal("precondition: the census must leave the fee-only Refractor as a gap")
	}
	return census.gaps
}

func feeOnlyCensusFees(t *testing.T, e *Engine) int {
	t.Helper()
	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	return census.fees()
}

func mustObj(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("precondition: %s is not on the battlefield", name)
	return 0
}
