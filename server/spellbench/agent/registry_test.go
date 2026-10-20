package agent_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/minihost"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/validate"
)

func TestAgentServesEveryEligibleRegistryPolicy(t *testing.T) {
	served := 0
	for _, name := range bots.Names() {
		entry, ok := bots.Lookup(name)
		if !ok {
			t.Fatalf("registry names %q but Lookup misses it", name)
		}
		if entry.Env || entry.Search {
			continue
		}
		served++
		t.Run(name, func(t *testing.T) {
			first := playPolicyGame(t, name, 7)
			second := playPolicyGame(t, name, 7)
			if first != second {
				t.Fatalf("replay digest %s, first run %s", second, first)
			}
		})
	}
	if served == 0 {
		t.Fatal("no eligible registry policy was served")
	}
}

// TestAgentHelloNamesRequestedPolicy pins that New actually serves the named
// registry policy: a server that ignored name and always built "bot" would
// still replay identically and pass the game test above.
func TestAgentHelloNamesRequestedPolicy(t *testing.T) {
	for _, name := range bots.Names() {
		a, err := agent.New(name)
		if err != nil {
			t.Fatalf("agent.New(%q): %v", name, err)
		}
		var m map[string]any
		if err := json.Unmarshal(a.Handle([]byte(`{"request_type":"hello","protocol":"spellbench/v2","request_id":"r-0"}`)), &m); err != nil {
			t.Fatalf("%s hello: %v", name, err)
		}
		bot, ok := m["bot"].(map[string]any)
		if !ok {
			t.Fatalf("%s hello has no bot object: %v", name, m)
		}
		if got, want := bot["name"], "gorge-"+name; got != want {
			t.Errorf("%s hello bot.name = %v, want %q", name, got, want)
		}
	}
}

func playPolicyGame(t *testing.T, name string, seed uint64) string {
	t.Helper()
	reg := testcorpus.Registry(t)
	kinds := map[string]bool{}
	for _, k := range env.DecisionKinds {
		kinds[k] = true
	}
	h := &minihost.Host{RunSecret: make([]byte, 32), Engine: &minihost.EngineLink{S: env.New(reg, nil)},
		Profile:  validate.Profile{Kinds: kinds, Flags: observe.Flags, Extensions: map[string]bool{"x_gorge_view_v1": true}},
		MaxSteps: 3000, MaxDecisions: 2999}
	burn, _ := catalog.ByID("Burn")
	var links [2]minihost.Link
	for i := range links {
		a, err := agent.New(name)
		if err != nil {
			t.Fatal(err)
		}
		links[i] = a
	}
	res, err := h.Play(seed, burn, "london", []string{"x_gorge_view_v1"}, links)
	if err != nil {
		t.Fatal(err)
	}
	if res.Terminal.Classification != "natural" {
		t.Fatalf("terminal %q: %s", res.Terminal.Classification, res.Terminal.Reason)
	}
	return res.Digest
}

func TestAgentRefusesDecklistPolicyOnHiddenLists(t *testing.T) {
	var name string
	for _, candidate := range bots.Names() {
		if bots.ReadsDecklists(candidate) {
			name = candidate
			break
		}
	}
	if name == "" {
		t.Skip("no registered policy reads decklists")
	}
	a, err := agent.New(name)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(a.Handle([]byte(`{"request_type":"game_start","protocol":"spellbench/v2","request_id":"r-0","game_id":"g","seat":"p0","agent_seed":1,"rules":{"opponent_decklist":"hidden"},"engine_profile":{"engine_defaults":{"mana_payment":"engine_autopay"}}}`)), &m)
	if m["response_type"] != "error" || m["error"].(map[string]any)["code"] != "malformed_request" {
		t.Fatalf("hidden lists: %v", m)
	}
	if !strings.Contains(m["error"].(map[string]any)["message"].(string), "decklist") {
		t.Fatalf("hidden lists message %v", m["error"])
	}
}

func TestAgentRefusesUnknownPolicy(t *testing.T) {
	if _, err := agent.New("not-a-registered-policy"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown policy error = %v", err)
	}
}

func TestAgentNeutralWorldPlaysBot(t *testing.T) {
	reg := testcorpus.Registry(t)
	kinds := map[string]bool{}
	for _, k := range env.DecisionKinds {
		kinds[k] = true
	}
	h := &minihost.Host{RunSecret: make([]byte, 32), Engine: &minihost.EngineLink{S: env.New(reg, nil)},
		Profile:  validate.Profile{Kinds: kinds, Flags: observe.Flags, Extensions: map[string]bool{}},
		MaxSteps: 20000, MaxDecisions: 9999}
	burn, _ := catalog.ByID("Burn")
	var links [2]minihost.Link
	for i := range links {
		a, err := agent.New("bot")
		if err != nil {
			t.Fatal(err)
		}
		a.SetRegistry(reg)
		if err := a.EnableNeutral(); err != nil {
			t.Fatal(err)
		}
		links[i] = a
	}
	res, err := h.Play(1, burn, "london", nil, links)
	if err != nil {
		t.Fatal(err)
	}
	if res.Terminal.Classification != "natural" {
		t.Fatalf("terminal %q: %s", res.Terminal.Classification, res.Terminal.Reason)
	}
}
