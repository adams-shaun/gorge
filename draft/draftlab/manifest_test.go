package draftlab

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// TestManifestDeterministic: the same inputs marshal identically, agents
// are sorted unique, and the revision is filled.
func TestManifestDeterministic(t *testing.T) {
	l := testLab(t)
	agents := []RoleRef{
		{Role: RolePlayer, Ref: AgentRef{Name: "bot", Version: "1"}},
		{Role: RoleDrafter, Ref: AgentRef{Name: "rating-colour", Version: "1"}},
		{Role: RoleBuilder, Ref: AgentRef{Name: "best23", Version: "1"}},
		{Role: RolePlayer, Ref: AgentRef{Name: "bot", Version: "1"}}, // duplicate
	}
	games := []GameResult{
		{Winner: 0, Head: "h1"},
		{Winner: -1, Stall: "turns", Head: "h2"},
		{Winner: -1, Head: "h3"},
	}
	want := l.NewManifest("games", 5, agents, games)
	for i := 0; i < 2; i++ {
		got := l.NewManifest("games", 5, agents, games)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("manifest %d differs:\n got %+v\nwant %+v", i, got, want)
		}
	}
	b1, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	b2, err := json.Marshal(l.NewManifest("games", 5, agents, games))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b1) != string(b2) {
		t.Errorf("manifests marshal differently:\n%s\n%s", b1, b2)
	}
	if got := want.Format; got != "gorge-draftlab/1" {
		t.Errorf("format %q, want gorge-draftlab/1", got)
	}
	if got := want.GorgeRevision; got == "" {
		t.Error("gorge revision is empty")
	}
	if !sort.StringsAreSorted(want.Agents) {
		t.Errorf("agents %v are not sorted", want.Agents)
	}
	seen := map[string]bool{}
	for _, a := range want.Agents {
		if seen[a] {
			t.Errorf("agent %q appears twice", a)
		}
		seen[a] = true
	}
	if expect := []string{"builder:best23@1", "drafter:rating-colour@1", "player:bot@1"}; !reflect.DeepEqual(want.Agents, expect) {
		t.Errorf("agents %v, want %v", want.Agents, expect)
	}
	if want.Games != 3 || want.Draws != 2 || want.Stalls != 1 {
		t.Errorf("manifest counts games %d draws %d stalls %d, want 3/2/1", want.Games, want.Draws, want.Stalls)
	}
	if want.MaxTurns != 150 || want.MaxIntents != 40000 || want.Mulligans != 2 {
		t.Errorf("manifest engine knobs %+v, want the defaults 150/40000/2", want)
	}
	if want.AllowDecklistBots {
		t.Error("manifest carries the decklist-bot opt-in the lab does not have")
	}
	// The run knobs never appear: a Workers/Verify lab marshals the same.
	lv := testLabOpts(t, Options{Workers: 4, Verify: true})
	b3, err := json.Marshal(lv.NewManifest("games", 5, agents, games))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b1) != string(b3) {
		t.Errorf("run knobs leaked into the manifest:\n%s\n%s", b1, b3)
	}
}

// TestManifestAgentsNeverNil: a manifest with no agents still marshals an
// empty list, not null.
func TestManifestAgentsNeverNil(t *testing.T) {
	l := testLab(t)
	m := l.NewManifest("games", 1, nil, nil)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) == "" {
		t.Fatal("empty marshal")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if string(raw["agents"]) != "[]" {
		t.Errorf("agents marshals %s, want []", raw["agents"])
	}
}
