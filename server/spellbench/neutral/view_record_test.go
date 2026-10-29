package neutral_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestBuilderViewRebuildsAfterInPlaceRecordMutation checks the record key is a
// deep copy: mutating the observation's counters map and Power pointer in
// place between two View calls must rebuild the card, not reuse the cached one.
func TestBuilderViewRebuildsAfterInPlaceRecordMutation(t *testing.T) {
	var o protocol.Observation
	if err := json.Unmarshal([]byte(kernelObs), &o); err != nil {
		t.Fatal(err)
	}
	var rec *protocol.ObjectRecord
	for pi := range o.Players {
		for ri := range o.Players[pi].Battlefield {
			if r := &o.Players[pi].Battlefield[ri]; r.ObjectID == "o-epi" {
				rec = r
			}
		}
	}
	if rec == nil || rec.Permanent == nil || rec.Characteristics == nil || rec.Characteristics.Power == nil {
		t.Fatal("kernelObs lost o-epi's permanent or Power; the mutation check has nothing to change")
	}
	rec.Permanent.Counters = map[string]uint32{"P1P1": 1}
	inc := neutral.NewBuilder(testcorpus.Registry(t))
	first, err := inc.View(&o)
	if err != nil {
		t.Fatal(err)
	}
	before, ok := cardByProtocolID(inc, first, "o-epi")
	if !ok {
		t.Fatal("first view has no o-epi")
	}
	if got := before.Counters["P1P1"]; got != 1 {
		t.Fatalf("first view o-epi counters[P1P1] = %d, want 1", got)
	}
	if before.Power != 1 {
		t.Fatalf("first view o-epi power = %d, want 1", before.Power)
	}

	rec.Permanent.Counters["P1P1"] = 2
	*rec.Characteristics.Power = 2
	second, err := inc.View(&o)
	if err != nil {
		t.Fatal(err)
	}
	after, ok := cardByProtocolID(inc, second, "o-epi")
	if !ok {
		t.Fatal("second view has no o-epi")
	}
	if got := after.Counters["P1P1"]; got != 2 {
		t.Fatalf("cached card kept counters %v after the map changed", after.Counters)
	}
	if after.Power != 2 {
		t.Fatalf("cached card kept power %d after the pointer value changed", after.Power)
	}
	fresh := neutral.NewBuilder(testcorpus.Registry(t))
	neutral.CopyIDsForTest(fresh.IDs, inc.IDs)
	freshView, err := fresh.View(&o)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, freshView) {
		t.Fatal("after mutating o-epi in place, the incremental view differs from a fresh build")
	}
}
