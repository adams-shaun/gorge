package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The token-plan cap (tokenPlanMintCap): a CreateToken replacement plan
// grows exponentially in the applicable matches, and a game that stacks
// enough doublers under one creation asks for an answer that cannot be
// represented. The abort rides the livelock contract -- a *LivelockError
// panic the harness recovers and records, named reason "token plan" --
// because that is exactly the watcher's object-cap shape: rules-correct,
// unrepresentable, abort-and-report.

// TestTokenPlanCapAbortsStackedDoublers drives one would-be token through
// 21 Doubling Seasons: the plan reaches 2^21 mints, past the 1<<20 cap, so
// the drive aborts with the token-plan diagnostic instead of growing an
// unrepresentable slice (measured: the pre-cap engine grew 2^27 mints in
// one FDN sweep game and OOM-killed the training box). Without the cap the
// drive builds the plan and the emission trips the watcher's runaway
// backstop with a DIFFERENT reason ("runaway resolution"), which is what
// makes this assertion pin the cap specifically.
func TestTokenPlanCapAbortsStackedDoublers(t *testing.T) {
	t.Parallel()
	ds := corpusAlternativeCard(t, "Doubling Season")
	maker := cardByName(t, tokenForgeSrc("g_1_1_squirrel"))
	deck := make([]*cards.Card, 0, 22)
	for i := 0; i < 21; i++ {
		deck = append(deck, ds)
	}
	deck = append(deck, maker)
	e, _ := tokenReplGame(t, 47, deck...)
	for i := 0; i < 21; i++ {
		moveSeededCard(t, e, 0, ds, state.ZBattlefield)
	}
	m := moveSeededCard(t, e, 0, maker, state.ZBattlefield)
	ll := func() (ll *LivelockError) {
		defer func() {
			if r := recover(); r != nil {
				ll, _ = r.(*LivelockError)
			}
		}()
		activateTokenForge(t, e, m)
		return nil
	}()
	if ll == nil {
		t.Fatalf("21 stacked Doubling Seasons did not abort the token creation")
	}
	if ll.Reason != "token plan" {
		t.Fatalf("abort reason %q, want \"token plan\": %v", ll.Reason, ll)
	}
	if ll.Count <= tokenPlanMintCap {
		t.Fatalf("abort names %d mints, want past the cap %d: %v", ll.Count, tokenPlanMintCap, ll)
	}
	if ll.Error() == "" {
		t.Fatalf("abort rendered no diagnostic")
	}
}

// TestTokenPlanCapAbortsCopyProposalAmount pins the proposal-side guard: a
// token-copy proposal whose caller-side Amount alone is past the cap aborts
// before the plan slice exists. The abort must not touch the game's state:
// the same engine keeps driving afterwards.
func TestTokenPlanCapAbortsCopyProposalAmount(t *testing.T) {
	t.Parallel()
	e, _ := tokenReplGame(t, 47)
	ev := events.Event{Kind: events.TokenCreate, Obj: 1, Player: 0, Amount: tokenPlanMintCap + 1}
	ll := func() (ll *LivelockError) {
		defer func() {
			if r := recover(); r != nil {
				ll, _ = r.(*LivelockError)
			}
		}()
		e.continueCopyTokenProposal(ev, nil)
		return nil
	}()
	if ll == nil {
		t.Fatalf("a %d-mint copy proposal did not abort", ev.Amount)
	}
	if ll.Reason != "token plan" {
		t.Fatalf("abort reason %q, want \"token plan\": %v", ll.Reason, ll)
	}
	if ll.Count != tokenPlanMintCap+1 || ll.Object != 1 {
		t.Fatalf("abort misreports the plan: %v", ll)
	}
}

// TestTokenPlanCapAllowsRealisticDoublerStack pins the cap's headroom: ten
// Doubling Seasons under one creation plan 2^10 mints, well inside the cap,
// and the game plays them all out and replays byte-identically -- the cap
// exists for the unrepresentable tail, not for any realistic stack.
func TestTokenPlanCapAllowsRealisticDoublerStack(t *testing.T) {
	t.Parallel()
	ds := corpusAlternativeCard(t, "Doubling Season")
	maker := cardByName(t, tokenForgeSrc("g_1_1_squirrel"))
	deck := make([]*cards.Card, 0, 11)
	for i := 0; i < 10; i++ {
		deck = append(deck, ds)
	}
	deck = append(deck, maker)
	e, cfg := tokenReplGame(t, 47, deck...)
	for i := 0; i < 10; i++ {
		moveSeededCard(t, e, 0, ds, state.ZBattlefield)
	}
	m := moveSeededCard(t, e, 0, maker, state.ZBattlefield)
	activateTokenForge(t, e, m)
	if got := countTokensNamedOnSeat(t, e, 0, "Squirrel Token"); got != 1024 {
		t.Fatalf("10 Doubling Seasons made %d Squirrel Tokens, want 1024", got)
	}
	replayCheck(t, e, cfg)
}
