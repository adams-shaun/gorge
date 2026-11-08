package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmUnforgivingAim pins the ECL farm windows (ECL-s623/s630/s636/s647,
// 3way:FX|G, field stack) where gorge cast Unforgiving Aim under Raiding
// Schemes' conspire grant WITHOUT the conspire tap and still put a Conspire
// ability above the spell, while Forge and XMage put only the spell. CR
// 702.78a's trigger has an intervening "if" ("if its conspire cost was
// paid", CR 603.4), so a declined conspire never triggers. The two boards are
// the s636 window (empty stack before the cast) and the s630 window (Surly
// Farrier's ability already on the stack), replayed from their farm inputs.
func TestFarmUnforgivingAim(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const aimCast = `{"op":"cast","seat":0,"card":"p0:Unforgiving Aim","targets":["p0:Raiding Schemes"],` +
		`"answers":[{"kind":"modes","pick":["Destroy target enchantment."]}]}`
	cases := []struct {
		name, setup, steps string
		checkpoint         string
		want               []string
	}{
		{
			name: "ECL-s636",
			setup: `"p0":{"battlefield":["Forest","Forest","Mountain","Mountain","Forest","Raiding Schemes",` +
				`"Safewright Cavalry","Bristlebane Outrider","Sizzling Changeling"],"hand":["Unforgiving Aim"],"life":10},` +
				`"p1":{"battlefield":["Plains","Island","Thoughtweft Imbuer","Adept Watershaper"],"life":2}`,
			steps: `{"op":"activate","seat":0,"card":"p0:Forest","ability_index":0},` +
				`{"op":"activate","seat":0,"card":"p0:Mountain","ability_index":0},` +
				`{"op":"activate","seat":0,"card":"p0:Mountain#2","ability_index":0},` + aimCast,
			checkpoint: "step 3 (cast)",
			want:       []string{"spell:p0:Unforgiving Aim"},
		},
		{
			name: "ECL-s630",
			setup: `"p0":{"battlefield":["Mountain","Mountain","Forest","Surly Farrier","Mountain","Bristlebane Outrider",` +
				`"Forest","Moon-Vigil Adherents","Raiding Schemes","Forest"],"hand":["Unforgiving Aim"],"life":16},` +
				`"p1":{"battlefield":["Forest","Swamp","Safewright Cavalry"],"life":9}`,
			steps: `{"op":"activate","seat":0,"card":"p0:Surly Farrier","targets":["p0:Moon-Vigil Adherents"],"ability_index":0},` +
				`{"op":"activate","seat":0,"card":"p0:Forest","ability_index":0},` +
				`{"op":"activate","seat":0,"card":"p0:Mountain#3","ability_index":0},` +
				`{"op":"activate","seat":0,"card":"p0:Forest#3","ability_index":0},` + aimCast,
			checkpoint: "step 4 (cast)",
			want:       []string{"spell:p0:Unforgiving Aim", "ability:p0:Surly Farrier"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sc := `{"name":"farm unforgiving aim ` + tc.name + `","turn":1,"cr":["702.78a","603.4"],"why":"inline",` +
				`"setup":{` + tc.setup + `},"steps":[` + tc.steps + `]}`
			res, err := RunOracleScenarioJSON(reg, []byte(sc))
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("scenario failed: %v", res.Fails)
			}
			var snap *OracleSnapshot
			for i := range res.Snapshots {
				if res.Snapshots[i].Checkpoint == tc.checkpoint {
					snap = &res.Snapshots[i]
				}
			}
			if snap == nil {
				t.Fatalf("no %q snapshot", tc.checkpoint)
			}
			// Precondition: the conspire grant is live (Raiding Schemes on
			// p0's battlefield) and p0 has two untapped green creatures, so
			// the plain cast is a real decline of an affordable conspire.
			schemes, green := false, 0
			for _, p := range snap.Permanents {
				if p.Controller != 0 {
					continue
				}
				if p.Name == "Raiding Schemes" {
					schemes = true
				}
				if p.PT != "" && !p.Tapped && strings.ContainsRune(p.Colors, 'G') {
					green++
				}
			}
			if !schemes || green < 2 {
				t.Fatalf("precondition: Raiding Schemes=%v, untapped green creatures=%d (want true, >=2)", schemes, green)
			}
			var got []string
			for _, s := range snap.Stack {
				got = append(got, s.Kind+":"+s.Source)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("stack at %s = %v, want %v (Forge and XMage: no Conspire ability for an unpaid conspire)",
					tc.checkpoint, got, tc.want)
			}
		})
	}
}
