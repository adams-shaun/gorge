package host

// BP-11 (spec 2026-09-28-hosted-bot-packages §11): the hosted-policy round
// table. This file links every built-in hosted policy gorge carries
// (bots/all) so the tests below see the full registry, and owns the two
// whole-policy gates, both run through the exported contract harness
// (contract*.go):
//
//   - TestEveryHostedPolicyIsDeterministic (§8): every registered hosted
//     entry plays two in-memory 2-seat tables from the same seed — capped at
//     150 intents for search entries (a searched decision costs ~0.2-0.7 s),
//     a whole game otherwise — and the two runs must produce identical event
//     logs and intents, each of which replays cleanly (RunContractTable).
//   - One row per Env entry of the seat-level leak test (§5.3,
//     RunEnvLeakContract): the REAL adapter through the REAL host Env path,
//     with the swap fixture applied to one match's live engine, answers the
//     asserted decision identically from two honest roots whose hidden
//     worlds are name-identical. For sb-tactical the asserted decision is
//     the swap boundary itself; for az-redeal it is a decision the seat
//     actually SEARCHED in both matches (the az search skips a decision with
//     fewer than two candidates without asking its world source at all).

import (
	"reflect"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/sbtactical"
	"github.com/adams-shaun/gorge/bots/spellbench/builtins"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// azRedealPolicy is the az-redeal entry's registry key, spelled out rather
// than imported: bots/all links the package once, above.
const azRedealPolicy = "az-redeal"

// detSearchIntentsCap is the intent cap a search entry plays (§8): a whole
// game of searches would take minutes, and 150 intents already covers the
// first several combat turns of a 2-seat game — enough searched decisions to
// make nondeterminism in the teacher show up.
const detSearchIntentsCap = 150

// TestEveryHostedPolicyIsDeterministic is §8's registry gate: for each
// registered hosted entry, two in-memory 2-seat tables from the same seed
// must produce identical event logs and intents, and each log must replay
// cleanly to the recorded head. The bot entry keeps its existing golden
// check (server/host/host_test.go TestHostedPoliciesReplayDeterministically); this
// gate is the same property, driven through the table-policy path, for every
// entry as it lands.
func TestEveryHostedPolicyIsDeterministic(t *testing.T) {
	entries := bots.Entries()
	if len(entries) < 2 {
		t.Fatalf("the registry holds %d entries; the host must see the built-in policies (known: %v)", len(entries), bots.Names())
	}
	played := false
	for _, e := range entries {
		if e.Name == bp10SpyPolicy || e.Name == closedTestPolicy {
			// Test fixtures that register themselves in this binary
			// (slots_test.go, hosted_closed_test.go); harnesses, not hosted
			// policies.
			continue
		}
		played = true
		e := e
		t.Run(e.Name, func(t *testing.T) {
			// The per-policy rows are independent (each builds its own host
			// and card dependency below) and together dominated the package's
			// sequential phase, which is the module gate's critical path. They
			// run concurrently; every assertion is unchanged.
			t.Parallel()
			intents := 0
			if e.Search {
				intents = detSearchIntentsCap
			}
			run := func(tag string) ContractRun {
				opts := testOptions(t)
				opts.MaxIntents = intents // 0 = play the whole game
				// BP-13: every hosted policy is built with the same card
				// dependency a served table gets, so the registry gate covers
				// the corpus-hungry entries exactly as the host threads them.
				opts.BotDeps = sampleBotDeps(t)
				// Zero budget = unbounded, byte-identical: a wall-clock
				// budget would make a deadline bail-out answer from the
				// fallback in one run and not the other (load, not seed).
				opts.BotUnboundedDecisions = true
				if e.Search {
					// Parallelism changes latency only, never an answer.
					opts.BotSearchParallelism = 4
				}
				return RunContractTable(t, opts, TableConfig{ID: "t1", Name: "det" + tag, Seats: 2, Decks: []string{"a", "b"},
					Seed: 20260928, Pace: 0, Spectator: view.Public, BotPolicy: e.Name,
					// A decklist-reading policy (Info.Env) seats only on a bot-only table that opted in.
					AllowDecklistBots: e.Env})
			}
			a, b := run("a"), run("b")
			assertDeterministic(t, e.Name, a, b, intents)
		})
	}
	if !played {
		t.Fatal("no registered entry was played: the skip list above matched everything")
	}
}

// assertDeterministic is the §8 comparison of two RunContractTable runs of
// one seed. intents > 0 is the cap a search entry played to.
func assertDeterministic(t *testing.T, policy string, a, b ContractRun, intents int) {
	t.Helper()
	if intents > 0 {
		// Precondition: the capped entry actually reached the cap — a
		// truncated run that never happened would compare two empty logs.
		if n := len(a.Log.Intents); n != intents {
			t.Fatalf("run a logged %d intents, want the cap %d", n, intents)
		}
	} else if a.Info.State != protocol.MatchFinished || (a.Info.Result != "win" && a.Info.Result != "draw") {
		// Precondition for the whole-game entries: a real match ran.
		t.Fatalf("%q did not finish with a valid outcome: %+v", policy, a.Info)
	}
	if !reflect.DeepEqual(a.Log.Events, b.Log.Events) || !reflect.DeepEqual(a.Log.Intents, b.Log.Intents) {
		t.Fatalf("two %q runs differ (events %d/%d, intents %d/%d, heads %s/%s, results %q/%q)",
			policy, len(a.Log.Events), len(b.Log.Events), len(a.Log.Intents), len(b.Log.Intents), a.Info.Head, b.Info.Head, a.Info.Result, b.Info.Result)
	}
	if a.Info.Head != b.Info.Head || a.Info.Result != b.Info.Result || !reflect.DeepEqual(a.Info.Winner, b.Info.Winner) {
		t.Fatalf("two %q runs differ in outcome: %+v vs %+v", policy, a.Info, b.Info)
	}
	if n := len(a.Log.Intents); n < 40 {
		t.Fatalf("%q logged only %d intents; too short a game to be evidence of anything", policy, n)
	}
}

// sampleBotDeps is the card dependency the host tests thread: a cards
// registry over the same sample deck pool testOptions deals, built without
// the corpus so the gate runs everywhere. A policy whose factory reads card
// facts (sb-tactical) resolves them from these cards, exactly what its own
// package's tests resolve.
func sampleBotDeps(t *testing.T) bots.Deps {
	t.Helper()
	_, decks := testutil.SampleDecks(t, 4)
	reg := cards.NewRegistry()
	for _, cs := range decks {
		for _, c := range cs {
			reg.Add(c)
		}
	}
	return bots.Deps{Cards: reg}
}

// TestHostedEnvSeatsIgnoreTheRealHiddenCardsSBTactical is the sb-tactical row
// of the leak property (BP-13, §5.3): the REAL sb-tactical adapter at slot 0,
// with the swap applied at the first decision the adapter wants an Env for —
// a priority ask, the only kind sb-tactical's WantsEnv claims. The adapter's
// whole world source is the honest root as its payment planner, so a root
// that depended on the real hidden cards would price the plays differently.
//
// sb-tactical implements no DecideBoard (§5.1), so the row takes the
// View-path spy; PlannerOf adds the routing assertion: at the asserted
// boundary the adapter's planner is the honest root the ENV carried, in both
// matches. The spy row of the same property lives in botenv_test.go.
func TestHostedEnvSeatsIgnoreTheRealHiddenCardsSBTactical(t *testing.T) {
	t.Parallel()
	RunEnvLeakContract(t, EnvLeakContract{
		Options: testOptions(t),
		Policy:  sbtactical.Policy,
		NewSeat: func(actor uint64) seat.Seat {
			s, err := bots.New(sbtactical.Policy, bots.Options{Seed: actor, Deps: sampleBotDeps(t)})
			if err != nil {
				t.Fatalf("the sb-tactical policy is not linked into this binary: %v", err)
			}
			return s
		},
		ViewPath:  true,
		PlannerOf: builtinsPlanner(t),
	})
}

// builtinsPlanner is EnvLeakContract.PlannerOf for an adapter over a
// SpellBench builtins.Seat (sb-tactical here).
func builtinsPlanner(t *testing.T) func(seat.Seat) any {
	return func(s seat.Seat) any {
		ad, ok := s.(interface{ Seat() *builtins.Seat })
		if !ok {
			t.Fatalf("%T does not expose its wrapped builtins seat", s)
		}
		return ad.Seat().Planner()
	}
}

// TestHostedEnvSeatsIgnoreTheRealHiddenCardsAZRedeal is the az-redeal row of
// the leak property (BP-12, §5.3): the REAL az adapter at slot 0, the swap at
// a decision the seat actually SEARCHED (azmcts.Watch, Stats.Searched == 1,
// found by the contract's pre-pass and re-verified in both matches). The az
// seat's whole world source is a redeal over the honest root, so a root that
// depended on the real hidden cards would deal different worlds — and search
// them to different answers — in the two matches. This row is the only az
// driver in the binary's parallel set, so the package-level azmcts.Watch hook
// belongs to it alone.
func TestHostedEnvSeatsIgnoreTheRealHiddenCardsAZRedeal(t *testing.T) {
	t.Parallel()
	probe, restore := azWatchProbe()
	defer restore()
	RunEnvLeakContract(t, EnvLeakContract{
		Options: testOptions(t),
		Policy:  azRedealPolicy,
		NewSeat: func(actor uint64) seat.Seat {
			s, err := bots.New(azRedealPolicy, bots.Options{Seed: actor})
			if err != nil {
				t.Fatalf("the az-redeal policy is not linked into this binary: %v", err)
			}
			return s
		},
		Probe: probe,
	})
}

// azWatchProbe installs azmcts.Watch for one leak row and returns the probe
// over it: a diagnostic of a searched kind with Stats.Searched == 1 is a real
// search. restore puts the previous hook back.
func azWatchProbe() (*SearchProbe, func()) {
	var mu sync.Mutex
	var diags []azmcts.Diag
	prev := azmcts.Watch
	azmcts.Watch = func(dg azmcts.Diag) {
		mu.Lock()
		defer mu.Unlock()
		diags = append(diags, dg)
	}
	return &SearchProbe{
		Mark: func() int {
			mu.Lock()
			defer mu.Unlock()
			return len(diags)
		},
		Searched: func(from, to int) bool {
			mu.Lock()
			defer mu.Unlock()
			for _, dg := range diags[from:to] {
				if dg.Kind != "" && dg.Stats.Searched == 1 {
					return true
				}
			}
			return false
		},
	}, func() { azmcts.Watch = prev }
}
