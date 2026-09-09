package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Keep wall time and run count unchanged: only parsed skips can refuse this row.
func TestParsedSkipStreamRefusesVacuousHistory(t *testing.T) {
	const pkg = "example.invalid/fixture"
	var stream strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&stream, "{\"action\":\"run\",\"Package\":%q,\"Test\":\"Test%d\"}\n", pkg, i)
		action := "pass"
		if i < 80 {
			action = "skip"
		}
		fmt.Fprintf(&stream, "{\"action\":%q,\"Package\":%q,\"Test\":\"Test%d\"}\n", action, pkg, i)
	}
	fmt.Fprintf(&stream, "{\"action\":\"pass\",\"Package\":%q,\"Elapsed\":10}\n", pkg)
	res := parseJSON(strings.NewReader(stream.String()))[pkg]
	if res.tests != 100 || res.skipped != 80 {
		t.Fatalf("TEST_HISTORY.md header: must distinguish ran from skipped; got %+v", res)
	}
	path := filepath.Join(t.TempDir(), "TEST_HISTORY.md")
	before := "# Test history\n\nbudget_s: 30\n\n| date (UTC) | commit | wall_s | tests | skipped | runner |\n|---|---|---|---|---|---|\n| prior | base | 10 | 100 | 0 | fixture |\n"
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	code, wrote := measurePackage("fixture", path, pkg, "now", "base", "jj11", res)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code != exitInfra || wrote || string(after) != before {
		t.Fatalf("TEST_HISTORY.md header: vacuous skip jump must refuse recording; code=%d wrote=%v", code, wrote)
	}
	t.Logf("parsed tests=%d skipped=%d; exit=%d wrote=%v history unchanged", res.tests, res.skipped, code, wrote)
}
