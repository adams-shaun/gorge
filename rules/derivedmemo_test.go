package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestDerivedMemoScopedToOneWalk pins derivedmemo.go's invalidation contract:
// inside one scope a repeated Derived is served from the memo with owned
// slices (a Derived of another object does not rewrite them), and a NEW scope
// never serves an entry the previous walk built, even when the board changed
// with no event at all (a direct counter write, as tests do).
func TestDerivedMemoScopedToOneWalk(t *testing.T) {
	e := layerEngine(t)
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nK:Trample\nOracle:x\n")
	angel := onBoard(t, e, 0, "Name:Angel\nManaCost:3 WW\nTypes:Creature Angel\nPT:4/4\nK:Flying\nOracle:x\n")

	e.beginDerivedMemo()
	a := e.Derived(bear)
	if a.Power != 2 || !slices.Equal(a.Keywords, []string{"Trample"}) {
		t.Fatalf("first derive = %+v", a)
	}
	// Another object's derive rewrites the shared scratch; the memoized
	// bear slices must be owned and therefore untouched.
	if kw := e.Derived(angel).Keywords; !slices.Equal(kw, []string{"Flying"}) {
		t.Fatalf("angel keywords = %v", kw)
	}
	if !slices.Equal(a.Keywords, []string{"Trample"}) || !slices.Equal(a.Types, []string{"Creature", "Bear"}) {
		t.Fatalf("memoized bear slices rewritten by another derive: %+v", a)
	}
	b := e.Derived(bear)
	if &b.Keywords[0] != &a.Keywords[0] {
		t.Fatalf("second derive in one walk was not served from the memo")
	}
	e.endDerivedMemo()

	// No event: a direct write. A new walk must still see it.
	e.G.Obj(bear).AddCounter("P1P1", 1)
	e.beginDerivedMemo()
	if p := e.Derived(bear).Power; p != 3 {
		t.Fatalf("new walk served the previous walk's entry: power %d, want 3", p)
	}
	e.endDerivedMemo()
}

// TestDerivedMemoVerifyCatchesStaleness proves verify mode is live in the
// rules test binary (derivedmemo_verify_test.go), so the whole suite really
// does cross-check every memo hit: a no-event write inside one walk -- the
// thing the walk contract forbids -- must trip it.
func TestDerivedMemoVerifyCatchesStaleness(t *testing.T) {
	if !derivedMemoVerify {
		t.Fatal("verify mode is off in the rules test binary")
	}
	e := layerEngine(t)
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	_ = e.Derived(bear)
	e.G.Obj(bear).AddCounter("P1P1", 1)
	defer func() {
		r := recover()
		if s, ok := r.(string); !ok || !strings.Contains(s, "derived memo stale") {
			t.Fatalf("verify did not flag the stale hit: %v", r)
		}
	}()
	_ = e.Derived(bear)
}

// TestDerivedMemoBypassesZoneOverride pins that the convoke zone-override
// read (atStack != 0) never touches the memo.
func TestDerivedMemoBypassesZoneOverride(t *testing.T) {
	e := layerEngine(t)
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	_ = e.derivedWith(bear, state.ZStack)
	if len(e.derivedMemo) > int(bear) && e.derivedMemo[bear].gen != 0 {
		t.Fatal("an atStack derive was memoized")
	}
}
