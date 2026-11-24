package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// mayplay_grant_budget_gap_test.go pins the may-play fallback of gorge's
// potential walk (potentialPrefixScript's last run, dirac's mirror is
// grantSearch's): a may-play cast whose only payment runs through a granted
// mana ability the plain prefix search expires before reaching.
//
// The board is tail-08 pair 54, decision 573: nine lands under The World
// Tree ("as long as you control six or more lands, lands you control have
// '{T}: Add one mana of any color'"), five of them untapped. Four of those
// make mana by their own printed abilities (Swamp {B}, Mountain {R}, Sunlit
// Marsh {W}/{B}, The World Tree {G}); the fifth, Dwarven Hold with no storage
// counter, makes none, so Age of Ultron's {4}{B} may-play cast is payable only
// by tapping it for the granted any-colour ability (CR 601.2g, 605.3a).
// The plain search (five gaps: every land has the granted ability) spends all
// potentialScriptNodes (164) clones and finds no script (measured: the script
// is the 657th node), and the phased and subsume fallbacks find none; the
// may-play fallback's plain run, potentialMayPlayNodes (1024) clones, finds
// it, so the cast is priced by that script and offered.

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

// TestMayPlayGrantBudgetGapPricesTheCast: the cast is payable (a search with
// room finds the script) and gorge's walk prices it by that script.
func TestMayPlayGrantBudgetGapPricesTheCast(t *testing.T) {
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
	if plan.Reason != "" || len(plan.Script) == 0 {
		t.Fatalf("the walk leaves the payable cast unpriced (%s, script %d)", planString(*plan), len(plan.Script))
	}
	// the plain run alone could not: its 164 clones expire first
	if _, why := e.PotentialPlayScript(0, decision.PotentialAction{Kind: "cast", Obj: age, Mode: "mayplay"}, potentialScriptNodes); why == "" {
		t.Fatalf("precondition: the plain search's budget now reaches the script; the fallback this test pins is dead code")
	}
}
