package kgen

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// twoSeatWord is how a lowering comment names a reduction that holds only
// with two players.
var twoSeatWord = regexp.MustCompile(`(?i)two[- ](player|seat)`)

// TestTwoSeatMarkerCensus: every comment in kgen that names a two-player
// reduction sits beside an lc.twoSeat call (12 lines before it to 40 after,
// within one top-level declaration), or its comment block says why none is
// needed: "seat-exact" (the reading holds at any seat count) or
// "twoSeat-at-caller" (a predicate whose caller marks the card). A refusal's
// message and the op docs (opsdoc.go) name the kernel's game, not a
// reduction, and are not read.
func TestTwoSeatMarkerCensus(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sites := 0
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") || n == "opsdoc.go" || n == "lower_twoseat.go" {
			continue
		}
		src, err := os.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(src), "\n")
		for i, l := range lines {
			c := strings.Index(l, "//")
			if c < 0 || !twoSeatWord.MatchString(l[c:]) || strings.Contains(l, "refuse(") {
				continue
			}
			sites++
			if !twoSeatCovered(lines, i) {
				t.Errorf("%s:%d names a two-player reduction with no lc.twoSeat call beside it (or seat-exact / twoSeat-at-caller in its comment): %s", n, i+1, strings.TrimSpace(l))
			}
		}
	}
	if sites == 0 {
		t.Fatal("no two-player comment found: the census reads nothing")
	}
	t.Logf("%d two-player comment lines, each marked or explained", sites)
}

func twoSeatCovered(lines []string, i int) bool {
	isComment := func(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), "//") }
	// the comment block holding line i
	lo, hi := i, i
	for lo > 0 && isComment(lines[lo-1]) {
		lo--
	}
	for hi+1 < len(lines) && isComment(lines[hi+1]) {
		hi++
	}
	for _, l := range lines[lo : hi+1] {
		if strings.Contains(l, "seat-exact") || strings.Contains(l, "twoSeat-at-caller") {
			return true
		}
	}
	if strings.Contains(lines[i][strings.Index(lines[i], "//"):], "seat-exact") {
		return true
	}
	call := func(l string) bool { return strings.Contains(l, "lc.twoSeat(") }
	for j := i; j <= i+40 && j < len(lines); j++ {
		if call(lines[j]) {
			return true
		}
		if j > i && lines[j] == "}" {
			break
		}
	}
	for j := i - 1; j >= i-12 && j >= 0; j-- {
		if lines[j] == "}" {
			break
		}
		if call(lines[j]) {
			return true
		}
	}
	return false
}
