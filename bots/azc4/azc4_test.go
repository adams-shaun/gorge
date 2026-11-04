package azc4_test

// The registry is process-wide and Enable registers once, so these tests are
// ordered: the first ones prove nothing is registered before a valid Enable,
// the rest run after it. Go runs a file's tests in source order.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azc4"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func belief(t *testing.T) *searchprobe.Belief {
	t.Helper()
	_, decks := testutil.SampleDecks(t, 2)
	b, err := searchprobe.NewBelief(decks)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNothingIsRegisteredUntilEnabled(t *testing.T) {
	if _, ok := bots.Lookup(azc4.Policy); ok {
		t.Fatal("az-c4 is registered before Enable: linking the package must register nothing")
	}
	if _, err := bots.New(azc4.Policy, bots.Options{AutoPayMana: true}); err == nil {
		t.Error("bots.New built az-c4 before Enable")
	}
}

// az-c4 plays closed lists only: Enable without a belief is refused, and so
// is a network that does not load; neither registers anything.
func TestEnableRefusesNoBeliefAndAnUnusableNetwork(t *testing.T) {
	path, err := gonettest.Write(t.TempDir(), "net.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	if err := azc4.Enable(path, nil); err == nil {
		t.Error("Enable without a belief succeeded")
	}
	if err := azc4.Enable(filepath.Join(t.TempDir(), "absent.dzgw"), belief(t)); err == nil {
		t.Error("Enable over a missing network succeeded")
	}
	if _, ok := bots.Lookup(azc4.Policy); ok {
		t.Error("a refused Enable left az-c4 registered")
	}
}

func TestEnableRegistersAClosedListEntry(t *testing.T) {
	path, err := gonettest.Write(t.TempDir(), "net.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	if err := azc4.Enable(path, belief(t)); err != nil {
		t.Fatal(err)
	}
	e, ok := bots.Lookup(azc4.Policy)
	if !ok {
		t.Fatal("Enable did not register az-c4")
	}
	if e.Tier != bots.Experimental {
		t.Errorf("tier %q, want experimental", e.Tier)
	}
	if !e.Env || !e.Search || !e.ClosedLists {
		t.Errorf("Env=%v Search=%v ClosedLists=%v: want all three", e.Env, e.Search, e.ClosedLists)
	}
	if bots.ReadsDecklists(azc4.Policy) {
		t.Error("az-c4 reads decklists: a closed-list policy must not")
	}
	if !reflect.DeepEqual(e.Formats, []string{"constructed"}) || e.MaxSeats != 2 || e.Caretaker != "bot" || !e.NeedsAutoPay {
		t.Errorf("formats %v, max seats %d, caretaker %q, needs autopay %v: want 1v1 constructed, bot caretaker, autopay", e.Formats, e.MaxSeats, e.Caretaker, e.NeedsAutoPay)
	}
	if e.Cost.MeanMS <= 0 || e.Cost.P95MS < e.Cost.MeanMS {
		t.Errorf("cost %+v: want a measured cost", e.Cost)
	}
	if _, err := os.Stat(filepath.Join("..", "..", e.Cost.Source)); err != nil {
		t.Errorf("cited source %q: %v", e.Cost.Source, err)
	}
	if err := azc4.Enable(path, belief(t)); err == nil {
		t.Error("a second Enable succeeded")
	}
}

// The config is az-c3's measured spec closed under the belief: the keys of
// the C4 loop's az:sims=100:autopay:oppnodes:oppfull:morekinds:closedlist:topk=6.
func TestConfigIsTheClosedMeasuredSpec(t *testing.T) {
	b := belief(t)
	c := azc4.Config(b)
	if c.World != azmcts.WorldRedealClosed || c.Belief != b || c.Source != nil {
		t.Fatalf("world %q belief %v source %v: want the closed honest redeal", c.World, c.Belief == b, c.Source != nil)
	}
	if !reflect.DeepEqual(c, azc3.ClosedConfig(b)) {
		t.Fatal("az-c4's config differs from az-c3's closed config")
	}
}

// Every seat the factory builds is a closed-list search seat with the
// belief: the property the host checks before seating it against a human.
func TestFactoryBuildsAClosedListSeat(t *testing.T) {
	if _, err := bots.New(azc4.Policy, bots.Options{Seed: 1}); err == nil {
		t.Error("built az-c4 without automatic mana payment")
	}
	s, err := bots.New(azc4.Policy, bots.Options{Seed: 1, AutoPayMana: true, DecisionDeadlineMS: 1234})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(bots.EnvSeat); !ok {
		t.Errorf("%T is not a bots.EnvSeat", s)
	}
	if _, ok := s.(heuristic.BoardSeat); !ok {
		t.Errorf("%T is not a heuristic.BoardSeat", s)
	}
	cl, ok := s.(searchseat.ClosedListSeat)
	if !ok || cl.SearchBelief() == nil {
		t.Fatalf("%T is not a closed-list seat with a belief (ok %v)", s, ok)
	}
	if b, ok := s.(bots.BudgetedSeat); !ok || b.DecisionBudgetMS() != 1234 {
		t.Errorf("the decision budget did not reach the seat: %v", ok)
	}
}
