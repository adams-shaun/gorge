package oracle

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const unrefusedRegister = "dirac/support/unrefused.tsv"

// unrefusalBase is the revision the check compares with: KERNEL_UNREFUSE_BASE (make precheck sets
// it to its BASE), else land. "" when neither resolves, which skips the test.
func unrefusalBase(t *testing.T) string {
	t.Helper()
	if b := os.Getenv("KERNEL_UNREFUSE_BASE"); b != "" {
		return b
	}
	if _, err := gitAtRoot("rev-parse", "--verify", "-q", "land"); err != nil {
		return ""
	}
	return "land"
}

func gitAtRoot(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = moduleRootFromOracle
	return cmd.Output()
}

// refusedCardsAt is the cards that have a refused entry at rev.
func refusedCardsAt(t *testing.T, rev string) map[string]bool {
	t.Helper()
	out, err := gitAtRoot("ls-tree", "--name-only", rev+":"+cardset.RefusedDir)
	if err != nil {
		return map[string]bool{} // no refused directory at rev
	}
	cards := map[string]bool{}
	for _, f := range strings.Fields(string(out)) {
		b, err := gitAtRoot("show", rev+":"+cardset.RefusedDir+"/"+f)
		if err != nil {
			t.Fatalf("git show %s:%s: %v", rev, f, err)
		}
		var e struct {
			Card string `json:"card"`
		}
		if err := json.Unmarshal(b, &e); err != nil || e.Card == "" {
			t.Fatalf("%s at %s: no card (%v)", f, rev, err)
		}
		cards[e.Card] = true
	}
	return cards
}

// parseUnrefused is the register's rows by card: card, proof and why, tab separated, # for comments.
func parseUnrefused(b []byte) (map[string]string, error) {
	rows := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 3 || strings.TrimSpace(f[0]) == "" || strings.TrimSpace(f[1]) == "" || strings.TrimSpace(f[2]) == "" {
			return nil, fmt.Errorf("%s:%d: want card<TAB>proof<TAB>why", unrefusedRegister, n)
		}
		if _, dup := rows[f[0]]; dup {
			return nil, fmt.Errorf("%s:%d: %s twice", unrefusedRegister, n, f[0])
		}
		rows[f[0]] = f[1]
	}
	return rows, sc.Err()
}

// unrefusalGaps names each card in before that is not in now and has no row in rows whose proof
// passes, one message per card, sorted by card.
func unrefusalGaps(before, now map[string]bool, rows map[string]string, proof func(string) error) []string {
	var gaps []string
	for _, c := range sortedKeys(before) {
		if now[c] {
			continue
		}
		p, ok := rows[c]
		if !ok {
			gaps = append(gaps, fmt.Sprintf("%s is no longer refused and has no row in %s: add the test that now passes and a row naming it, or re-record its game (pool custom)", c, unrefusedRegister))
			continue
		}
		if err := proof(p); err != nil {
			gaps = append(gaps, fmt.Sprintf("%s: proof %q: %v", c, p, err))
		}
	}
	return gaps
}

func sortedKeys(m map[string]bool) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}

var testFuncName = regexp.MustCompile(`^Test[A-Za-z0-9_]+$`)

// checkUnrefusalProof: proof is an existing TestXxx, or an existing repro that replays clean.
func checkUnrefusalProof(t *testing.T, proof string) error {
	t.Helper()
	const reproPrefix = "dirac/oracle/testdata/repro/"
	switch {
	case testFuncName.MatchString(proof):
		if !testFuncExists(proof) {
			return fmt.Errorf("no func %s in any _test.go file", proof)
		}
		return nil
	case strings.HasPrefix(proof, reproPrefix) && strings.HasSuffix(proof, ".repro.json"):
		if _, err := os.Stat(filepath.Join(moduleRootFromOracle, filepath.FromSlash(proof))); err != nil {
			return err
		}
		name := strings.TrimSuffix(strings.TrimPrefix(proof, reproPrefix), ".repro.json")
		res := replayRawRepro(t, name)
		if res.Div != nil || res.StallUnmodelled {
			return fmt.Errorf("does not replay clean: %+v", res.Div)
		}
		return nil
	}
	return fmt.Errorf("want a TestXxx or %s<name>.repro.json", reproPrefix)
}

func testFuncExists(name string) bool {
	re := regexp.MustCompile(`(?m)^func ` + name + `\(`)
	found := false
	filepath.WalkDir(moduleRootFromOracle, func(p string, d fs.DirEntry, err error) error {
		if found {
			return filepath.SkipAll
		}
		if err != nil {
			return nil
		}
		base := d.Name()
		if d.IsDir() && (strings.HasPrefix(base, ".") && p != moduleRootFromOracle || base == "node_modules" || base == "web") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(base, "_test.go") {
			if b, err := os.ReadFile(p); err == nil && re.Match(b) {
				found = true
			}
		}
		return nil
	})
	return found
}

// replayRawRepro replays a recorded game the way checkRefusedRepro does: by its script on a raw
// table of its own decks' cards.
func replayRawRepro(t *testing.T, name string) Result {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.Raw(printedDelveSet(t, r.Cards))
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}

// TestUnrefusalNeedsProof: a card that had a dirac/support/refused entry at the merge base with
// land and has none now must have a row in dirac/support/unrefused.tsv naming a TestXxx that
// exists or a recorded repro that replays clean. A stale entry means the defect is fixed or the
// pinned game moved; only the first is a reason to unrefuse.
func TestUnrefusalNeedsProof(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(moduleRootFromOracle, filepath.FromSlash(unrefusedRegister)))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := parseUnrefused(b)
	if err != nil {
		t.Fatal(err)
	}
	base := unrefusalBase(t)
	if base == "" {
		t.Log("no land ref and no KERNEL_UNREFUSE_BASE: nothing to compare with (make precheck sets it)")
		return
	}
	mb, err := gitAtRoot("merge-base", base, "HEAD")
	if err != nil {
		t.Fatalf("git merge-base %s HEAD: %v", base, err)
	}
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	now := map[string]bool{}
	for i := range entries {
		now[entries[i].Card] = true
	}
	before := refusedCardsAt(t, strings.TrimSpace(string(mb)))
	if len(before) == 0 {
		t.Fatalf("no refused entry at the merge base with %s: the check would pass vacuously", base)
	}
	for _, g := range unrefusalGaps(before, now, rows, func(p string) error { return checkUnrefusalProof(t, p) }) {
		t.Error(g)
	}
}

// TestUnrefusalGapsLogic: the check's own decisions, on made-up inputs.
func TestUnrefusalGapsLogic(t *testing.T) {
	before := map[string]bool{"A": true, "B": true, "C": true, "D": true}
	now := map[string]bool{"A": true}
	rows := map[string]string{"B": "TestOk", "C": "TestBad"}
	proof := func(p string) error {
		if p == "TestBad" {
			return fmt.Errorf("missing")
		}
		return nil
	}
	got := unrefusalGaps(before, now, rows, proof)
	if len(got) != 2 || !strings.HasPrefix(got[0], "C: proof") || !strings.HasPrefix(got[1], "D is no longer refused") {
		t.Fatalf("gaps = %q", got)
	}
	if _, err := parseUnrefused([]byte("# c\nX\tTestY\n")); err == nil {
		t.Fatal("a two-field row parsed")
	}
	if r, err := parseUnrefused([]byte("# c\nX\tTestY\twhy\n")); err != nil || r["X"] != "TestY" {
		t.Fatalf("parse = %v, %v", r, err)
	}
}
