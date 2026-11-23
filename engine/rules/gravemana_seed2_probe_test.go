package rules

// Scratch probe of gorge's planner in the shape of gravemana seed 2 at its
// divergence: Heap Gate untapped (free {T}: Add {C} + {1},{T}: any colour),
// a tapped Swamp, floating {C}, a Jack-o'-Lantern in the graveyard and one in
// hand, Lightning Bolt ({R}) and Stitcher's Supplier ({1}{B}) in hand.
// Diagnostic only: not committed with the fix.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestGraveManaSeed2Probe(t *testing.T) {
	t.Parallel()
	run(t, true)
	run(t, false)
}

func run(t *testing.T, poolOn bool) {
	// two Jacks: one on the battlefield (untapped), one in the graveyard
	corpus := []string{"Jack-o'-Lantern", "Jack-o'-Lantern", "Heap Gate", "Lightning Bolt", "Stitcher's Supplier"}
	e, _, reg := altCostEngine(t, 7, corpus, nil, nil)
	jack := findCardObj(t, e, 0, "Jack-o'-Lantern", state.ZGraveyard)
	_ = jack

	// Heap Gate tapped on the battlefield, a Swamp tapped, a Jack untapped
	gateCard, ok := reg.Lookup("Heap Gate")
	if !ok {
		t.Fatal("registry lacks Heap Gate")
	}
	gate := onBoardReadyCard(t, e, 0, gateCard)
	e.G.Obj(gate).SummonSick = false
	e.G.Obj(gate).Tapped = true
	swamp := onBoard(t, e, 0, "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n")
	e.G.Obj(swamp).SummonSick = false
	e.G.Obj(swamp).Tapped = true
	var boardJack state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Jack-o'-Lantern" {
			boardJack = id
			break
		}
	}
	if boardJack == 0 {
		t.Fatal("no second Jack in the library")
	}
	e.G.Obj(boardJack).Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), boardJack))
	e.G.Obj(boardJack).SummonSick = false

	var bolt, stitcher state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		lib := e.G.Zone(z, 0)
		for _, id := range lib {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Lightning Bolt":
				bolt = id
			case "Stitcher's Supplier":
				stitcher = id
			}
		}
		if bolt != 0 && stitcher != 0 {
			break
		}
	}
	if bolt == 0 || stitcher == 0 {
		t.Fatal("no Bolt or Stitcher in hand or library")
	}
	for _, id := range []state.ObjID{bolt, stitcher} {
		if e.G.Obj(id).Zone != state.ZHand {
			e.G.Obj(id).Zone = state.ZHand
			e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), id))
		}
	}

	if poolOn {
		addMana(t, e, 0, "C")
	}
	e.pending = nil
	e.askPriority(0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("precondition: no priority decision: %+v", d)
	}
	t.Logf("pool before %v", e.G.Players[0].Pool)
	// gorge's census over the floating pool: complete / relaxable / gaps
	hyp := e.G.Players[0].Pool
	cen := e.paymentPlanCensusOf(0, &hyp)
	t.Logf("census complete=%v relaxable=%v gaps=%v paid=%d relaxed=%d onceGap=%v fund=%d",
		cen.complete, cen.relaxable, cen.gaps, len(cen.paid), len(cen.relaxed), cen.onceGap, cen.fund)
	for i, pd := range cen.paid {
		t.Logf("census paid[%d] fee %d plain %v alts %v", i, pd.fee, pd.plain, pd.alts)
	}
	units := pay.PaymentPlanQueryUnits(asPayer(e), 0)
	for _, u := range units {
		t.Logf("census unit obj %d alts %d", u.ID, len(pay.PaymentPlanQueryAlternatives(asPayer(e), u)))
	}
	// priority options: casts?
	var opts []string
	for _, o := range d.Options {
		n := "?"
		z := "-"
		if ob := e.G.Obj(o.Obj); ob != nil {
			z = ob.Zone.String()
			if ob.Face() != nil {
				n = ob.Face().Name
			}
		}
		opts = append(opts, fmt.Sprintf("%s:%s@%s", o.Kind, n, z))
	}
	t.Logf("priority options %v", opts)

	// pool state after taps: verify Heap Gate's abilities
	for _, id := range []state.ObjID{gate} {
		o := e.G.Obj(id)
		t.Logf("gate %d tapped %v face abilities %d", id, o.Tapped, len(o.Face().Abilities))
	}

	for _, id := range []state.ObjID{bolt, stitcher} {
		script, why := e.PotentialPlayScript(0, decision.PotentialAction{Kind: "cast", Obj: id}, potentialScriptNodes)
		var steps []string
		for _, st := range script {
			var picks []string
			for _, p := range st.Picks {
				picks = append(picks, fmt.Sprintf("%s:%s:%s", p.Kind, bName(e, p.Obj), p.ManaSymbol))
			}
			steps = append(steps, string(st.Kind)+"("+strings.Join(picks, ",")+")")
		}
		t.Logf("script %s: steps=%v reason=%q", bName(e, id), steps, why)
	}
	_, optsW := e.potentialWalkOf(0, true)
	var casts []string
	for _, o := range optsW {
		if o.Kind == "cast" {
			casts = append(casts, bName(e, o.Obj))
		}
	}
	t.Errorf("potential walk casts %v (probe: logs above, forced failure to print them)", casts)
}

func bName(e *Engine, id state.ObjID) string {
	if o := e.G.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return "?"
}
