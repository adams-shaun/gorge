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
// Re-pinned when Reinforce (Swell of Courage) made one more MBC card playable,
// when Awaken made Earthen Arms and Encircling Fissure playable, and when
// stat:ManaRestriction made Hogaak, Arisen Necropolis playable (it left MBC's
// substitute list, so the deal names it instead of its substitute).
const onlyDealsDigest = "dd3c0ee80338789221dc2e57740ad33fb262be6209ec83f9b46890f8f305c950"

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
