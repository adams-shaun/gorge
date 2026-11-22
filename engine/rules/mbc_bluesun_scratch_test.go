package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// scratch: gorge vs the Refractor-funded Blue Sun's Zenith cast
func TestScratchMBCBlueSunGorge(t *testing.T) {
	var all string
	for _, chamber := range []bool{false, true} {
		hand := []string{"Blue Sun's Zenith"}
		if chamber {
			hand = []string{"Chamber of Manipulation", "Blue Sun's Zenith"}
		}
		e := boundBoard(t, []string{"Island", "Island", "Plains", "Racers' Ring", "Energy Refractor"}, hand)
		zen := boundHandID(t, e, "Blue Sun's Zenith")
		all += fmt.Sprintf("== chamber=%v\n", chamber)
		_, opts := e.potentialWalkOf(0, true)
		play := -1
		for i, o := range opts {
			if o.Kind == "cast" && o.Obj == zen && o.Mode == "" {
				play = i
			}
		}
		all += fmt.Sprintf("potential walk lists the Zenith: %v (opts=%d)\n", play >= 0, len(opts))
		if play >= 0 {
			hyp := e.PotentialMana(0)
			census := e.paymentPlanCensusOf(0, &hyp)
			all += fmt.Sprintf("census complete=%v relaxable=%v fund=%d gaps=%d fees=%d\n", census.complete, census.relaxable, census.fund, len(census.gaps), census.fees())
			script, ok := e.potentialPrefixScript(0, opts[play], census.gaps, census.fees())
			all += fmt.Sprintf("prefix script ok=%v len=%d\n", ok, len(script))
			// replicate the plain run to read its node accounting
			a := decision.PotentialAction{Kind: opts[play].Kind, Obj: opts[play].Obj, Ability: opts[play].Ability, Mode: opts[play].Mode}
			sr := e.newScriptSearch(0, a, opts[play].AltCostIndex, potentialScriptNodes)
			if sr != nil {
				sr.gaps, sr.gapsOnly, sr.funding, sr.printedOnly = census.gaps, true, census.fees(), opts[play].Kind == optAbility
				sr.twinCut = potentialTwinCut(opts[play])
				sr.feeGaps = map[state.ObjID]bool{}
				sr.passes = true
				sr.feeStall = true
				for _, id := range census.gaps {
					o2 := e.G.Obj(id)
					for _, ma := range o2.Face().Abilities {
						if ma.API == "Mana" && repeatableFeeOnlyMana(e, 0, id, ma) {
							sr.feeOnly = true
						}
					}
				}
				_, found2 := sr.run(e)
				all += fmt.Sprintf("replica plain run found=%v nodes=%d limited=%v feeOnly=%v feeGaps=%v\n", found2, sr.nodes, sr.limited, sr.feeOnly, len(sr.feeGaps))
			}
			for i, st := range script {
				all += fmt.Sprintf("step %d kind=%s picks=%v\n", i, st.Kind, st.Picks)
			}
			v, witness := e.potentialPlayVerdict(0, opts[play])
			all += fmt.Sprintf("root verdict reason=%q detail=%q plan!=nil=%v witness=%v\n", v.Reason, v.Detail, v.Plan != nil, witness)
			if !census.relaxable && census.bounded {
				r := e.paymentPlanRelaxProof(census.boundedProof(), func() PaymentPlanOutcome {
					got, _ := e.potentialPlayVerdict(0, opts[play])
					return got
				})
				all += fmt.Sprintf("bounded proof reason=%q detail=%q plan!=nil=%v\n", r.Reason, r.Detail, r.Plan != nil)
			}
		}
		d := e.Pending()
		if d == nil {
			all += "no pending decision\n"
			continue
		}
		all += fmt.Sprintf("decision kind=%v player=%v n=%d\n", d.Kind, d.Player, len(d.Options))
		for i := range d.Options {
			o := &d.Options[i]
			all += fmt.Sprintf("opt[%d] kind=%s obj=%d mode=%s\n", i, o.Kind, o.Obj, o.Mode)
		}
	}
	t.Fatal(all)
}

var _ = decision.Option{}
var _ = state.ZBattlefield
