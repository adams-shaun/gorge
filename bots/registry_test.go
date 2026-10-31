package bots_test

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all"
)

func TestRegister(t *testing.T) {
	for _, name := range []string{"bot", "lethal-pressure", "az-redeal", "sb-tactical", "forge-heuristic", "forge-heuristic-hold"} {
		if _, ok := bots.Lookup(name); !ok {
			t.Fatalf("built-in %q was not registered", name)
		}
	}
}

func TestNormalize(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"", "bot"}, {"bot", "bot"}, {"lethal-pressure", "lethal-pressure"}, {"az-redeal", "az-redeal"}, {"sb-tactical", "sb-tactical"}, {"forge-heuristic", "forge-heuristic"}} {
		got, err := bots.Normalize(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	if _, err := bots.Normalize("no-such-policy"); err == nil {
		t.Fatal("Normalize(no-such-policy) accepted an unregistered policy")
	}
}

func TestEntries(t *testing.T) {
	entries := bots.Entries()
	// gorge links six built-ins (bots/all); the research policies register
	// from four-yay. The count pin and name-based blocks below move together
	// with the built-in set; update this test deliberately on the next policy.
	if len(entries) != 6 {
		t.Fatalf("Entries() has %d entries, want 6", len(entries))
	}
	if entries[0].Name != "bot" || entries[0].Tier != bots.Production {
		t.Fatalf("production entry is not first: %+v", entries[0])
	}
	var azEntry *bots.Entry
	for i := range entries {
		if entries[i].Name == "az-redeal" {
			azEntry = &entries[i]
		}
	}
	if azEntry == nil {
		t.Fatal("no az-redeal entry in Entries()")
	}
	if azEntry.Tier != bots.Experimental || !azEntry.Env || !azEntry.Search {
		t.Errorf("az-redeal entry is not the experimental Env policy: %+v", azEntry.Info)
	}
	var sbEntry *bots.Entry
	for i := range entries {
		if entries[i].Name == "sb-tactical" {
			sbEntry = &entries[i]
		}
	}
	if sbEntry == nil {
		t.Fatal("no sb-tactical entry in Entries()")
	}
	if sbEntry.Tier != bots.Experimental || !sbEntry.Env || sbEntry.Search {
		t.Errorf("sb-tactical entry is not the experimental Env-only policy: %+v", sbEntry.Info)
	}
	var forgeEntry *bots.Entry
	for i := range entries {
		if entries[i].Name == "forge-heuristic" {
			forgeEntry = &entries[i]
		}
	}
	if forgeEntry == nil {
		t.Fatal("no forge-heuristic entry in Entries()")
	}
	if forgeEntry.Tier != bots.Experimental || forgeEntry.Env || forgeEntry.Search {
		t.Errorf("forge-heuristic entry is not the plain experimental policy: %+v", forgeEntry.Info)
	}
	for _, e := range entries {
		if e.Label == "" || e.Description == "" || e.Cost.Note == "" || len(e.Formats) == 0 {
			t.Errorf("entry %q has incomplete Info: %+v", e.Name, e.Info)
		}
	}
}

// TestValidateUnlinkedCaretakerRefused pins the caretaker rule from the
// UNLINKED side (the linked side is bots.Validate above): a caretaker name
// the binary has not registered is an error, not a silent nil.
func TestValidateUnlinkedCaretakerRefused(t *testing.T) {
	_, ok := bots.Lookup("definitely-not-registered")
	if ok {
		t.Fatal("precondition: a policy named definitely-not-registered is linked")
	}
	if _, err := bots.New("definitely-not-registered", bots.Options{}); err == nil {
		t.Error("New(definitely-not-registered) built a seat for an unregistered policy")
	}
}

func TestValidate(t *testing.T) {
	if err := bots.Validate(); err != nil {
		t.Fatal(err)
	}
}
