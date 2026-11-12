package archtest

import (
	"strings"
	"testing"
)

// TestTaskTagRejectedInEngine pins the pipeline tag form the boundary added
// for engine/: a lowercase "(task <id>)" is a ticket label a reader of the
// public tree cannot resolve, while the capitalised English use of "Task" is
// not the label form.
func TestTaskTagRejectedInEngine(t *testing.T) {
	cases := []struct {
		path string
		blob string
		want string
	}{
		{"engine/x.go", "// the rider (task api-winsgame) leaked\n", "unresolvable-reference"},
		{"engine/rules/x.go", "// (task foo-bar)\n", "unresolvable-reference"},
		{"draft/x.go", "// (task foo-bar)\n", "unresolvable-reference"},
		{"engine/x.go", "// Task 4 is prose, not the label form\n", ""},
		{"docs/x.md", "// (task foo-bar) outside the label-free trees\n", ""},
	}
	for _, c := range cases {
		got := publicBoundaryViolations(c.path, []byte(c.blob))
		switch {
		case c.want == "" && len(got) != 0:
			t.Errorf("%s %.40q: want clean, got %q", c.path, c.blob, got)
		case c.want != "" && (len(got) != 1 || !strings.HasPrefix(got[0], c.want+":")):
			t.Errorf("%s %.40q: want %s, got %q", c.path, c.blob, c.want, got)
		}
	}
}
