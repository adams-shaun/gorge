package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmMysterio pins the farm window SPM-s247-rand (xe:RAND:4:14:w11),
// where gorge's stack lacked the leave trigger Forge and XMage both showed.
// Mysterio, Master of Illusion's enter trigger creates a 3/3 Illusion Villain
// token for each nontoken Villain its controller controls and then creates a
// delayed triggered ability: "Exile those tokens when Mysterio leaves the
// battlefield." CR 603.7: an effect may create a delayed triggered ability; it
// exists once the enter trigger resolves with Mysterio on the battlefield, so
// CR 603.7b's next departure triggers it, even when CR 603.7c makes the body
// skip a token that already left. Here p0 controls a nontoken Villain (Kree
// Commandos) besides Mysterio itself, so the enter trigger makes two tokens;
// moving Mysterio to the graveyard must put the leave ability on the stack,
// and resolving it must exile both tokens.
func TestFarmMysterio(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const scenario = `{
	  "name": "farm-mysterio-leave-trigger",
	  "cr": ["603.7", "603.7b", "603.7c"],
	  "why": "the delayed exile-those-tokens ability exists while Mysterio is on the battlefield and triggers when it leaves",
	  "setup": {
	    "p0": {"battlefield": ["Kree Commandos", "Island", "Island", "Island", "Island"],
	           "hand": ["Mysterio, Master of Illusion"]}
	  },
	  "steps": [
	    {"op": "cast", "seat": 0, "card": "p0:Mysterio, Master of Illusion", "mana": "UUUU"},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1},
	    {"op": "move", "seat": 0, "card": "p0:Mysterio, Master of Illusion", "to": "graveyard"},
	    {"op": "resolve", "seat": 0}
	  ]
	}`
	res, err := RunOracleScenarioJSON(reg, []byte(scenario))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v", res.Fails)
	}
	snap := func(name string) OracleSnapshot {
		t.Helper()
		for _, s := range res.Snapshots {
			if s.Checkpoint == name {
				return s
			}
		}
		t.Fatalf("missing snapshot %q (have %d)", name, len(res.Snapshots))
		return OracleSnapshot{}
	}
	// Precondition: Mysterio entered and its enter trigger made one token per
	// nontoken Villain (Mysterio itself plus Kree Commandos).
	before := snap("step 4 (pass)")
	mysterio, tokens := false, 0
	for _, p := range before.Permanents {
		if p.Name == "Mysterio, Master of Illusion" {
			mysterio = true
		}
		if p.Token && p.Name == "Illusion Villain Token" {
			tokens++
		}
	}
	if !mysterio || tokens != 2 {
		t.Fatalf("precondition: Mysterio on battlefield=%v, Illusion Villain tokens=%d, want true/2", mysterio, tokens)
	}
	// The departure: Mysterio left, and its delayed trigger must be the only
	// ability on the stack.
	left := snap("step 5 (move)")
	for _, p := range left.Permanents {
		if p.Name == "Mysterio, Master of Illusion" {
			t.Fatalf("precondition: Mysterio is still on the battlefield after the move")
		}
	}
	onStack := false
	for _, e := range left.Stack {
		if e.Kind == "ability" && strings.Contains(e.Source, "Mysterio") {
			onStack = true
		}
	}
	if !onStack {
		t.Fatalf("Mysterio's leaves-the-battlefield trigger is not on the stack (CR 603.7b): stack=%v", left.Stack)
	}
	// Its resolution exiles the remembered tokens (CR 603.7c: a token that
	// already left would simply be skipped, and the effect still fires). A
	// token exiled from the battlefield ceases to exist (CR 111.7), so the
	// observable result is that neither token remains on the battlefield.
	last := res.Snapshots[len(res.Snapshots)-1]
	remaining := 0
	for _, p := range last.Permanents {
		if p.Token && p.Name == "Illusion Villain Token" {
			remaining++
		}
	}
	if remaining != 0 {
		t.Fatalf("after the leave trigger resolves %d Illusion Villain tokens remain on the battlefield, want 0 exiled",
			remaining)
	}
}
