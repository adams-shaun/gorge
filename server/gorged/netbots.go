package gorged

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/adams-shaun/gorge/bots/searchprobe"
)

// The hosted net policies gorged serves. Their names, flags, deck pools and
// demo tables are gorged's (azc3.go, azclosed.go); their implementation is
// linked in by the binary through RegisterNetBot.
const (
	policyAZC3 = "az-c3"
	policyAZC4 = "az-c4"
	policyAZC5 = "az-c5"
)

// NetBot is a hosted policy whose seat loads a trained network at startup
// (az-c3/az-c4/az-c5, linked in by a binary that carries them). gorged owns the
// policy names, their flags and the closed deck-list machinery; the binary
// supplies the implementation with RegisterNetBot before Main.
type NetBot struct {
	// Enable loads the network at path and registers the policy in the bots
	// registry. closed is the opponents' deck-list belief for a closed-list
	// policy (az-c4, az-c5) and nil for an open one (az-c3).
	Enable func(path string, closed *searchprobe.Belief) error
	// SetBelief, when non-nil, closes an open policy's opponent lists after
	// Enable (az-c3's -az-c3-belief).
	SetBelief func(*searchprobe.Belief)
}

var (
	netBotsMu sync.Mutex
	netBots   = map[string]NetBot{}
	// extraClosed is every closed-list policy RegisterClosedNetBot added, in
	// registration order.
	extraClosed []extraClosedNetBot
)

// extraClosedNetBot is one RegisterClosedNetBot registration.
type extraClosedNetBot struct{ policy, netHelp string }

// policyNameRE is what a RegisterClosedNetBot policy name may be: it becomes
// a registry key and the prefix of five flags.
var policyNameRE = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// RegisterNetBot links policy's implementation into this binary. Call it
// before Main (from an init or main). policy must be one gorged serves
// (az-c3, az-c4, az-c5) and nb.Enable non-nil; registering a policy twice
// panics.
func RegisterNetBot(policy string, nb NetBot) {
	switch policy {
	case policyAZC3, policyAZC4, policyAZC5:
	default:
		panic(fmt.Sprintf("gorged: RegisterNetBot: %q is not a net bot policy gorged serves (%s, %s, %s)", policy, policyAZC3, policyAZC4, policyAZC5))
	}
	if nb.Enable == nil {
		panic("gorged: RegisterNetBot: " + policy + " has no Enable")
	}
	netBotsMu.Lock()
	defer netBotsMu.Unlock()
	if _, dup := netBots[policy]; dup {
		panic("gorged: RegisterNetBot: " + policy + " registered twice")
	}
	netBots[policy] = nb
}

// RegisterClosedNetBot links a closed-list net policy that gorged does not
// name itself. gorged serves it exactly as it serves az-c4 and az-c5 (the
// closed-list machinery in azclosed.go): its own -<policy>-net, -decks,
// -belief, -belief-pool and -demo-table flags, a reserved deck pool, closed
// opponent lists, offered against humans; ordered after the named policies,
// in registration order. enable loads the network and registers the policy
// in the bots registry with its opponents' lists closed under the belief it
// is given; the registered bots.Info must set ClosedLists, or the host
// refuses to seat it against a human. netHelp describes the network in
// -<policy>-net's help ("its .dzgw network").
//
// Call it before Main, Serve or ServeFlagSet (from an init or main): the
// flags are built from the registrations. policy must be a lower-case,
// dash-separated name that is neither a policy gorged names nor one already
// registered; a violation panics.
func RegisterClosedNetBot(policy, netHelp string, enable func(path string, closed *searchprobe.Belief) error) {
	switch {
	case policy == policyAZC3 || policy == policyAZC4 || policy == policyAZC5:
		panic("gorged: RegisterClosedNetBot: " + policy + " is a policy gorged names; use RegisterNetBot")
	case !policyNameRE.MatchString(policy):
		panic(fmt.Sprintf("gorged: RegisterClosedNetBot: %q is not a lower-case dash-separated policy name", policy))
	case enable == nil:
		panic("gorged: RegisterClosedNetBot: " + policy + " has no enable")
	}
	netBotsMu.Lock()
	defer netBotsMu.Unlock()
	if _, dup := netBots[policy]; dup {
		panic("gorged: RegisterClosedNetBot: " + policy + " registered twice")
	}
	netBots[policy] = NetBot{Enable: enable}
	extraClosed = append(extraClosed, extraClosedNetBot{policy: policy, netHelp: netHelp})
}

// extraClosedNetBots returns RegisterClosedNetBot's registrations in order.
func extraClosedNetBots() []extraClosedNetBot {
	netBotsMu.Lock()
	defer netBotsMu.Unlock()
	return append([]extraClosedNetBot(nil), extraClosed...)
}

// netBot returns policy's linked implementation.
func netBot(policy string) (NetBot, bool) {
	netBotsMu.Lock()
	defer netBotsMu.Unlock()
	nb, ok := netBots[policy]
	return nb, ok
}

// notLinked is the startup refusal for a net flag naming a policy this
// binary does not carry.
func notLinked(flag, policy string) error {
	return fmt.Errorf("%s: %s is not linked into this binary (a net policy's implementation is registered with gorged.RegisterNetBot)", flag, policy)
}
