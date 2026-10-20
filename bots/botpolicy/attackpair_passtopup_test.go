package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestClampPassTopUpKeepsOneDefenderPerAttacker pins the review finding that
// Clamp's pass-option top-up loop applies the KAttackers one-defender rule as
// well as the ordinary loop: two pass options for the same attacker Obj must
// not both be added, or Clamp returns an intent Decision.Validate rejects.
func TestClampPassTopUpKeepsOneDefenderPerAttacker(t *testing.T) {
	d := decision.Decision{
		Seq: 1, Player: 0, Kind: decision.KAttackers, Min: 2, Max: 2,
		Options: []decision.Option{
			{Index: 0, Kind: "pass", Obj: state.ObjID(38)},
			{Index: 1, Kind: "pass", Obj: state.ObjID(38)},
			{Index: 2, Kind: "pass", Obj: state.ObjID(39)},
		},
	}
	in := Clamp(&d, decision.Intent{Seq: 1, Player: 0})
	if err := d.Validate(in); err != nil {
		t.Fatalf("Clamp top-up returned an intent Validate rejects: %v (choices %v)", err, in.Choices)
	}
}
