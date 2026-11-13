package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/server/gorged"
	"github.com/adams-shaun/gorge/server/protocol"
)

// linkedPolicies are exactly the always-registered entries this binary links:
// bots/all's built-ins and the experimental search and SpellBench policies.
var linkedPolicies = []string{
	"bot", "lethal-pressure", "az-redeal", "az-redeal-maxn", "sb-tactical", "forge-heuristic", "forge-heuristic-hold",
	"cast-profile", "search", "sb-search-lite-atk", "sb-heuristic", "sb-uniform", "sb-first",
}

func TestLinkedPoliciesRegister(t *testing.T) {
	for _, name := range linkedPolicies {
		got, err := bots.Normalize(name)
		if err != nil || got != name {
			t.Errorf("Normalize(%q) = %q, %v; want %q", name, got, err, name)
		}
	}
	var names []string
	for _, e := range bots.Entries() {
		names = append(names, e.Name)
	}
	want := slices.Clone(linkedPolicies)
	slices.Sort(want)
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("Entries() = %q, want %q", names, want)
	}
}

// TestSearchEntryShapes pins the registry fields the host and the UI read for
// the two search policies, and that each experimental entry this binary adds
// says what it is and what it costs.
func TestSearchEntryShapes(t *testing.T) {
	search, ok := bots.Lookup("search")
	if !ok {
		t.Fatal("no search entry")
	}
	if search.Tier != bots.Experimental || !search.Env || !search.Search {
		t.Errorf("search entry is not the experimental Env policy: %+v", search.Info)
	}
	sbs, ok := bots.Lookup("sb-search-lite-atk")
	if !ok {
		t.Fatal("no sb-search-lite-atk entry")
	}
	if sbs.Tier != bots.Experimental || !sbs.Env || !sbs.Search || sbs.MaxSeats != 2 {
		t.Errorf("sb-search-lite-atk entry is not the experimental Env+search 2-seat policy: %+v", sbs.Info)
	}
	for _, name := range []string{"cast-profile", "search", "sb-search-lite-atk", "sb-heuristic", "sb-uniform", "sb-first"} {
		e, _ := bots.Lookup(name)
		if e.Label == "" || e.Description == "" || e.Cost.Note == "" || len(e.Formats) == 0 {
			t.Errorf("entry %q has incomplete Info: %+v", name, e.Info)
		}
	}
}

// TestNetBotFlags pins the flag the demo deploy passes (scripts/deploy-demo.sh).
func TestNetBotFlags(t *testing.T) {
	if gorged.ServeFlagSet().Lookup("allow-decklist-bots") == nil {
		t.Error("-allow-decklist-bots is not a flag of this binary")
	}
}

// TestServedEventSeatsCastProfile: an event on this binary seats cast-profile,
// which resolves only because this binary links bots/castprofile on top of
// bots/all (the draft tree's own tests seat bots/all's policies only).
func TestServedEventSeatsCastProfile(t *testing.T) {
	base := startGorged(t, t.TempDir(), "-events", "-humans", "1")
	raw, err := json.Marshal(protocol.CreateEventRequest{Set: "FDN", Seats: []protocol.CreateEventSeat{
		{Kind: "bot", Policy: "bot"}, {Kind: "bot", Policy: "lethal-pressure"},
		{Kind: "bot", Policy: "cast-profile"}, {Kind: "bot", Policy: "bot"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(base+"/api/events", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		t.Fatalf("POST /api/events = %d: %s", resp.StatusCode, body)
	}
	var created protocol.CreateEventResponse
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	if len(created.State.Seats) != 4 {
		t.Fatalf("%d seats, want 4: %s", len(created.State.Seats), body)
	}
	if got := created.State.Seats[2].Policy; got != "cast-profile" {
		t.Fatalf("seat 2 policy %q, want cast-profile", got)
	}
}
