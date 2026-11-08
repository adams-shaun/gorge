package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestChandraReversedCastStillDealsDamage pins the RememberPlayed$ begun
// guard (CR 733.1-2): Chandra, Torch of Defiance's +1 exiles the top card and
// offers a free cast; if the cast is refused or reversed, the card was never
// played, so RememberPlayed$ must not record it and the "if you don't"
// damage must still be dealt. The control case casts a free Ornithopter,
// which commits, so the record is made and the damage is turned off.
func TestChandraReversedCastStillDealsDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name string
		top  string
		life int32
	}{
		{"reversed", "Grizzly Bears", 18},
		{"committed", "Ornithopter", 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := strings.ReplaceAll(chandraReversedCast, "%TOP%", tc.top)
			res, err := RunOracleScenarioJSON(reg, []byte(sc))
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
			}
			last := res.Snapshots[len(res.Snapshots)-1]
			// Precondition: the offered cast went where the case expects, so
			// the record the rule reads is actually exercised.
			if tc.top == "Grizzly Bears" {
				if got := strings.Join(last.Players[0].Exile, ","); !strings.Contains(","+got+",", ",Grizzly Bears,") {
					t.Fatalf("p0 exile = %q, want the reversed Grizzly Bears back in exile", got)
				}
				for _, p := range last.Permanents {
					if p.Name == "Grizzly Bears" {
						t.Fatalf("a Grizzly Bears permanent exists after the reversed cast")
					}
				}
			} else {
				found := false
				for _, p := range last.Permanents {
					if p.Name == "Ornithopter" && p.Controller == 0 {
						found = true
					}
				}
				if !found {
					t.Fatalf("no Ornithopter permanent controlled by seat 0 after the free cast")
				}
			}
			if got := last.Players[1].Life; got != tc.life {
				t.Fatalf("seat 1 life = %d, want %d", got, tc.life)
			}
		})
	}
}

const chandraReversedCast = `{"name":"chandra-reversed-cast","cr":["733.1"],"why":"inline","setup":{"p0":{"battlefield":["Chandra, Torch of Defiance"],"library_top":["%TOP%","Grizzly Bears"]}},"steps":[` +
	`{"op":"activate","seat":0,"card":"p0:Chandra, Torch of Defiance","ability_index":0},` +
	`{"op":"resolve","seat":0,"answers":[{"kind":"modes","pick":["Play %TOP%"]}]}]}`
