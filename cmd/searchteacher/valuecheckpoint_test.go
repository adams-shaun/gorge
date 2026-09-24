package main

import (
	"io"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/policynet"
)

func saveModel(t *testing.T, withValue bool) string {
	t.Helper()
	m := policynet.NewModel(policynet.TableRows, 8, 6, rand.New(rand.NewPCG(1, 2)))
	if withValue {
		m.InitValue(5, rand.New(rand.NewPCG(3, 4)))
	}
	p := filepath.Join(t.TempDir(), "m.gpol")
	if err := m.SaveCheckpoint(p); err != nil {
		t.Fatal(err)
	}
	return p
}

// -value-checkpoint is refused up front, before the corpus is opened: with
// -horizon 0 (a game-end rollout never reaches a non-terminal leaf), for a
// checkpoint without a value head, and for one the loader refuses.
func TestValueCheckpointPreflight(t *testing.T) {
	withValue, bare := saveModel(t, true), saveModel(t, false)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-value-checkpoint", withValue}, "needs -horizon > 0"},
		{[]string{"-value-checkpoint", withValue, "-horizon", "0"}, "needs -horizon > 0"},
		{[]string{"-value-checkpoint", bare, "-horizon", "2"}, "has no value head"},
		{[]string{"-value-checkpoint", filepath.Join(t.TempDir(), "missing.gpol"), "-horizon", "2"}, "-value-checkpoint:"},
		// Accepted: the run proceeds to the (absent) corpus.
		{[]string{"-value-checkpoint", withValue, "-horizon", "2"}, "/nonexistent"},
	} {
		err := run(append(c.args, "-cards", "/nonexistent"), io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%v: want %q, got %v", c.args, c.want, err)
		}
	}
}

func TestSummaryNamesTheLeaf(t *testing.T) {
	m := policynet.NewModel(policynet.TableRows, 8, 6, rand.New(rand.NewPCG(1, 2)))
	for _, c := range []struct {
		cfg  config
		want string
	}{{config{horizon: 2}, " horizon=2 leaf=heuristic "}, {config{horizon: 2, value: m}, " horizon=2 leaf=value "}} {
		var b strings.Builder
		summarize(&b, nil, c.cfg, 30_000_000, 1, 0)
		if first, _, _ := strings.Cut(b.String(), "\n"); !strings.Contains(first, c.want) {
			t.Fatalf("header %q missing %q", first, c.want)
		}
	}
}
