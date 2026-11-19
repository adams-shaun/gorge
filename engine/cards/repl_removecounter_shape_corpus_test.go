package cards_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRemoveCounterShapeMatchesTheParsedCorpusLine pins the RemoveCounter
// shape split against the parsed corpus lines themselves: Fear of Sleep
// Paralysis's R: line is the implemented CantHappen shape and labels
// "repl:RemoveCounter", while Deification's damage arm labels
// "repl:RemoveCounter.OtherShape" and stays unsupported.
func TestRemoveCounterShapeMatchesTheParsedCorpusLine(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for name, want := range map[string]string{
		"Fear of Sleep Paralysis": "repl:RemoveCounter",
		"Deification":             "repl:RemoveCounter.OtherShape",
	} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("corpus missing %s", name)
		}
		got := ""
		for _, p := range c.Primitives() {
			if len(p) >= 5 && p[:5] == "repl:" {
				got = p
			}
		}
		if got != want {
			t.Fatalf("%s primitive = %q, want %q (params: %v)", name, got, want, replParams(c))
		}
	}
}

// replParams returns the first face's first R: line's params for the
// failure message.
func replParams(c *cards.Card) map[string]string {
	if len(c.Faces) == 0 || len(c.Faces[0].Repls) == 0 {
		return nil
	}
	return c.Faces[0].Repls[0].Params
}
