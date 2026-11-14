package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// TestDraftsimCommanderRatingsFile: -ratings-file reaches the Commander
// persona instead of being refused for its drafter type.
func TestDraftsimCommanderRatingsFile(t *testing.T) {
	dir := corpusReady(t)
	rf := filepath.Join(t.TempDir(), "r.json")
	body := `{"Sol Ring": {"blended": 0.9}, "Command Tower": {"blended": 0.5}, "Lightning Bolt": {"blended": 0.7}}`
	if err := os.WriteFile(rf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	o := options{set: "MBC", collation: "draft", format: event.FormatCommander, seed: 1, events: 1,
		players: "bot", drafter: "rating-colour", builder: "best23", structure: "none", dir: dir,
		ratingsFile: rf, ratingsField: "blended"}
	var out strings.Builder
	if _, err := run(o, &out); err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "ratings override:") {
		t.Fatalf("no override line:\n%s", out.String())
	}
}
