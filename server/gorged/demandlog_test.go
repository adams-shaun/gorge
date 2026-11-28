package gorged

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDeckDemandLogWritesNamesOnly: with -deck-demand-log on, a check of a
// named deck with one unsupported card and one unknown name writes exactly
// the day, the engine and the unsupported card's name; off, it writes
// nothing at all.
func TestDeckDemandLogWritesNamesOnly(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const card = "Lightning Bolt"
	bolt, ok := reg.Lookup(card)
	if !ok || len(bolt.Primitives()) == 0 {
		t.Fatalf("%s is not in the corpus with a primitive", card)
	}
	body, err := json.Marshal(map[string]any{"name": "my secret deck", "cards": []map[string]any{
		{"identity": map[string]string{"name": card}}, {"identity": map[string]string{"name": "Sol Rnig"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	run := func(on bool) (string, protocol.DeckReport) {
		dir := t.TempDir()
		c := config{engine: "gorge", dir: dir, deckDemandLog: on, imports: newDeckImports(reg)}
		c.imports.supported = map[string]bool{}
		c.imports.demand = c.demandLog()
		if c.imports.demand != nil {
			// 02:00 on the 28th at UTC+5 is the 27th in UTC.
			c.imports.demand.now = func() time.Time { return time.Date(2026, 11, 28, 2, 0, 0, 0, time.FixedZone("east", 5*3600)) }
		}
		rep, err := c.checkDeck(httpapi.DeckCheckRequest{Format: "constructed", Deck: body})
		if err != nil {
			t.Fatal(err)
		}
		return dir, rep
	}

	dir, rep := run(true)
	if rep.Name != "my secret deck" || len(rep.Unsupported) != 1 || rep.Unsupported[0].Name != card || len(rep.Unknown) != 1 {
		t.Fatalf("report %+v: want the named deck with %s unsupported and Sol Rnig unknown", rep, card)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "demand", "unsupported.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"day":"2026-11-27","engine":"gorge","unsupported":["` + card + `"]}` + "\n"
	if string(raw) != want {
		t.Fatalf("demand log = %q, want %q", raw, want)
	}
	for _, leak := range []string{"secret", "Rnig"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("demand log carries %q: %s", leak, raw)
		}
	}

	dir, _ = run(false)
	if _, err := os.Stat(filepath.Join(dir, "demand")); !os.IsNotExist(err) {
		t.Fatalf("with the flag off, %s/demand: %v, want it absent", dir, err)
	}
}

// TestDeckDemandLogRotates: past max the file moves to .1 and a new one
// starts, so the two never hold more than max plus a line each.
func TestDeckDemandLogRotates(t *testing.T) {
	l := newDemandLog(t.TempDir())
	l.max = 200
	l.now = func() time.Time { return time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC) }
	rep := protocol.DeckReport{Unsupported: []protocol.DeckReportCard{{Name: "Some Card With A Long Name"}, {Name: "Another Card"}}}
	for range 10 {
		l.record("gorge", rep)
	}
	line, _ := json.Marshal(demandLine{Day: "2026-11-27", Engine: "gorge", Unsupported: []string{"Another Card", "Some Card With A Long Name"}})
	total := 0
	for _, p := range []string{l.path, l.path + ".1"} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Size() > l.max+int64(len(line))+1 {
			t.Errorf("%s is %d bytes, past %d plus one %d-byte line", p, st.Size(), l.max, len(line)+1)
		}
		total += int(st.Size())
	}
	if total >= 10*(len(line)+1) {
		t.Errorf("the two files hold %d bytes of %d records: nothing rotated away", total, 10)
	}
}
