package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	// tmSpell costs {3}{G}: four mana.
	tmSpell = "Name:Tap Merge Spell\nManaCost:3 G\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	// tmGranter gives every land its controller has a tap for one mana of any colour.
	tmGranter = "Name:Tap Merge Granter\nManaCost:1\nTypes:Creature Scout\nPT:1/1\n" +
		"S:Mode$ Continuous | Affected$ Land.YouCtrl | AddAbility$ TmAny | Description$ Lands tap for any colour.\n" +
		"SVar:TmAny:AB$ Mana | Cost$ T | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any colour.\nOracle:x\n"
	tmForest = "Name:Tap Merge Forest\nManaCost:no cost\nTypes:Basic Land Forest\nOracle:x\n"
)

// CR 602.2b: a Forest with a granted "{T}: Add one mana of any colour" taps
// for one mana, through its own ability or the granted one, never both. The
// relaxed proof lists the granted ability as a second unit beside the Forest's
// own, so two Forests look like four sources and a four-mana spell passes it;
// with each tap-only source's units merged (mergeTap) the same proof is
// insufficient, which is what lets the fallback skip a prefix search that can
// only run to its node budget.
func TestRelaxedProofMergesATapOnlySourceIntoOneUnit(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9411, ppFiller)
	spell := handCard(e, card(t, tmSpell), 0)
	onBoardReady(t, e, 0, tmGranter)
	forests := []state.ObjID{onBoard(t, e, 0, tmForest), onBoard(t, e, 0, tmForest)}
	e.G.Step = state.StepMain1
	ppPriority(t, e)

	hyp := state.Mana{}
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || !census.relaxable {
		t.Fatalf("precondition: want an incomplete relaxable census, got complete=%v relaxable=%v", census.complete, census.relaxable)
	}
	if len(census.tapOnly) != len(forests) {
		t.Fatalf("tapOnly = %v, want the %d Forests %v", census.tapOnly, len(forests), forests)
	}
	opt := decision.Option{Kind: "cast", Obj: spell}
	verdict := func() PaymentPlanOutcome {
		got, _ := e.potentialPlayVerdict(0, opt)
		return got
	}
	if got := e.paymentPlanRelaxProof(census, verdict); got.Reason == pay.ReasonInsufficient {
		t.Fatalf("precondition: the plain relaxed proof must pass (two Forests counted as four units), got %+v", got)
	}
	tight := census
	tight.mergeTap = true
	if got := e.paymentPlanRelaxProof(tight, verdict); got.Reason != pay.ReasonInsufficient {
		t.Fatalf("merged proof = %+v, want insufficient: two taps cannot make four mana", got)
	}
	if len(e.PaymentPlanRelaxedMerge) != 0 || len(e.PaymentPlanRelaxed) != 0 {
		t.Fatalf("the proof left its scratch set: merge %v relaxed %v", e.PaymentPlanRelaxedMerge, e.PaymentPlanRelaxed)
	}
}

// tapMergeHost is a census host whose relaxed proof passes unless the census
// asks for merged tap units, and which counts the prefix searches attempted.
type tapMergeHost struct {
	census      paymentPlanCensus
	prefixCalls int
}

func (h *tapMergeHost) paymentPlanCensusOf(state.PlayerID, *state.Mana) paymentPlanCensus {
	return h.census
}

func (h *tapMergeHost) paymentPlanRelaxProof(c paymentPlanCensus, _ func() PaymentPlanOutcome) PaymentPlanOutcome {
	if c.mergeTap {
		return PaymentPlanOutcome{Reason: pay.ReasonInsufficient}
	}
	return PaymentPlanOutcome{}
}

func (h *tapMergeHost) potentialTargetPriced(state.PlayerID, decision.Option) bool { return false }

func (h *tapMergeHost) potentialPrefixScript(state.PlayerID, decision.Option, []state.ObjID, int) ([]ScriptStep, bool) {
	h.prefixCalls++
	return nil, false
}

// A play the relaxed proof passes but the merged proof refuses runs no prefix
// search, yet still takes its slot of the walk's search budget (dirac counts
// the slot whether or not gorge searched), and a census with no tap-only
// source still searches.
func TestPotentialCensusFallbackSkipsASearchTheMergedProofRefuses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		tapOnly   []state.ObjID
		wantCalls int
	}{
		{"every gap taps", []state.ObjID{7}, 0},
		{"no tap-only source", nil, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := &tapMergeHost{census: paymentPlanCensus{relaxable: true, gaps: []state.ObjID{7}, tapOnly: tc.tapOnly}}
			var census paymentPlanCensus
			censused, scripts := false, 0
			verdict := func() PaymentPlanOutcome { return PaymentPlanOutcome{Reason: pay.ReasonInsufficient} }
			got, script := potentialCensusFallback(h, 0, decision.Option{Kind: "ability"}, state.Mana{},
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
