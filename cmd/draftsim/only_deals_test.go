package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// TestOnlyDealsListedCardsOnly: an MBC -only event deals only the names its
// allow-list, computed from the set's source sheets, allows; its output and
// event log are deterministic. The old digest literal moved with every card
// support change; the allow-list is measured here instead, so a support PR
// never has to re-pin it.
func TestOnlyDealsListedCardsOnly(t *testing.T) {
	dir := corpusReady(t)
	spec, err := setdata.LoadCollation("MBC", "draft")
	if err != nil {
		t.Fatal(err)
	}
	newCmd := map[string]bool{}
	for _, c := range spec.Config.Sheets["newCommander"].Cards {
		newCmd[c.Name] = true
	}
	var lines []string
	allowed := onlyList{}
	for i, n := range sheetNamesOf(spec) {
		if i%2 == 0 && !newCmd[n] {
			lines = append(lines, n+"\ttrue")
			allowed[n] = true
		} else {
			lines = append(lines, n+"\tfalse")
		}
	}
	if len(allowed) == 0 {
		t.Fatal("empty allow-list")
	}
	list := filepath.Join(t.TempDir(), "half.tsv")
	if err := os.WriteFile(list, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := defaultRuns(dir)[0]
	o.only, o.events = list, 1
	_, res, out := runDigest(t, o)
	if len(res.events) == 0 {
		t.Fatal("no events ran")
	}
	if !strings.Contains(out, "  sheet newCommander: 0/60 entries allowed") {
		t.Fatalf("the list did not empty newCommander, so the borrow path is unexercised:\n%s", out)
	}
	for _, er := range res.events {
		for _, c := range dealt(t, er.log) {
			if !allowed.allows(c.Name) {
				t.Fatalf("dealt %q, which the list does not name", c.Name)
			}
		}
	}
	// The same source list and seed must reproduce the same bytes: this is the
	// regression pin the digest carried, without a committed value that two
	// support PRs each have to move.
	_, again, outAgain := runDigest(t, o)
	if out != outAgain {
		t.Errorf("-only run is not deterministic:\nfirst:\n%s\nsecond:\n%s", out, outAgain)
	}
	if len(again.events) != len(res.events) {
		t.Fatalf("-only run produced %d events, then %d", len(res.events), len(again.events))
	}
	for i := range res.events {
		if string(res.events[i].log) != string(again.events[i].log) {
			t.Errorf("-only event %d log is not deterministic", i+1)
		}
	}
}
