package manifest

import (
	"bytes"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// overlay is base with add's files laid over it: a tree that differs from the
// real one by exactly the added files.
type overlay struct {
	base fs.FS
	add  fstest.MapFS
}

func (o overlay) Open(name string) (fs.File, error) {
	if f, err := o.base.Open(name); err == nil {
		return f, nil
	}
	return o.add.Open(name)
}

func (o overlay) ReadFile(name string) ([]byte, error) {
	if b, err := fs.ReadFile(o.add, name); err == nil {
		return b, nil
	}
	return fs.ReadFile(o.base, name)
}

func (o overlay) ReadDir(name string) ([]fs.DirEntry, error) {
	byName := map[string]fs.DirEntry{}
	var found bool
	for _, f := range []fs.FS{o.base, o.add} {
		des, err := fs.ReadDir(f, name)
		if err != nil {
			continue
		}
		found = true
		for _, de := range des {
			byName[de.Name()] = de
		}
	}
	if !found {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	out := make([]fs.DirEntry, 0, len(byName))
	for _, de := range byName {
		out = append(out, de)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

// demoFeature is the source of a feature file that only holds directives.
const demoFeature = `package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// demoNoop runs ExtDemoNoop.
//
//kmanifest:ext ExtDemoNoop
func (s *State) demoNoop(k *prep, in *kop.Instr) (ok, done bool) { return true, true }

// demoPickOptions builds raZDemoPick's options.
//
//kmanifest:askopts raZDemoPick
func (s *State) demoPickOptions(k *prep, d *Decision) {}

// demoPickAnswer applies raZDemoPick's answer.
//
//kmanifest:askapply raZDemoPick
func (s *State) demoPickAnswer(k *prep, a Action) {}
`

// TestNewOpTouchesOnlyItsOwnFiles is the point of the directives and the
// derived ordinals: a new sub-op, a new predicate and a new ask kind, each an
// entry file, plus one feature file holding the handlers' directives, change
// no shared file (ordinals.tsv, asks.tsv, the hand dispatch, kgen's names) and
// only add lines to the generated ones.
func TestNewOpTouchesOnlyItsOwnFiles(t *testing.T) {
	real := os.DirFS("../..")
	ov := overlay{real, fstest.MapFS{
		Dir + "/ExtOp/ExtDemoNoop": {Data: []byte("family: ExtOp\nname: ExtDemoNoop\nvalue: next\nstatus: active\n")},
		Dir + "/Pred/PredDemo":     {Data: []byte("family: Pred\nname: PredDemo\nvalue: next\nstatus: reserved\n")},
		Dir + "/ask/raZDemoPick":   {Data: []byte("family: ask\nname: raZDemoPick\nvalue: next\nstatus: active\n")},
		"dirac/act_demonoop.go":    {Data: []byte(demoFeature)},
	}}

	// 1. The overlay loads and passes the change check against the real tree;
	// every shared file is the real tree's bytes.
	baseTree, err := LoadGen(real)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := LoadGen(ov)
	if err != nil {
		t.Fatal(err)
	}
	if errs := Check(baseTree, tree, CheckOptions{}); len(errs) > 0 {
		t.Fatalf("Check: %v", errs)
	}
	for _, p := range []string{OrdinalsPath, AsksPath, "dirac/dispatch.go", "dirac/interp.go", "dirac/interp_ask.go", "dirac/interp_gen.go", "dirac/extop.go", "dirac/kgen/names.go"} {
		a, err := fs.ReadFile(real, p)
		if err != nil {
			t.Fatal(err)
		}
		if b, err := fs.ReadFile(ov, p); err != nil || !bytes.Equal(a, b) {
			t.Errorf("%s differs in the overlay (%v)", p, err)
		}
	}
	if len(tree.Handlers.Ext) != len(baseTree.Handlers.Ext)+1 || len(tree.Handlers.Ask) != len(baseTree.Handlers.Ask)+1 {
		t.Fatalf("handlers: ext %d -> %d, ask %d -> %d; want one more of each", len(baseTree.Handlers.Ext), len(tree.Handlers.Ext), len(baseTree.Handlers.Ask), len(tree.Handlers.Ask))
	}

	// 2. Each generated file only gains lines, and the gained ones are the
	// new entries' and handlers'.
	before, err := Generate(baseTree)
	if err != nil {
		t.Fatal(err)
	}
	after, err := Generate(tree)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		KopGen:   {"const ExtDemoNoop ExtOp =", "const PredDemo Pred ="},
		DiracGen: {"const raZDemoPick uint8 =", "return s.demoNoop(k, in)", "s.demoPickOptions(k, d)", "s.demoPickAnswer(k, a)"},
		KgenGen:  {`enum[kop.Pred]("kop.PredDemo", kop.PredDemo)`},
	}
	for _, p := range GenFiles {
		added, ok := addedLines(strings.Split(string(before[p]), "\n"), strings.Split(string(after[p]), "\n"))
		if !ok {
			t.Errorf("%s: a line of the real output is gone or changed", p)
			continue
		}
		all := strings.Join(added, "\n")
		for _, w := range want[p] {
			if !strings.Contains(all, w) {
				t.Errorf("%s: the added lines lack %q:\n%s", p, w, all)
			}
		}
	}

	// 3. The new constants' ordinals rows derive from their entries, so the
	// file TestKopOrdinalsUnchanged reads needs no edit.
	got := map[string]string{}
	for _, r := range tree.EntryOrdinals() {
		got[r.Name] = r.Type + "\t" + r.Value
	}
	for _, e := range tree.Entries {
		if e.Name != "ExtDemoNoop" && e.Name != "PredDemo" {
			continue
		}
		if w := e.Family + "\t" + itoa(e.Value); got[e.Name] != w || e.Value == 0 {
			t.Errorf("EntryOrdinals[%s] = %q, want %q at a provisional non-zero value", e.Name, got[e.Name], w)
		}
		delete(got, e.Name)
	}
	if _, has := got["ExtDemoNoop"]; has {
		t.Errorf("ExtDemoNoop is not a derived row")
	}
	if len(tree.Ordinals) != len(baseTree.Ordinals) {
		t.Errorf("ordinals.tsv rows: %d -> %d", len(baseTree.Ordinals), len(tree.Ordinals))
	}
}

// addedLines is the lines of cur that old's lines, taken in order, do not
// account for; false when old is not a subsequence of cur.
func addedLines(old, cur []string) ([]string, bool) {
	var added []string
	i := 0
	for _, l := range cur {
		if i < len(old) && old[i] == l {
			i++
			continue
		}
		added = append(added, l)
	}
	return added, i == len(old)
}
