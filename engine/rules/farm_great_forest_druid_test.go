package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmGreatForestDruid pins the ECL farm windows ECL-s208 (w13) and
// ECL-s266 (w11), class 3way:FX|G, field stack, attributed to Great Forest
// Druid and Sapling Nursery. Under Raiding Schemes every noncreature spell
// its controller casts has conspire, and the farm's gorge put a Conspire
// ability above Sapling Nursery even though the cast never paid the conspire
// tap (Forge and XMage put only the spell, plus Enraged Flamecaster's own
// cast trigger in s208). Conspire's trigger has an intervening "if" ("if its
// conspire cost was paid", CR 702.78a, 603.4), so a declined conspire never
// triggers; the Druid in each board is an untapped green creature that the
// bot left untapped, which is why the diverging step names it. The boards
// are the two windows' setups, replayed from their farm inputs.
func TestFarmGreatForestDruid(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const cast = `{"op":"cast","seat":0,"card":"p0:Sapling Nursery"}`
	act := func(refs ...string) string {
		var b strings.Builder
		for _, r := range refs {
			b.WriteString(`{"op":"activate","seat":0,"card":"p0:` + r + `","ability_index":0},`)
		}
		return b.String()
	}
	const p1 = `"p1":{"battlefield":["Island","Plains"],"life":20}`
	cases := []struct {
		name, p0, steps string
		wantGreen       int
		want            []string
	}{
		{
			// Vinebred Brawler and the Druid are two untapped green creatures,
			// so the conspire tap was affordable and declined.
			name: "ECL-s266",
			p0: `"p0":{"battlefield":["Forest","Forest","Great Forest Druid","Forest","Vinebred Brawler","Mountain",` +
				`"Flamekin Gildweaver","Raiding Schemes","Forest"],"hand":["Sapling Nursery","Forest","Mountain"],"life":18}`,
			steps:     act("Forest", "Forest#2", "Forest#3", "Forest#4") + cast,
			wantGreen: 2,
			want:      []string{"spell:p0:Sapling Nursery"},
		},
		{
			// The Flamecaster is red: only the Druid shares green with the
			// spell. Its own cast trigger (mana value 8 >= 4) is the lone
			// ability, listed above the spell.
			name: "ECL-s208",
			p0: `"p0":{"battlefield":["Forest","Forest","Great Forest Druid","Enraged Flamecaster","Forest","Mountain",` +
				`"Raiding Schemes","Forest"],"hand":["Squawkroaster","Sapling Nursery","Cinder Strike","Forest"],"life":20}`,
			steps:     act("Forest#2", "Forest#3", "Mountain", "Forest#4") + cast,
			wantGreen: 1,
			want:      []string{"ability:p0:Enraged Flamecaster", "spell:p0:Sapling Nursery"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sc := `{"name":"farm great forest druid ` + tc.name + `","turn":1,"cr":["702.78a","603.4"],"why":"inline",` +
				`"setup":{` + tc.p0 + `,` + p1 + `},"steps":[` + tc.steps + `]}`
			res, err := RunOracleScenarioJSON(reg, []byte(sc))
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("scenario failed: %v", res.Fails)
			}
			var snap *OracleSnapshot
			for i := range res.Snapshots {
				if strings.HasSuffix(res.Snapshots[i].Checkpoint, "(cast)") {
					snap = &res.Snapshots[i]
				}
			}
			if snap == nil {
				t.Fatal("no cast snapshot")
			}
			// Precondition: the conspire grant is live and the untapped green
			// creatures are as the window had them, so the plain cast really
			// is a decline (s266) or an unaffordable conspire (s208).
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
			if !schemes || green != tc.wantGreen {
				t.Fatalf("precondition: Raiding Schemes=%v, untapped green creatures=%d (want true, %d)", schemes, green, tc.wantGreen)
			}
			var got []string
			for _, s := range snap.Stack {
				got = append(got, s.Kind+":"+s.Source)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("stack = %v, want %v (Forge and XMage: no Conspire ability for an unpaid conspire)", got, tc.want)
			}
		})
	}
}
