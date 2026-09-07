package cards

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestManaProductionIndeterminateFlag pins the bl1 distinction between the
// two honesty flags on a source that mixes a known and an unknown amount:
//
//   - Indeterminate is set when at least one mana ability's Amount$ cannot be
//     statically priced (here the Amount$ X green ability);
//   - a KNOWN amount ability on the same source (the Amount$ 1 blue ability)
//     still contributes its guaranteed colour, so a mixed source is both
//     flagged and a real producer -- a policy that refuses all indeterminate
//     sources would wrongly drop the blue it can count on.
//
// It also pins that the indeterminate green contribution is zero, so the
// source's guaranteed-production counts (ProducesColour, DistinctColours)
// are exactly the known abilities' -- nothing claimed the pool will not get.
func TestManaProductionIndeterminateFlag(t *testing.T) {
	mp := mpOf(t, "Name:ManaBug2\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ G | Amount$ X | Oracle:x\nA:AB$ Mana | Cost$ T | Produced$ U | Amount$ 1 | Oracle:x\n")
	if !mp.Indeterminate {
		t.Error("Amount$ X ability must set Indeterminate")
	}
	if mp.Colour[1] != 1 {
		t.Errorf("known blue ability should contribute 1 blue; got %v", mp.Colour)
	}
	if mp.Colour[4] != 0 {
		t.Errorf("green from Amount$ X must not be claimed; got %v", mp.Colour)
	}
	if !mp.ProducesColour(1) || mp.ProducesColour(4) {
		t.Errorf("mixed source must produce blue but not green; got %v", mp.Colour)
	}
}

// TestManaProductionJSONOmitsIndeterminate pins that the Indeterminate flag
// is a server-only field and must not ride the human wire: the web client
// consumes nothing in ManaProduction, so surfacing it as
// `indeterminate?: boolean` in protocol.ts is dead payload on every card
// view that projects a Produces (view/view.go). json:"-" keeps it out of
// tsgen's jsonName. The Go field is kept as the signal a future policy
// tier needs (the batched "is any ability on this source
// indeterminate" fact), it is off the human wire behind json:"-", and
// today it is written once (cards/mana_production.go) and read only by
// tests -- no production Go reads it: chooseTap keys on prod.Colour[i] > 0
// and both adapters copy the struct wholesale, so the field rides along
// unread. The JSON of a projectable ManaProduction must carry colour and
// any and never "indeterminate".
func TestManaProductionJSONOmitsIndeterminate(t *testing.T) {
	mp := ManaProduction{Colour: [6]int32{0, 1, 0, 0, 0, 0}, Indeterminate: true}
	blob, err := json.Marshal(mp)
	if err != nil {
		t.Fatal(err)
	}
	s := string(blob)
	if strings.Contains(s, "indeterminate") {
		t.Fatalf("marshalled ManaProduction leaked the server-only Indeterminate flag: %s", s)
	}
	for _, want := range []string{`"colour":`, `"any":false`} {
		if !strings.Contains(s, want) {
			t.Fatalf("marshalled ManaProduction missing %s: %s", want, s)
		}
	}
}
