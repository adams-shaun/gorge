package main

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azc6"
)

// Draft events offer the closed-list az policies (az-c6 here: a reserved deck
// pool, but no decklist reading -- the event deals its own drafted decks and
// the policy plays them) and still leave out every decklist reader: az-c3
// (also reserved-pool) and the built-in Env policy sb-tactical.
func TestEventPoliciesOfferClosedListAZAndExcludeDecklistReaders(t *testing.T) {
	enableAZ(t)
	enableClosed(t, azc6.Policy)
	if !bots.ReadsDecklists(azc3.Policy) || !bots.ReadsDecklists("sb-tactical") || bots.ReadsDecklists(azc6.Policy) {
		t.Fatalf("premise: az-c3 %v, sb-tactical %v, az-c6 %v reads decklists", bots.ReadsDecklists(azc3.Policy), bots.ReadsDecklists("sb-tactical"), bots.ReadsDecklists(azc6.Policy))
	}
	c := config{
		botPolicies: []string{bots.Default, "sb-tactical", azc3.Policy, azc6.Policy},
		policyPools: map[string][]string{azc3.Policy: {"FDN_a"}, azc6.Policy: {"FDN_a"}},
	}
	if got, want := c.eventPolicies(), []string{bots.Default, azc6.Policy}; !reflect.DeepEqual(got, want) {
		t.Fatalf("event policies %v, want %v", got, want)
	}
}
