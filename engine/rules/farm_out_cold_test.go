package rules

import (
	"fmt"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmOutCold pins the MKM farm windows (MKM-s102 xe:RAND:3:11:w10,
// MKM-s175 xe:RAND:1:3:w14; 3way:FX|G, field p0.graveyard) where p0 cast Out
// Cold targeting Axebane Ferox (Ward—Collect evidence 4) with too little
// graveyard mana value to pay. The ward trigger resolves first and its
// "counter it" does nothing: Out Cold "can't be countered (This includes by
// the ward ability)" (CR 702.21a, CR 101.2). Forge and XMage leave Out Cold on
// the stack, and the active player gets priority (CR 117.3b); gorge had put
// it into the graveyard. The board is the s175 window, trimmed.
func TestFarmOutCold(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"farm out cold MKM-s175","turn":1,"cr":["702.21a","101.2"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Island","Swamp","Island","Swamp"],"hand":["Out Cold"],` +
		`"graveyard":["Candlestick","Furtive Courier"],"life":6},` +
		`"p1":{"battlefield":["Pompous Gadabout","Axebane Ferox"],"life":18}},` +
		`"steps":[{"op":"activate","seat":0,"card":"p0:Island","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Swamp","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Island#2","ability_index":0},` +
		`{"op":"activate","seat":0,"card":"p0:Swamp#2","ability_index":0},` +
		`{"op":"cast","seat":0,"card":"p0:Out Cold","targets":["p1:Axebane Ferox","p1:Pompous Gadabout"]},` +
		`{"op":"pass","seat":0},{"op":"pass","seat":1}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
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
	// Precondition: the ward trigger is above Out Cold after the cast.
	if got, want := stackOf(snap("step 4 (cast)")), []string{"ability:p1:Axebane Ferox", "spell:p0:Out Cold"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("precondition: stack after cast = %v, want %v", got, want)
	}
	after := snap("step 6 (pass)")
	if gy := after.Players[0].Graveyard; slices.Contains(gy, "Out Cold") {
		t.Fatalf("p0 graveyard after the ward trigger resolved = %v; Out Cold can't be countered by ward (Forge and XMage keep it on the stack)", gy)
	}
	if got, want := stackOf(after), []string{"spell:p0:Out Cold"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("stack after the ward trigger resolved = %v, want %v", got, want)
	}
}
