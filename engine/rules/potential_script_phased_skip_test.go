package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestScriptSearchReturnsOnTheConduitPosition replays the class of the
// Conduit of Worlds focus game's hang (tail-33 pair 1306, turn 54 upkeep):
// Stonework Packbeast's "{2}: Add one mana of any color" is the census's only
// gap, a fee-only repeatable source, beside six lands that cannot pay Ravenous
// Leucrocota's {6}{G} monstrosity. The plain run completes in a handful of
// nodes and finds nothing; the phased fallback, whose script is only taken in
// phasedScope or after a budget-limited plain run, then walked the
// Packbeast's five-way colour branches to the whole node budget at every
// priority decision, though no node of it could return a script. The query
// must cost about the plain run's clones, not the budget's.
func TestScriptSearchReturnsOnTheConduitPosition(t *testing.T) {
	t.Parallel()
	e := boundBoard(t, []string{"Forest", "Forest", "Forest", "Forest", "Forest", "Plains", "Stonework Packbeast", "Ravenous Leucrocota"}, nil)
	var packbeast, leucrocota state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil {
			switch o.Face().Name {
			case "Stonework Packbeast":
				packbeast = id
			case "Ravenous Leucrocota":
				leucrocota = id
			}
		}
	}
	if packbeast == 0 || leucrocota == 0 {
		t.Fatal("precondition: Stonework Packbeast and Ravenous Leucrocota must be on the battlefield")
	}
	_, opts := e.potentialWalkOf(0, true)
	play := -1
	for i, o := range opts {
		if o.Kind == optAbility && o.Obj == leucrocota {
			play = i
		}
	}
	if play < 0 {
		t.Fatal("precondition: the potential walk has no Leucrocota activation option")
	}
	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || len(census.gaps) != 1 || census.gaps[0] != packbeast {
		t.Fatalf("precondition: the census must leave the Packbeast as its one gap (complete=%v gaps=%v)", census.complete, census.gaps)
	}
	o := opts[play]
	if !repeatableFeeOnlyMana(e, 0, packbeast, e.appendAvailableManaAbilitiesGate(nil, nil, 0, packbeast, true)[0]) {
		t.Fatal("precondition: the Packbeast's ability must be a fee-only repeatable source")
	}
	// The phased run's script could never be taken on this board: the play
	// is an ability, no gap has a hybrid fee, and the plain run completes.
	plain := e.newScriptSearch(0, decision.PotentialAction{Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode}, o.AltCostIndex, potentialScriptNodes)
	if plain == nil {
		t.Fatal("precondition: no script search")
	}
	plain.gaps, plain.gapsOnly, plain.funding, plain.printedOnly = census.gaps, true, census.fees(), true
	plain.feeOnly, plain.feeStall, plain.passes = true, true, true
	plain.actDesc, plain.actDescOK = scriptPlayDesc(o, func() Cost { return e.castOfferBase(0, o.Obj) })
	plainBefore := e.hypPool().takes
	_, found := plain.run(e)
	plainClones := e.hypPool().takes - plainBefore
	if found || plain.limited {
		t.Fatalf("precondition: the plain run must complete without a script (found=%v limited=%v nodes=%d)", found, plain.limited, plain.nodes)
	}
	before := e.hypPool().takes
	script, ok := e.potentialPrefixScript(0, o, census.gaps, census.fees())
	clones := e.hypPool().takes - before
	if ok || len(script) != 0 {
		t.Fatalf("no prefix pays {6}{G} from five Forests, a Plains and the Packbeast, got %v", script)
	}
	if clones > 2*plainClones {
		t.Fatalf("the prefix search took %d clones, the plain run alone %d (%d nodes): the out-of-scope phased run walked the Packbeast's branches", clones, plainClones, plain.nodes)
	}
}
