package gorged

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
)

// TestBotPolicyListCarriesForgeHeuristic proves the experimental
// forge-heuristic policy reaches the wire listing, not merely the registry:
// the default offered set names it, and buildBotPolicyList converts its tier
// and formats.
func TestBotPolicyListCarriesForgeHeuristic(t *testing.T) {
	const name = "forge-heuristic"
	e, ok := bots.Lookup(name)
	if !ok {
		t.Fatalf("bots.Lookup(%q): not registered", name)
	}
	offered, err := botPolicyOffered("")
	if err != nil {
		t.Fatalf("botPolicyOffered(\"\"): %v", err)
	}
	if !containsString(offered, name) {
		t.Fatalf("default offered set %v lost %q", offered, name)
	}
	list := buildBotPolicyList(bots.Default, offered)
	for _, p := range list.Policies {
		if p.Name != name {
			continue
		}
		if p.Tier != string(bots.Experimental) {
			t.Fatalf("tier = %q, want %q", p.Tier, bots.Experimental)
		}
		if len(p.Formats) != len(e.Formats) {
			t.Fatalf("formats = %v, want %v", p.Formats, e.Formats)
		}
		return
	}
	t.Fatalf("wire listing does not carry %q", name)
}
