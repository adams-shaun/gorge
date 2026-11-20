package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// fixtureFactsOpts stands in for the run's: a fixed HEAD and lock, and a
// script reader that hashes the script's path, so the golden does not depend
// on where the corpus sits on disk.
func fixtureFactsOpts() factsOpts {
	return factsOpts{
		GorgeSHA:    "0123456789abcdef0123456789abcdef01234567",
		ForgeCommit: "fedcba9876543210fedcba9876543210fedcba98",
		ForgeBase:   "forge-gui/res/cardsfolder",
		ReadScript: func(p string) ([]byte, error) {
			return []byte("script of " + filepath.Base(p)), nil
		},
	}
}

func fixtureFactsSupport() map[string]kgen.CardSupport {
	return map[string]kgen.CardSupport{
		"Sol Ring": {Status: cardset.StatusSupported, TwoSeat: true, LIRKey: "1111222233334444"},
		"Push": {Status: cardset.StatusNotLowered, TwoSeat: true, LIRKey: "aaaabbbbccccdddd",
			Nodes: []string{"T[1] Mode=Phase"}},
	}
}

func fixtureFactsBytes(t *testing.T) []byte {
	t.Helper()
	reg := twoCardCorpus(t)
	lower := func([]string) (lowerResult, error) {
		return lowerResult{keys: fixtureLIR(nil), support: fixtureFactsSupport()}, nil
	}
	rows, in, err := auditInputs(fixtureFS(), reg, lower, opts{})
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := writeFacts(&b, buildFacts(rows, in, fixtureFactsOpts())); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestFactsGolden(t *testing.T) {
	got := fixtureFactsBytes(t)
	const golden = "testdata/facts_golden.jsonl"
	if os.Getenv("PINS_RECORD") != "" {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("facts differ from %s\n--- got ---\n%s--- want ---\n%s", golden, got, want)
	}
	if n := bytes.Count(got, []byte("\n")); n != 4 {
		t.Errorf("got %d lines, want one per fixture card (4)", n)
	}
}

func TestFactsTwiceByteIdentical(t *testing.T) {
	a, b := fixtureFactsBytes(t), fixtureFactsBytes(t)
	if !bytes.Equal(a, b) {
		t.Fatalf("two runs differ:\n%s\n---\n%s", a, b)
	}
	names := []string{}
	for _, line := range bytes.Split(bytes.TrimSuffix(a, []byte("\n")), []byte("\n")) {
		var f fact
		if err := json.Unmarshal(line, &f); err != nil {
			t.Fatal(err)
		}
		names = append(names, f.Name)
	}
	if want := "Gone Card,Lost Card,Push,Sol Ring"; strings.Join(names, ",") != want {
		t.Fatalf("order = %v, want sorted %s", names, want)
	}
}

// TestFactsFields pins the values the golden would hide behind a re-record: a
// real card's script hash, its ref, its refusal and its private primitives.
func TestFactsFields(t *testing.T) {
	var by = map[string]fact{}
	for _, line := range bytes.Split(bytes.TrimSuffix(fixtureFactsBytes(t), []byte("\n")), []byte("\n")) {
		var f fact
		if err := json.Unmarshal(line, &f); err != nil {
			t.Fatal(err)
		}
		by[f.Name] = f
	}
	sol := by["Sol Ring"]
	sum := sha256.Sum256([]byte("script of sol_ring.txt"))
	if sol.ScriptSHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("Sol Ring script_sha256 = %q", sol.ScriptSHA256)
	}
	if want := "fedcba9876543210fedcba9876543210fedcba98:forge-gui/res/cardsfolder/s/sol_ring.txt"; sol.ForgeRef != want {
		t.Errorf("Sol Ring forge_ref = %q, want %q", sol.ForgeRef, want)
	}
	if sol.NormName != "sol ring" || sol.GorgeSHA == "" || len(sol.Private.Prims) == 0 || sol.Abilities.Abilities == 0 {
		t.Errorf("Sol Ring fact is missing a field: %+v", sol)
	}
	if sol.Refused != nil || sol.Lowering.Status != "supported" || sol.Keys != "3/4" || sol.Branches != "2/3" {
		t.Errorf("Sol Ring claims are wrong: %+v", sol)
	}
	push := by["Push"]
	if push.Refused == nil || push.Refused.Why != "fixture" || push.DClaim != "refused" {
		t.Errorf("Push refused = %+v d_claim %q, want the fixture refusal", push.Refused, push.DClaim)
	}
	if strings.Join(push.Lowering.Nodes, ";") != "T[1] Mode=Phase" || push.Lowering.Status != "not lowered" {
		t.Errorf("Push lowering = %+v", push.Lowering)
	}
	gone := by["Gone Card"]
	if gone.ForgeRef != "" || gone.ScriptSHA256 != "" || gone.Lowering.Nodes == nil || gone.Private.Prims == nil {
		t.Errorf("a card with no script must carry empty strings and empty lists, not null: %+v", gone)
	}
}

