package policynet

import (
	"bytes"
	"math/rand/v2"
	"slices"
	"testing"
)

// fullModel builds a checkpoint-rounded model (Rows = TableRows, the shape a
// real training run produces) for the round-trip tests.
func fullModel() *Model {
	m := NewModel(TableRows, 8, 4, rand.New(rand.NewPCG(3, 4)))
	m.ResidualW = 2.5 // a non-zero prior so the round trip proves it is carried
	m.InitValue(3, rand.New(rand.NewPCG(5, 6)))
	for i := range m.VHidB {
		m.VHidB[i] = 0.05 * float32(i+1) // non-zero so the round trip proves they are carried
	}
	m.VOutB = -0.25
	return m
}

func TestCheckpointRoundTrip(t *testing.T) {
	m := fullModel()
	st := State{Dense: make([]float32, DenseWidth)}
	for i := range st.Dense {
		st.Dense[i] = float32(i) * 0.01
	}
	st.Sparse = []Feature{{Row: 5, Value: 1}, {Row: 7000, Value: 0.5}}
	opts := []Option{
		{Hashed: []Feature{{Row: 12, Value: 1}}, Slots: []Feature{{Row: 3, Value: 1}}, Dense: make([]float32, OptionDenseWidth)},
		{Hashed: []Feature{{Row: 999, Value: 1}}, Dense: make([]float32, OptionDenseWidth)},
	}
	want := m.Score(st, opts)

	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	m2, err := LoadCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	if m2.Rows != m.Rows || m2.H != m.H || m2.Hidden != m.Hidden || m2.InW != m.InW {
		t.Fatalf("geometry: got rows %d h %d hidden %d inW %d, want %d/%d/%d/%d", m2.Rows, m2.H, m2.Hidden, m2.InW, m.Rows, m.H, m.Hidden, m.InW)
	}
	if !slices.Equal(m.Table, m2.Table) || !slices.Equal(m.StateW, m2.StateW) ||
		!slices.Equal(m.StateB, m2.StateB) || !slices.Equal(m.HidW, m2.HidW) ||
		!slices.Equal(m.HidB, m2.HidB) || !slices.Equal(m.OutW, m2.OutW) || m.OutB != m2.OutB || m.ResidualW != m2.ResidualW {
		t.Fatal("round trip changed at least one weight")
	}
	if m2.ValueHidden != m.ValueHidden || !slices.Equal(m.VHidW, m2.VHidW) || !slices.Equal(m.VHidB, m2.VHidB) ||
		!slices.Equal(m.VOutW, m2.VOutW) || m.VOutB != m2.VOutB {
		t.Fatal("round trip changed the value head")
	}
	if a, b := m.Value(st), m2.Value(st); a != b {
		t.Fatalf("value %g after round trip, want %g", b, a)
	}
	got := m2.Score(st, opts)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("option %d: score %g after round trip, want %g — the scorer must be bit-identical across a save/load", i, got[i], want[i])
		}
	}

	// File path round trip (the atomic save the trainer uses).
	dir := t.TempDir()
	path := dir + "/ckpt.bin"
	if err := m.SaveCheckpoint(path); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}
	m3, err := LoadCheckpointFile(path)
	if err != nil {
		t.Fatalf("LoadCheckpointFile: %v", err)
	}
	if !slices.Equal(m3.Table, m.Table) {
		t.Fatal("file round trip changed the table")
	}
	// The saved bytes must be reproducible byte-for-byte (a same-seed
	// re-encode is the determinism test's foundation).
	var buf2 bytes.Buffer
	if err := WriteCheckpoint(m, &buf2); err != nil {
		t.Fatalf("WriteCheckpoint 2: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), buf2.Bytes()) {
		t.Fatal("two encodings of the same model differ — the checkpoint is not deterministic")
	}
}

// patchHeader overwrites four bytes at off in a serialized checkpoint.
func patchHeader(t *testing.T, m *Model, off int, repl []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	b := buf.Bytes()
	copy(b[off:off+len(repl)], repl)
	return b
}

