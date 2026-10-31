package archtest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRootModuleHasNoRequires enforces invariant 7 (docs/agents/invariants.md):
// the root go.mod carries no require directive, so the card pipeline, the
// rules core and everything else in the root module build from the standard
// library alone. It also holds go.sum empty or absent, since a checksum line is
// only ever there for a dependency.
//
// An experiment that genuinely needs a third-party module lives in its own
// module of its own (a companion module's game pool requires purego for its CUDA binding)
// and imports the root module through a replace; the root never requires it
// back. wazero, the one planned exception (the M3 plugin tier), lands by
// changing this test in the same commit, under an operator decision.
//
// It reads go.mod as text rather than through golang.org/x/mod, which would
// itself be a dependency: a directive is the first token of a line outside a
// comment, and `require` opens either a single line or a parenthesised block.
func TestRootModuleHasNoRequires(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read root go.mod: %v", err)
	}
	if bad := requireLines(string(data)); len(bad) > 0 {
		t.Errorf("the root go.mod has a require directive (invariant 7: no third-party "+
			"dependencies in the root module):\n\t%s\n"+
			"move the code that needs it into a module of its own "+
			"instead of adding the dependency here", strings.Join(bad, "\n\t"))
	}

	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read root go.sum: %v", err)
	}
	if s := strings.TrimSpace(string(sum)); s != "" {
		t.Errorf("the root go.sum is not empty, so the module resolved a dependency "+
			"(invariant 7); run `go mod tidy` and delete it:\n%s", s)
	}
}

// requireLines returns every go.mod line that is, or opens, a require
// directive.
func requireLines(gomod string) []string {
	var bad []string
	for i, line := range strings.Split(gomod, "\n") {
		if c := strings.Index(line, "//"); c >= 0 {
			line = line[:c]
		}
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "require")
		if !ok {
			continue
		}
		if rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '(' {
			bad = append(bad, fmt.Sprintf("go.mod:%d: %s", i+1, line))
		}
	}
	return bad
}

// TestRequireLinesParsesDirectives pins the scanner the guard above relies on,
// so a reformatted go.mod cannot slip a require past it.
func TestRequireLinesParsesDirectives(t *testing.T) {
	cases := []struct {
		gomod string
		want  int
	}{
		{"module m\n\ngo 1.25\n", 0},
		{"module m\nrequire example.com/x v1.0.0\n", 1},
		{"module m\nrequire (\n\texample.com/x v1.0.0\n)\n", 1},
		{"module m\nrequire(\n\texample.com/x v1.0.0\n)\n", 1},
		{"module m\n  require\texample.com/x v1.0.0 // indirect\n", 1},
		{"module m\n// require example.com/x v1.0.0\n", 0},
		{"module m\nrequirements example.com/x v1.0.0\n", 0},
		{"module m\nreplace example.com/x => ../x\n", 0},
	}
	for _, c := range cases {
		if got := len(requireLines(c.gomod)); got != c.want {
			t.Errorf("requireLines(%q) = %d lines, want %d", c.gomod, got, c.want)
		}
	}
}
