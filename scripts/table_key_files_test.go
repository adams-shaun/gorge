package scripts

// TestTableKeyCoversEveryKgenDependency: the box's table directory (ci-corpus.sh's dk) is keyed by
// scripts/table-key-files.sh, which must list every non-test Go file and every embedded file of each in-module
// package ./dirac/cmd/kgen is built from. 11-21: keyed by tablesrc.go's embeds alone, a box served tables an older
// tree had built after engine/rules/pay changed, and batch #3110 passed a replay its own tables fail. The expected
// set is derived here from go list's JSON, independently of the script's template, so a new kgen dependency is
// covered without editing either.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type listedPkg struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	EmbedFiles []string
	Module     *struct{ Path string }
}

func TestTableKeyCoversEveryKgenDependency(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	modOut, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		t.Fatalf("go list -m: %v", err)
	}
	mod := strings.TrimSpace(string(modOut))

	cmd := exec.Command("go", "list", "-deps", "-json", "./dirac/cmd/kgen")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps -json ./dirac/cmd/kgen: %v", err)
	}
	want := map[string]string{} // file -> its package
	pkgs := 0
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p listedPkg
		if err := dec.Decode(&p); err != nil {
			t.Fatal(err)
		}
		if p.Module == nil || p.Module.Path != mod {
			continue
		}
		pkgs++
		for _, f := range append(append([]string{}, p.GoFiles...), p.EmbedFiles...) {
			rel, err := filepath.Rel(root, filepath.Join(p.Dir, f))
			if err != nil {
				t.Fatal(err)
			}
			want[filepath.ToSlash(rel)] = p.ImportPath
		}
	}
	if pkgs < 5 || len(want) == 0 {
		t.Fatalf("go list found %d in-module kgen packages, %d files: the derivation is broken", pkgs, len(want))
	}

	script := exec.Command("bash", "table-key-files.sh")
	script.Dir = "."
	sout, err := script.Output()
	if err != nil {
		t.Fatalf("table-key-files.sh: %v", err)
	}
	got := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(sout))
	for sc.Scan() {
		got[sc.Text()] = true
	}
	missingPkgs := map[string]int{}
	for f, p := range want {
		if !got[f] {
			missingPkgs[p]++
		}
	}
	for p, n := range missingPkgs {
		t.Errorf("kgen dependency %s: %d of its files are missing from the table key (table-key-files.sh)", p, n)
	}

	corpus, err := os.ReadFile("ci-corpus.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(corpus), "dk=$(bash scripts/table-key-files.sh") {
		t.Error("ci-corpus.sh's table directory key (dk) is not computed from scripts/table-key-files.sh")
	}
}
