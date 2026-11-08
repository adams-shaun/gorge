package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmGoatnap pins the ECL farm windows (ECL-s602/s633, 3way:FX|G, field
// stack) where gorge cast Goatnap under Raiding Schemes' conspire grant
// without paying conspire and still put a Conspire ability above the spell,
// while Forge and XMage put only the spell. CR 702.78a's trigger has an
// intervening "if" ("if its conspire cost was paid", CR 603.4), so a declined
// conspire never triggers.
func TestFarmGoatnap(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"farm goatnap","turn":1,"cr":["702.78a","603.4"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Mountain","Mountain","Mountain","Raiding Schemes",` +
		`"Enraged Flamecaster","Sizzling Changeling","Bristlebane Outrider"],"hand":["Goatnap"],"life":10},` +
		`"p1":{"battlefield":["Plains","Thoughtweft Imbuer"],"life":10}},"steps":[` +
		`{"op":"activate","seat":0,"card":"p0:Mountain","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Mountain#2","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Mountain#3","ability_index":0},` +
		`{"op":"cast","seat":0,"card":"p0:Goatnap","targets":["p1:Thoughtweft Imbuer"]}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	const checkpoint = "step 3 (cast)"
	var snap *OracleSnapshot
	for i := range res.Snapshots {
		if res.Snapshots[i].Checkpoint == checkpoint {
			snap = &res.Snapshots[i]
		}
	}
	if snap == nil {
		t.Fatalf("no %q snapshot", checkpoint)
	}
	// Precondition: the grant is live and p0 has two untapped red creatures,
	// so the plain cast is a real decline of an affordable conspire.
	schemes, red := false, 0
	for _, p := range snap.Permanents {
		if p.Controller != 0 {
			continue
		}
		if p.Name == "Raiding Schemes" {
			schemes = true
		}
		if p.PT != "" && !p.Tapped && strings.ContainsRune(p.Colors, 'R') {
			red++
		}
	}
	if !schemes || red < 2 {
		t.Fatalf("precondition: Raiding Schemes=%v, untapped red creatures=%d (want true, >=2)", schemes, red)
	}
	var got []string
	for _, s := range snap.Stack {
		got = append(got, s.Kind+":"+s.Source)
	}
	if want := []string{"spell:p0:Goatnap"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("stack at %s = %v, want %v (Forge and XMage: no Conspire ability for an unpaid conspire)", checkpoint, got, want)
	}
}
