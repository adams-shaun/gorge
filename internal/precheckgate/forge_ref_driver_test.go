package precheckgate

// scripts/check-forge-ref-driver.sh refuses a FORGE_REF move that the Forge oracle driver (the fork's oracle-driver
// branch) does not contain: on 2026-11-25 FORGE_REF moved to 5bc5851a54 ahead of the driver and every
// xproof cycle failed the pin invariant. The test runs the real script in a temporary repository with a stub gh.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	forgeRefOld = "ff076d4432844aaab45693a971ce84bfb95841fd"
	forgeRefNew = "5bc5851a54a0fbbbf7f1a38b48d8ff9b7714ddb2"
)

// runForgeRefCheck commits a Makefile at oldRef, writes newRef into the working tree and runs the script against
// the base commit with a stub gh that answers the compare API with status. It returns the exit code, the output and
// the stub's call log.
func runForgeRefCheck(t *testing.T, oldRef, newRef, status string, env ...string) (int, string, string) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "check-forge-ref-driver.sh"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	bin := filepath.Join(root, "bin")
	log := filepath.Join(root, "gh.log")
	gitIn(t, root, "init", "-q", repo)
	if err := os.WriteFile(filepath.Join(repo, "Makefile"), []byte("FORGE_REF  ?= "+oldRef+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "add", "Makefile")
	gitIn(t, repo, "commit", "-q", "-m", "base")
	if err := os.WriteFile(filepath.Join(repo, "Makefile"), []byte("# pin\nFORGE_REF  ?= "+newRef+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := "#!/usr/bin/env bash\necho \"gh $*\" >> \"$GH_LOG\"\ncase \"$2\" in\n" +
		"*/compare/*) echo \"$GH_STATUS\" ;;\n" +
		"*) exit 2 ;;\nesac\n"
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, "HEAD")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "GH_LOG="+log, "GH_STATUS="+status)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	return code, string(out), string(calls)
}

func TestForgeRefDriverUnchangedMakesNoCall(t *testing.T) {
	code, out, calls := runForgeRefCheck(t, forgeRefNew, forgeRefNew, "diverged")
	if code != 0 || !strings.Contains(out, "FORGE_REF unchanged") || calls != "" {
		t.Fatalf("code=%d out=%q calls=%q", code, out, calls)
	}
}

func TestForgeRefDriverContainedPasses(t *testing.T) {
	for _, st := range []string{"behind", "identical"} {
		code, out, calls := runForgeRefCheck(t, forgeRefOld, forgeRefNew, st)
		if code != 0 || !strings.Contains(out, "is contained in the driver oracle-driver on adams-shaun/forge") {
			t.Fatalf("%s: code=%d out=%q", st, code, out)
		}
		if !strings.Contains(calls, "gh api repos/adams-shaun/forge/compare/oracle-driver..."+forgeRefNew+" --jq .status") {
			t.Fatalf("%s: calls=%q", st, calls)
		}
	}
}

// The 2026-11-25 shape: FORGE_REF moved to a commit the driver lacks.
func TestForgeRefDriverNotContainedFails(t *testing.T) {
	for _, st := range []string{"diverged", "ahead"} {
		code, out, _ := runForgeRefCheck(t, forgeRefOld, forgeRefNew, st)
		if code != 1 || !strings.Contains(out, "is not contained in the Forge oracle driver") || !strings.Contains(out, "forge-driver-follow") {
			t.Fatalf("%s: code=%d out=%q", st, code, out)
		}
	}
}

func TestForgeRefDriverOverride(t *testing.T) {
	other := strings.Repeat("a", 40)
	code, out, calls := runForgeRefCheck(t, forgeRefOld, forgeRefNew, "behind", "FORGE_DRIVER_REF="+other)
	if code != 0 || !strings.Contains(calls, "compare/"+other+"...") {
		t.Fatalf("code=%d out=%q calls=%q", code, out, calls)
	}
}
