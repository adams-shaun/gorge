package policynet

import (
	"bytes"
	"math/rand/v2"
	"testing"
)

// randomOption builds one encoded-looking option with a seeded source, for
// the scorer's parity/determinism pins.
func randomOption(rng *rand.Rand, rows int) Option {
	o := Option{Dense: make([]float32, OptionDenseWidth)}
	for i := range o.Dense {
		o.Dense[i] = float32(rng.NormFloat64()) * 0.4
	}
	o.Hashed = []Feature{
		{Row: uint16(rng.IntN(rows)), Value: 1},
		{Row: uint16(rng.IntN(rows)), Value: 0.5},
		{Row: uint16(rng.IntN(rows)), Value: 1},
	}
	for _, r := range []int{0, 24, 37, 55, 71, 103} {
		o.Slots = append(o.Slots, Feature{Row: uint16(r + rng.IntN(4)), Value: 1})
	}
	return o
}

// randomState builds one encoded-looking state.
func randomState(rng *rand.Rand, rows int) State {
	st := State{Dense: make([]float32, DenseWidth)}
	for i := range st.Dense {
		st.Dense[i] = float32(rng.NormFloat64()) * 0.7
		st.Dense[i] = float32(int(st.Dense[i]*4)) / 4 // a few exact zeros stay in
		if i%7 == 3 {
			st.Dense[i] = 0
		}
	}
	for i := 0; i < 60; i++ {
		st.Sparse = append(st.Sparse, Feature{Row: uint16(rng.IntN(rows)), Value: 1})
	}
	return st
}

// TestScorerMatchesModelScore pins the parity contract: the scratch-reusing
// inference path returns EXACTLY the scores Model.Score returns — the same
// arithmetic in the same order, bit for bit, so a trained checkpoint's
// deployment path (Scorer) and its training path (Model) can never disagree.
func TestScorerMatchesModelScore(t *testing.T) {
	rng := rand.New(rand.NewPCG(97, 98))
	for _, geom := range [][3]int{{64, 6, 5}, {256, 16, 12}, {TableRows, 8, 4}} {
		m := NewModel(geom[0], geom[1], geom[2], rng)
		sc := NewScorer(m)
		for trial := 0; trial < 3; trial++ {
			st := randomState(rng, geom[0])
			opts := make([]Option, 4)
			for i := range opts {
				opts[i] = randomOption(rng, geom[0])
			}
			want := m.Score(st, opts)
			got := sc.Score(st, opts)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("geometry %v trial %d option %d: scorer %v, model %v — the inference path drifted from the training path", geom, trial, i, got[i], want[i])
				}
			}
			// Also the split form (SetState once, per-option scores) must be
			// bit-identical to the batch form.
			sc.SetState(st)
			for i := range opts {
				if y := sc.ScoreOption(opts[i]); y != want[i] {
					t.Fatalf("geometry %v trial %d option %d: SetState+ScoreOption %v, model %v", geom, trial, i, y, want[i])
				}
			}
		}
	}
}

// TestScorerRepeatedCallsAreBitIdentical pins the scratch-reuse contract: the
// same (state, option) scored repeatedly — with OTHER options interleaved in
// between, so any scratch residue from a neighbour would show — returns the
// same float32 bits every time.
func TestScorerRepeatedCallsAreBitIdentical(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	m := NewModel(TableRows, 8, 4, rng)
	sc := NewScorer(m)
	st := randomState(rng, TableRows)
	a := randomOption(rng, TableRows)
	b := randomOption(rng, TableRows)
	c := randomOption(rng, TableRows)

	first := sc.Score(st, []Option{a})
	for iter := 0; iter < 20; iter++ {
		// Interleave other options so the scratch is never clean between the
		// two a-scores.
		inter := sc.Score(st, []Option{b, c, a, b})
		if inter[2] != first[0] {
			t.Fatalf("iteration %d: option a scored %v after b/c ran through the scratch, first call %v — the scratch leaks", iter, inter[2], first[0])
		}
		sc.SetState(st)
		if y := sc.ScoreOption(a); y != first[0] {
			t.Fatalf("iteration %d: ScoreOption %v, first call %v", iter, y, first[0])
		}
	}
}

// TestScorerLoadCheckpointRoundTrip pins the L9c entry path: a checkpoint
// written by the trainer loads through LoadScorer (the full checkpoint gate
// runs) and scores bit-identically to the model it was written from.
func TestScorerLoadCheckpointRoundTrip(t *testing.T) {
	m := fullModel()
	rng := rand.New(rand.NewPCG(21, 22))
	st := randomState(rng, TableRows)
	opts := []Option{randomOption(rng, TableRows), randomOption(rng, TableRows)}
	want := m.Score(st, opts)

	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	sc, err := LoadScorer(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadScorer: %v", err)
	}
	got := sc.Score(st, opts)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("option %d: scorer-after-load %v, model-before-save %v", i, got[i], want[i])
		}
	}
}

// TestLoadScorerRejectsBadCheckpoint pins that the scorer's constructor
// refuses a drifted checkpoint rather than scoring noise: a one-bit encoder
// hash change must fail with the checkpoint loader's own named error.
func TestLoadScorerRejectsBadCheckpoint(t *testing.T) {
	m := fullModel()
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatalf("WriteCheckpoint: %v", err)
	}
	raw := buf.Bytes()
	raw[8] ^= 0x01 // the encoder-hash field's first byte
	if _, err := LoadScorer(bytes.NewReader(raw)); err == nil {
		t.Fatal("drifted encoder hash accepted by LoadScorer")
	}
}

// BenchmarkScoreOption measures the per-decision inference cost at the
// trainer's default geometry (H=128, hidden=128, TableRows rows): one
// SetState (the trunk, paid once per decision) and per-option ScoreOption
// calls over a realistic option list, so the reported ns per iteration is
// the number a seat-side policy pays for one decision with 8 offered
// options. BenchmarkScoreOptionOnly isolates the per-option half (the trunk
// is set once outside the loop).
func BenchmarkScoreOption(b *testing.B) {
	rng := rand.New(rand.NewPCG(1, 2))
	m := NewModel(TableRows, 128, 128, rng)
	sc := NewScorer(m)
	st := randomState(rng, TableRows)
	opts := make([]Option, 8)
	for i := range opts {
		opts[i] = randomOption(rng, TableRows)
	}
	sc.SetState(st)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, o := range opts {
			sc.ScoreOption(o)
		}
	}
}

// BenchmarkScoreDecision measures one whole decision: trunk + 8 options.
func BenchmarkScoreDecision(b *testing.B) {
	rng := rand.New(rand.NewPCG(1, 2))
	m := NewModel(TableRows, 128, 128, rng)
	sc := NewScorer(m)
	st := randomState(rng, TableRows)
	opts := make([]Option, 8)
	for i := range opts {
		opts[i] = randomOption(rng, TableRows)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sc.SetState(st)
		for _, o := range opts {
			sc.ScoreOption(o)
		}
	}
}
