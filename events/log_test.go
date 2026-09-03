package events

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestEncodingIsDeterministicAndDiscriminating(t *testing.T) {
	e := Event{Seq: 3, Kind: Damage, Player: 1, Obj: 9, Amount: 3, Text: "bolt"}
	a := e.Append(nil)
	b := e.Append(nil)
	if string(a) != string(b) {
		t.Fatal("encoding is not deterministic")
	}
	for _, mut := range []Event{
		{Seq: 4, Kind: Damage, Player: 1, Obj: 9, Amount: 3, Text: "bolt"},
		{Seq: 3, Kind: LifeChange, Player: 1, Obj: 9, Amount: 3, Text: "bolt"},
		{Seq: 3, Kind: Damage, Player: 2, Obj: 9, Amount: 3, Text: "bolt"},
		{Seq: 3, Kind: Damage, Player: 1, Obj: 10, Amount: 3, Text: "bolt"},
		{Seq: 3, Kind: Damage, Player: 1, Obj: 9, Amount: 4, Text: "bolt"},
		{Seq: 3, Kind: Damage, Player: 1, Obj: 9, Amount: 3, Text: "shock"},
	} {
		if string(mut.Append(nil)) == string(a) {
			t.Fatalf("encoding collides for %+v", mut)
		}
	}
}

func TestEncodingCoversSliceFields(t *testing.T) {
	base := Event{Kind: DeclareAttackers, IDs: []state.ObjID{1, 2}}
	other := Event{Kind: DeclareAttackers, IDs: []state.ObjID{2, 1}}
	if string(base.Append(nil)) == string(other.Append(nil)) {
		t.Fatal("IDs order must affect the encoding")
	}
	p1 := Event{Kind: DeclareBlockers, Pairs: [][2]state.ObjID{{1, 2}}}
	p2 := Event{Kind: DeclareBlockers, Pairs: [][2]state.ObjID{{2, 1}}}
	if string(p1.Append(nil)) == string(p2.Append(nil)) {
		t.Fatal("Pairs order must affect the encoding")
	}
}

func TestLogAssignsSequenceAndChains(t *testing.T) {
	l := NewLog(42)
	for i := 0; i < 5; i++ {
		got := l.Append(Event{Kind: Draw, Player: state.PlayerID(i % 2)})
		if got.Seq != uint64(i) {
			t.Fatalf("event %d got seq %d", i, got.Seq)
		}
	}
	if len(l.Events) != 5 {
		t.Fatalf("events = %d", len(l.Events))
	}
	head := l.Head()
	if head == "" || head == l.HeadAt(4) {
		t.Fatal("Head must cover every event and differ from a prefix")
	}
	if l.HeadAt(5) != head {
		t.Fatal("HeadAt(len) must equal Head")
	}
	if l.HeadAt(0) == l.HeadAt(1) {
		t.Fatal("HeadAt must advance")
	}
}

func TestIdenticalEventStreamsChainIdentically(t *testing.T) {
	build := func() *Log {
		l := NewLog(7)
		l.Append(Event{Kind: Draw, Player: 0, Obj: 1, Secret: true})
		l.Append(Event{Kind: Damage, Player: 1, Amount: 3})
		l.Append(Event{Kind: StepChange, Step: state.StepMain1})
		return l
	}
	if build().Head() != build().Head() {
		t.Fatal("identical streams produced different chains")
	}
	other := build()
	other.Append(Event{Kind: Draw, Player: 0})
	if other.Head() == build().Head() {
		t.Fatal("appending an event did not change the chain")
	}
}

func TestNoHashSkipsChainButKeepsEvents(t *testing.T) {
	l := NewLog(1)
	l.NoHash = true
	l.Append(Event{Kind: Draw})
	if len(l.Events) != 1 {
		t.Fatal("NoHash dropped events")
	}
	if l.Head() != "" {
		t.Fatal("NoHash must report an empty head, not a stale one")
	}
}

