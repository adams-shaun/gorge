package neutral_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// cardByProtocolID finds the CardView the builder projected for a protocol
// object id.
func cardByProtocolID(b *neutral.Builder, v view.View, id string) (view.CardView, bool) {
	for _, p := range v.Players {
		for _, zone := range [][]view.CardView{p.Hand, p.Battlefield, p.Graveyard, p.Exile, p.Command} {
			for _, cv := range zone {
				if b.IDs.V2(cv.ID) == id {
					return cv, true
				}
			}
		}
	}
	return view.CardView{}, false
}

// cloneObservation deep-copies an observation through its wire form, so a
// test can mutate one record without touching the recorded corpus.
func cloneObservation(t *testing.T, src *protocol.Observation) *protocol.Observation {
	t.Helper()
	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatal(err)
	}
	var o protocol.Observation
	if err := json.Unmarshal(raw, &o); err != nil {
		t.Fatal(err)
	}
	return &o
}

// firstPermanent returns the object id and permanent of the first battlefield
// record in o.
func firstPermanent(o *protocol.Observation) (string, *protocol.Permanent, bool) {
	for pi := range o.Players {
		for ri := range o.Players[pi].Battlefield {
			r := &o.Players[pi].Battlefield[ri]
			if r.Permanent != nil {
				return r.ObjectID, r.Permanent, true
			}
		}
	}
	return "", nil, false
}

// TestBuilderViewIncrementalEqualsFresh replays every decision of the corpus
// the adapter benchmark uses through one incremental builder and a fresh
// builder per decision: the two views must be deep-equal, so reuse never
// leaks a stale card. It also mutates one record between two decisions and
// requires that card to be rebuilt rather than reused.
func TestBuilderViewIncrementalEqualsFresh(t *testing.T) {
	reg := testcorpus.Registry(t)
	streams := v2Corpus(t, reg, 2)
	decisions := 0
	for si, str := range streams {
		inc := neutral.NewBuilder(reg)
		for k := range str.steps {
			obs := &str.steps[k].sd.Observation
			got, err := inc.View(obs)
			if err != nil {
				t.Fatalf("seat %d step %d: incremental view: %v", si, k, err)
			}
			fresh := neutral.NewBuilder(reg)
			neutral.CopyIDsForTest(fresh.IDs, inc.IDs)
			freshView, err := fresh.View(obs)
			if err != nil {
				t.Fatalf("seat %d step %d: fresh view: %v", si, k, err)
			}
			if !reflect.DeepEqual(got, freshView) {
				t.Fatalf("seat %d step %d: incremental view differs from a fresh build", si, k)
			}
			decisions++
		}
		if ids := neutral.CachedObjectIDsForTest(inc); len(ids) == 0 {
			t.Fatalf("seat %d: the cache is empty after %d decisions; no reuse happened", si, len(str.steps))
		}
	}
	if decisions == 0 {
		t.Fatal("the corpus recorded no decisions")
	}

	// A card whose record changed must be rebuilt. Use a real corpus
	// observation and flip one permanent field between two Views.
	var obs *protocol.Observation
	var pid string
	var perm *protocol.Permanent
	for _, str := range streams {
		for k := range str.steps {
			cand := cloneObservation(t, &str.steps[k].sd.Observation)
			if id, p, ok := firstPermanent(cand); ok {
				obs, pid, perm = cand, id, p
				break
			}
		}
		if obs != nil {
			break
		}
	}
	if obs == nil {
		t.Fatal("the corpus has no observation with a battlefield permanent")
	}
	inc := neutral.NewBuilder(reg)
	first, err := inc.View(obs)
	if err != nil {
		t.Fatal(err)
	}
	before, ok := cardByProtocolID(inc, first, pid)
	if !ok {
		t.Fatalf("first view has no card for %s", pid)
	}
	perm.Tapped = !perm.Tapped
	second, err := inc.View(obs)
	if err != nil {
		t.Fatal(err)
	}
	fresh := neutral.NewBuilder(reg)
	neutral.CopyIDsForTest(fresh.IDs, inc.IDs)
	freshView, err := fresh.View(obs)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, freshView) {
		t.Fatalf("after mutating %s, the incremental view differs from a fresh build", pid)
	}
	after, ok := cardByProtocolID(inc, second, pid)
	if !ok {
		t.Fatalf("second view has no card for %s", pid)
	}
	if after.Tapped != perm.Tapped {
		t.Fatalf("%s: rebuilt card has tapped=%v, want %v", pid, after.Tapped, perm.Tapped)
	}
	if before.Tapped == after.Tapped {
		t.Fatalf("%s: the cached card was reused after its record changed (tapped stayed %v)", pid, after.Tapped)
	}
}

// TestBuilderViewCacheDropsStaleObjects checks that an object which leaves the
// observation leaves the cache too. It also proves reuse by requiring an
// unchanged land to keep the same built Mana pointer in the next view.
func TestBuilderViewCacheDropsStaleObjects(t *testing.T) {
	var o protocol.Observation
	if err := json.Unmarshal([]byte(kernelObs), &o); err != nil {
		t.Fatal(err)
	}
	inc := neutral.NewBuilder(testcorpus.Registry(t))
	first, err := inc.View(&o)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := neutral.CachedObjectIDsForTest(inc), []string{"o-bolt", "o-epi", "o-mtn1", "o-mtn3"}; !slices.Equal(got, want) {
		t.Fatalf("cache keys after the first view: %v, want %v", got, want)
	}
	mountain, ok := cardByProtocolID(inc, first, "o-mtn1")
	if !ok {
		t.Fatal("first view has no Mountain")
	}
	if mountain.Produces == nil {
		t.Fatal("the Mountain has no printed mana production; the reuse check has nothing to compare")
	}
	second, err := inc.View(&o)
	if err != nil {
		t.Fatal(err)
	}
	again, ok := cardByProtocolID(inc, second, "o-mtn1")
	if !ok {
		t.Fatal("second view has no Mountain")
	}
	if again.Produces != mountain.Produces {
		t.Fatal("the second view rebuilt the unchanged Mountain instead of reusing its cached card")
	}

	// The opponent's only permanent leaves. Its cache entry must go with it.
	o.Players[1].Battlefield = nil
	if _, err := inc.View(&o); err != nil {
		t.Fatal(err)
	}
	if got, want := neutral.CachedObjectIDsForTest(inc), []string{"o-bolt", "o-mtn1", "o-mtn3"}; !slices.Equal(got, want) {
		t.Fatalf("cache keys after o-epi left the observation: %v, want %v", got, want)
	}
}

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
