package hosttest_test

// The closed-list and decklist gates for the hosted az policies
// (docs/design/2026-11-03-hosted-az-policies.md):
//
//   - TestAZC4/AZC5NeverSeesTheHumansHiddenCards: the privacy proof, three
//     hosted games against a human (host.RunClosedPrivacyContract).
//   - TestDecklistGateAtHumanTables: open-list az-c3 is refused at a human
//     table with or without the opt-in; az-c4 and az-c5 are accepted
//     without it.
//   - TestClosedListGuardRefusesAnOpenSeatAtAHumanTable: the host checks the
//     seats it built, not the declaration.
//   - TestHostRefusesAZC3WithoutAutoPay.

import (
	"context"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azc4"
	"github.com/adams-shaun/gorge/bots/azc5"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestAZC4NeverSeesTheHumansHiddenCards(t *testing.T) { closedPrivacyGames(t, azc4.Policy) }
func TestAZC5NeverSeesTheHumansHiddenCards(t *testing.T) { closedPrivacyGames(t, azc5.Policy) }

// closedPrivacyGames plays three hosted games, the policy against a human
// whose deck (mono-red goblins) and sideboard (mono-black aggro) are made of
// cards that appear nowhere in the belief corpus or the bot's own list
// (mono-white equipment), and proves the human's unrevealed cards never
// reach the bot.
func closedPrivacyGames(t *testing.T, policy string) {
	belief := enableAZ(t, policy)
	reg := testutil.CorpusRegistry(t)
	host.RunClosedPrivacyContract(t, host.ClosedPrivacyContract{
		Options:        testOptions(t),
		Policy:         policy,
		Belief:         belief,
		BotDeck:        testutil.RepoDeck(t, reg, "mono-white-equipment"),
		HumanDeck:      testutil.RepoDeck(t, reg, "mono-red-goblins"),
		HumanSideboard: testutil.RepoDeck(t, reg, "mono-black-aggro"),
		Seeds:          []uint64{20261014, 20261015, 20261016},
	})
}

// az-c3 (enabled without a belief, as gorged serves it) is refused at a
// table with a human, with or without the opt-in; az-c4 and az-c5 are
// accepted there without it, and on a bot-only table without it too. (The
// az-redeal and sb-tactical rows are server/host's TestDecklistGateAtHumanTables.)
func TestDecklistGateAtHumanTables(t *testing.T) {
	enableAZ(t, azc3.Policy)
	enableAZ(t, azc4.Policy)
	enableAZ(t, azc5.Policy)
	table := func(policy string, humans []int, optIn bool) host.TableConfig {
		return host.TableConfig{ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Seed: 1, Spectator: view.Public,
			Humans: humans, BotPolicy: policy, AutoMana: true, BotAutoPayMana: true, AllowDecklistBots: optIn}
	}
	if !bots.ReadsDecklists(azc3.Policy) {
		t.Fatalf("precondition: %q does not read decklists", azc3.Policy)
	}
	for _, optIn := range []bool{false, true} {
		r, err := host.New(testOptions(t))
		if err != nil {
			t.Fatal(err)
		}
		err = r.AddTable(table(azc3.Policy, []int{0}, optIn))
		if err == nil || !strings.Contains(err.Error(), "it cannot seat at a table with a human") {
			t.Errorf("%s (opt-in %v) at a human table: error %v, want the decklist refusal", azc3.Policy, optIn, err)
		}
		r.Close()
	}
	for _, policy := range []string{azc4.Policy, azc5.Policy} {
		if bots.ReadsDecklists(policy) {
			t.Fatalf("%s reads decklists", policy)
		}
		for _, humans := range [][]int{{0}, nil} {
			r, err := host.New(testOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			if err := r.AddTable(table(policy, humans, false)); err != nil {
				t.Errorf("%s (humans %v) without the opt-in: %v", policy, humans, err)
			}
			r.Close()
		}
	}
}

// guardSpy wraps the open-list seat and records whether the host asked it
// anything; it forwards every interface the host dispatches on.
type guardSpy struct {
	inner seat.Seat
	asked *bool
}

var (
	_ bots.EnvSeat              = guardSpy{}
	_ heuristic.BoardSeat       = guardSpy{}
	_ searchseat.ClosedListSeat = guardSpy{}
)

func (s guardSpy) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	*s.asked = true
	return s.inner.Decide(ctx, v, d)
}

func (s guardSpy) DecideBoard(ctx context.Context, b botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	*s.asked = true
	return s.inner.(heuristic.BoardSeat).DecideBoard(ctx, b, d)
}

func (s guardSpy) WantsEnv(d *decision.Decision) bool { return s.inner.(bots.EnvSeat).WantsEnv(d) }

func (s guardSpy) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	*s.asked = true
	return s.inner.(bots.EnvSeat).DecideEnv(ctx, env, d)
}

