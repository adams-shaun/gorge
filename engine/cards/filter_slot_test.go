package cards

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// fakeFilters is the test binder's fixed answer per text; "bad" is refused.
var (
	fakeFilters = map[string]*ir.Filter{
		"Creature.YouCtrl": {Subject: ir.SubjPermanent, Word: "Creature"},
		"Card.Other":       {Subject: ir.SubjCard, Word: "Other"},
		"Land":             {Subject: ir.SubjPermanent, Word: "Land"},
		"Artifact":         {Subject: ir.SubjPermanent, Word: "Artifact"},
		"Enchantment":      {Subject: ir.SubjPermanent, Word: "Enchantment"},
	}
	fakeBinderCalls atomic.Int64
)

const fakeBinderOwner = "cards.filter_slot_test"

func fakeBinder(text string) (*ir.Filter, string) {
	fakeBinderCalls.Add(1)
	if f, ok := fakeFilters[text]; ok {
		return f, ""
	}
	return nil, "fake: unread " + text
}

func init() { RegisterFilterBinder(fakeBinderOwner, fakeBinder) }

func boundSA(t *testing.T, line string) *SA {
	t.Helper()
	sa, diags := parseSA("test", line)
	if sa == nil || len(diags) != 0 {
		t.Fatalf("parseSA(%q): %v", line, diags)
	}
	sa.ps = newParamSet(sa.Params)
	if !sa.ps.bound(sa.Params) {
		t.Fatal("precondition: the ability's ParamSet is not bound")
	}
	return sa
}

// TestFilterSlotsBound pins FilterOf on bound, unbound, copied and shared
// nodes against the test binary's fake binder.
func TestFilterSlotsBound(t *testing.T) {
	sa := boundSA(t, "SP$ Destroy | ValidTgts$ Creature.YouCtrl | Defined$ Targeted | ValidCards$ bad")

	// Bound read, cached pointer.
	n := fakeBinderCalls.Load()
	f, ok := sa.FilterOf(PKValidTgts)
	if !ok || f != fakeFilters["Creature.YouCtrl"] {
		t.Fatalf("FilterOf(ValidTgts) = (%p,%v), want the fake's filter", f, ok)
	}
	if got := fakeBinderCalls.Load(); got != n+1 {
		t.Fatalf("first read: binder calls rose by %d, want 1", got-n)
	}
	f2, ok2 := sa.FilterOf(PKValidTgts)
	if !ok2 || f2 != f {
		t.Fatalf("second read = (%p,%v), want the same pointer %p", f2, ok2, f)
	}
	if got := fakeBinderCalls.Load(); got != n+1 {
		t.Fatalf("second read called the binder (calls rose by %d)", got-n)
	}

	// Absent key and non-filter key: no binder call.
	n = fakeBinderCalls.Load()
	if f, ok := sa.FilterOf(PKValidTarget); f != nil || ok {
		t.Fatal("absent filter key answered")
	}
	if !sa.HasParam(PKDefined) || IsFilterKey(PKDefined) {
		t.Fatal("precondition: Defined$ is present and not a filter key")
	}
	if f, ok := sa.FilterOf(PKDefined); f != nil || ok {
		t.Fatal("non-filter key answered")
	}
	if got := fakeBinderCalls.Load(); got != n {
		t.Fatalf("absent / non-filter reads called the binder %d times", got-n)
	}

	// Refused text: absent, and cached.
	n = fakeBinderCalls.Load()
	for range 2 {
		if f, ok := sa.FilterOf(PKValidCards); f != nil || ok {
			t.Fatal("refused text answered a filter")
		}
	}
	if got := fakeBinderCalls.Load(); got != n+1 {
		t.Fatalf("refused text: %d binder calls for two reads, want 1", got-n)
	}

	// A copy with a replaced Params map reads its own text.
	cp := *sa
	cp.Params = map[string]string{"ValidTgts": "Land"}
	if f, ok := cp.FilterOf(PKValidTgts); !ok || f != fakeFilters["Land"] {
		t.Fatalf("replaced map: got %p, want Land's filter", f)
	}
	if f, _ := sa.FilterOf(PKValidTgts); f != fakeFilters["Creature.YouCtrl"] {
		t.Fatal("original lost its own entry after the copy's read")
	}
	// A copy whose bound set is shared but whose map was resized falls back too.
	cp2 := *sa
	cp2.Params = map[string]string{"ValidTgts": "Land", "Defined": "Targeted", "ValidCards": "bad"}
	if cp2.ps.bound(cp2.Params) {
		t.Fatal("precondition: the copy's new map must not be bound")
	}
	if f, ok := cp2.FilterOf(PKValidTgts); !ok || f != fakeFilters["Land"] {
		t.Fatal("same-size replaced map answered from the original's cache")
	}

	// Trigger, Static and Repl receivers.
	tr, ok := ParseTriggerLine("Mode$ ChangesZone | ValidCard$ Card.Other | Destination$ Battlefield")
	if !ok {
		t.Fatal("trigger did not parse")
	}
	tr.ps = newParamSet(tr.Params)
	if f, ok := tr.FilterOf(PKValidCard); !ok || f != fakeFilters["Card.Other"] {
		t.Fatal("Trigger.FilterOf")
	}
	st, ok := ParseStaticLine("Mode$ Continuous | Affected$ Artifact | AddPower$ 1")
	if !ok {
		t.Fatal("static did not parse")
	}
	st.ps = newParamSet(st.Params)
	if f, ok := st.FilterOf(PKAffected); !ok || f != fakeFilters["Artifact"] {
		t.Fatal("Static.FilterOf")
	}
	rp, ok := ParseReplacementLine("Event$ Moved | ValidCard$ Enchantment | Destination$ Graveyard")
	if !ok {
		t.Fatal("replacement did not parse")
	}
	rp.ps = newParamSet(rp.Params)
	if f, ok := rp.FilterOf(PKValidCard); !ok || f != fakeFilters["Enchantment"] {
		t.Fatal("Repl.FilterOf")
	}

	// Two goroutines on a fresh node agree on the pointer.
	fresh := boundSA(t, "SP$ Destroy | ValidTgts$ Creature.YouCtrl")
	var wg sync.WaitGroup
	got := make([]*ir.Filter, 2)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], _ = fresh.FilterOf(PKValidTgts)
		}()
	}
	wg.Wait()
	if got[0] == nil || got[0] != got[1] {
		t.Fatalf("goroutines disagree: %p vs %p", got[0], got[1])
	}
}

// TestFilterBinderSealed: a registration after a read panics, and so does a
// second owner.
func TestFilterBinderSealed(t *testing.T) {
	mustPanic := func(what, want string, f func()) {
		t.Helper()
		defer func() {
			r := recover()
			s, _ := r.(string)
			if r == nil || !strings.Contains(s, want) {
				t.Fatalf("%s: recovered %v, want a panic naming %q", what, r, want)
			}
		}()
		f()
	}
	// Before sealing is observable, a second owner panics on ownership.
	// (filterBinderSealed may already be set by TestFilterSlotsBound; reset
	// it for this case and restore it after.)
	was := filterBinderSealed.Load()
	filterBinderSealed.Store(false)
	mustPanic("second owner", "registered by", func() { RegisterFilterBinder("other.owner", fakeBinder) })
	filterBinderSealed.Store(was)

	sa := boundSA(t, "SP$ Destroy | ValidTgts$ Creature.YouCtrl")
	sa.FilterOf(PKValidTgts)
	mustPanic("after a read", "after a FilterOf", func() { RegisterFilterBinder(fakeBinderOwner, fakeBinder) })
}
