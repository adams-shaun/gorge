package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// mayplay_grant_budget_gap_test.go pins a measured gap in gorge's potential
// walk, the one dirac mirrors by refusing the same cast (dirac/script_gen.go,
// mayPlayJoinSearch): a may-play cast whose only payment runs through a
// granted mana ability the budgeted prefix search expires before reaching.
//
// The board is tail-08 pair 54, decision 573: nine lands under The World
// Tree ("as long as you control six or more lands, lands you control have
// '{T}: Add one mana of any color'"), five of them untapped. Four of those
// make mana by their own printed abilities (Swamp {B}, Mountain {R}, Sunlit
// Marsh {W}/{B}, The World Tree {G}); the fifth, Dwarven Hold with no storage
// counter, makes none, so Age of Ultron's {4}{B} may-play cast is payable only
// by tapping it for the granted any-colour ability (CR 601.2g, 605.3a).
// potentialPrefixScript's plain search (five gaps: every land has the granted
// ability) spends all potentialScriptNodes (164) clones and finds no script
// (measured: with the budget lifted for this one play it finds the script at
// 657 nodes, 4x the budget), the phased and subsume fallbacks find none, and the
// play is left census_incomplete: unpriced, so not offered. Raising the budget
// or pricing the granted ability's ceiling before the search would move this
// verdict; whoever does it also moves dirac's mirrored search
// (mana_grantedoffers.go) and updates this pin, which asserts today's refusal.

// mayPlayGrantBudgetGapAge puts Age of Ultron in seat 0's exile under its
// own may-play grant (Chase Stein's, as the recording had it).
func mayPlayGrantBudgetGapAge(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	grant := "S:Mode$ Continuous | MayPlay$ True | Affected$ Card.Self | AffectedZone$ Exile | EffectZone$ Exile | Description$ x\n"
	src := strings.Replace(corpusCardText(t, "a/age_of_ultron.txt"), "Oracle:", grant+"Oracle:", 1)
	o := e.G.AddObject(card(t, src), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZExile})
	return o.ID
}

// TestMayPlayGrantBudgetGapLeavesTheCastUnpriced: the cast is payable (a
// search with room finds the script) and gorge's walk leaves it unpriced.
func TestMayPlayGrantBudgetGapLeavesTheCastUnpriced(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	for _, l := range []struct {
		src    string
		tapped bool
	}{
		{"s/swamp.txt", false}, {"d/dwarven_hold.txt", false}, {"m/mountain.txt", true},
		{"s/swamp.txt", true}, {"m/mountain.txt", false}, {"s/swamp.txt", true},
		{"s/sunlit_marsh.txt", false}, {"t/the_world_tree.txt", false}, {"s/swamp.txt", true},
	} {
		id := onBoardReady(t, e, 0, corpusCardText(t, l.src))
		e.G.Obj(id).Tapped = l.tapped
	}
	age := mayPlayGrantBudgetGapAge(t, e)
	e.pending = nil
	e.askPriority(0)

	// the precondition: the cast is payable, a search with room finds the script
	script, why := e.PotentialPlayScript(0, decision.PotentialAction{Kind: "cast", Obj: age, Mode: "mayplay"}, 5000)
	if why != "" || len(script) == 0 {
		t.Fatalf("precondition: the cast is not payable through the granted ability (script %v, verdict %q)", script, why)
	}

	var plan *PotentialPlan
	pps := e.PotentialPaymentPlans(0)
	for i := range pps {
		if pps[i].Action.Kind == "cast" && pps[i].Action.Obj == age && pps[i].Action.Mode == "mayplay" {
			plan = &pps[i]
		}
	}
	if plan == nil {
		t.Fatalf("no may-play plan for Age of Ultron: %s", plansString(pps))
	}
	if plan.Plan != nil || len(plan.Script) != 0 || plan.Reason != "unsupported" || plan.Detail != "census_incomplete" {
		t.Fatalf("the walk now prices the cast (%s, script %d): the budget gap this test pins is closed; update it and dirac's mirror", planString(*plan), len(plan.Script))
	}
}
