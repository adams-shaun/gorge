package host

// Closed-list hosted policies through the host. gorge runs the
// contract and gate rows on closedTestPolicy, a test-only registration of az-redeal's search closed
// under a belief (azmcts.WorldRedealClosed) at a small simulation budget, so
// the exported harness (RunClosedPrivacyContract, contract_privacy.go) and
// the host's closed-list gate and guard stay exercised in this module.
//
//   - TestClosedTestPolicyNeverSeesTheHumansHiddenCards: the privacy proof.
//   - TestDecklistGateAtHumanTables: open-list Env policies are refused at a
//     human table; a closed-list policy is not, and needs no opt-in.
//   - TestClosedListGuardRefusesAnOpenSeatAtAHumanTable: the host checks the
//     seats it built, not the declaration.
//   - TestClosedSetupCutsAnOpenSetupAtAHumanTable: envData's second line.

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// closedTestPolicy is the test-only closed-list policy. It is registered at
// init (so no test races the registry) and builds seats only once
// enableClosedTest has set its belief.
const closedTestPolicy = "test-closed-az"

// closedTestBeliefDecks is the belief corpus: the five Foundations repo
// decks (az-c4's test belief).
var closedTestBeliefDecks = []string{
	"foundations-calling-all-angels", "foundations-keen-engineering", "foundations-reign-of-dragons",
	"foundations-tramplesaurus-rex", "foundations-wretched-ranks",
}

var (
	closedTestBelief atomic.Pointer[searchprobe.Belief]
	closedTestMu     sync.Mutex
)

func init() {
	bots.Register(bots.Entry{Info: bots.Info{
		Name: closedTestPolicy, Label: "closed-list az (test)", Tier: bots.Experimental,
		Description: "test-only: az-redeal's search with closed opponent lists",
		Env:         true, Search: true, ClosedLists: true,
		Formats: []string{"constructed"}, MaxSeats: 2, Caretaker: "bot",
	}, New: func(o bots.Options) (seat.Seat, error) {
		b := closedTestBelief.Load()
		if b == nil {
			return nil, errors.New("test-closed-az: no belief (enableClosedTest)")
		}
		cfg := azredeal.Hosted()
		cfg.World, cfg.Belief = azmcts.WorldRedealClosed, b
		cfg.Search.Sims = 8 // the privacy of the inputs does not depend on the budget
		return azredeal.New(o, cfg)
	}})
}

// enableClosedTest builds the test belief from the corpus once per binary
// and returns it.
func enableClosedTest(t *testing.T) *searchprobe.Belief {
	t.Helper()
	closedTestMu.Lock()
	defer closedTestMu.Unlock()
	if b := closedTestBelief.Load(); b != nil {
		return b
	}
	reg := testutil.CorpusRegistry(t)
	var decks [][]*cards.Card
	for _, n := range closedTestBeliefDecks {
		decks = append(decks, testutil.RepoDeck(t, reg, n))
	}
	b, err := searchprobe.NewBelief(decks)
	if err != nil {
		t.Fatal(err)
	}
	closedTestBelief.Store(b)
	return b
}

// TestClosedTestPolicyNeverSeesTheHumansHiddenCards runs the closed-list
// privacy proof on gorge's own closed-list seat: one hosted game against a
// human whose deck (mono-red goblins) and sideboard (mono-black aggro) hold
// cards that appear nowhere in the belief corpus or the bot's own list.
func TestClosedTestPolicyNeverSeesTheHumansHiddenCards(t *testing.T) {
	belief := enableClosedTest(t)
	reg := testutil.CorpusRegistry(t)
	RunClosedPrivacyContract(t, ClosedPrivacyContract{
		Options:        testOptions(t),
		Policy:         closedTestPolicy,
		Belief:         belief,
		BotDeck:        testutil.RepoDeck(t, reg, "mono-white-equipment"),
		HumanDeck:      testutil.RepoDeck(t, reg, "mono-red-goblins"),
		HumanSideboard: testutil.RepoDeck(t, reg, "mono-black-aggro"),
		Seeds:          []uint64{20261014},
	})
}

