package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggerBodyCostPayFirstCensus is the class census behind the Frenzied
// Goblin report (respay.go, optionalFoldsIntoCost): every corpus trigger body
// whose optional (non-Mandatory) Cost$ enters the triggered-cost window.
//
//   - Fold: a body whose trigger also carries OptionalDecider$ You must fold
//     the CR 603.5 yes/no into the pay election (one ask, not two). A body
//     with any OTHER decider keeps both asks (they are different players'
//     questions) and is only counted.
//   - Auto route: every plain mana cost among those bodies (after the X fold a
//     real window makes, at X = 1) must be fundable by the payment planner
//     from a land base that covers it, so the election's plain "pay" taps
//     without a manual window. A shape the V1 planner does not model
//     (pay.PlanCostDetail: snow, hybrid, ...) takes the manual route; those
//     details are counted, never silently dropped.
func TestTriggerBodyCostPayFirstCensus(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg)
	for _, col := range []string{"W", "U", "B", "R", "G", "C"} {
		for i := 0; i < 6; i++ {
			xFoldLand(t, e, "Census "+col, col)
		}
	}

	window := map[string]bool{}
	folded := map[string]bool{}
	otherDecider := map[string]bool{}
	manaCosts := map[string][]string{} // raw cost -> cards
	var unfolded []string
	for i := 0; i < reg.Len(); i++ {
		c := reg.Card(i)
		if len(c.Faces) == 0 {
			continue
		}
		name := c.Faces[0].Name
		for _, f := range c.Faces {
			for _, tr := range f.Triggers {
				sa := tr.Effect
				// Cumulative upkeep and echo bodies open their own windows
				// (startCumulativeUpkeep, startEcho), pay-first as well.
				if sa == nil || sa.API == "CumulativeUpkeep" || sa.API == "Echo" {
					continue
				}
				raw := sa.ParamStr(cards.PKCost)
				if raw == "" || strings.HasPrefix(raw, "Mandatory") || !e.triggerBodyNeedsCostWindow(sa) {
					continue
				}
				window[name] = true
				if spec := triggerOptionalSpec(tr); spec != "" {
					if spec == "You" {
						folded[name] = true
						if !e.optionalFoldsIntoCost(0, &state.Object{Ability: sa, Controller: 0}) {
							unfolded = append(unfolded, name)
						}
					} else {
						otherDecider[name] = true
					}
				}
				if cost := e.parseCost(raw); cost.Priceable() && cost.HasManaPayment() {
					manaCosts[raw] = append(manaCosts[raw], name)
				}
			}
		}
	}
	if len(window) == 0 {
		t.Fatal("census found no Cost$ trigger bodies: the corpus walk is broken")
	}
	if len(unfolded) > 0 {
		sort.Strings(unfolded)
		t.Errorf("%d OptionalDecider$ You bodies still pose a separate yes/no before the pay ask: %v", len(unfolded), unfolded)
	}

	planned, manual := 0, map[string]int{}
	var unplanned []string
	for raw, names := range manaCosts {
		cost := e.parseCost(raw)
		if costCarriesUnfoldedX(cost) {
			cost = foldCostX(cost, 1)
		}
		shape := cost
		shape.Life = 0
		if detail := pay.PlanCostDetail(shape); detail != "" {
			manual[detail] += len(names)
			continue
		}
		if e.resPayPlan(0, 0, cost, paymentOther) == nil {
			unplanned = append(unplanned, raw+" ("+names[0]+")")
			continue
		}
		planned += len(names)
	}
	if len(unplanned) > 0 {
		sort.Strings(unplanned)
		t.Errorf("%d plain mana trigger costs have no auto-pay plan from a covering land base: %v", len(unplanned), unplanned)
	}
	t.Logf("Cost$ trigger bodies entering the window: %d cards; OptionalDecider$ You folded: %d; other deciders (two asks kept): %d",
		len(window), len(folded), len(otherDecider))
	t.Logf("mana trigger-cost bodies auto-planned: %d; manual-route shapes: %v", planned, manual)
}
