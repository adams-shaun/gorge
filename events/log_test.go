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
