package gorged

// Operator ruling 2026-10-13: a bot policy that reads every seat's real
// decklist (bots.Info.Env) is never offered on gorged's public
// /api/bot-policies list and is never the vs-bot default, because every
// request the offered set feeds always seats a human. The server's own
// bot-only tables may run one behind the explicit -allow-decklist-bots opt-in;
// it cannot be served without it and is unreachable from the vs-bot flow either way.

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
)

// formatSpyPolicy is a test-only NON-decklist constructed-only entry, so
// botpolicies_test's "default lacks a format" refusal stays testable: the
// ruling of 2026-10-13 refuses a decklist-reading default before the format
// check runs, and every real non-Env entry plays both formats.
const formatSpyPolicy = "dlb-format-spy"

var formatSpyOnce sync.Once

func registerFormatSpy(t *testing.T) {
	t.Helper()
	formatSpyOnce.Do(func() {
		bots.Register(bots.Entry{Info: bots.Info{
			Name: formatSpyPolicy, Label: "test-only constructed-only", Tier: bots.Experimental,
			Description: "test-only entry for the default-format refusal",
			Formats:     []string{"constructed"}, MaxSeats: 2,
		}, New: func(o bots.Options) (seat.Seat, error) { return heuristic.NewBot(o.Seed), nil }})
	})
	e, ok := bots.Lookup(formatSpyPolicy)
	if !ok {
		t.Fatal("the format-spy entry was not registered")
	}
	// Precondition: it is NOT a decklist reader (or the Env refusal would
	// fire first and the format case would be vacuous) and it lacks
	// commander.
	if bots.ReadsDecklists(formatSpyPolicy) {
		t.Fatal("the format-spy entry unexpectedly reads decklists")
	}
	if contains(e.Formats, host.FormatCommander.String()) {
		t.Fatalf("the format-spy entry unexpectedly supports commander (formats %v)", e.Formats)
	}
}

// nonDecklistPolicyNames is the offered set an empty -bot-policies resolves
// to: every registered entry that does not read opponents' decklists, in
// registry order.
func nonDecklistPolicyNames() []string {
	var out []string
	for _, name := range bots.Names() {
		if !bots.ReadsDecklists(name) {
			out = append(out, name)
		}
	}
	return out
}

func TestBotPoliciesDefaultExcludesDecklistBots(t *testing.T) {
	offered, err := botPolicyOffered("")
	if err != nil {
		t.Fatalf("botPolicyOffered(\"\"): %v", err)
	}
	// Precondition: the set is non-empty and carries the server's real
	// defaults, so "excludes" is not trivially true of an empty list.
	for _, want := range []string{bots.Default, host.LethalPressurePolicy} {
		if !containsString(offered, want) {
			t.Fatalf("offered set %v lost the default %q", offered, want)
		}
	}
	var excluded []string
	for _, name := range bots.Names() {
		if !bots.ReadsDecklists(name) {
			if !containsString(offered, name) {
				t.Errorf("non-decklist policy %q missing from the offered set", name)
			}
			continue
		}
		excluded = append(excluded, name)
		if containsString(offered, name) {
			t.Errorf("decklist-reading policy %q is offered on a server where vs-bot seats a human", name)
		}
	}
	if len(excluded) == 0 {
		t.Fatal("no registered policy reads decklists; the test's exclusion is vacuous")
	}
	// The wire listing gorged arms at startup shows the same set.
	list := buildBotPolicyList(bots.Default, offered)
	for _, p := range list.Policies {
		if bots.ReadsDecklists(p.Name) {
			t.Errorf("/api/bot-policies lists decklist-reading policy %q", p.Name)
		}
	}
}

func TestBotPoliciesNamingDecklistBotIsAnError(t *testing.T) {
	var envNames []string
	for _, name := range bots.Names() {
		if bots.ReadsDecklists(name) {
			envNames = append(envNames, name)
		}
	}
	if len(envNames) == 0 {
		t.Fatal("no registered policy reads decklists; the test would be vacuous")
	}
	for _, name := range envNames {
		_, err := botPolicyOffered("bot," + name)
		if err == nil {
			t.Errorf("-bot-policies accepted decklist-reading policy %q", name)
		} else if !strings.Contains(err.Error(), "reads opponents' decklists") {
			t.Errorf("%q: error %v, want one naming the reason", name, err)
		}
		// A decklist-reading default is refused the same way: it is what an
		// omitted-field vs-bot request plays against, and that request
		// always seats a human.
		_, _, err = validateBotPolicyFlags(name, "")
		if err == nil {
			t.Errorf("-bot-policy %q accepted as the default", name)
		} else if !strings.Contains(err.Error(), "reads opponents' decklists") {
			t.Errorf("%q default: error %v, want one naming the reason", name, err)
		}
	}
}

// Without the opt-in a decklist reader's flags are refused, naming the rule.
func TestDecklistReaderNeedsAllowDecklistBots(t *testing.T) {
	c := withNet(config{autoMana: true, botAutoPayMana: true}, testOpenPolicy, netPolicy{net: "n", decks: "d"})
	err := c.checkNetFlags()
	if err == nil {
		t.Fatal("a decklist reader's -net was accepted without -allow-decklist-bots")
	}
	if !strings.Contains(err.Error(), "-allow-decklist-bots") {
		t.Fatalf("error %v, want one naming -allow-decklist-bots", err)
	}
	c.allowDecklistBots = true
	if err := c.checkNetFlags(); err != nil {
		t.Fatalf("with -allow-decklist-bots: %v", err)
	}
}

func TestDecklistReaderNotReachableFromVsBot(t *testing.T) {
	enableOpen(t)
	// The opt-in itself is set, to prove the refusal is not just the missing
	// flag: the vs-bot flow seats a HUMAN at seat 0, and no opt-in buys that.
	c := openConfig()
	c.allowDecklistBots = true
	r, gate := freshGameLock(t)
	create := c.createGame(r, gate, nil, []string{"c", "d"}, view.Omniscient)
	for _, tc := range []struct {
		name string
		req  httpapi.CreateGameOptions
	}{
		{"explicit pick", httpapi.CreateGameOptions{Format: host.FormatConstructed, BotPolicy: testOpenPolicy}},
		// A deck reserved for the policy must refuse rather than seat a human
		// against it.
		{"reserved-deck pick", httpapi.CreateGameOptions{Format: host.FormatConstructed, HumanDeck: "a"}},
	} {
		_, err := create(tc.req)
		if err == nil {
			t.Errorf("%s: the vs-bot flow seated a decklist reader", tc.name)
		} else if !strings.Contains(err.Error(), "reads opponents' decklists") {
			t.Errorf("%s: error %v, want the decklist refusal", tc.name, err)
		}
	}
	if n := len(r.Tables()); n != 0 {
		t.Errorf("refused requests created %d table(s)", n)
	}
}
