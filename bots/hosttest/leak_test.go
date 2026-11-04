package hosttest_test

// The seat-level hidden-card leak row (design §5.3,
// docs/design/2026-09-28-hosted-bot-packages.md) for every hosted Env policy
// this package gates, through server/host's host.RunEnvLeakContract: the REAL adapter at slot 0 through the REAL host
// Env path, the opponent's hidden cards swapped in one match's live engine,
// and the decision at the asserted boundary answered identically from two
// honest roots whose hidden worlds are name-identical.

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azc4"
	"github.com/adams-shaun/gorge/bots/azc5"
	"github.com/adams-shaun/gorge/bots/sbbuiltin"
	"github.com/adams-shaun/gorge/bots/sbsearch"
	"github.com/adams-shaun/gorge/bots/search"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/host"
)

// TestHostedEnvSeatsIgnoreTheRealHiddenCardsSearch: the search adapter,
// whose WantsEnv gates the Env path on searchseat.Eligible; the swap is at
// the first decision it wants an Env for.
func TestHostedEnvSeatsIgnoreTheRealHiddenCardsSearch(t *testing.T) {
	t.Parallel()
	host.RunEnvLeakContract(t, host.EnvLeakContract{
		Options: testOptions(t),
		Policy:  search.Policy,
		NewSeat: func(actor uint64) seat.Seat {
			return newSeat(t, search.Policy, bots.Options{Seed: actor, SearchParallelism: 2})
		},
	})
}

// TestHostedEnvSeatsIgnoreTheRealHiddenCardsSBPlainPolicies covers the three
// lightweight SpellBench adapters. Each decision is priority (their only Env
// kind); the View-path spy is used (they implement no DecideBoard), and the
// honest root must be installed as the wrapped builtin planner before it
// answers.
func TestHostedEnvSeatsIgnoreTheRealHiddenCardsSBPlainPolicies(t *testing.T) {
	t.Parallel()
	for _, policy := range []string{sbbuiltin.HeuristicPolicy, sbbuiltin.UniformPolicy, sbbuiltin.FirstPolicy} {
		t.Run(policy, func(t *testing.T) {
			host.RunEnvLeakContract(t, host.EnvLeakContract{
				Options: testOptions(t),
				Policy:  policy,
				NewSeat: func(actor uint64) seat.Seat {
					return newSeat(t, policy, bots.Options{Seed: actor, AutoPayMana: true, Deps: sampleBotDeps(t)})
				},
				ViewPath:  true,
				PlannerOf: builtinsPlanner(t),
			})
		})
	}
}

// TestHostedEnvSeatsIgnoreTheRealHiddenCardsSBSearch: sb-search-lite-atk
// redeals its worlds from the honest root. Its priority path plays
// sb-tactical's own pick without dealing a world when the pick is a land or
// there is no choice, so the swap is placed at a decision the search really
// ran on (sbsearch.Watch), re-verified in both matches.
func TestHostedEnvSeatsIgnoreTheRealHiddenCardsSBSearch(t *testing.T) {
	watchMu.Lock()
	defer watchMu.Unlock()
	probe, restore := sbWatchProbe()
	defer restore()
	host.RunEnvLeakContract(t, host.EnvLeakContract{
		Options: testOptions(t),
		Policy:  sbsearch.Policy,
		NewSeat: func(actor uint64) seat.Seat {
			return newSeat(t, sbsearch.Policy, bots.Options{Seed: actor, Deps: sampleBotDeps(t)})
		},
		ViewPath: true,
		Probe:    probe,
	})
}

// The az-c3/c4/c5 rows: the swap at a decision the az seat actually
// SEARCHED (azmcts.Watch), re-verified in both matches. They share the
// package-level azmcts.Watch hook, so they run one at a time.
func TestHostedEnvSeatsIgnoreTheRealHiddenCardsAZC3(t *testing.T) { azLeakRow(t, azc3.Policy) }
func TestHostedEnvSeatsIgnoreTheRealHiddenCardsAZC4(t *testing.T) { azLeakRow(t, azc4.Policy) }
func TestHostedEnvSeatsIgnoreTheRealHiddenCardsAZC5(t *testing.T) { azLeakRow(t, azc5.Policy) }

func azLeakRow(t *testing.T, policy string) {
	enableAZ(t, policy)
	watchMu.Lock()
	defer watchMu.Unlock()
	probe, restore := azWatchProbe()
	defer restore()
	host.RunEnvLeakContract(t, host.EnvLeakContract{
		Options: testOptions(t),
		Policy:  policy,
		NewSeat: func(actor uint64) seat.Seat {
			return newSeat(t, policy, bots.Options{Seed: actor, AutoPayMana: true})
		},
		Probe: probe,
	})
}
