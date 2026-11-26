package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOracleSnapshotOfferedGrantedTapManaHonoursSummoningSickness pins CR
// 302.6 on the snapshot's granted-mana half: Cryptolith Rite grants "{T}: Add
// one mana of any color" to every creature its controller has, and a
// creature that came under control this turn cannot pay a {T} cost, so the
// granted ability is not an offered activation. The control row (the same
// Memnite placed at setup, not sick) is offered.
func TestOracleSnapshotOfferedGrantedTapManaHonoursSummoningSickness(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	offered := func(snap OracleSnapshot) bool {
		return slices.ContainsFunc(snap.Offered, func(o OracleSnapOffered) bool {
			return o.Source == "p0:Memnite" && o.Kind == "activate"
		})
	}

	const control = `{"name":"offered-granted-control","cr":["302.6"],"why":"a creature present since the turn began taps for the granted mana","setup":{"p0":{"battlefield":["Cryptolith Rite","Memnite"]}},"steps":[]}`
	if snap := setupOfferedSnap(t, reg, control); !offered(snap) {
		t.Fatalf("control: a non-sick Memnite beside Cryptolith Rite must be offered its granted mana ability: %+v", snap.Offered)
	}

	const sc = `{"name":"offered-granted-sick","cr":["302.6"],"why":"a creature that entered this turn cannot pay a {T} cost","setup":{"p0":{"hand":["Memnite"],"battlefield":["Cryptolith Rite"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Memnite","mana":""},{"op":"resolve","seat":0}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatal(err)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	if !slices.ContainsFunc(last.Permanents, func(p OracleSnapPerm) bool { return p.Ref == "p0:Memnite" && !p.Tapped }) {
		t.Fatalf("precondition: untapped p0:Memnite on the battlefield after the resolve step, permanents = %+v", last.Permanents)
	}
	if offered(last) {
		t.Fatalf("%s offered = %+v, want no p0:Memnite|activate: a summoning-sick creature's granted {T} mana ability is not activatable (CR 302.6)", last.Checkpoint, last.Offered)
	}
}
