package policynet

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// relSeatView is an n-seat view: seat p has life 10+p, hand size p, and
// seat 0 controls one creature (ObjID 100).
func relSeatView(n int) view.View {
	v := view.View{Step: "declare-attackers"}
	for p := 0; p < n; p++ {
		pv := view.PlayerView{ID: state.PlayerID(p), Life: int32(10 + p), HandSize: p}
		v.Players = append(v.Players, pv)
	}
	v.Players[0].Battlefield = []view.CardView{{ID: 100, Name: "Grizzly Bears", Types: "Creature Bear", Power: 2, Toughness: 2}}
	return v
}

func TestDenseSeatBlocksTurnOrder(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		for viewer := 0; viewer < n; viewer++ {
			v := relSeatView(n)
			if n == 4 {
				v.Players[(viewer+2)%4].Lost = true // a seat that has left keeps its block
			}
			st := EncodeState(v, state.PlayerID(viewer))
			for r := 0; r < maxSeatBlocks; r++ {
				base := denseSeat0 + r*seatBlockWidth
				want := float32(0)
				if r < n {
					want = float32(10 + (viewer+r)%n)
				}
				if got := st.Dense[base]; got != want {
					t.Errorf("%d seats, viewer %d: block %d life %v, want %v (seat %d)", n, viewer, r, got, want, (viewer+r)%n)
				}
			}
		}
	}
	// two seats: the old order (me, then the other seat) exactly
	for viewer := 0; viewer < 2; viewer++ {
		st := EncodeState(relSeatView(2), state.PlayerID(viewer))
		if st.Dense[denseSeat0] != float32(10+viewer) || st.Dense[denseSeat0+seatBlockWidth] != float32(11-viewer) {
			t.Errorf("2 seats, viewer %d: blocks %v %v", viewer, st.Dense[denseSeat0], st.Dense[denseSeat0+seatBlockWidth])
		}
	}
}

func TestOptionNamesDefenderSeat(t *testing.T) {
	v := relSeatView(4)
	for viewer := 0; viewer < 4; viewer++ {
		seat := state.PlayerID(viewer)
		enc := func(o decision.Option) Option {
			return EncodeOption(v, seat, decision.KAttackers, o, 0, 2)
		}
		a := enc(decision.Option{Kind: "attacker", Obj: 100, Player: 2})
		b := enc(decision.Option{Kind: "attacker", Obj: 100, Player: 3})
		if len(a.Dense) != OptionDenseWidth || len(b.Dense) != OptionDenseWidth {
			t.Fatalf("dense widths %d %d", len(a.Dense), len(b.Dense))
		}
		for i := 0; i < OptionDenseWidth; i++ {
			if i >= odSeat0 && i < odSeat0+4 {
				continue
			}
			if a.Dense[i] != b.Dense[i] {
				t.Errorf("viewer %d: dense[%d] %v vs %v outside the seat one-hot", viewer, i, a.Dense[i], b.Dense[i])
			}
		}
		if !equalFeatures(a.Slots, b.Slots) || !equalFeatures(a.Hashed, b.Hashed) {
			t.Errorf("viewer %d: slots or hashed rows differ", viewer)
		}
		for _, c := range []struct {
			o      Option
			target int
		}{{a, 2}, {b, 3}} {
			rel := (c.target - viewer + 4) % 4
			for k := 0; k < 4; k++ {
				want := float32(0)
				if k == rel {
					want = 1
				}
				if got := c.o.Dense[odSeat0+k]; got != want {
					t.Errorf("viewer %d, defender %d: seat column %d = %v, want %v", viewer, c.target, k, got, want)
				}
			}
		}
		// a player target names its seat, by Kind "player" or a player-ref Obj
		for p := 0; p < 4; p++ {
			rel := (p - viewer + 4) % 4
			for _, o := range []decision.Option{
				{Kind: "player", Player: state.PlayerID(p)},
				{Kind: "target", Obj: state.PlayerRef(state.PlayerID(p))},
			} {
				got := EncodeOption(v, seat, decision.KTarget, o, 0, 1)
				if got.Dense[odIsPlayer] != 1 || got.Dense[odSeat0+rel] != 1 {
					t.Errorf("viewer %d, %s option naming seat %d: %v", viewer, o.Kind, p, got.Dense[odSeat0:])
				}
			}
		}
		// an option naming no player sets no seat column
		pass := EncodeOption(v, seat, decision.KPriority, decision.Option{Kind: "pass", Player: 3}, 0, 1)
		for k := 0; k < 4; k++ {
			if pass.Dense[odSeat0+k] != 0 {
				t.Errorf("viewer %d: a pass option set seat column %d", viewer, k)
			}
		}
	}
}

func equalFeatures(a, b []Feature) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A 24-wide checkpoint (the option row before the relative-seat columns)
// writes the pre-widening hash, loads, and scores an option row with the
// new columns set bit-identically to the row without them.
func TestOptionWidth24CheckpointUnchanged(t *testing.T) {
	m := NewModelOptDense(TableRows, 4, 6, OptionDenseWidthV1, rand.New(rand.NewPCG(5, 6)))
	m.Features = FeaturesMZ
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatal(err)
	}
	if h := binary.LittleEndian.Uint64(buf.Bytes()[8:16]); h != EncoderHashForV1Options(FeaturesMZ) {
		t.Fatalf("24-wide checkpoint hash %#x, want the pre-widening %#x", h, EncoderHashForV1Options(FeaturesMZ))
	}
	back, err := LoadCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if back.OptDenseW() != OptionDenseWidthV1 || back.Features != FeaturesMZ {
		t.Fatalf("loaded width %d set %s", back.OptDenseW(), back.Features)
	}
	st := State{Dense: make([]float32, DenseWidth), Sparse: []Feature{{Row: 9, Value: 1}}}
	for i := range st.Dense {
		st.Dense[i] = float32(i%5) / 5
	}
	var full, short []Option
	for i := 0; i < 4; i++ {
		d := make([]float32, OptionDenseWidth)
		for j := range d {
			d[j] = float32((i+j)%3) / 2
		}
		d[odSeat0+i] = 1
		o := Option{Slots: []Feature{{Row: uint16(i), Value: 1}}, Hashed: []Feature{{Row: uint16(40 + i), Value: 1}}, Dense: d}
		full = append(full, o)
		o.Dense = d[:OptionDenseWidthV1]
		short = append(short, o)
	}
	want := m.Score(st, short)
	for name, got := range map[string][]float32{"source full": m.Score(st, full), "loaded full": back.Score(st, full), "loaded short": back.Score(st, short)} {
		for i := range want {
			if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
				t.Errorf("%s: score %d %v vs %v", name, i, got[i], want[i])
			}
		}
	}
}
