package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestStaticTriggerGateSkipsOffZoneRepl: a library card's ActiveZones$
// Battlefield replacement (Vizier of Remedies' AddCounter line, whose
// ReplaceCounter body may ask) is not active from the library (CR 614.4
// via ActiveZones$), and Exhaustion's Static$ True self-exile moves only the
// Effect card itself, so the line cannot become active while it resolves.
// The self-exile must still resolve inline (CR 611.2a): the stack stays
// empty at the target's upkeep.
func TestStaticTriggerGateSkipsOffZoneRepl(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	steps := []string{
		`{"op":"activate","seat":0,"card":"p0:Island","ability_index":0}`,
		`{"op":"activate","seat":0,"card":"p0:Island#2","ability_index":0}`,
		`{"op":"activate","seat":0,"card":"p0:Island#3","ability_index":0}`,
		`{"op":"cast","seat":0,"card":"p0:Exhaustion","targets":["p1"]}`,
	}
	for range 8 {
		steps = append(steps, `{"op":"pass","seat":0}`, `{"op":"pass","seat":1}`)
	}
	sc := `{"name":"static-trigger-offzone-repl","cr":["611.2a","614.4"],"setup":{` +
		`"p0":{"battlefield":["Island","Island","Island"],"hand":["Exhaustion"],"library":["Vizier of Remedies","Island","Island"]},` +
		`"p1":{"battlefield":["Grizzly Bears","Forest"],"library":["Vizier of Remedies","Forest","Forest"]}},"steps":[` +
		strings.Join(steps, ",") + `]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	found := false
	for _, s := range res.Snapshots {
		if s.Active != 1 || s.Step != "upkeep" {
			continue
		}
		found = true
		vizier := false
		for _, p := range s.Players {
			for _, c := range p.LibraryTop {
				vizier = vizier || c == "Vizier of Remedies"
			}
		}
		if !vizier {
			t.Fatalf("precondition: no Vizier of Remedies in a library at p1's upkeep")
		}
		var stack []string
		for _, it := range s.Stack {
			stack = append(stack, it.Kind+":"+it.Source)
		}
		if len(stack) != 0 {
			t.Fatalf("stack at p1's upkeep = %v, want []", stack)
		}
	}
	if !found {
		t.Fatalf("precondition: no snapshot at p1's upkeep (fails %v)", res.Fails)
	}
}