func (s guardSpy) SearchBelief() *searchprobe.Belief {
	return s.inner.(searchseat.ClosedListSeat).SearchBelief()
}

// The host does not trust the declaration: an az-c4 (or az-c5) table whose
// bot seat is an open-list search seat (az-c3 without a belief, swapped in
// through Options.Seats) crashes at match start, before the seat is asked
// anything.
func TestClosedListGuardRefusesAnOpenSeatAtAHumanTable(t *testing.T) {
	for _, policy := range []string{azc4.Policy, azc5.Policy} {
		t.Run(policy, func(t *testing.T) {
			enableAZ(t, azc3.Policy)
			enableAZ(t, policy)
			opts := testOptions(t)
			asked := false
			opts.Seats = func(names []string, seed uint64) []seat.Seat {
				open, err := bots.New(azc3.Policy, bots.Options{Seed: seed ^ 1, AutoPayMana: true})
				if err != nil {
					panic(err)
				}
				if cl, ok := open.(searchseat.ClosedListSeat); !ok || cl.SearchBelief() != nil {
					panic("precondition: az-c3 without a belief is not an open-list seat")
				}
				return []seat.Seat{guardSpy{inner: open, asked: &asked}, heuristic.NewBot(seed ^ 2)}
			}
			r, err := host.New(opts)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if err := r.AddTable(host.TableConfig{ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Seed: 1, Spectator: view.Public,
				Humans: []int{1}, BotPolicy: policy, AutoMana: true, BotAutoPayMana: true}); err != nil {
				t.Fatal(err)
			}
			if err := r.Start("t1"); err != nil {
				t.Fatal(err)
			}
			r.Wait("t1")
			ms, err := r.Matches("t1")
			if err != nil || len(ms) != 1 {
				t.Fatalf("matches %v, %v", ms, err)
			}
			snap, err := r.SnapshotMatch("t1", ms[0].Match, nil)
			if err != nil {
				t.Fatal(err)
			}
			if reason := snap.Match.Reason; ms[0].State != protocol.MatchCrashed || !strings.Contains(reason, "opponents' real decklists") {
				t.Fatalf("state %s reason %q: want a crash naming the open-list seat", ms[0].State, reason)
			}
			if asked {
				t.Fatal("the open-list seat was asked a decision before the guard refused it")
			}
		})
	}
}

// A table that names az-c3 without payment plans is refused when it is
// registered. Before the check, the factory's refusal surfaced as a panic at
// the match's first seat and crashed the match.
func TestHostRefusesAZC3WithoutAutoPay(t *testing.T) {
	enableAZ(t, azc3.Policy)
	for _, c := range []struct {
		name             string
		autoMana, botPay bool
	}{{"neither", false, false}, {"auto_mana only", true, false}, {"bot_auto_pay_mana only", false, true}} {
		r, err := host.New(testOptions(t))
		if err != nil {
			t.Fatal(err)
		}
		cfg := host.TableConfig{ID: "t1", Name: "refused", Seats: 2, Decks: []string{"a", "b"}, Seed: 1,
			Spectator: view.Public, BotPolicy: azc3.Policy, AutoMana: c.autoMana, BotAutoPayMana: c.botPay, AllowDecklistBots: true}
		if err := r.AddTable(cfg); err == nil {
			t.Errorf("%s: AddTable accepted an az-c3 table without automatic mana payment", c.name)
		}
		r.Close()
	}
}
