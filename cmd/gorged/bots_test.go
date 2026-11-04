package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azc4"
	"github.com/adams-shaun/gorge/bots/azc5"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/server/gorged"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// linkedPolicies are exactly the always-registered entries this binary links:
// bots/all's built-ins and the experimental search and SpellBench policies.
// az-c3, az-c4 and az-c5 register only when a -<policy>-net flag (or a test)
// enables them, so the registry check below leaves them out.
var linkedPolicies = []string{
	"bot", "lethal-pressure", "az-redeal", "sb-tactical", "forge-heuristic", "forge-heuristic-hold",
	"cast-profile", "search", "sb-search-lite-atk", "sb-heuristic", "sb-uniform", "sb-first",
}

func TestLinkedPoliciesRegister(t *testing.T) {
	for _, name := range linkedPolicies {
		got, err := bots.Normalize(name)
		if err != nil || got != name {
			t.Errorf("Normalize(%q) = %q, %v; want %q", name, got, err, name)
		}
	}
	var names []string
	for _, e := range bots.Entries() {
		if !strings.HasPrefix(e.Name, "az-c") {
			names = append(names, e.Name)
		}
	}
	want := slices.Clone(linkedPolicies)
	slices.Sort(want)
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("Entries() = %q, want %q", names, want)
	}
}

// TestSearchEntryShapes pins the registry fields the host and the UI read for
// the two search policies, and that each experimental entry this binary adds
// says what it is and what it costs.
func TestSearchEntryShapes(t *testing.T) {
	search, ok := bots.Lookup("search")
	if !ok {
		t.Fatal("no search entry")
	}
	if search.Tier != bots.Experimental || !search.Env || !search.Search {
		t.Errorf("search entry is not the experimental Env policy: %+v", search.Info)
	}
	sbs, ok := bots.Lookup("sb-search-lite-atk")
	if !ok {
		t.Fatal("no sb-search-lite-atk entry")
	}
	if sbs.Tier != bots.Experimental || !sbs.Env || !sbs.Search || sbs.MaxSeats != 2 {
		t.Errorf("sb-search-lite-atk entry is not the experimental Env+search 2-seat policy: %+v", sbs.Info)
	}
	for _, name := range []string{"cast-profile", "search", "sb-search-lite-atk", "sb-heuristic", "sb-uniform", "sb-first"} {
		e, _ := bots.Lookup(name)
		if e.Label == "" || e.Description == "" || e.Cost.Note == "" || len(e.Formats) == 0 {
			t.Errorf("entry %q has incomplete Info: %+v", name, e.Info)
		}
	}
}

// netBotChildEnv marks the child process TestRealNetBotsEnable runs its
// checks in. Enabling registers a policy for the life of the process, and the
// parent's other tests (the flags reference among them) must see the registry a
// server sees before any -<policy>-net flag is read.
const netBotChildEnv = "GORGED_TEST_NETBOT_CHILD"

// TestRealNetBotsEnable is the real-network half of server/gorged's stub
// net-bot tests, run in a child process: the closed-list az-c4 refuses to
// enable without a belief, and the NetBot.Enable functions this binary
// registers load a .dzgw network and register az-c3, az-c4 and az-c5, exactly
// as gorged's flag checks assume.
func TestRealNetBotsEnable(t *testing.T) {
	if os.Getenv(netBotChildEnv) == "" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run", "^TestRealNetBotsEnable$", "-test.count", "1")
		cmd.Env = append(os.Environ(), netBotChildEnv+"=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("child: %v\n%s", err, out)
		}
		return
	}
	path, err := gonettest.Write(t.TempDir(), "net.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	if err := azc4.Enable(path, nil); err == nil {
		t.Fatal("az-c4 enabled without a closed-list belief")
	}
	_, decks := testutil.SampleDecks(t, 2)
	belief, err := searchprobe.NewBelief(decks)
	if err != nil {
		t.Fatal(err)
	}
	fs := gorged.ServeFlagSet()
	for _, nb := range []struct {
		policy string
		belief *searchprobe.Belief
	}{{azc3.Policy, nil}, {azc4.Policy, belief}, {azc5.Policy, belief}} {
		if fs.Lookup(nb.policy+"-net") == nil {
			t.Fatalf("-%s-net is not a flag of this binary", nb.policy)
		}
		if err := netBotEnable(nb.policy)(path, nb.belief); err != nil {
			t.Fatalf("%s: Enable: %v", nb.policy, err)
		}
		if _, ok := bots.Lookup(nb.policy); !ok {
			t.Errorf("%s is not registered after Enable", nb.policy)
		}
	}
}

// netBotEnable is the Enable this binary registers for policy.
func netBotEnable(policy string) func(string, *searchprobe.Belief) error {
	for _, nb := range netBots {
		if nb.policy == policy {
			return nb.bot.Enable
		}
	}
	return func(string, *searchprobe.Belief) error {
		return errors.New(policy + " is not a network policy of this binary")
	}
}

// TestNetBotFlags pins the flags the demo deploy passes (scripts/deploy-demo.sh).
func TestNetBotFlags(t *testing.T) {
	fs := gorged.ServeFlagSet()
	for _, f := range []string{
		"az-c3-net", "az-c3-decks", "allow-decklist-bots",
		"az-c4-net", "az-c4-decks", "az-c4-belief", "az-c4-belief-pool",
		"az-c5-net", "az-c5-decks", "az-c5-belief", "az-c5-belief-pool",
	} {
		if fs.Lookup(f) == nil {
			t.Errorf("-%s is not a flag of this binary", f)
		}
	}
}

// TestServedEventSeatsCastProfile: an event on this binary seats cast-profile,
// which resolves only because this binary links bots/castprofile on top of
// bots/all (the draft tree's own tests seat bots/all's policies only).
func TestServedEventSeatsCastProfile(t *testing.T) {
	base := startGorged(t, t.TempDir(), "-events", "-humans", "1")
	raw, err := json.Marshal(protocol.CreateEventRequest{Set: "FDN", Seats: []protocol.CreateEventSeat{
		{Kind: "bot", Policy: "bot"}, {Kind: "bot", Policy: "lethal-pressure"},
		{Kind: "bot", Policy: "cast-profile"}, {Kind: "bot", Policy: "bot"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(base+"/api/events", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		t.Fatalf("POST /api/events = %d: %s", resp.StatusCode, body)
	}
	var created protocol.CreateEventResponse
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	if len(created.State.Seats) != 4 {
		t.Fatalf("%d seats, want 4: %s", len(created.State.Seats), body)
	}
	if got := created.State.Seats[2].Policy; got != "cast-profile" {
		t.Fatalf("seat 2 policy %q, want cast-profile", got)
	}
}
