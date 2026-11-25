package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPainlandAndBasicAdjacentIdsRankByCreationOrder: a plan's key-8 rank is
// gorge's ObjID (creation order), and every unit the planner adds must use
// that one scale. A plan that takes units from two census fills -- the main
// fill (a basic land) and painUnit (a painland's normal alternative, phase 1)
// used to mix the raw object id with creation rank id+1, so adjacent ids
// tied and key 8 fell through to the produced colour: the painland's {C}
// sorted before the basic's {G} (a later colour in WUBRGC is smaller), while
// gorge takes the lower ObjID. The first painland's painful ability pays the
// white pip, its normal {C} and the Forest's {G} are the interchangeable
// generic source.
func TestPainlandAndBasicAdjacentIdsRankByCreationOrder(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Adarkar Wastes", Count: 1},
		{Name: "Karplusan Forest", Count: 1},
		{Name: "Forest", Count: 1},
		{Name: "Plains", Count: 20},
	}
	sc, id := newSetScenTweak(t, "pain", deck, nil)
	// Adjacent object ids: the painful white source, the basic, the later
	// painland whose normal {C} is the interchangeable generic source.
	a := sc.perm(id["Adarkar Wastes"], 0)
	forest := sc.perm(id["Forest"], 0)
	b := sc.perm(id["Karplusan Forest"], 0)
	// The scenario fixture writes its permanents straight into the table; a
	// real game creates each one through a minted birth.
	for _, o := range []Obj{a, forest, b} {
		sc.s.mintBirth(o)
	}
	if int(b) != int(forest)+1 {
		t.Fatalf("precondition: forest %d and painland %d are not adjacent", forest, b)
	}
	if pc := sc.s.manaPCGen(sc.k, sc.slot(a)); pc.painCol == 0 || pc.painOnly {
		t.Fatalf("precondition: Adarkar Wastes is no painland (painCol %#x painOnly %v)", pc.painCol, pc.painOnly)
	}
	if len(coldBorn.of(sc.s)) == 0 {
		t.Fatal("precondition: the table has no birth-order region")
	}
	if pc := sc.s.manaPCGen(sc.k, sc.slot(b)); pc.painCol&(1<<kop.W) != 0 {
		t.Fatalf("precondition: Karplusan Forest can pay the white pip (painCol %#x)", pc.painCol)
	}

	// {1}{W}: phase 0 has only the {C} normals and the Forest's {G}, so the
	// painful {W} is required; phase 1 then chooses the generic source.
	var c kop.ManaCost
	c.Generic, c.Pips[kop.W] = 1, 1
	var pk planner
	sc.s.tapNeed(&pk, 0, &c, 0, false)
	sc.s.plan(sc.k, 0, &pk, NoObj)
	if !pk.found {
		t.Fatal("no plan for {1}{W} from a painland, a Forest and a painland")
	}

	// The basic's unit and the second painland's normal unit are both in the
	// plan's census: their key-8 ranks must be creation order, not two scales.
	forestU, bU := -1, -1
	for u := 0; u < pk.nu; u++ {
		if pk.units[u] == sc.slot(forest) {
			forestU = u
		}
		if pk.units[u] == sc.slot(b) {
			bU = u
		}
	}
	if forestU < 0 || bU < 0 {
		t.Fatalf("precondition: the plan's census holds forest unit %d and later painland unit %d", forestU, bU)
	}
	if pk.uobj[forestU] >= pk.uobj[bU] {
		t.Fatalf("key-8 ranks: Forest (created %d) %d, later painland (created %d) %d; want the Forest rank lower (gorge's ObjID order)",
			forest, pk.uobj[forestU], b, pk.uobj[bU])
	}

	// gorge picks the lower ObjID for the interchangeable generic pip: the
	// Forest, not the later painland's {C}.
	if pk.bestN != 2 {
		t.Fatalf("plan taps %d units, want 2 ({W} and {1})", pk.bestN)
	}
	gotForest, gotPain := false, 0
	for i := 0; i < pk.bestN; i++ {
		switch sl := pk.units[pk.bestSeq[i]]; sl {
		case sc.slot(forest):
			gotForest = true
		case sc.slot(a), sc.slot(b):
			gotPain++
		default:
			t.Fatalf("plan unit %d is an unexpected source", sl)
		}
	}
	if !gotForest || gotPain != 1 {
		t.Fatalf("plan taps the Forest %v and %d painland(s), want the Forest and the first painland (created %d, later one %d)",
			gotForest, gotPain, a, b)
	}
}
