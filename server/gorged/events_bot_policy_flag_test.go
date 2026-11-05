package gorged

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
)

func TestEventsBotPolicyFlag(t *testing.T) {
	enableClosed(t, testClosedA)
	c := config{botPolicy: bots.Default, botPolicies: []string{bots.Default, testClosedA}}
	if got, err := c.eventsDefaultPolicy(); err != nil || got != bots.Default {
		t.Fatalf("unset: %q, %v; want -bot-policy %q", got, err, bots.Default)
	}
	c.eventsBotPolicy = testClosedA
	if got, err := c.eventsDefaultPolicy(); err != nil || got != testClosedA {
		t.Fatalf("set: %q, %v; want %q", got, err, testClosedA)
	}
	c.eventsBotPolicy = "not-a-policy"
	if _, err := c.eventsDefaultPolicy(); err == nil {
		t.Fatal("unregistered value accepted")
	}
	c.eventsBotPolicy = "sb-tactical"
	if _, err := c.eventsDefaultPolicy(); err == nil {
		t.Fatal("decklist reader accepted")
	}
}
