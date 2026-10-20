package gorged

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
)

// TestBotPolicyHelpListsTheRegistry holds -bot-policy's usage to the
// registry: cast-profile is a downstream policy gorge never registers, so
// naming it sent operators to a policy that fails validation.
func TestBotPolicyHelpListsTheRegistry(t *testing.T) {
	if _, ok := bots.Lookup("cast-profile"); ok {
		t.Skip("cast-profile is registered here, so the help may name it")
	}
	f := ServeFlagSet().Lookup("bot-policy")
	if f == nil {
		t.Fatal("no -bot-policy flag")
	}
	if strings.Contains(f.Usage, "cast-profile") {
		t.Errorf("-bot-policy help names unregistered cast-profile: %q", f.Usage)
	}
	for _, name := range bots.Names() {
		if !strings.Contains(f.Usage, name) {
			t.Errorf("-bot-policy help omits registered policy %q: %q", name, f.Usage)
		}
	}
}
