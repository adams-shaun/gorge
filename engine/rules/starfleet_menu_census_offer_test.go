package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestStarfleetMenuOfferFallsBackToCensusScript pins the interactive priority
// menu's census-gap arm (legal_walk_autopay.go's autoPayCensus): at a live
// priority ask (hyp nil) the menu offers Academy Ruins' {1}{U}, {T}
// graveyard-recall ability although the floating pool holds no {U} and the
// only {U} on the board is Starfleet Academy's Federation ability ({5},{T}:
// Add X of one chosen colour, X = 1 beside one Grizzly Bear) — the shape the
// planner's source census (paymentPlanManaUnits) misses, so
// planAbilityPayment answers "insufficient" and the canonical pricing proves
// the activation payable through its census relaxation and prefix script
// (CR 601.2g, 605.3b; TestStarfleetFundedActivationPriced). The menu must
// match that pricing: a seat that reads the posed decision never saw the
// option, so the Ruins card was dead in the seat's hands.
//
// The same ask also pins the pricing agreement around the arm's own offer:
// the arm marks its option (decision.Option.CensusOffer), so the canonical
// pricing's prefix search — which reads the posed decision through its
// search clones — still prices the activation payable with its script after
// the menu offered it (sr.offered skips the arm's script-backed offer, as
// the kernel's own listing probe excludes the arm's listings,
// actScriptPays' availNoActScript).
func TestStarfleetMenuOfferFallsBackToCensusScript(t *testing.T) {
	t.Parallel()

	e, ruins := sfuRuinsFixture(t, true, true)

	// Preconditions: the canonical pricing prices the activation (through its
	// prefix script), while the planner arm the menu arm falls back from
	// proves nothing — so the option below can only come from the census arm.
	got, script := sfuRuinsPlan(t, e, ruins)
	if got.Reason != "" || script == 0 {
		t.Fatalf("precondition: canonical pricing does not price the {1}{U} activation: reason %q script %d", got.Reason, script)
	}
	if out := e.planAbilityPayment(0, ruins, 1); out.Reason != "insufficient" {
		t.Fatalf("precondition: planAbilityPayment answers %q (want insufficient, detail %q)", out.Reason, out.Detail)
	}

	opts := e.legalActions(0)
	found := false
	for _, o := range opts {
		if o.Kind == "ability" && o.Obj == ruins && o.Ability == 1 {
			if !o.CensusOffer {
				t.Fatalf("the %v option came from the plain walk, not the census arm", o.Kind)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("priority menu offers no ability option for the Federation-funded {1}{U} activation of ruins obj")
	}

	// The arm's offer must not unprice the activation the canonical pricing
	// proved: the search reads the posed decision through its clones, and an
	// unmarked offer there is taken for "the plain walk lists it" — an empty
	// prefix that prices nothing (dirac/legal.go availNoActScript).
	after, afterScript := sfuRuinsPlan(t, e, ruins)
	if after.Reason != "" || afterScript == 0 {
		t.Fatalf("after the menu offered it, canonical pricing no longer prices the {1}{U} activation: reason %q script %d", after.Reason, afterScript)
	}
}

// sfuPickWindow answers the activation's payment ask the way a seat funding
// the Federation does: the first five Mountains, then the Starfleet source,
// its "Pay 5" production, the {U} colour, and Done when the pool is whole.
func sfuPickWindow(t *testing.T, e *Engine, taps *int) {
	t.Helper()
	d := e.Pending()
	name := func(o decision.Option) string {
		if ob := e.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	pick := -1
	for _, o := range d.Options {
		switch {
		case strings.HasPrefix(d.Prompt, "Activate mana abilities") && *taps < 5 && name(o) == "Mountain":
			*taps++
			pick = o.Index
		case strings.HasPrefix(d.Prompt, "Activate mana abilities") && *taps >= 5 && name(o) == "Starfleet Academy":
			pick = o.Index
		case strings.Contains(o.Label, "Pay 5") && (o.ManaSymbol == "" || o.ManaSymbol == "U"):
			pick = o.Index
		}
		if pick >= 0 {
			break
		}
	}
	if pick < 0 {
		t.Fatalf("no answer for the payment ask %q (%d options)", d.Prompt, len(d.Options))
	}
	submitChoices(t, e, pick)
}

// TestStarfleetMenuCensusOfferIsPlayable pins the offer/payment agreement of
// the arm: a seat that takes the offered "ability" option, answers the target
// and funds the Federation in the ordinary CR 601.2g window ends with Academy
// Ruins' ability on the stack and Ruins tapped -- not an aborted activation
// that leaves the seat a priority-window hold-out (CR 733.2). The window lists
// the Mountains although the {U} owed is not a colour they add: their mana
// pays the Federation's fee (pc.censusWindow, manaWindowAsk).
func TestStarfleetMenuCensusOfferIsPlayable(t *testing.T) {
	t.Parallel()

	e, ruins := sfuRuinsFixture(t, true, true)
	picked := false
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == ruins && o.Ability == 1 {
			if !o.CensusOffer {
				t.Fatal("precondition: the offered option is not the census arm's")
			}
			submitChoices(t, e, o.Index)
			picked = true
			break
		}
	}
	if !picked {
		t.Fatal("precondition: the menu offers no census-backed Ruins activation")
	}
	submitChoices(t, e, 0) // the target ask: Sol Ring is the one artifact card
	taps := 0
	for i := 0; i < 24 && e.Pending() != nil && e.Pending().Kind == decision.KChoose; i++ {
		sfuPickWindow(t, e, &taps)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack has %d objects after funding the window, want the Ruins activation (taps %d, pending %v)", len(e.G.Stack), taps, e.Pending().Prompt)
	}
	if !e.G.Obj(ruins).Tapped {
		t.Fatal("Academy Ruins is untapped: its {T} was not paid")
	}
	if top := e.G.Obj(e.G.Stack[0]); top == nil || top.StackKind != state.StackKindActivated {
		t.Fatalf("the stack object is not an activated ability: %+v", top)
	}
}
