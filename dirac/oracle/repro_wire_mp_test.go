//go:build dirac_mp

package oracle

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestReproActionsWireKeepsAFourthSeat: an action no default byte can hold (the
// fourth seat, an instance past 235) is written in the mp packing and marked,
// and reads back whole.
func TestReproActionsWireKeepsAFourthSeat(t *testing.T) {
	as := []Canon{Pack(3, 300, RefPlayer+3, 1), Pack(2, 5, RefNone, 0)}
	b, err := json.Marshal(&Repro{Format: reproFormat, Actions: as})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"packing":"mp"`) {
		t.Fatalf("an unportable action carries no packing marker:\n%s", b)
	}
	var back Repro
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Actions) != len(as) || back.Actions[0] != as[0] || back.Actions[1] != as[1] {
		t.Fatalf("reads back %v, want %v", back.Actions, as)
	}
}
