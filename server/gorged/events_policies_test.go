package gorged

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots"
)

// Draft events offer the closed-list net policies (a reserved
// deck pool, but no decklist reading -- the event deals its own drafted
// decks and the policy plays them) and still leave out every decklist
// reader: a reserved-pool policy and the built-in Env policy sb-tactical.
func TestEventPoliciesOfferClosedListNetAndExcludeDecklistReaders(t *testing.T) {
	enableOpen(t)
	enableClosed(t, testClosedA)
	if !bots.ReadsDecklists(testOpenPolicy) || !bots.ReadsDecklists("sb-tactical") || bots.ReadsDecklists(testClosedA) {
		t.Fatalf("premise: open %v, sb-tactical %v, closed %v reads decklists",
			bots.ReadsDecklists(testOpenPolicy), bots.ReadsDecklists("sb-tactical"), bots.ReadsDecklists(testClosedA))
	}
	c := config{
		botPolicies: []string{bots.Default, "sb-tactical", testOpenPolicy, testClosedA},
		policyPools: map[string][]string{testOpenPolicy: {"FDN_a"}, testClosedA: {"FDN_a"}},
	}
	if got, want := c.eventPolicies(), []string{bots.Default, testClosedA}; !reflect.DeepEqual(got, want) {
		t.Fatalf("event policies %v, want %v", got, want)
	}
}