func TestCheckpointRejections(t *testing.T) {
	m := fullModel()

	if _, err := LoadCheckpoint(bytes.NewReader(patchHeader(t, m, 0, []byte("XPOL")))); err == nil {
		t.Fatal("bad magic accepted")
	} else if !bytes.Contains([]byte(err.Error()), []byte("magic")) {
		t.Fatalf("bad-magic error should name the magic, got: %v", err)
	}

	if _, err := LoadCheckpoint(bytes.NewReader(patchHeader(t, m, 4, []byte{byte(CheckpointVersion + 1), 0, 0, 0}))); err == nil {
		t.Fatal("wrong schema version accepted")
	} else if !bytes.Contains([]byte(err.Error()), []byte("version")) {
		t.Fatalf("version error should name the version, got: %v", err)
	}

	// Encoder hash off by one bit — the drifted-encoder tripwire.
	bad := patchHeader(t, m, 8, []byte{0x01})
	if _, err := LoadCheckpoint(bytes.NewReader(bad)); err == nil {
		t.Fatal("wrong encoder hash accepted")
	} else if !bytes.Contains([]byte(err.Error()), []byte("encoder hash")) {
		t.Fatalf("hash error should name the encoder hash, got: %v", err)
	}

	// Geometry: a different table row count cannot serve this encoder's
	// hashed rows.
	rows := patchHeader(t, m, 16, []byte{0x40, 0x00, 0x00, 0x00}) // 64 != TableRows
	if _, err := LoadCheckpoint(bytes.NewReader(rows)); err == nil {
		t.Fatal("wrong table-row geometry accepted")
	} else if !bytes.Contains([]byte(err.Error()), []byte("table rows")) {
		t.Fatalf("rows error should name the row count, got: %v", err)
	}

	// Geometry: the option slot width is an encoder constant.
	slotW := patchHeader(t, m, 32, []byte{0x07, 0x00, 0x00, 0x00}) // 7 != OptionSlotWidth
	if _, err := LoadCheckpoint(bytes.NewReader(slotW)); err == nil {
		t.Fatal("wrong slot-width geometry accepted")
	} else if !bytes.Contains([]byte(err.Error()), []byte("option slot width")) {
		t.Fatalf("slot-width error should name the width, got: %v", err)
	}

	// Truncated body.
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	full := buf.Bytes()
	if _, err := LoadCheckpoint(bytes.NewReader(full[:len(full)-4])); err == nil {
		t.Fatal("truncated checkpoint accepted")
	}
	if _, err := LoadCheckpoint(bytes.NewReader(append(append([]byte{}, full...), 0))); err == nil {
		t.Fatal("overlong checkpoint accepted")
	}
}

// TestCheckpointV1Refused pins the schema-bump tripwire the doc comment
// claims: a genuine version-1 checkpoint (the pre-residual format — version
// field 1, body WITHOUT the trailing ResidualW float) is refused, never
// silently loaded with ResidualW 0. Built by truncating a real v2
// serialization's trailing float and setting the version field to 1 —
// exactly the bytes the v1 writer produced.
func TestCheckpointV1Refused(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCheckpoint(fullModel(), &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	b := buf.Bytes()
	copy(b[4:8], []byte{1, 0, 0, 0})
	v1 := b[:len(b)-4] // the v1 body ends at OutB; ResidualW is the v2 tail
	if _, err := LoadCheckpoint(bytes.NewReader(v1)); err == nil {
		t.Fatal("a version-1 checkpoint was silently accepted — the doc's refusal claim is unpinned")
	} else if !bytes.Contains([]byte(err.Error()), []byte("version")) {
		t.Fatalf("v1 refusal should name the version, got: %v", err)
	}
}

// TestCheckpointV2Refused pins the v3 tripwire: a genuine version-2
// checkpoint (six geometry words, body ending at ResidualW) is refused, never
// silently loaded as a model without a value head. Built from a real v3
// serialisation of a model WITHOUT a value head: drop the seventh geometry
// word (valueHidden, bytes 40..44) and the trailing VOutB float, and set the
// version field to 2 — exactly the bytes the v2 writer produced.
func TestCheckpointV2Refused(t *testing.T) {
	m := NewModel(TableRows, 8, 4, rand.New(rand.NewPCG(3, 4)))
	m.ResidualW = 2.5
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	b := buf.Bytes()
	v2 := append(append([]byte{}, b[:40]...), b[44:len(b)-4]...)
	copy(v2[4:8], []byte{2, 0, 0, 0})
	if _, err := LoadCheckpoint(bytes.NewReader(v2)); err == nil {
		t.Fatal("a version-2 checkpoint was silently accepted — the v3 refusal is unpinned")
	} else if !bytes.Contains([]byte(err.Error()), []byte("version")) {
		t.Fatalf("v2 refusal should name the version, got: %v", err)
	}
}

// TestCheckpointNoValueHeadRoundTrip: a policy-only model (ValueHidden 0,
// what -value-weight 0 trains) round-trips as a policy-only model.
func TestCheckpointNoValueHeadRoundTrip(t *testing.T) {
	m := NewModel(TableRows, 8, 4, rand.New(rand.NewPCG(3, 4)))
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	m2, err := LoadCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	if m2.HasValue() || m2.ValueHidden != 0 || len(m2.VHidW)+len(m2.VHidB)+len(m2.VOutW) != 0 || m2.VOutB != 0 {
		t.Fatalf("policy-only model loaded with a value head: hidden %d", m2.ValueHidden)
	}
}
