package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// onlyDealsDigest is the digest (stdout and event logs) of the MBC -only run
// below, recorded from the code before restrictSpec moved to setdata.Restrict.
const onlyDealsDigest = "4b8c8b3fb7df1c7ed9fbb4603dd199d2e34d58e3acb6938d30c87f56f6fa0a3f"

// TestOnlyDealsListedCardsOnly: an MBC -only event deals only listed names and
// its output is the bytes it was before the restriction moved to setdata.
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
	// The report names the temp list; hash it under a fixed name.
	h := sha256.New()
	h.Write([]byte(strings.ReplaceAll(out, list, "LIST")))
	for _, er := range res.events {
		h.Write(er.log)
	}
	got := hex.EncodeToString(h.Sum(nil))
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
	if got != onlyDealsDigest {
		t.Errorf("-only run digest %s, want %s", got, onlyDealsDigest)
	}
}
