package scripts

// scripts/regen-main.sh decides, from the open regen/main PR, whether to leave it or rebuild it. 11-28: #5600 sat
// 13 hours as a draft that conflicted and was dequeued while every run printed "left alone", because GitHub answers
// UNKNOWN for a PR nobody has asked about and only CONFLICTING was rebuilt. The script runs here with a stub gh on
// PATH, in a scratch git repo whose remote does not exist: "fetch failed" means it chose to rebuild.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runRegenSelect runs regen-main.sh with gh answering each `gh pr list` call with the next line of answers (the last
// line repeats).
func runRegenSelect(t *testing.T, answers ...string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "answers"), []byte(strings.Join(answers, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gh := `#!/bin/sh
d=` + dir + `
n=$(cat "$d/n" 2>/dev/null || echo 0); n=$((n+1)); echo $n >"$d/n"
total=$(wc -l <"$d/answers"); [ "$n" -gt "$total" ] && n=$total
sed -n "${n}p" "$d/answers"
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0o755); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "repo")
	for _, args := range [][]string{{"init", "-q", repo}, {"-C", repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	script, err := filepath.Abs("regen-main.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, "no-such-remote")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "REGEN_UNKNOWN_WAIT=0")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func TestRegenMainLeavesAMergeablePR(t *testing.T) {
	if out := runRegenSelect(t, "5600 MERGEABLE queue"); !strings.Contains(out, "left alone") {
		t.Fatalf("a mergeable regen PR must be left alone; got:\n%s", out)
	}
}

func TestRegenMainRebuildsStuckPRs(t *testing.T) {
	for _, state := range []string{"CONFLICTING", "RED", "DRAFT", "DEQUEUED"} {
		if out := runRegenSelect(t, "5600 "+state+" queue"); !strings.Contains(out, "fetch failed") {
			t.Errorf("%s: a regen PR that cannot merge must be rebuilt; got:\n%s", state, out)
		}
	}
}

func TestRegenMainAsksAgainOnUnknown(t *testing.T) {
	if out := runRegenSelect(t, "5600 UNKNOWN queue", "5600 CONFLICTING queue"); !strings.Contains(out, "fetch failed") {
		t.Fatalf("UNKNOWN then CONFLICTING must rebuild; got:\n%s", out)
	}
	if out := runRegenSelect(t, "5600 UNKNOWN queue", "5600 MERGEABLE queue"); !strings.Contains(out, "left alone") {
		t.Fatalf("UNKNOWN then MERGEABLE must leave it alone; got:\n%s", out)
	}
}
