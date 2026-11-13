package policynet

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// kModel is fullModel with a random K-way value head on top of InitValue's
// hidden layer: the scalar output blocks zeroed (a K-way model's invariant),
// the K-way blocks drawn from rng.
func kModel(k int, rng *rand.Rand) *Model {
	m := NewModel(TableRows, 8, 4, rand.New(rand.NewPCG(9, 10)))
	m.InitValue(3, rand.New(rand.NewPCG(11, 12)))
	m.ValueK = k
	m.VOutWK = make([]float32, k*m.ValueHidden)
	m.VOutBK = make([]float32, k)
	fillUniform(m.VOutWK, glorot(m.ValueHidden, k), rng)
	fillUniform(m.VOutBK, 1, rng)
	return m
}

// kVecState is a non-trivial encoded state for the value-head readouts.
func kVecState() State {
	st := State{Dense: make([]float32, DenseWidth)}
	for i := range st.Dense {
		st.Dense[i] = float32(i) * 0.037
	}
	st.Sparse = []Feature{{Row: 5, Value: 1}, {Row: 7000, Value: 0.5}}
	return st
}

// TestValueVecRoundTrip: a random K=5 head saves and loads; the loaded
// model's vector sums to 1, Value is its component 0 bit for bit, and every
// weight block round-trips. A zero-weight head pins the softmax itself
// against the textbook form computed here.
func TestValueVecRoundTrip(t *testing.T) {
	m := kModel(5, rand.New(rand.NewPCG(13, 14)))
	st := kVecState()
	want := m.ValueVec(st)
	if len(want) != 5 {
		t.Fatalf("ValueVec len %d, want 5", len(want))
	}
	if s := sum32(want); math.Abs(float64(s-1)) > 1e-5 {
		t.Fatalf("precondition: the vector sums to %v, want 1", s)
	}
	if math.Float32bits(m.Value(st)) != math.Float32bits(want[0]) {
		t.Fatalf("precondition: Value %v is not the vector's component 0 %v", m.Value(st), want[0])
	}

	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	m2, err := LoadCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	if m2.ValueK != 5 || !slices.Equal(m.VOutWK, m2.VOutWK) || !slices.Equal(m.VOutBK, m2.VOutBK) {
		t.Fatal("round trip changed the K-way head")
	}
	// The scalar blocks are inert on a K-way model: carried through the
	// round trip, but the readouts above never read them.
	if !slices.Equal(m.VOutW, m2.VOutW) || m.VOutB != m2.VOutB {
		t.Fatal("round trip changed the inert scalar head")
	}
	got := m2.ValueVec(st)
	if !slices.Equal(got, want) {
		t.Fatalf("ValueVec after round trip %v, want %v", got, want)
	}
	if math.Float32bits(m2.Value(st)) != math.Float32bits(want[0]) {
		t.Fatalf("Value %v after round trip, want %v", m2.Value(st), want[0])
	}

	// The zero-weight head: the logits are the biases alone, so the vector is
	// the softmax of five known numbers, computed here in float64.
	zero := kModel(5, rand.New(rand.NewPCG(13, 14)))
	for i := range zero.VOutWK {
		zero.VOutWK[i] = 0
	}
	biases := []float32{0.5, -0.5, 1, -1, 0}
	copy(zero.VOutBK, biases)
	z := zero.ValueVec(st)
	total := 0.0
	exp := make([]float64, 5)
	for i, b := range biases {
		exp[i] = math.Exp(float64(b))
		total += exp[i]
	}
	for i := range exp {
		if math.Abs(float64(z[i])-exp[i]/total) > 1e-6 {
			t.Fatalf("softmax component %d: %v, want %v (biases %v)", i, z[i], exp[i]/total, biases)
		}
	}

	// A K-way head with no scalar blocks still round-trips through the file
	// path, and the written schema version is the vector one.
	path := t.TempDir() + "/ckpt.bin"
	if err := m.SaveCheckpoint(path); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}
	if _, err := LoadCheckpointFile(path); err != nil {
		t.Fatalf("LoadCheckpointFile: %v", err)
	}
	var again bytes.Buffer
	if err := WriteCheckpoint(m, &again); err != nil {
		t.Fatalf("WriteCheckpoint 2: %v", err)
	}
	if v := binary.LittleEndian.Uint32(again.Bytes()[4:8]); v != CheckpointVersionValueVec {
		t.Fatalf("a K-way model wrote schema version %d, want %d", v, CheckpointVersionValueVec)
	}
}

// TestScalarCheckpointUnchanged: an existing scalar checkpoint keeps its
// schema version, its byte-identical Value across a save/load, and a nil
// ValueVec — the K-way head changes nothing about the scalar head.
func TestScalarCheckpointUnchanged(t *testing.T) {
	m := fullModel()
	st := kVecState()
	want := m.Value(st)

	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	if v := binary.LittleEndian.Uint32(buf.Bytes()[4:8]); v != CheckpointVersion {
		t.Fatalf("a scalar model wrote schema version %d, want %d", v, CheckpointVersion)
	}
	m2, err := LoadCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	if got := m2.Value(st); math.Float32bits(got) != math.Float32bits(want) {
		t.Fatalf("scalar Value %v after round trip, want %v", got, want)
	}
	if got := m.ValueVec(st); got != nil {
		t.Fatalf("a scalar model's ValueVec is %v, want nil", got)
	}
	if m2.ValueK != 0 || len(m2.VOutWK) != 0 || len(m2.VOutBK) != 0 {
		t.Fatalf("the loaded scalar model carries K-way blocks: K %d, WK %d, BK %d", m2.ValueK, len(m2.VOutWK), len(m2.VOutBK))
	}
}

func sum32(fs []float32) float32 {
	s := float32(0)
	for _, f := range fs {
		s += f
	}
	return s
}