// TestFactsRefusedCardCarriesNode lowers a real card kgen refuses to lower
// (Bubbling Muck) and checks its fact names the IR node of the refusal.
func TestFactsRefusedCardCarriesNode(t *testing.T) {
	cardsDir := testutil.CorpusDirOrSkip(t)
	reg := twoCardCorpus(t)
	muck, ok := testutil.CorpusRegistry(t).Lookup("Bubbling Muck")
	if !ok {
		t.Fatal("Bubbling Muck is not in the corpus")
	}
	reg.Add(muck)
	support, err := kgen.SupportFor(cardsDir, []string{"Bubbling Muck"})
	if err != nil {
		t.Fatal(err)
	}
	if cs := support["Bubbling Muck"]; cs.Status != cardset.StatusNotLowered {
		t.Fatalf("precondition: Bubbling Muck status = %v, want not lowered", cs.Status)
	}
	lower := func([]string) (lowerResult, error) {
		return lowerResult{keys: map[string]string{}, support: support}, nil
	}
	fsys := fixtureFS()
	rows, in, err := auditInputs(fsys, reg, lower, opts{})
	if err != nil {
		t.Fatal(err)
	}
	var got *fact
	for _, r := range rows {
		if r.Card == "Bubbling Muck" {
			f := buildFact(r, in, factsOpts{})
			got = &f
		}
	}
	if got == nil {
		t.Fatal("Bubbling Muck has no row")
	}
	if len(got.Lowering.Nodes) == 0 || strings.TrimSpace(got.Lowering.Nodes[0]) == "" {
		t.Fatalf("Bubbling Muck lowering = %+v, want the node of its refusal", got.Lowering)
	}
	if got.Lowering.Status != "not lowered" {
		t.Errorf("Bubbling Muck lowering status = %q", got.Lowering.Status)
	}
}

func TestFactsNoScriptText(t *testing.T) {
	corpus := testutil.CorpusRegistry(t)
	c, ok := corpus.Lookup("Sol Ring")
	if !ok {
		t.Fatal("no Sol Ring")
	}
	out := string(fixtureFactsBytes(t))
	for _, f := range c.Faces {
		for _, text := range []string{f.Oracle, f.ManaCost} {
			if len(text) > 4 && strings.Contains(out, text) {
				t.Errorf("facts hold card text %q", text)
			}
		}
	}
	for _, tok := range []string{"AB$", "SP$", "Mode$", "DB$", "SVar:"} {
		if strings.Contains(out, tok) {
			t.Errorf("facts hold script syntax %q", tok)
		}
	}
}

func TestFactsPathInsideModule(t *testing.T) {
	root := t.TempDir()
	never := func(string, string) bool { return false }
	always := func(string, string) bool { return true }
	if err := checkFactsPath(root, filepath.Join(t.TempDir(), "f.jsonl"), never); err != nil {
		t.Errorf("a path outside the module was refused: %v", err)
	}
	inside := filepath.Join(root, "sub", "f.jsonl")
	if err := checkFactsPath(root, inside, never); err == nil {
		t.Error("a path inside the module that git does not ignore was accepted")
	}
	if err := checkFactsPath(root, inside, always); err != nil {
		t.Errorf("an ignored path inside the module was refused: %v", err)
	}
}

// TestFactsPathUsesGitIgnore runs the real check-ignore in a throwaway repo.
func TestFactsPathUsesGitIgnore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	if err := exec.Command("git", "-C", root, "init", "-q").Run(); err != nil {
		t.Skipf("git init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("out/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkFactsPath(root, filepath.Join(root, "out", "f.jsonl"), gitIgnored); err != nil {
		t.Errorf("a gitignored path was refused: %v", err)
	}
	if err := checkFactsPath(root, filepath.Join(root, "f.jsonl"), gitIgnored); err == nil {
		t.Error("a tracked-location path was accepted")
	}
}
