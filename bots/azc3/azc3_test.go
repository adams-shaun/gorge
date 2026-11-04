package azc3_test

// The registry is process-wide and Enable registers once, so these tests are
// ordered: the first proves nothing is registered before Enable, the rest run
// after it. Go runs a file's tests in source order.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestNothingIsRegisteredUntilEnabled(t *testing.T) {
	if _, ok := bots.Lookup(azc3.Policy); ok {
		t.Fatal("az-c3 is registered before Enable: linking the package must register nothing")
	}
	if _, err := bots.New(azc3.Policy, bots.Options{AutoPayMana: true}); err == nil {
		t.Error("bots.New built az-c3 before Enable")
	}
}

func TestEnableRefusesAnUnusableNetworkAndRegistersNothing(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.dzgw")
	if err := os.WriteFile(bad, []byte(strings.Repeat("x", 100)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(dir, "absent.dzgw"), bad} {
		if err := azc3.Enable(p); err == nil {
			t.Errorf("Enable(%s) succeeded", p)
		}
	}
	if _, ok := bots.Lookup(azc3.Policy); ok {
		t.Error("a failed Enable left az-c3 registered")
	}
}

func TestEnableRegistersAnExperimentalSearchEntry(t *testing.T) {
	path, err := gonettest.Write(t.TempDir(), "net.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	if err := azc3.Enable(path); err != nil {
		t.Fatal(err)
	}
	e, ok := bots.Lookup(azc3.Policy)
	if !ok {
		t.Fatal("Enable did not register az-c3")
	}
	if e.Tier != bots.Experimental {
		t.Errorf("tier %q, want experimental: az-c3 has not passed the promotion gate", e.Tier)
	}
	if !e.Env || !e.Search {
		t.Errorf("Env=%v Search=%v: the policy reads the honest root, so both must be set", e.Env, e.Search)
	}
	if !reflect.DeepEqual(e.Formats, []string{"constructed"}) || e.MaxSeats != 2 || e.Caretaker != "bot" {
		t.Errorf("formats %v, max seats %d, caretaker %q: want 1v1 constructed with a bot caretaker", e.Formats, e.MaxSeats, e.Caretaker)
	}
	if e.Cost.MeanMS <= 0 {
		t.Errorf("no cost: %+v", e.Info)
	}
	// The cost names a source that exists in the repo.
	if _, err := os.Stat(filepath.Join("..", "..", e.Cost.Source)); err != nil {
		t.Errorf("cited source %q: %v", e.Cost.Source, err)
	}
	if err := azc3.Enable(path); err == nil {
		t.Error("a second Enable succeeded")
	}
}

// The config is the one the c3 net was trained and measured with, and an
// honest one: the redeal world with no clairvoyant source.
func TestConfigMatchesTheMeasuredSpec(t *testing.T) {
	c := azc3.Config()
	if c.World != azmcts.WorldRedeal || c.Source != nil || c.Worlds != 0 {
		t.Errorf("world %q source set %v worlds %d: want the honest redeal world, a fresh deal per simulation", c.World, c.Source != nil, c.Worlds)
	}
	if c.PriorOnly || c.Explore {
		t.Errorf("PriorOnly=%v Explore=%v: a hosted seat searches and never explores", c.PriorOnly, c.Explore)
	}
	s := c.Search
	if s.Sims != 100 || s.PriorTopK != 6 || !s.AutoPayment || !s.OpponentNodes || !s.OpponentMirror {
		t.Errorf("search %+v: want sims=100 topk=6 autopay oppnodes oppfull", s)
	}
	if s.Kinds != azmcts.FullKinds() {
		t.Errorf("kinds %+v, want azmcts.FullKinds (:morekinds)", s.Kinds)
	}
	if def := azmcts.DefaultOptions(); s.CPUCT != def.CPUCT || s.FPU != def.FPU || s.Limit != def.Limit || s.MaxSteps != def.MaxSteps {
		t.Errorf("cpuct/fpu/limit/maxsteps %v/%v/%v/%v differ from the defaults the spec leaves alone", s.CPUCT, s.FPU, s.Limit, s.MaxSteps)
	}
	if c.Mulligan != botpolicy.MulliganLands {
		t.Errorf("mulligan rule %v, want the land-count London rule (-mull-heuristic)", c.Mulligan)
	}
}

func TestFactoryNeedsAutoPayAndBuildsAnEnvSeat(t *testing.T) {
	if _, err := bots.New(azc3.Policy, bots.Options{Seed: 1}); err == nil {
		t.Error("built az-c3 without automatic mana payment")
	}
	s, err := bots.New(azc3.Policy, bots.Options{Seed: 1, AutoPayMana: true, DecisionDeadlineMS: 1234})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(bots.EnvSeat); !ok {
		t.Errorf("%T is not a bots.EnvSeat", s)
	}
	if _, ok := s.(heuristic.BoardSeat); !ok {
		t.Errorf("%T is not a heuristic.BoardSeat", s)
	}
	b, ok := s.(bots.BudgetedSeat)
	if !ok || b.DecisionBudgetMS() != 1234 {
		t.Errorf("the decision budget did not reach the seat: %v", ok)
	}
}

// The host builds an honest root only for decisions WantsEnv accepts, so the
// seat must accept every kind its config searches, :morekinds included, and
// refuse the rest.
func TestWantsEnvCoversEverySearchedKind(t *testing.T) {
	s, err := bots.New(azc3.Policy, bots.Options{Seed: 1, AutoPayMana: true})
	if err != nil {
		t.Fatal(err)
	}
	env := s.(bots.EnvSeat)
	opts := func(kinds ...string) []decision.Option {
		var out []decision.Option
		for i, k := range kinds {
			out = append(out, decision.Option{Index: i, Kind: k})
		}
		return out
	}
	cases := []struct {
		name string
		d    decision.Decision
		want bool
	}{
		{"priority", decision.Decision{Kind: decision.KPriority, Options: opts("pass", "cast")}, true},
		{"attackers", decision.Decision{Kind: decision.KAttackers, Options: opts("attacker")}, true},
		{"blockers", decision.Decision{Kind: decision.KBlockers, Options: opts("block")}, true},
		{"single target", decision.Decision{Kind: decision.KTarget, Min: 1, Max: 1, Options: opts("permanent", "player")}, true},
		{"multi target", decision.Decision{Kind: decision.KTarget, Min: 1, Max: 2, Options: opts("permanent", "permanent", "player")}, true},
		{"modes", decision.Decision{Kind: decision.KModes, Min: 1, Max: 1, Options: opts("mode", "mode")}, true},
		{"optional trigger", decision.Decision{Kind: decision.KTriggerOptional, Options: opts("yes", "no")}, true},
		{"small choice", decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Options: opts("number", "number", "number")}, true},
		{"mana window", decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Options: opts("activate", "done")}, false},
		{"large choice", decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Options: opts("a", "b", "c", "d", "e", "f", "g", "h")}, false},
		{"mulligan", decision.Decision{Kind: decision.KMulligan, Options: opts("keep", "mulligan")}, false},
		{"trigger order", decision.Decision{Kind: decision.KTriggerOrder, Options: opts("trigger", "trigger")}, false},
	}
	for _, c := range cases {
		c := c
		if got := env.WantsEnv(&c.d); got != c.want {
			t.Errorf("WantsEnv(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

// ClosedConfig is Config plus the closed world under a belief, and Config
// itself (the open lists) without one.
func TestClosedConfigClosesTheOpponentsLists(t *testing.T) {
	if !reflect.DeepEqual(azc3.ClosedConfig(nil), azc3.Config()) {
		t.Fatal("ClosedConfig(nil) differs from Config")
	}
	_, decks := testutil.SampleDecks(t, 2)
	b, err := searchprobe.NewBelief(decks)
	if err != nil {
		t.Fatal(err)
	}
	c := azc3.ClosedConfig(b)
	if c.World != azmcts.WorldRedealClosed || c.Belief != b {
		t.Fatalf("closed config: world %q, belief %v", c.World, c.Belief != nil)
	}
	c.World, c.Belief = azc3.Config().World, nil
	if !reflect.DeepEqual(c, azc3.Config()) {
		t.Fatal("ClosedConfig changed more than the world and the belief")
	}
}
