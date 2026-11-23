package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	// ssGranter gives every land its controller has a tap for one mana of any colour.
	ssGranter = "Name:Supply Granter\nManaCost:1\nTypes:Creature Scout\nPT:1/1\n" +
		"S:Mode$ Continuous | Affected$ Land.YouCtrl | AddAbility$ SsAny | Description$ Lands tap for any colour.\n" +
		"SVar:SsAny:AB$ Mana | Cost$ T | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any colour.\nOracle:x\n"
	ssForest = "Name:Supply Forest\nManaCost:no cost\nTypes:Basic Land Forest\nOracle:x\n"
	// ssBig and ssSmall are artifacts whose ability costs five and two generic mana.
	ssBig   = "Name:Supply Big\nManaCost:no cost\nTypes:Artifact\nA:AB$ Draw | Cost$ 5 | NumCards$ 1 | SpellDescription$ Draw.\nOracle:x\n"
	ssSmall = "Name:Supply Small\nManaCost:no cost\nTypes:Artifact\nA:AB$ Draw | Cost$ 2 | NumCards$ 1 | SpellDescription$ Draw.\nOracle:x\n"
)

// ssAbilityOption is the offer for the first activated (non-mana) ability of
// the permanent id.
func ssAbilityOption(t *testing.T, e *Engine, id state.ObjID) decision.Option {
	t.Helper()
	ob := e.G.Obj(id)
	for i := 0; i < 4; i++ {
		if pa, ok := ob.PileAbilityAt(i); ok && pa.SA != nil && pa.SA.Kind == "AB" {
			return decision.Option{Kind: optAbility, Obj: id, Ability: i}
		}
	}
	t.Fatalf("object %d has no activated ability", id)
	return decision.Option{}
}

// CR 602.2b: a land taps once, through its own mana ability or a granted one,
// so two lands make at most two mana however many abilities they list. The
// relaxed census proof counts the granted ability beside the printed one and
// cannot tell a five-mana ability from a payable one; the supply count, one
// mana per source plus the pool, can. A cost the supply covers is never proven.
func TestCensusOutrunsProvesAnAbilityNoSourceCountPays(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9412, ppFiller)
	big := onBoardReady(t, e, 0, ssBig)
	small := onBoardReady(t, e, 0, ssSmall)
	onBoardReady(t, e, 0, ssGranter)
	onBoard(t, e, 0, ssForest)
	onBoard(t, e, 0, ssForest)
	e.G.Step = state.StepMain1
	ppPriority(t, e)

	census := e.paymentPlanCensusOf(0, &state.Mana{})
	if census.complete || !census.relaxable {
		t.Fatalf("precondition: want an incomplete relaxable census, got complete=%v relaxable=%v", census.complete, census.relaxable)
	}
	if census.outruns == nil {
		t.Fatal("census.outruns is nil: every mana ability on the board is a lone {T} for one mana")
	}
	if !census.outruns(ssAbilityOption(t, e, big)) {
		t.Error("a five-mana ability beside two lands (two mana) is not proven unpayable")
	}
	if census.outruns(ssAbilityOption(t, e, small)) {
		t.Error("a two-mana ability beside two lands is proven unpayable, though two taps pay it")
	}
	if census.outruns(decision.Option{Kind: "cast", Obj: big}) {
		t.Error("a play that is not an activated ability is proven unpayable")
	}
}

// outrunsHost is a census host that counts the prefix searches attempted; its
// relaxed proof always passes, so only the supply count can skip a search.
type outrunsHost struct {
	census      paymentPlanCensus
	prefixCalls int
}

func (h *outrunsHost) paymentPlanCensusOf(state.PlayerID, *state.Mana) paymentPlanCensus {
	return h.census
}

func (h *outrunsHost) paymentPlanRelaxProof(paymentPlanCensus, func() PaymentPlanOutcome) PaymentPlanOutcome {
	return PaymentPlanOutcome{}
}

func (h *outrunsHost) potentialTargetPriced(state.PlayerID, decision.Option) bool { return false }

func (h *outrunsHost) potentialPrefixScript(state.PlayerID, decision.Option, []state.ObjID, int) ([]ScriptStep, bool) {
	h.prefixCalls++
	return nil, false
}

// A play the census proves unpayable by its supply runs no prefix search, yet
// still takes its slot of the walk's search budget (dirac counts the slot
// whether or not gorge searched); a census with no such proof still searches.
func TestPotentialCensusFallbackSkipsASearchTheSupplyCountRefuses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		outruns   func(decision.Option) bool
		wantCalls int
	}{
		{"supply outrun", func(decision.Option) bool { return true }, 0},
		{"supply covers the cost", func(decision.Option) bool { return false }, 1},
		{"no supply proof", nil, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := &outrunsHost{census: paymentPlanCensus{relaxable: true, gaps: []state.ObjID{7}, outruns: tc.outruns}}
			var census paymentPlanCensus
			censused, scripts := false, 0
			verdict := func() PaymentPlanOutcome { return PaymentPlanOutcome{Reason: pay.ReasonInsufficient} }
			got, script := potentialCensusFallback(h, 0, decision.Option{Kind: optAbility}, state.Mana{},
				PaymentPlanOutcome{Reason: pay.ReasonInsufficient}, false, verdict, potentialScriptPlays, &census, &censused, &scripts)
			if h.prefixCalls != tc.wantCalls {
				t.Fatalf("prefix searches = %d, want %d", h.prefixCalls, tc.wantCalls)
			}
			if scripts != 1 {
				t.Fatalf("search slots spent = %d, want 1 (the slot counts even when the search is skipped)", scripts)
			}
			if script != nil || got.Detail != "census_incomplete" {
				t.Fatalf("got %+v script %v, want the unpriced census_incomplete verdict and no script", got, script)
			}
		})
	}
}
