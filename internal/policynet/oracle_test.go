package policynet

import (
	"bytes"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/view"
)

// An oracle (diagnostic feature set) value model round-trips through its own
// writer/loader pair only: the ordinary loader refuses the file, the oracle
// loader refuses an ordinary one, and the oracle writer refuses a redacted
// model or one without a value head.
func TestOracleCheckpointIsItsOwnPath(t *testing.T) {
	m := fullModel()
	m.Features = FeaturesMZOppHand
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err == nil {
		t.Fatal("WriteCheckpoint accepted a diagnostic model")
	}
	buf.Reset()
	if err := WriteOracleCheckpoint(m, &buf); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCheckpoint(bytes.NewReader(buf.Bytes())); err == nil || !strings.Contains(err.Error(), "LoadOracleCheckpoint") {
		t.Fatalf("LoadCheckpoint on an oracle checkpoint: %v", err)
	}
	m2, err := LoadOracleCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if m2.Features != FeaturesMZOppHand || !m2.HasValue() {
		t.Fatalf("loaded %s value=%v", m2.Features, m2.HasValue())
	}
	st := EncodeStateWith(FeaturesMZOppHand, view.View{Turn: 2, Players: []view.PlayerView{{ID: 0, Life: 20}, {ID: 1, Life: 7}}}, 0,
		&Diag{OppHand: []view.CardView{{Name: "Counterspell", Types: "Instant"}}})
	if a, b := m.Value(st), m2.Value(st); a != b {
		t.Fatalf("value %g after round trip, want %g", b, a)
	}

	plain := fullModel()
	buf.Reset()
	if err := WriteOracleCheckpoint(plain, &buf); err == nil {
		t.Fatal("WriteOracleCheckpoint accepted a redacted model")
	}
	if err := WriteCheckpoint(plain, &buf); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOracleCheckpoint(bytes.NewReader(buf.Bytes())); err == nil || !strings.Contains(err.Error(), "not an oracle") {
		t.Fatalf("LoadOracleCheckpoint on an ordinary checkpoint: %v", err)
	}
	noValue := NewModel(TableRows, 8, 4, rand.New(rand.NewPCG(3, 4)))
	noValue.Features = FeaturesMZOppHand
	if err := WriteOracleCheckpoint(noValue, &buf); err == nil {
		t.Fatal("WriteOracleCheckpoint accepted a model without a value head")
	}
}

// SplitOmniscientView gives back a training record's shape: the seat's own
// view (other hands removed, the seat's own kept) and the opponents' hands, in
// player order, as the Diag -- and it encodes exactly as that record would.
func TestSplitOmniscientViewIsTheTrainingRecord(t *testing.T) {
	a := view.CardView{Name: "Counterspell", Types: "Instant", ManaCost: "U U"}
	b := view.CardView{Name: "Grizzly Bears", Types: "Creature", ManaCost: "1 G", Power: 2, Toughness: 2}
	c := view.CardView{Name: "Island", Types: "Basic Land Island"}
	own := view.CardView{Name: "Lightning Bolt", Types: "Instant", ManaCost: "R"}
	omni := view.View{Turn: 4, Players: []view.PlayerView{
		{ID: 0, Life: 18, HandSize: 1, Hand: []view.CardView{a}},
		{ID: 1, Life: 20, HandSize: 1, Hand: []view.CardView{own}},
		{ID: 2, Life: 11, HandSize: 2, Hand: []view.CardView{b, c}},
	}}
	before := omni.Players[0].Hand
	got, diag := SplitOmniscientView(omni, 1)
	if omni.Players[0].Hand == nil || &omni.Players[0].Hand[0] != &before[0] {
		t.Fatal("SplitOmniscientView modified its argument")
	}
	seat := view.View{Turn: 4, Players: []view.PlayerView{
		{ID: 0, Life: 18, HandSize: 1},
		{ID: 1, Life: 20, HandSize: 1, Hand: []view.CardView{own}},
		{ID: 2, Life: 11, HandSize: 2},
	}}
	if !reflect.DeepEqual(got, seat) {
		t.Fatalf("own view %+v, want %+v", got, seat)
	}
	if want := []view.CardView{a, b, c}; !reflect.DeepEqual(diag.OppHand, want) || diag.OppLibraryTop != nil || diag.OwnLibraryTop != nil {
		t.Fatalf("diag %+v", diag)
	}
	if !reflect.DeepEqual(EncodeStateWith(FeaturesMZOppHand, got, 1, diag),
		EncodeStateWith(FeaturesMZOppHand, seat, 1, &Diag{OppHand: []view.CardView{a, b, c}})) {
		t.Fatal("split view does not encode as the training record")
	}
}
