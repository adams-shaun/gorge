package crcoverage

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var deviationClass = regexp.MustCompile(`^- \[(C|capacity)\] `)

// unclassedDeviations are text's rows, its unindented "- " lines, without a class tag.
func unclassedDeviations(text string) (rows int, bad []string) {
	for _, l := range strings.Split(text, "\n") {
		if !strings.HasPrefix(l, "- ") {
			continue
		}
		rows++
		if !deviationClass.MatchString(l) {
			bad = append(bad, l)
		}
	}
	return rows, bad
}

// TestDeviationsRegisterHasNoRealDifference: the public register holds only
// canonical-surface rows and declared capacity limits. A real difference from
// gorge is fixed or refused, so it has no place to be written down as accepted.
func TestDeviationsRegisterHasNoRealDifference(t *testing.T) {
	b, err := os.ReadFile("../docs/DEVIATIONS.md")
	if err != nil {
		t.Fatal(err)
	}
	rows, bad := unclassedDeviations(string(b))
	if rows == 0 {
		t.Fatal("no rows: the register's format changed, so this test checks nothing")
	}
	for _, l := range bad {
		t.Errorf("row is neither [C] nor [capacity]: %s", l)
	}
	if _, bad := unclassedDeviations("- [C] a\n- [real] b\n- c\n  - [real] nested\n"); len(bad) != 2 {
		t.Errorf("the detector found %d of the 2 unclassed rows", len(bad))
	}
}
