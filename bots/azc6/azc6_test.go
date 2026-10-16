package azc6_test

// The registry is process-wide and Enable registers once, so these tests are
// ordered: the first ones prove nothing is registered before a valid Enable,
// the rest run after it. Go runs a file's tests in source order.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azc4"
	"github.com/adams-shaun/gorge/bots/azc5"
	"github.com/adams-shaun/gorge/bots/azc6"
	"github.com/adams-shaun/gorge/internal/azmcts"
	"github.com/adams-shaun/gorge/internal/gonet/gonettest"
	"github.com/adams-shaun/gorge/internal/searchprobe"
	"github.com/adams-shaun/gorge/internal/searchseat"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/seat"
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
	if _, ok := bots.Lookup(azc6.Policy); ok {
		t.Fatal("az-c6 is registered before Enable: linking the package must register nothing")
	}
	if _, err := bots.New(azc6.Policy, bots.Options{AutoPayMana: true}); err == nil {
		t.Error("bots.New built az-c6 before Enable")
	}
}

// az-c6 plays closed lists only: Enable without a belief is refused, and so
// is a network that does not load; neither registers anything.
func TestEnableRefusesNoBeliefAndAnUnusableNetwork(t *testing.T) {
	path, err := gonettest.Write(t.TempDir(), "net.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	if err := azc6.Enable(path, nil); err == nil {
		t.Error("Enable without a belief succeeded")
	}
	if err := azc6.Enable(filepath.Join(t.TempDir(), "absent.dzgw"), belief(t)); err == nil {
		t.Error("Enable over a missing network succeeded")
	}
	if _, ok := bots.Lookup(azc6.Policy); ok {
		t.Error("a refused Enable left az-c6 registered")
	}
}

func TestEnableRegistersAClosedListEntry(t *testing.T) {
	path, err := gonettest.Write(t.TempDir(), "net.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	if err := azc6.Enable(path, belief(t)); err != nil {
		t.Fatal(err)
	}
	e, ok := bots.Lookup(azc6.Policy)
	if !ok {
		t.Fatal("Enable did not register az-c6")
	}
	if e.Tier != bots.Experimental {
		t.Errorf("tier %q, want experimental", e.Tier)
	}
	if !e.Env || !e.Search || !e.ClosedLists {
		t.Errorf("Env=%v Search=%v ClosedLists=%v: want all three", e.Env, e.Search, e.ClosedLists)
	}
	if bots.ReadsDecklists(azc6.Policy) {
		t.Error("az-c6 reads decklists: a closed-list policy must not")
	}
	if !reflect.DeepEqual(e.Formats, []string{"constructed"}) || e.MaxSeats != 2 || e.Caretaker != "bot" || !e.NeedsAutoPay {
		t.Errorf("formats %v, max seats %d, caretaker %q, needs autopay %v: want 1v1 constructed, bot caretaker, autopay", e.Formats, e.MaxSeats, e.Caretaker, e.NeedsAutoPay)
	}
	// Nothing about C6 is measured yet: the entry must say so, not carry numbers.
	if len(e.Strength) != 1 || !strings.HasPrefix(e.Strength[0].Claim, "not yet measured") || e.Cost.MeanMS != 0 || e.Cost.P95MS != 0 {
		t.Errorf("strength %+v, cost %+v: want one honest not-yet-measured claim and no cost numbers", e.Strength, e.Cost)
	}
	sources := []string{e.Cost.Source}
	for _, m := range e.Strength {
		sources = append(sources, m.Source)
	}
	for _, s := range sources {
		if _, err := os.Stat(filepath.Join("..", "..", s)); err != nil {
			t.Errorf("cited source %q: %v", s, err)
		}
	}
	if !strings.Contains(e.Description, "data loss") {
		t.Errorf("description %q does not say what the policy is", e.Description)
	}
	if err := azc6.Enable(path, belief(t)); err == nil {
		t.Error("a second Enable succeeded")
	}
}

// The config is az-c3's measured spec closed under the belief: the keys of
// the C5 loop's (az-c4's) az:sims=100:autopay:oppnodes:oppfull:morekinds:closedlist:topk=6.
func TestConfigIsTheClosedMeasuredSpec(t *testing.T) {
	b := belief(t)
	c := azc6.Config(b)
	if c.World != azmcts.WorldRedealClosed || c.Belief != b || c.Source != nil {
		t.Fatalf("world %q belief %v source %v: want the closed honest redeal", c.World, c.Belief == b, c.Source != nil)
	}
	if !reflect.DeepEqual(c, azc3.ClosedConfig(b)) || !reflect.DeepEqual(c, azc4.Config(b)) || !reflect.DeepEqual(c, azc5.Config(b)) {
		t.Fatal("az-c6's config differs from az-c3's closed config, az-c4's or az-c5's")
	}
}

// Every seat the factory builds is a closed-list search seat with the
// belief: the property the host checks before seating it against a human.
func TestFactoryBuildsAClosedListSeat(t *testing.T) {
	if _, err := bots.New(azc6.Policy, bots.Options{Seed: 1}); err == nil {
		t.Error("built az-c6 without automatic mana payment")
	}
	s, err := bots.New(azc6.Policy, bots.Options{Seed: 1, AutoPayMana: true, DecisionDeadlineMS: 1234})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(bots.EnvSeat); !ok {
		t.Errorf("%T is not a bots.EnvSeat", s)
	}
	if _, ok := s.(seat.BoardSeat); !ok {
		t.Errorf("%T is not a seat.BoardSeat", s)
	}
	cl, ok := s.(searchseat.ClosedListSeat)
	if !ok || cl.SearchBelief() == nil {
		t.Fatalf("%T is not a closed-list seat with a belief (ok %v)", s, ok)
	}
	if b, ok := s.(bots.BudgetedSeat); !ok || b.DecisionBudgetMS() != 1234 {
		t.Errorf("the decision budget did not reach the seat: %v", ok)
	}
}
