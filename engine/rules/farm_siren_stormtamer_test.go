package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmSirenStormtamer pins the SUP-s344-supp farm window
// xe:RAND:4:7:w11 step 6, where p0 casts Cyber Conversion at its own Siren
// Stormtamer and then activates Stormtamer ("{U}, Sacrifice Siren Stormtamer:
// Counter target spell or ability that targets you or a creature you
// control") at Cyber Conversion. The sacrifice is an activation cost (CR
// 602.2b, 601.2h), so when the ability resolves Cyber Conversion no longer
// targets a creature p0 controls. CR 115.9b reads the CURRENT state of the
// spell's targets: an object no longer in the zone it is expected to be in is
// ignored, and its last known information is not used. CR 608.2b then makes
// the ability's target illegal, so the ability does not resolve and Cyber
// Conversion stays on the stack. Forge does exactly that
// (SpellAbility.canTargetSpellAbility, "CR 115.9b need to check current
// target state"); XMage reads last known information here and is not a
// witness. Gorge used to counterspell anyway.
//
// The second case is the control: a second creature p0 controls is still the
// spell's current target, so Stormtamer's ability remains legal and does
// counter Cyber Conversion.
func TestFarmSirenStormtamer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	cases := []struct {
		name        string
		battlefield string
		castTarget  string
		// wantStack is the stack after the Stormtamer ability resolves;
		// wantCryptCountered is true when the ability itself resolved and
		// countered Cyber Conversion.
		wantStack          []string
		wantCryptCountered bool
	}{
		{
			name:        "the sacrificed target is ignored and the spell stays",
			battlefield: `"Siren Stormtamer"`,
			castTarget:  "p0:Siren Stormtamer",
			wantStack:   []string{"spell:p0:Cyber Conversion"},
		},
		{
			name:               "a surviving creature you control keeps the ability legal",
			battlefield:        `"Siren Stormtamer","Grizzly Bears"`,
			castTarget:         "p0:Grizzly Bears",
			wantStack:          nil,
			wantCryptCountered: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sc := `{"name":"farm siren stormtamer","turn":1,"cr":["115.9b","608.2b"],` +
				`"why":"CR 115.9b: the resolution recheck reads the targeting spell's current targets, not last known information",` +
				`"setup":{"p0":{"battlefield":[` + tc.battlefield + `],"hand":["Cyber Conversion"]},"p1":{}},` +
				`"steps":[{"op":"cast","seat":0,"card":"p0:Cyber Conversion","mana":"UU","targets":["` + tc.castTarget + `"]},` +
				`{"op":"activate","seat":0,"card":"p0:Siren Stormtamer","ability_index":0,"mana":"U","targets":["p0:Cyber Conversion"]},` +
				`{"op":"pass","seat":0},{"op":"pass","seat":1}]}`
			res, err := RunOracleScenarioJSON(reg, []byte(sc))
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
			}
			snap := func(cp string) *OracleSnapshot {
				for i := range res.Snapshots {
					if res.Snapshots[i].Checkpoint == cp {
						return &res.Snapshots[i]
					}
				}
				t.Fatalf("no %q snapshot", cp)
				return nil
			}
			stackOf := func(s *OracleSnapshot) []string {
				var got []string
				for _, it := range s.Stack {
					got = append(got, it.Kind+":"+it.Source)
				}
				return got
			}

			// Precondition: the spell is on the stack aimed at the named
			// creature, and Stormtamer is on the battlefield to pay its own
			// sacrifice cost.
			if got := stackOf(snap("step 0 (cast)")); fmt.Sprint(got) != fmt.Sprint([]string{"spell:p0:Cyber Conversion"}) {
				t.Fatalf("precondition: stack after the cast = %v, want the Cyber Conversion spell", got)
			}
			// Precondition: activating paid the sacrifice, so Stormtamer is
			// no longer on the battlefield while the ability is on the stack.
			afterActivate := snap("step 1 (activate)")
			if got := stackOf(afterActivate); fmt.Sprint(got) != fmt.Sprint([]string{"ability:p0:Siren Stormtamer", "spell:p0:Cyber Conversion"}) {
				t.Fatalf("precondition: stack after the activation = %v, want the ability above the spell", got)
			}
			if gy := afterActivate.Players[0].Graveyard; len(gy) != 1 || gy[0] != "Siren Stormtamer" {
				t.Fatalf("precondition: p0 graveyard after the sacrifice = %v, want [Siren Stormtamer]", gy)
			}

			// The resolution the farm window reads: p0 and p1 pass, and the
			// Stormtamer ability is the top object.
			after := snap("step 3 (pass)")
			if got, want := stackOf(after), tc.wantStack; fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("stack after the Stormtamer ability resolved = %v, want %v", got, want)
			}
			gy := after.Players[0].Graveyard
			cyberCountered := len(gy) == 2 && gy[0] == "Siren Stormtamer" && gy[1] == "Cyber Conversion"
			if cyberCountered != tc.wantCryptCountered {
				t.Fatalf("p0 graveyard = %v, want Cyber Conversion countered=%v", gy, tc.wantCryptCountered)
			}
			if !tc.wantCryptCountered {
				if len(gy) != 1 || gy[0] != "Siren Stormtamer" {
					t.Fatalf("p0 graveyard = %v, want only the sacrificed Siren Stormtamer", gy)
				}
			}
		})
	}
}
