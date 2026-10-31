package seat

import (
	"bytes"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/internal/schemacheck"
)

// TestRequestsMatchTheSchemas holds every line the seat sends an agent over a
// real game to the published schemas.
func TestRequestsMatchTheSchemas(t *testing.T) {
	set, err := schemacheck.Load("../../../docs/site/api/schemas/spellbench-v2")
	if err != nil {
		t.Fatal(err)
	}
	argv, record := recordingMinimal(t)
	e, _, _ := testEngine(t)
	agent, fallback := attachTestSeatArgv(t, e, 0, argv, 0)
	seats := []seat.Seat{agent, fallback}
	if _, err := drive(t, e, seats, 20000); err != nil {
		t.Fatal(err)
	}
	endTestMatch(seats)
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		doc, err := schemacheck.Decode(line)
		if err == nil {
			err = set.Validate("requests.schema.json#/$defs/agent_request", doc)
		}
		if err != nil {
			t.Fatalf("%v\nline: %.2000s", err, line)
		}
		seen[doc.(map[string]any)["request_type"].(string)]++
	}
	for _, kind := range []string{"hello", "game_start", "choose", "game_over"} {
		if seen[kind] == 0 {
			t.Errorf("no %s was checked: %v", kind, seen)
		}
	}
}
