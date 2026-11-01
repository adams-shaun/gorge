package draftlab

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// testLab opens the lab over the shared corpus. Skips, honestly, when the
// corpus is absent.
func testLab(t *testing.T) *Lab {
	t.Helper()
	l, err := Open(testutil.CorpusDirOrSkip(t), Options{})
	if err != nil {
		t.Fatalf("draftlab Open: %v", err)
	}
	return l
}

// testDrafter resolves name as a drafter and fails the test on an error.
func testDrafter(t *testing.T, l *Lab, name string) NamedDrafter {
	t.Helper()
	d, err := l.Drafter(AgentRef{Name: name})
	if err != nil {
		t.Fatalf("Drafter(%q): %v", name, err)
	}
	return d
}

// testBuilder resolves name as a builder and fails the test on an error.
func testBuilder(t *testing.T, l *Lab, name string) NamedBuilder {
	t.Helper()
	b, err := l.Builder(AgentRef{Name: name})
	if err != nil {
		t.Fatalf("Builder(%q): %v", name, err)
	}
	return b
}

func TestOpenRefusesBadOptions(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	for _, tc := range []struct {
		name string
		opts Options
		want string
	}{
		{"workers", Options{Workers: -1}, "Workers"},
		{"maxTurns", Options{MaxTurns: -5}, "MaxTurns"},
		{"maxIntents", Options{MaxIntents: -1}, "MaxIntents"},
	} {
		_, err := Open(dir, tc.opts)
		if err == nil {
			t.Fatalf("%s: Open accepted a negative option", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not name %s", tc.name, err, tc.want)
		}
	}
}

func TestOptionsDefaults(t *testing.T) {
	l := testLab(t)
	o := l.Options()
	if o.MaxTurns != 150 || o.MaxIntents != 40000 || o.Mulligans != 2 {
		t.Errorf("defaults: got %d/%d/%d, want 150/40000/2", o.MaxTurns, o.MaxIntents, o.Mulligans)
	}
	if o.Workers != 0 || o.Verify || o.AllowDecklistBots {
		t.Errorf("defaults: run knobs mutated: %+v", o)
	}
}
