package gorged

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/seat"
)

// gorged's tests exercise the network-policy machinery -- flags, reserved
// pools, create-game refusals, demo tables, beliefs -- over stub net bots that
// register the bots.Info fields gorged reads and play the heuristic bot's seat.

// stubNetMagic opens every stub network file; anything else is refused, as a
// real policy refuses a file that is not its network.
const stubNetMagic = "STUBNET"

// writeStubNet writes a stub network file named name in dir.
func writeStubNet(dir, name string) (string, error) {
	path := filepath.Join(dir, name)
	return path, os.WriteFile(path, []byte(stubNetMagic+"\n"), 0o644)
}

// stubNetBot is one stub policy: its registry Info and the belief its Enable
// was last given.
type stubNetBot struct {
	info    bots.Info
	mu      sync.Mutex
	enabled bool
	belief  *searchprobe.Belief
}

func (s *stubNetBot) enable(path string, closed *searchprobe.Belief) error {
	name := s.info.Name
	if s.info.ClosedLists && closed == nil {
		return fmt.Errorf("%s: no belief: the opponents' lists must be closed", name)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if !bytes.HasPrefix(raw, []byte(stubNetMagic)) {
		return fmt.Errorf("%s: %s is not a network", name, path)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enabled {
		return fmt.Errorf("%s: already enabled", name)
	}
	s.belief = closed
	bots.Register(bots.Entry{Info: s.info, New: s.newSeat})
	s.enabled = true
	return nil
}

// newSeat refuses a table without payment plans and otherwise plays the heuristic bot's seat.
func (s *stubNetBot) newSeat(o bots.Options) (seat.Seat, error) {
	if !o.AutoPayMana {
		return nil, errors.New(s.info.Name + " needs automatic mana payment")
	}
	return bots.New(bots.Default, o)
}

// stubInfo is a bots.Info with the fields gorged reads.
func stubInfo(policy string, closed bool) bots.Info {
	return bots.Info{
		Name: policy, Label: policy + " (test stub)", Description: "stub net bot for gorged's tests",
		Tier: bots.Experimental,
		Env:  true, Search: true, ClosedLists: closed,
		Formats:      []string{"constructed"},
		MaxSeats:     2,
		Caretaker:    "bot",
		NeedsAutoPay: true,
	}
}

// The stub policies: one that reads decklists, two that guess them.
const (
	testOpenPolicy = "net-open"
	testClosedA    = "net-closed-a"
	testClosedB    = "net-closed-b"
)

var stubNetBots = map[string]*stubNetBot{
	testOpenPolicy: {info: stubInfo(testOpenPolicy, false)},
	testClosedA:    {info: stubInfo(testClosedA, true)},
	testClosedB:    {info: stubInfo(testClosedB, true)},
}

func init() {
	RegisterNetBot(testOpenPolicy, NetBot{NetHelp: "its test network", ReadsDecklists: true, Enable: stubNetBots[testOpenPolicy].enable})
	RegisterNetBot(testClosedA, NetBot{NetHelp: "its test network", Enable: stubNetBots[testClosedA].enable})
	RegisterNetBot(testClosedB, NetBot{NetHelp: "its test network", Enable: stubNetBots[testClosedB].enable})
}

// stubPolicy returns policy's netPolicy with the flag values of a.
func stubPolicy(policy string, a netPolicy) netPolicy {
	a.policy = policy
	for _, r := range registeredNetBots() {
		if r.policy == policy {
			a.bot = r.bot
			return a
		}
	}
	panic("gorged: no stub net bot " + policy)
}
