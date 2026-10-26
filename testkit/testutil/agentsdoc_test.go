package testutil

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The Known-approximations register is a CLOSING REGISTER, frozen on
// 2026-09-22 (it had grown to ~100 rows and 264 KB in AGENTS.md, loaded into
// every agent's context on every turn). On 2026-10-26 it moved out of AGENTS.md
// into one file per row, docs/agents/known-approximations/<slug>.md, so that
// tickets closing different rows never edit the same line.
//
// ALLOWED.txt lists every remaining file with its byte cap. This test is the
// ratchet: a file not listed fails (no row may be added), a file over its cap
// fails (no row may grow), and a listed file that is gone fails until its line
// is deleted too. Closing a row = delete its file and its ALLOWED.txt line.
const knownApproxDir = "docs/agents/known-approximations"

func knownApproxRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return filepath.Join(root, knownApproxDir)
}

// knownApproxAllowed reads ALLOWED.txt: slug -> byte cap.
func knownApproxAllowed(t *testing.T, dir string) map[string]int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "ALLOWED.txt"))
	if err != nil {
		t.Fatalf("read %s/ALLOWED.txt: %v", knownApproxDir, err)
	}
	allowed := map[string]int{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 {
			t.Fatalf("ALLOWED.txt: want `<slug> <max bytes>`, got %q", line)
		}
		n, err := strconv.Atoi(f[1])
		if err != nil {
			t.Fatalf("ALLOWED.txt: bad byte cap in %q", line)
		}
		allowed[f[0]] = n
	}
	return allowed
}

func TestKnownApproximationsOnlyShrinks(t *testing.T) {
	dir := knownApproxRoot(t)
	allowed := knownApproxAllowed(t, dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", knownApproxDir, err)
	}
	present := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == "ALLOWED.txt" {
			continue
		}
		slug, ok := strings.CutSuffix(name, ".md")
		if !ok {
			t.Errorf("%s/%s: only <slug>.md files and ALLOWED.txt belong here", knownApproxDir, name)
			continue
		}
		present[slug] = true
		limit, listed := allowed[slug]
		if !listed {
			t.Errorf("%s/%s is not in ALLOWED.txt. The register is CLOSED: no row may be added. "+
				"A deviation your ticket cannot close goes in the COMMIT MESSAGE and the ticket "+
				"report; the operator files a ticket if it needs tracking.", knownApproxDir, name)
			continue
		}
		info, err := e.Info()
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if int(info.Size()) > limit {
			t.Errorf("%s/%s is %d bytes, over its ALLOWED.txt cap of %d: a row may never grow.",
				knownApproxDir, name, info.Size(), limit)
		}
	}
	var stale []string
	for slug := range allowed {
		if !present[slug] {
			stale = append(stale, slug)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("ALLOWED.txt lists closed rows %v: delete their lines in the same commit that "+
			"deleted the files.", stale)
	}
	t.Logf("known approximations: %d rows remain", len(present))
}
