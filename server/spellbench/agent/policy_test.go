package agent

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
)

func strptr(s string) *string { return &s }

func TestLookupPolicyUnknown(t *testing.T) {
	if _, err := lookupPolicy("not-a-registered-policy"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown policy error = %v", err)
	}
}

// TestCheckPolicy pins each game_start refusal and the decklist-before-scope
// order: a decklist-reading Env policy refused on hidden lists must name the
// decklist, not the engine root, and the same policy on visible lists must
// then fall through to the scope refusal.
func TestCheckPolicy(t *testing.T) {
	decklistEnv := Policy{Key: "decklist-env", Info: bots.Info{Env: true, Search: true}}
	scope := Policy{Key: "scope", Info: bots.Info{Env: true, ClosedLists: true}}
	autoPay := Policy{Key: "auto-pay", Info: bots.Info{NeedsAutoPay: true}}
	for _, c := range []struct {
		name     string
		identity Policy
		decklist string
		defaults map[string]*string
		want     string
	}{
		{"decklist-hidden", decklistEnv, "hidden", nil, "decklist"},
		{"decklist-visible", decklistEnv, "visible", nil, "engine root"},
		{"env-scope", scope, "hidden", nil, "engine root"},
		{"auto-pay-missing", autoPay, "hidden", nil, "auto-pay"},
		{"auto-pay-present", autoPay, "hidden", map[string]*string{"mana_payment": strptr("engine_autopay")}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &Server{identity: c.identity}
			q := request{}
			q.Rules.OpponentDecklist = c.decklist
			q.EngineProfile.EngineDefaults = c.defaults
			err := s.checkPolicy(q)
			switch {
			case c.want == "" && err != nil:
				t.Fatalf("checkPolicy = %v, want nil", err)
			case c.want != "" && err == nil:
				t.Fatalf("checkPolicy = nil, want %q refusal", c.want)
			case c.want != "" && !strings.Contains(err.Error(), c.want):
				t.Fatalf("checkPolicy = %v, want %q refusal", err, c.want)
			}
		})
	}
}
