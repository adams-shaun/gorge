package main

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/oracle"
)

// TestRemapReproRewritesFromLabels: a replay that served other ints for the
// labelled picks is written back, labels kept; a skipped or short replay is
// refused and writes nothing.
func TestRemapReproRewritesFromLabels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.repro.json")
	rep := &oracle.Repro{Format: "koracle-repro-2", Actions: []oracle.Canon{1, 2, 3}, Labels: []oracle.ActionLabel{{At: 1, Label: "Soldier"}}}
	served := []oracle.Canon{1, 9, 3, 4}
	res := &oracle.Result{Actions: served, Labels: []oracle.ActionLabel{{At: 1, Label: "Soldier"}, {At: 3, Label: "Elf"}}}
	if code := remapRepro(path, rep, &oracle.ScriptPolicy{}, res); code != 0 {
		t.Fatalf("remap exited %d", code)
	}
	got, err := oracle.ReadRepro(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Actions, []oracle.Canon{1, 9, 3}) || !slices.Equal(got.Labels, rep.Labels) {
		t.Fatalf("wrote actions %v labels %v, want [1 9 3] and the label kept without the tail's", got.Actions, got.Labels)
	}
	other := filepath.Join(t.TempDir(), "o.repro.json")
	if remapRepro(other, rep, &oracle.ScriptPolicy{Skipped: 1}, res) == 0 || remapRepro(other, rep, &oracle.ScriptPolicy{}, &oracle.Result{Actions: served[:2]}) == 0 {
		t.Fatal("a skipped or short replay was remapped")
	}
	if _, err := oracle.ReadRepro(other); err == nil {
		t.Fatal("a refused remap wrote a file")
	}
}
