package hosttest_test

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/search"
	"github.com/adams-shaun/gorge/server/host"
)

// TestSearchIsHostedWhenLinked is the linked half of server/host's
// TestHostedVocabularyRefusesSearchPolicies: the real search teacher
// (search) is a hosted policy exactly when its package is linked, as it is
// here, and its factory builds an EnvSeat that is also a BoardSeat.
func TestSearchIsHostedWhenLinked(t *testing.T) {
	name, err := host.NormalizeBotPolicy(search.Policy)
	if err != nil || name != search.Policy {
		t.Fatalf("NormalizeBotPolicy(%q) = %q, %v; want the normalised name", search.Policy, name, err)
	}
	s, err := host.NewBotPolicySeat(search.Policy, 7)
	if err != nil {
		t.Fatalf("NewBotPolicySeat(%q): %v", search.Policy, err)
	}
	if _, ok := s.(bots.EnvSeat); !ok {
		t.Errorf("NewBotPolicySeat(%q) built %T, which does not implement bots.EnvSeat", search.Policy, s)
	}
	if _, ok := s.(heuristic.BoardSeat); !ok {
		t.Errorf("NewBotPolicySeat(%q) built %T, which does not implement heuristic.BoardSeat", search.Policy, s)
	}
	if _, err := host.NewBotPolicySeatWithAutoPayMana(search.Policy, 7, true); err != nil {
		t.Errorf("NewBotPolicySeatWithAutoPayMana(%q, auto-pay): %v", search.Policy, err)
	}
}

// TestShadowParityZeroDivergencesSearch is the search row of server/host's
// TestShadowParityZeroDivergences (HD-3), and the row whose honest root
// actually moves the answer. It plays the whole game (maxIntents 0, ~3s) like
// server/host's own rows. A cap is unsound here: the host parks and answers the
// decision at intent index == cap before its cap check crashes the match, so
// when that decision is the shadow seat's, the seat has answered one decision
// the log (and so the scan) never holds, and the contract's "scan visited ==
// seat answered" precondition fails by one. The old 400 cap passed only while
// intent 400 belonged to the opponent; the engine's end-of-combat priority fix
// (93b6057e4, CR 511.1/117.3a) shifted it onto seat 0.
func TestShadowParityZeroDivergencesSearch(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	o.BotDeps = sampleBotDeps(t)
	host.RunShadowParityContract(t, o, search.Policy, 20261010, 0)
}

// TestCaretakerNeverSelectsLifePayingPlanCastProfile is the cast-profile row
// of server/host's TestCaretakerNeverSelectsLifePayingPlan.
func TestCaretakerNeverSelectsLifePayingPlanCastProfile(t *testing.T) {
	host.RunCaretakerLifePlanContract(t, host.CastProfilePolicy)
}