// Open-list Env policies -- az-redeal and sb-tactical -- are refused at a
// table with a human, with or without the opt-in; a closed-list policy is
// accepted there without it, and on a bot-only table without it too.
func TestDecklistGateAtHumanTables(t *testing.T) {
	enableClosedTest(t)
	table := func(policy string, humans []int, optIn bool) TableConfig {
		return TableConfig{ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Seed: 1, Spectator: view.Public,
			Humans: humans, BotPolicy: policy, AutoMana: true, BotAutoPayMana: true, AllowDecklistBots: optIn}
	}
	for _, policy := range []string{azRedealPolicy, "sb-tactical"} {
		if !bots.ReadsDecklists(policy) {
			t.Fatalf("precondition: %q does not read decklists", policy)
		}
		for _, optIn := range []bool{false, true} {
			r, err := New(testOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			err = r.AddTable(table(policy, []int{0}, optIn))
			if err == nil || !strings.Contains(err.Error(), "it cannot seat at a table with a human") {
				t.Errorf("%s (opt-in %v) at a human table: error %v, want the decklist refusal", policy, optIn, err)
			}
			r.Close()
		}
	}
	if bots.ReadsDecklists(closedTestPolicy) {
		t.Fatalf("%s reads decklists", closedTestPolicy)
	}
	for _, humans := range [][]int{{0}, nil} {
		r, err := New(testOptions(t))
		if err != nil {
			t.Fatal(err)
		}
		if err := r.AddTable(table(closedTestPolicy, humans, false)); err != nil {
			t.Errorf("%s (humans %v) without the opt-in: %v", closedTestPolicy, humans, err)
		}
		r.Close()
	}
}

// The host does not trust the declaration: a closed-list table whose bot
// seat is an open-list search seat (az-redeal, swapped in through
// Options.Seats) crashes at match start, before the seat is asked anything.
func TestClosedListGuardRefusesAnOpenSeatAtAHumanTable(t *testing.T) {
	enableClosedTest(t)
	opts := testOptions(t)
	asked := false
	opts.Seats = func(names []string, seed uint64) []seat.Seat {
		open, err := bots.New(azRedealPolicy, bots.Options{Seed: seed ^ 1, AutoPayMana: true})
		if err != nil {
			panic(err)
		}
		if cl, ok := open.(searchseat.ClosedListSeat); !ok || cl.SearchBelief() != nil {
			panic("precondition: az-redeal is not an open-list seat")
		}
		return []seat.Seat{&privacySpy{inner: open, check: func(string, *decision.Decision, *bots.Env, *botpolicy.Board, *view.View) { asked = true }}, heuristic.NewBot(seed ^ 2)}
	}
	r, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(TableConfig{ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Seed: 1, Spectator: view.Public,
		Humans: []int{1}, BotPolicy: closedTestPolicy, AutoMana: true, BotAutoPayMana: true}); err != nil {
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
	r.mu.RLock()
	tbl := r.tables["t1"]
	r.mu.RUnlock()
	tbl.mu.RLock()
	m := tbl.history[len(tbl.history)-1]
	tbl.mu.RUnlock()
	m.mu.RLock()
	reason := m.reason
	m.mu.RUnlock()
	if ms[0].State != protocol.MatchCrashed || !strings.Contains(reason, "opponents' real decklists") {
		t.Fatalf("state %s reason %q: want a crash naming the open-list seat", ms[0].State, reason)
	}
	if asked {
		t.Fatal("the open-list seat was asked a decision before the guard refused it")
	}
}

// envData's second line: at a table with a human, an open Setup is cut to
// the actor's own list and gets no root; a closed Setup, and any Setup at a
// bot-only table, pass unchanged.
func TestClosedSetupCutsAnOpenSetupAtAHumanTable(t *testing.T) {
	_, decks := testutil.SampleDecks(t, 4)
	b, err := searchprobe.NewBelief(decks[2:])
	if err != nil {
		t.Fatal(err)
	}
	open := searchprobe.PublicGame{Names: []string{"x", "y"}, Decks: decks[:2]}
	human := TableConfig{Humans: []int{1}}
	got, ok := closedSetup(human, open, 0)
	if ok || got.Decks[1] != nil || len(got.Decks[0]) != len(decks[0]) {
		t.Fatalf("open setup at a human table: ok %v, decks %d/%d", ok, len(got.Decks[0]), len(got.Decks[1]))
	}
	if open.Decks[1] == nil {
		t.Fatal("closedSetup wrote through to the caller's decks")
	}
	closed := open.Closed(0, b)
	if got, ok := closedSetup(human, closed, 0); !ok || got.Belief != b {
		t.Fatal("a closed setup was refused")
	}
	if got, ok := closedSetup(TableConfig{}, open, 0); !ok || got.Decks[1] == nil {
		t.Fatal("a bot-only table's setup was changed")
	}
}
