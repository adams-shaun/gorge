package gonet_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
)

func sampleState() (policynet.State, []policynet.Option) {
	dense := make([]float32, policynet.DenseWidth)
	for i := range dense {
		dense[i] = float32(i%7) / 7
	}
	raw := func(k int) []float32 {
		r := make([]float32, policynet.EntityRawWidth)
		for i := range r {
			r[i] = float32((i+k)%5) / 5
		}
		return r
	}
	st := policynet.State{
		Dense:  dense,
		Sparse: []policynet.Feature{{Row: 3, Value: 1}, {Row: 900, Value: 2}},
		Cards: []policynet.EntityCard{
			{Group: 0, Raw: raw(0), Rows: []policynet.Feature{{Row: 10, Value: 1}}},
			{Group: 1, Raw: raw(1), Rows: []policynet.Feature{{Row: 11, Value: 1}, {Row: 12, Value: 1}}},
			{Group: 2, Raw: raw(2), Rows: nil},
		},
	}
	od := make([]float32, policynet.OptionDenseWidth)
	opts := []policynet.Option{
		{Slots: []policynet.Feature{{Row: 1, Value: 1}}, Hashed: []policynet.Feature{{Row: 10, Value: 1}}, Dense: od},
		{Slots: []policynet.Feature{{Row: 5, Value: 1}}, Hashed: []policynet.Feature{{Row: 77, Value: 1}}, Dense: od},
		{Slots: []policynet.Feature{{Row: 9, Value: 1}}, Dense: od},
	}
	return st, opts
}

func load(t *testing.T) *gonet.Net {
	t.Helper()
	path, err := gonettest.Write(t.TempDir(), "test.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	g, err := gonet.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return g
}

// The same state and options always score the same, and a result-cache hit
// returns exactly what a miss computed: the cache is transparent.
func TestScoreAndValueAreDeterministicAndCacheTransparent(t *testing.T) {
	st, opts := sampleState()
	g := load(t)

	first := g.Score(st, opts)
	if len(first) != len(opts) {
		t.Fatalf("Score returned %d logits for %d options", len(first), len(opts))
	}
	if again := g.Score(st, opts); !slices.Equal(first, again) {
		t.Errorf("Score changed on a cache hit: %v vs %v", first, again)
	}
	v := g.Value(st)
	if v <= 0 || v >= 1 {
		t.Errorf("Value = %v, want a probability in (0,1)", v)
	}
	if again := g.Value(st); again != v {
		t.Errorf("Value changed on a cache hit: %v vs %v", v, again)
	}

	// The same network with no caches at all must agree to the bit.
	old, oldCard := gonet.CacheSize, gonet.CardCacheSize
	gonet.CacheSize, gonet.CardCacheSize = 0, 0
	t.Cleanup(func() { gonet.CacheSize, gonet.CardCacheSize = old, oldCard })
	bare := load(t)
	if got := bare.Score(st, opts); !slices.Equal(first, got) {
		t.Errorf("cached and uncached Score differ: %v vs %v", first, got)
	}
	if got := bare.Value(st); got != v {
		t.Errorf("cached and uncached Value differ: %v vs %v", v, got)
	}
	// Eval is the uncached core both use.
	scores := make([]float32, len(opts))
	logit := bare.Eval(&st, opts, scores)
	if !slices.Equal(scores, first) || gonet.Sigmoid32(logit) != v {
		t.Errorf("Eval disagrees with Score/Value: %v, %v", scores, gonet.Sigmoid32(logit))
	}
}

// Different options for one state give different scores: the network reads
// them (a constant answer would pass every determinism check above).
func TestOptionsChangeTheScores(t *testing.T) {
	st, opts := sampleState()
	g := load(t)
	s := g.Score(st, opts)
	if s[0] == s[1] && s[1] == s[2] {
		t.Errorf("three different options scored identically: %v", s)
	}
	if g.Value(st) == g.Value(policynet.State{Dense: make([]float32, policynet.DenseWidth)}) {
		t.Errorf("two different states have the same value")
	}
}

func TestSharedReturnsOneNetworkPerPath(t *testing.T) {
	path, err := gonettest.Write(t.TempDir(), "shared.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	a, err := gonet.Shared(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := gonet.Shared(path)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Error("Shared built two networks for one path")
	}
	m, err := gonet.Model(path)
	if err != nil || m == nil {
		t.Errorf("Model(%s) = %v, %v", path, m, err)
	}
}

func TestLoadRefusesBadFiles(t *testing.T) {
	dir := t.TempDir()
	good, err := gonettest.Write(dir, "good.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"empty":      {},
		"not-dzgw":   []byte(strings.Repeat("x", 200)),
		"bad-header": append([]byte("DZGW0001"), 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f),
		"truncated":  raw[:len(raw)/2],
	}
	for name, b := range cases {
		p := filepath.Join(dir, name+".dzgw")
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := gonet.Load(p); err == nil {
			t.Errorf("Load accepted a %s file", name)
		}
	}
	if _, err := gonet.Load(filepath.Join(dir, "absent.dzgw")); err == nil {
		t.Error("Load accepted a path that does not exist")
	}
}

// The file Write produces is byte-identical from call to call, so a test that
// pins a network's answers can rely on it.
func TestTestNetworkIsStable(t *testing.T) {
	a, _ := gonettest.Write(t.TempDir(), "a.dzgw")
	b, _ := gonettest.Write(t.TempDir(), "b.dzgw")
	ra, _ := os.ReadFile(a)
	rb, _ := os.ReadFile(b)
	if len(ra) == 0 || !slices.Equal(ra, rb) {
		t.Errorf("gonettest.Write is not deterministic (%d vs %d bytes)", len(ra), len(rb))
	}
}

// GORGE_DZGW names a real exported network (python -m dzg.export); the test
// is skipped without it, so CI never depends on a training artifact.
func TestRealNetworkLoadsAndAnswers(t *testing.T) {
	path := os.Getenv("GORGE_DZGW")
	if path == "" {
		t.Skip("GORGE_DZGW not set")
	}
	g, err := gonet.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	st, opts := sampleState()
	s := g.Score(st, opts)
	if len(s) != len(opts) || !slices.Equal(s, g.Score(st, opts)) {
		t.Errorf("real network: scores %v", s)
	}
	if v := g.Value(st); v <= 0 || v >= 1 {
		t.Errorf("real network: value %v not in (0,1)", v)
	}
}

// EvalBatch on an mlp network loops the serial forward: each state's value
// and logits equal Eval's, and a nil option list is value only.
func TestEvalBatchMLPMatchesEval(t *testing.T) {
	st, opts := sampleState()
	empty := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
	g := load(t)
	sts := []*policynet.State{&st, &empty, &st}
	bo := [][]policynet.Option{opts, nil, opts[:1]}
	scores := [][]float32{make([]float32, 3), nil, make([]float32, 1)}
	values := make([]float32, 3)
	g.EvalBatch(sts, bo, scores, values)
	for i := range sts {
		want := make([]float32, len(bo[i]))
		if v := g.Eval(sts[i], bo[i], want); v != values[i] || !slices.Equal(want, scores[i][:len(bo[i])]) {
			t.Errorf("state %d: batch %v %v, Eval %v %v", i, values[i], scores[i], v, want)
		}
	}
	vals := make([]float32, 2)
	g.EvalBatch(sts[:2], nil, nil, vals) // value only
	if vals[0] != values[0] || vals[1] != values[1] {
		t.Errorf("value-only batch %v, want %v", vals, values[:2])
	}
}
