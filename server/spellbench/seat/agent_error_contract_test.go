package seat

import (
	"context"
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
)

// TestUnposableClassifiesRawContractFailure: observe and identity raise
// engine_contract_failure errors raw, without a mapping sentinel. They are the
// adapter's limit, so the seat answers with its fallback and keeps playing
// instead of forfeiting the match as agent_error.
func TestUnposableClassifiesRawContractFailure(t *testing.T) {
	d := decision.Decision{Seq: 7, Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "yes"}, {Index: 1, Kind: "no"}}}
	a := &Agent{fallback: heuristic.NewBot(1), env: &mapping.Env{}}
	in := a.unposable(context.Background(), view.View{}, d, errors.New("engine_contract_failure:look_ref_not_hidden 98"))
	if a.forfeit != "" {
		t.Fatalf("raw contract failure forfeited the seat: %s", a.forfeit)
	}
	if a.unposed != 1 {
		t.Fatalf("Unposed = %d, want 1", a.unposed)
	}
	if in.Seq != d.Seq || in.Player != d.Player {
		t.Fatalf("fallback answered %d/%d, want %d/%d", in.Seq, in.Player, d.Seq, d.Player)
	}
}

// TestUnposableForfeitsANonContractError: the fallback's boundary stays where
// it was -- an error that is not an engine-contract limit is still the agent's
// and still forfeits.
func TestUnposableForfeitsANonContractError(t *testing.T) {
	d := decision.Decision{Seq: 8, Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "yes"}, {Index: 1, Kind: "no"}}}
	a := &Agent{fallback: heuristic.NewBot(1), env: &mapping.Env{}}
	a.unposable(context.Background(), view.View{}, d, errors.New("agent said no"))
	if a.forfeit != forfeitAgent {
		t.Fatalf("forfeit = %q, want %q", a.forfeit, forfeitAgent)
	}
}
