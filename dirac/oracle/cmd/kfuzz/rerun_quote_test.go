package main

import (
	"os"
	"strings"
	"testing"
)

// TestRerunCmdQuotesOddWords: a rerun command pastes back as the same argv, so
// an argument with a space or a quote is one quoted word.
func TestRerunCmdQuotesOddWords(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()
	os.Args = []string{"kfuzz", "-focus", "/tmp/a b/it's", "-pairs", "9", "-seed", "3"}
	got := rerunCmd(4)
	if !strings.Contains(got, `-focus '/tmp/a b/it'\''s'`) {
		t.Fatalf("rerunCmd = %q: want the -focus path as one quoted word", got)
	}
	if !strings.HasSuffix(got, "-seed 3 -first-pair 4 -pairs 1") {
		t.Fatalf("rerunCmd = %q: plain words stay bare and the pair range ends it", got)
	}
}
