package gorged

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// fakeDirac answers every deck with v and keeps the deck it was asked about.
type fakeDirac struct {
	v   adapter.DeckVerdict
	got []*cards.Card
}

func (f *fakeDirac) DeckSupport(d []*cards.Card) adapter.DeckVerdict {
	f.got = d
	return f.v
}

func refusing(name, reason string) *fakeDirac {
	return &fakeDirac{v: adapter.DeckVerdict{
		Set:   "fake",
		Cards: []adapter.CardVerdict{{Name: name, Count: 1, Status: "refused", Reason: reason}},
		Err:   errors.New(name + " is refused"),
	}}
}

// TestDeckReportDiracColumn: with a dirac column, a playable repo Commander
// deck's report stays gorge's at the top and carries dirac's verdict beside
// it, judged on the deck's 100 resolved cards; without one, the marshalled
// report has no dirac field.
func TestDeckReportDiracColumn(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cmdr, lines := repoCommanderText(t)
	text := listText(cmdr, lines)
	c := config{engine: "gorge", imports: newDeckImports(reg)}
	fake := refusing(cmdr, "r")
	c.imports.dirac = fake
	rep, err := c.checkDeck(httpapi.DeckCheckRequest{Text: text})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.got) != 100 {
		t.Fatalf("dirac judged %d cards, want the deck's 100", len(fake.got))
	}
	if !rep.Playable || len(rep.Unsupported) != 0 {
		t.Fatalf("gorge's verdict %+v: want the repo deck playable", rep)
	}
	want := protocol.DeckEngineVerdict{
		Engine: "dirac", Set: "fake", Playable: false,
		Unsupported: []protocol.DeckEngineCard{{Name: cmdr, Count: 1, Status: "refused", Reason: "r"}},
		Problems:    []string{},
	}
	if rep.Engine != "gorge" || rep.Dirac == nil || rep.Dirac.Engine != want.Engine || rep.Dirac.Set != want.Set ||
		rep.Dirac.Playable || !slices.Equal(rep.Dirac.Unsupported, want.Unsupported) || rep.Dirac.Problems == nil || len(rep.Dirac.Problems) != 0 {
		t.Fatalf("engine %q, dirac %+v: want gorge and %+v", rep.Engine, rep.Dirac, want)
	}

	c.imports.dirac = nil
	rep, err = c.checkDeck(httpapi.DeckCheckRequest{Text: text})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"engine":"gorge"`) || strings.Contains(string(raw), `"dirac"`) {
		t.Fatalf("with no dirac column the report is %s: want engine gorge and no dirac field", raw)
	}
}

// TestDeckReportEngineIsGorgeUnderDirac: under -engine dirac a report still
// names gorge as the engine its unsupported list measures, carries the
// startup-decks problem, and carries dirac's verdict.
func TestDeckReportEngineIsGorgeUnderDirac(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cmdr, lines := repoCommanderText(t)
	c := config{engine: adapter.Name, imports: newDeckImports(reg)}
	if !c.diracOn() {
		t.Fatal("config with engine dirac is not dirac")
	}
	c.imports.dirac = refusing(cmdr, "r")
	rep, err := c.checkDeck(httpapi.DeckCheckRequest{Text: listText(cmdr, lines)})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Engine != "gorge" || !slices.Contains(rep.Problems, diracDeckProblem) || rep.Dirac == nil || rep.Dirac.Engine != "dirac" {
		t.Fatalf("report %+v (dirac %+v): want engine gorge, the problem %q and a dirac verdict", rep, rep.Dirac, diracDeckProblem)
	}
}

// TestDeckDemandLogLinePerEngine: a checked deck writes one demand line per
// verdict, each under the engine whose support produced its names: gorge's
// card then dirac's; with no dirac column, gorge's line alone.
func TestDeckDemandLogLinePerEngine(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const a, b = "Lightning Bolt", "Some Dirac Card"
	if bolt, ok := reg.Lookup(a); !ok || len(bolt.Primitives()) == 0 {
		t.Fatalf("%s is not in the corpus with a primitive", a)
	}
	run := func(dirac diracSupport) string {
		dir := t.TempDir()
		c := config{engine: "gorge", dir: dir, deckDemandLog: true, imports: newDeckImports(reg)}
		c.imports.supported = map[string]bool{}
		c.imports.demand = c.demandLog()
		c.imports.demand.now = func() time.Time { return time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC) }
		c.imports.dirac = dirac
		if _, err := c.checkDeck(httpapi.DeckCheckRequest{Format: "constructed", Text: "1 " + a + "\n"}); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "demand", "unsupported.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	gorgeLine := `{"day":"2026-11-27","engine":"gorge","unsupported":["` + a + `"]}` + "\n"
	diracLine := `{"day":"2026-11-27","engine":"dirac","unsupported":["` + b + `"]}` + "\n"
	if got := run(refusing(b, "r")); got != gorgeLine+diracLine {
		t.Fatalf("with a dirac column the demand log is %q, want %q", got, gorgeLine+diracLine)
	}
	if got := run(nil); got != gorgeLine {
		t.Fatalf("with no dirac column the demand log is %q, want %q", got, gorgeLine)
	}
}
