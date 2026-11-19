package oracle

import (
	"encoding/json"
	"strings"
	"testing"
)

// wireWord is the default class's packing of a Canon with these fields: the
// number a repro file holds in every build.
func wireWord(kind CanonKind, src, tgt, mode uint8) uint64 {
	return uint64(kind)<<24 | uint64(src)<<16 | uint64(tgt)<<8 | uint64(mode)
}

// TestReproActionsWireIsTheDefaultPacking: a repro's actions are the default
// class's packed numbers whichever size class wrote them, and read back as the
// reader's own Refs: a seat, a stack ability and none keep their meaning.
func TestReproActionsWireIsTheDefaultPacking(t *testing.T) {
	as := []Canon{
		Pack(3, 5, RefPlayer+1, 7),
		Pack(2, RefStack+2, RefNone, 0),
		Pack(1, RefNone, 200, 9),
	}
	b, err := json.Marshal(&Repro{Format: reproFormat, Actions: as})
	if err != nil {
		t.Fatal(err)
	}
	want := []uint64{wireWord(3, 5, 0xFD, 7), wireWord(2, 0xEE, 0xFF, 0), wireWord(1, 0xFF, 200, 9)}
	var raw struct {
		Actions []uint64
		Packing string
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Actions) != len(want) || raw.Packing != "" {
		t.Fatalf("wire %v packing %q, want %v and none:\n%s", raw.Actions, raw.Packing, want, b)
	}
	for i := range want {
		if raw.Actions[i] != want[i] {
			t.Errorf("action %d is %d on the wire, want %d", i, raw.Actions[i], want[i])
		}
	}
	var back Repro
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	for i, a := range as {
		if back.Actions[i] != a {
			t.Errorf("action %d reads back as %v, want %v", i, back.Actions[i], a)
		}
	}
}

// TestReproActionsReadADefaultRecording: the recorded files are default-packed
// numbers, and a build of the other size class reads the same actions from them
// (a format-1 file's stack ability 0xE1 and seat 0xF1 included).
func TestReproActionsReadADefaultRecording(t *testing.T) {
	for _, tc := range []struct {
		format string
		words  []uint64
		src    []Ref
		tgt    []Ref
	}{
		{reproFormat, []uint64{wireWord(4, 0xED, 0xFC, 1), wireWord(4, 7, 0xFF, 0)}, []Ref{RefStack + 1, 7}, []Ref{RefPlayer, RefNone}},
		{reproFormat1, []uint64{wireWord(4, 0xE1, 0xF1, 1), wireWord(4, 7, 0xFF, 0)}, []Ref{RefStack + 1, 7}, []Ref{RefPlayer + 1, RefNone}},
	} {
		ws, _ := json.Marshal(tc.words)
		var r Repro
		in := `{"format":"` + tc.format + `","actions":` + string(ws) + `}`
		if err := json.Unmarshal([]byte(in), &r); err != nil {
			t.Fatalf("%s: %v", tc.format, err)
		}
		if r.Format != reproFormat || len(r.Actions) != len(tc.words) {
			t.Fatalf("%s: decoded format %q, %d actions", tc.format, r.Format, len(r.Actions))
		}
		for i, a := range r.Actions {
			if a.Kind() != 4 || a.Src() != tc.src[i] || a.Tgt() != tc.tgt[i] {
				t.Errorf("%s: action %d is %s/%d/%d, want kind 4 src %d tgt %d", tc.format, i, a, a.Src(), a.Tgt(), tc.src[i], tc.tgt[i])
			}
		}
	}
	var r Repro
	err := json.Unmarshal([]byte(`{"format":"`+reproFormat+`","actions":[1],"packing":"nine"}`), &r)
	if err == nil || !strings.Contains(err.Error(), "nine") {
		t.Errorf("an unknown packing: err %v", err)
	}
}
