package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOracleStackAbilityKeepsBackFaceNameAfterSourceLeaves pins that a loyalty
// ability of a transformed planeswalker that dies to its own cost stays named
// by the back face on the stack (CR 113.7a, 712.8e): the card is back to its
// front face in the graveyard, but the ability is the one the back face
// printed.
func TestOracleStackAbilityKeepsBackFaceNameAfterSourceLeaves(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup("Oko, Lorwyn Liege")
	if !ok || len(c.Faces) < 2 || c.Faces[0].Name == c.Faces[1].Name {
		t.Fatalf("Oko, Lorwyn Liege is not a two-named-face card in the corpus")
	}
	front, back := c.Faces[0].Name, c.Faces[1].Name

	run := func(index string) []OracleSnapStack {
		sc := `{"name":"backface-lki","setup":{"p0":{"battlefield":["Oko, Lorwyn Liege"],"back_face":["Oko, Lorwyn Liege"]},"p1":{"battlefield":["Grizzly Bears"]}},` +
			`"steps":[{"op":"resolve","seat":0},{"op":"activate","seat":0,"card":"p0:Oko, Lorwyn Liege","ability_index":` + index + `},{"op":"resolve","seat":0}]}`
		res, err := RunOracleScenarioJSON(reg, []byte(sc))
		if err != nil {
			t.Fatalf("run (ability %s): %v", index, err)
		}
		for _, s := range res.Snapshots {
			if strings.Contains(s.Checkpoint, "(activate)") {
				if len(s.Stack) == 0 {
					t.Fatalf("ability %s never reached the stack", index)
				}
				if len(s.Players[0].Graveyard) != 1 {
					t.Fatalf("the walker should have left the battlefield, graveyard = %v", s.Players[0].Graveyard)
				}
				return s.Stack
			}
		}
		t.Fatalf("no activate checkpoint")
		return nil
	}

	// The -3 takes the 3-loyalty walker to the graveyard before anyone has
	// priority (CR 704.5i), so the source is the front-face card by then.
	got := run("1")
	if got[0].Kind != "ability" || !strings.HasSuffix(got[0].Source, ":"+back) || strings.Contains(got[0].Source, front) {
		t.Errorf("stack source = %q (%s), want an ability named %q", got[0].Source, got[0].Kind, back)
	}
}
