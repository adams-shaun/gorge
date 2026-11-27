package scripts

// TestTableKeyCoversEveryKgenDependency: the box's table directory (dk, defined in scripts/ci-corpus-keys.sh,
// sourced by ci-corpus.sh and eval'd by ci.yml's prepare for the Actions cache key) is keyed by
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

	keys, err := os.ReadFile("ci-corpus-keys.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(keys), "kl=$(bash scripts/table-key-files.sh") {
		t.Error("ci-corpus-keys.sh's table directory key (dk) is not computed from scripts/table-key-files.sh")
	}
	if !strings.Contains(string(keys), `dk=$( { printf '%s\n' "$kl"`) {
		t.Error("ci-corpus-keys.sh's table directory key (dk) does not hash the file list (the published key covers each embed's path, so a bare content stream underkeys it)")
	}
	corpus, err := os.ReadFile("ci-corpus.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(corpus), "ci-corpus-keys.sh") {
		t.Error("ci-corpus.sh does not source ci-corpus-keys.sh: its tree key must come from the one definition")
	}
}

// TestCorpusTablesBuildUnderTheTreeLock: ci-corpus.sh builds a tree's tables holding that tree's lock, with the box
// lock released, so jobs whose tables differ build at once (11-22: queue-precheck's attribution probes, a tree per
// queue prefix, each 36-62 s of tables, queued one at a time behind the box lock). The tree is touched before the box
// lock is released, or another job's age trim could take a fresh cp -a copy (it keeps base's old mtime).
func TestCorpusTablesBuildUnderTheTreeLock(t *testing.T) {
	b, err := os.ReadFile("ci-corpus.sh")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	// each step is found after the one before it (the header comments name some of them)
	pos := 0
	for _, sub := range []string{`cp -a "$base" "$tree"`, `touch "$tree"`, "exec 9>&-", `exec 7>"$tree.lock"; flock 7`,
		"make dirac-tables", "exec 7>&-"} {
		i := strings.Index(s[pos:], sub)
		if i < 0 {
			t.Fatalf("ci-corpus.sh: no %q after byte %d; want cp -a, touch, release the box lock, take the tree lock, make dirac-tables, release it, in that order", sub, pos)
		}
		pos += i + len(sub)
	}
}

// TestTablesReuseOnlySoundTrees: a tree is marked .tables-ok — reusable by later checkouts — only by a tables build
// that left the checkout's committed bytes intact. kgen's exit 3 rewrites ids.tsv in the checkout's source tree and
// make rebuilds kgen around the rewritten bytes, so the published tables then carry a key (gorge.TableSourceHash)
// that exists only in that mutated workspace: a later fresh checkout of the same stack compiles its tests from the
// committed bytes and asks for tables under another key. 11-26: reusing across such a rewrite served every dirac
// check tables no committed tree could match (703 reds, 10 merge-queue batches bisected ~2 h, #5256 and #4754
// dequeued; the run that built the tree passed green — its tests compiled after the rewrite). The marker is the
// Makefile target's $(BIN_DIR)/.ids-rewritten, ci-corpus.sh consumes it and the tree key carries a rule epoch so
// trees marked under the earlier rule are orphaned, not served.
func TestTablesReuseOnlySoundTrees(t *testing.T) {
	b, err := os.ReadFile("ci-corpus.sh")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	// each step is found after the one before it: build, then the marker check, then .tables-ok only in the
	// no-rewrite else branch, then the tree lock released
	pos := 0
	for _, sub := range []string{"make dirac-tables", `if [ -e bin/.ids-rewritten ]`, "else", `touch "$tree/.tables-ok"`,
		"exec 7>&-"} {
		i := strings.Index(s[pos:], sub)
		if i < 0 {
			t.Fatalf("ci-corpus.sh: no %q after byte %d; want make dirac-tables, the ids.tsv-rewrite marker check, then touch .tables-ok in the else branch, then release the tree lock, in that order", sub, pos)
		}
		pos += i + len(sub)
	}
	mk, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	m := string(mk)
	if !strings.Contains(m, "BIN_DIR    := bin") {
		t.Error("Makefile: BIN_DIR is not bin; ci-corpus.sh consumes the marker at bin/.ids-rewritten")
	}
	if !strings.Contains(m, "rm -f $(BIN_DIR)/.ids-rewritten") {
		t.Error("Makefile: dirac-tables does not clear $(BIN_DIR)/.ids-rewritten at entry; a stale marker from an earlier run would mark every tree unsound")
	}
	if !strings.Contains(m, "if [ $$rc -eq 3 ]; then touch $(BIN_DIR)/.ids-rewritten") {
		t.Error("Makefile: dirac-tables does not leave the marker when kgen rewrote ids.tsv (exit 3)")
	}
	keys, err := os.ReadFile("ci-corpus-keys.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(keys), `tree=$root/tree/$ck-$dk-$RULE`) || !strings.Contains(string(keys), "RULE=2") {
		t.Error("ci-corpus-keys.sh: the tree key carries no reuse-rule epoch; trees marked before the rewrite guard would still be served")
	}
	if !strings.Contains(s, "ci-corpus-keys.sh") {
		t.Error("ci-corpus.sh does not source ci-corpus-keys.sh: its tree key must come from the one definition")
	}
}
