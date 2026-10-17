package gorged

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots"
)

// Draft events offer the closed-list net policies (az-c4 here: a reserved
// deck pool, but no decklist reading -- the event deals its own drafted
// decks and the policy plays them) and still leave out every decklist
// reader: az-c3 (also reserved-pool) and the built-in Env policy sb-tactical.
func TestEventPoliciesOfferClosedListAZAndExcludeDecklistReaders(t *testing.T) {
	enableAZ(t)
	enableClosed(t, policyAZC4)
	if !bots.ReadsDecklists(policyAZC3) || !bots.ReadsDecklists("sb-tactical") || bots.ReadsDecklists(policyAZC4) {
		t.Fatalf("premise: az-c3 %v, sb-tactical %v, az-c4 %v reads decklists",
			bots.ReadsDecklists(policyAZC3), bots.ReadsDecklists("sb-tactical"), bots.ReadsDecklists(policyAZC4))
	}
	c := config{
		botPolicies: []string{bots.Default, "sb-tactical", policyAZC3, policyAZC4},
		policyPools: map[string][]string{policyAZC3: {"FDN_a"}, policyAZC4: {"FDN_a"}},
	}
	if got, want := c.eventPolicies(), []string{bots.Default, policyAZC4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("event policies %v, want %v", got, want)
	}
}