// FIX 1: Test that IDs slices are copied and not aliased
func TestIDsSliceAliasing(t *testing.T) {
	l := NewLog(42)
	originalIDs := []state.ObjID{1, 2, 3}
	e := Event{Kind: DeclareAttackers, IDs: originalIDs}
	l.Append(e)
	head1 := l.Head()

	// Mutate the caller's slice
	originalIDs[0] = 99

	// Verify that the stored event wasn't affected
	head2 := l.HeadAt(len(l.Events))
	if head1 != head2 {
		t.Fatalf("IDs aliasing: Head() %s != HeadAt(1) %s after mutation", head1, head2)
	}
}

// FIX 1: Test that Pairs slices are copied and not aliased
func TestPairsSliceAliasing(t *testing.T) {
	l := NewLog(42)
	originalPairs := [][2]state.ObjID{{1, 2}, {3, 4}}
	e := Event{Kind: DeclareBlockers, Pairs: originalPairs}
	l.Append(e)
	head1 := l.Head()

	// Mutate the caller's slice
	originalPairs[0][0] = 99

	// Verify that the stored event wasn't affected
	head2 := l.HeadAt(len(l.Events))
	if head1 != head2 {
		t.Fatalf("Pairs aliasing: Head() %s != HeadAt(1) %s after mutation", head1, head2)
	}
}

// FIX 1: Test that nil slices remain nil after copy
func TestNilSlicePreservation(t *testing.T) {
	l := NewLog(42)
	e := Event{Kind: Draw, IDs: nil, Pairs: nil}
	l.Append(e)

	if l.Events[0].IDs != nil {
		t.Fatal("nil IDs slice not preserved")
	}
	if l.Events[0].Pairs != nil {
		t.Fatal("nil Pairs slice not preserved")
	}
}

// FIX 2: Test that NoHash immutability is enforced
func TestNoHashImmutability(t *testing.T) {
	// Setting NoHash before first append should work
	l := NewLog(42)
	l.NoHash = true
	l.Append(Event{Kind: Draw})
	// This should not panic

	// Toggling NoHash after first append should panic
	l2 := NewLog(42)
	l2.NoHash = false
	l2.Append(Event{Kind: Draw})
	l2.NoHash = true

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("toggling NoHash after first append did not panic")
		} else if r.(string) != "events: NoHash changed after the log was started" {
			t.Fatalf("unexpected panic message: %v", r)
		}
	}()
	l2.Append(Event{Kind: Damage})
}

// FIX 3: Test that seed affects the chain
func TestSeedSensitivity(t *testing.T) {
	build := func(seed uint64) *Log {
		l := NewLog(seed)
		l.Append(Event{Kind: Draw})
		l.Append(Event{Kind: Damage})
		return l
	}

	l1 := build(42)
	l2 := build(43)

	if l1.Head() == l2.Head() {
		t.Fatal("different seeds produced identical chain heads")
	}
}

// FIX 3: Test that same seed and events produce same chain
func TestSeedConsistency(t *testing.T) {
	build := func() *Log {
		l := NewLog(42)
		l.Append(Event{Kind: Draw})
		l.Append(Event{Kind: Damage})
		return l
	}

	l1 := build()
	l2 := build()

	if l1.Head() != l2.Head() {
		t.Fatal("identical logs with same seed produced different heads")
	}
}

// FIX 3: Test that Head() and HeadAt(len) agree with seeded chain
func TestHeadHeadAtAgreement(t *testing.T) {
	l := NewLog(42)
	l.Append(Event{Kind: Draw})
	l.Append(Event{Kind: Damage})
	l.Append(Event{Kind: Tap})

	head := l.Head()
	headAt := l.HeadAt(len(l.Events))

	if head != headAt {
		t.Fatalf("Head() %s != HeadAt(%d) %s", head, len(l.Events), headAt)
	}
}
