package hosttest_test

// Design §8 (docs/design/2026-09-28-hosted-bot-packages.md) for the policies
// this package gates:
// two in-memory 2-seat tables from the same seed produce identical event
// logs and intents, and each replays to its recorded head
// (host.RunContractTable). server/host's TestEveryHostedPolicyIsDeterministic
// covers the policies bots/all links; this covers the rest cmd/gorged links.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// detSearchIntentsCap is the intent cap a search entry plays: 150 intents
// covers the first several combat turns of a 2-seat game.
const detSearchIntentsCap = 150

// linkedPolicies are the always-registered policies this package links on
// top of bots/all. forge-heuristic and forge-heuristic-hold are bots/all's
// (bots/forgeheuristic) but server/host's tests do not gate them, so their
// host rows live here.
var linkedPolicies = []string{"cast-profile", "sb-first", "sb-uniform", "sb-heuristic", "sb-search-lite-atk", "search", "forge-heuristic", "forge-heuristic-hold"}

// builtinPolicies are the ones bots/all links that server/host's own tests
// gate.
var builtinPolicies = []string{"bot", "lethal-pressure", "az-redeal", "az-redeal-maxn", "sb-tactical"}

// TestEveryRegisteredPolicyIsGated keeps linkedPolicies honest: every
// entry this binary registers is gated here or in server/host.
func TestEveryRegisteredPolicyIsGated(t *testing.T) {
	known := map[string]bool{}
	for _, n := range append(append([]string(nil), linkedPolicies...), builtinPolicies...) {
		known[n] = true
	}
	for _, e := range bots.Entries() {
		if !known[e.Name] {
			t.Errorf("registered policy %q has no host gate row (add it to linkedPolicies)", e.Name)
		}
	}
	for _, n := range linkedPolicies {
		if _, ok := bots.Lookup(n); !ok {
			t.Errorf("%q is not registered: this package no longer links it", n)
		}
	}
}

func TestEveryLinkedPolicyIsDeterministic(t *testing.T) {
	for _, name := range linkedPolicies {
		e, ok := bots.Lookup(name)
		if !ok {
			t.Fatalf("%q is not registered", name)
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			intents := 0
			if e.Search {
				intents = detSearchIntentsCap
			}
			run := func(tag string) host.ContractRun {
				opts := testOptions(t)
				opts.MaxIntents = intents // 0 = play the whole game
				opts.BotDeps = sampleBotDeps(t)
				// Zero budget = unbounded: a wall-clock budget would make a
				// deadline bail-out differ run to run (load, not seed).
				opts.BotUnboundedDecisions = true
				if e.Search {
					opts.BotSearchParallelism = 4
				}
				return host.RunContractTable(t, opts, host.TableConfig{ID: "t1", Name: "det" + tag, Seats: 2, Decks: []string{"a", "b"},
					Seed: 20260928, Pace: 0, Spectator: view.Public, BotPolicy: e.Name, AllowDecklistBots: e.Env})
			}
			a, b := run("a"), run("b")
			if intents > 0 {
				if n := len(a.Log.Intents); n != intents {
					t.Fatalf("run a logged %d intents, want the cap %d", n, intents)
				}
			} else if a.Info.State != protocol.MatchFinished || (a.Info.Result != "win" && a.Info.Result != "draw") {
				t.Fatalf("%q did not finish with a valid outcome: %+v", name, a.Info)
			}
			if !reflect.DeepEqual(a.Log.Events, b.Log.Events) || !reflect.DeepEqual(a.Log.Intents, b.Log.Intents) {
				t.Fatalf("two %q runs differ (events %d/%d, intents %d/%d, heads %s/%s, results %q/%q)",
					name, len(a.Log.Events), len(b.Log.Events), len(a.Log.Intents), len(b.Log.Intents), a.Info.Head, b.Info.Head, a.Info.Result, b.Info.Result)
			}
			if a.Info.Head != b.Info.Head || a.Info.Result != b.Info.Result || !reflect.DeepEqual(a.Info.Winner, b.Info.Winner) {
				t.Fatalf("two %q runs differ in outcome: %+v vs %+v", name, a.Info, b.Info)
			}
			if n := len(a.Log.Intents); n < 40 {
				t.Fatalf("%q logged only %d intents; too short a game to be evidence of anything", name, n)
			}
		})
	}
}

// TestCastProfileReplaysLikeTheBot is cast-profile's rows of server/host's
// TestHostedPoliciesReplayDeterministically: a 4-seat cast-profile table
// (manual pay and auto-pay) plays the same game twice, replays to its head,
// finishes with a valid outcome (an auto-pay run carries planned payments),
// and -- the embedded default profile being the production bot's weights --
// is intent-identical to the explicit-bot table of the same seed.
func TestCastProfileReplaysLikeTheBot(t *testing.T) {
	run := func(policy string, autoPay bool) host.ContractRun {
		return host.RunContractTable(t, testOptions(t), host.TableConfig{ID: "t1", Name: "Table t1", Seats: 4,
			Decks: []string{"a", "b", "c", "d"}, Seed: 99, Pace: 0, Spectator: view.Omniscient,
			BotPolicy: policy, AutoMana: autoPay, BotAutoPayMana: autoPay})
	}
	same := func(a, b host.ContractRun) bool {
		return reflect.DeepEqual(a.Log.Events, b.Log.Events) && reflect.DeepEqual(a.Log.Intents, b.Log.Intents) &&
			a.Info.Head == b.Info.Head && a.Info.Result == b.Info.Result && reflect.DeepEqual(a.Info.Winner, b.Info.Winner)
	}
	for _, autoPay := range []bool{false, true} {
		a, b := run(host.CastProfilePolicy, autoPay), run(host.CastProfilePolicy, autoPay)
		for _, got := range []host.ContractRun{a, b} {
			if got.Info.State != protocol.MatchFinished || (got.Info.Result != "win" && got.Info.Result != "draw") || (got.Info.Result == "win") != (got.Info.Winner != nil) {
				t.Fatalf("cast-profile (auto-pay %v) did not finish with a valid outcome: %+v", autoPay, got.Info)
			}
		}
		if !same(a, b) {
			t.Fatalf("two cast-profile (auto-pay %v) runs differ: %+v vs %+v", autoPay, a.Info, b.Info)
		}
		if autoPay {
			planned := false
			for _, ev := range a.Log.Events {
				if ev.Kind == events.DecisionMade && strings.Contains(ev.Text, ";payment:") {
					planned = true
					break
				}
			}
			if !planned {
				t.Fatal("cast-profile auto-pay run has no planned DecisionMade event")
			}
			continue
		}
		if bot := run(host.BotPolicy, false); !same(a, bot) {
			t.Fatalf("%q on the default profile and explicit %q differ: %+v vs %+v", host.CastProfilePolicy, host.BotPolicy, a.Info, bot.Info)
		}
	}
}
