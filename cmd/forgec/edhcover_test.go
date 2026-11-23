package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const edhTestManifest = "source\tid\tkind\tname\tcommander\tbracket\toverall_rank\tbracket_rank\tviews\tcards\treleased\turl\tfetched\n" +
	"mtgjson\tGood_C99\tprecon\tGood\t\t\t\t\t\t3\t2099-01-01\tu\t2099-01-01\n" +
	"mtgjson\tPool_CM9\tcollection\tPool\t\t\t\t\t\t1\t2099-01-01\tu\t2099-01-01\n" +
	"archidekt\t1\tpopular\tBad\tX\t3\t1\t2\t9\t3\t\tu\t2099-01-01\n" +
	"archidekt\t2\tpopular\tUnknown\tX\t5\t600\t1\t9\t2\t\tu\t2099-01-01\n" +
	"archidekt\t3\tpopular\tMissing file\tX\t2\t2\t1\t9\t2\t\tu\t2099-01-01\n"

func TestReadEDHManifest(t *testing.T) {
	t.Parallel()
	rows, err := readEDHManifest(strings.NewReader(edhTestManifest))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(rows))
	}
	if r := rows[2]; r.Bracket != 3 || r.OverallRank != 1 || r.BracketRank != 2 || r.path != filepath.Join("popular", "archidekt-1.json") {
		t.Errorf("archidekt row = %+v", r)
	}
	if r := rows[0]; r.path != filepath.Join("precon", "Good_C99.json") || r.Bracket != 0 {
		t.Errorf("mtgjson row = %+v", r)
	}
	if _, err := readEDHManifest(strings.NewReader("source\tid\n")); err == nil {
		t.Error("a manifest missing columns was accepted")
	}
	if _, err := readEDHManifest(strings.NewReader(strings.Replace(edhTestManifest, "archidekt\t1", "moxfield\t1", 1))); err == nil {
		t.Error("an unknown source was accepted")
	}
}

func TestEDHListsMembership(t *testing.T) {
	t.Parallel()
	rows, err := readEDHManifest(strings.NewReader(edhTestManifest))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, l := range edhLists() {
		for _, r := range rows {
			if l.in(r) {
				got[l.Name] = append(got[l.Name], r.ID)
			}
		}
	}
	want := map[string]string{
		"precon (all Commander Deck products)": "Good_C99",
		"precon + Arsenal/Collection pools":    "Good_C99 Pool_CM9",
		"Archidekt top 100":                    "1 3",
		"Archidekt top 500":                    "1 3",
		"Archidekt top 1000":                   "1 2 3",
		"Archidekt bracket 2 top 100":          "3",
		"Archidekt bracket 3 top 100":          "1",
		"Archidekt bracket 5 top 100":          "2",
		"Archidekt bracket 1 top 100":          "",
	}
	for list, w := range want {
		if g := strings.Join(got[list], " "); g != w {
			t.Errorf("%s = %q, want %q", list, g, w)
		}
	}
}

func TestReadDiracSupported(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for p, body := range map[string]string{
		"evidence/a.txt":        "# comment\nSol Ring\nDominate\n\n",
		"evidence/b.txt":        "Lightning Bolt\n",
		"refused/dominate.json": `{"card": "Dominate", "why": "x"}`,
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := readDiracSupported(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got["sol ring"] || !got["lightning bolt"] {
		t.Errorf("got %v", got)
	}
	if none, err := readDiracSupported(filepath.Join(dir, "absent")); err != nil || none != nil {
		t.Errorf("absent dir = %v, %v", none, err)
	}
}

func TestReadEDHDecklists(t *testing.T) {
	t.Parallel()
	got, err := readEDHDecklists(strings.NewReader(
		`{"source":"mtgjson","id":"Good_C99","kind":"precon","commanders":["Boss"],"cards":["Mountain","Mountain","Sol Ring"]}` + "\n" +
			`{"source":"archidekt","id":"7","kind":"popular","commanders":[],"cards":["Island"]}` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := deck.Parse(got[filepath.Join("precon", "Good_C99.json")])
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Cards) != 3 || f.Cards[0].Name != "Boss" || f.Cards[1].Count != 2 {
		t.Errorf("cards = %+v", f.Cards)
	}
	if _, ok := got[filepath.Join("popular", "archidekt-7.json")]; !ok {
		t.Errorf("archidekt deck not indexed: %v", got)
	}
	if _, err := readEDHDecklists(strings.NewReader(`{"source":"x","id":"1"}`)); err == nil {
		t.Error("an unknown source was accepted")
	}
}

// TestEDHCoverSkipsWithoutLists: absent decklists are not an error, and a
// stale report is removed rather than republished.
func TestEDHCoverSkipsWithoutLists(t *testing.T) {
	t.Parallel()
	out := t.TempDir()
	for _, f := range []string{"edh.md", "edh.json"} {
		if err := os.WriteFile(filepath.Join(out, f), []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	none := filepath.Join(t.TempDir(), "absent")
	if err := edhCoverCmd(t.TempDir(), none+".jsonl", none+".tsv", "", out, 10); err != nil {
		t.Fatalf("absent lists: %v", err)
	}
	for _, f := range []string{"edh.md", "edh.json"} {
		if _, err := os.Stat(filepath.Join(out, f)); !os.IsNotExist(err) {
			t.Errorf("%s survived: %v", f, err)
		}
	}
	if edhInputsPresent(t.TempDir()) {
		t.Error("a directory counted as an input file")
	}
}

func TestEDHPaperWithoutCard(t *testing.T) {
	t.Parallel()
	if edhPaper("A-Alrund's Epiphany", nil) {
		t.Error("an Alchemy A- name ranked as paper")
	}
	if !edhPaper("Sol Ring", nil) {
		t.Error("an ordinary name ranked as non-paper")
	}
}

// TestMeasureEDHOverCorpus measures four tiny decks: one fully playable, one
// blocked by Shahrazad (unplayable: api:Subgame has no primitive), one blocked
// by a name the corpus lacks, and a collection pool. Split, DFC and A- names resolve.
//
// Not parallel: it sets the shared corpus registry's ShapeGate, as the shape
// gate's own test does.
func TestMeasureEDHOverCorpus(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("precon/Good_C99.json", `{"name":"Good","format":"commander","cards":[{"name":"Lightning Bolt","count":1},{"name":"Fire // Ice","count":1},{"name":"Mountain","count":30}]}`)
	write("precon/Pool_CM9.json", `{"name":"Pool","format":"commander","cards":[{"name":"Sol Ring","count":1}]}`)
	write("popular/archidekt-1.json", `{"name":"Bad","format":"commander","cards":[{"name":"Shahrazad","count":1},{"name":"Lightning Bolt","count":1},{"name":"Delver of Secrets // Insectile Aberration","count":1}]}`)
	write("popular/archidekt-2.json", `{"name":"Unknown","format":"commander","cards":[{"name":"Not A Real Card Anywhere","count":1},{"name":"Shahrazad","count":1}]}`)
	rows, err := readEDHManifest(strings.NewReader(edhTestManifest))
	if err != nil {
		t.Fatal(err)
	}
	old := r.ShapeGate
	r.ShapeGate = etbCopyShapeGate
	rep, err := measureEDH(r, func(p string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, p)) }, rows, map[string]bool{"lightning bolt": true, "mountain": true, "fire": true, "sol ring": true})
	r.ShapeGate = old
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Skipped) != 1 || rep.Skipped[0] != filepath.Join("popular", "archidekt-3.json") {
		t.Errorf("skipped = %v", rep.Skipped)
	}
	if strings.Join(rep.Unresolved, "|") != "Not A Real Card Anywhere" {
		t.Errorf("unresolved = %v", rep.Unresolved)
	}
	lists := map[string]edhListResult{}
	for _, l := range rep.Lists {
		lists[l.Name] = l
	}
	if _, ok := lists["Archidekt bracket 4 top 100"]; ok {
		t.Error("a list with no deck files was reported")
	}
	if l := lists["precon (all Commander Deck products)"]; l.Decks != 1 || l.DecksSupported != 1 || l.Cards != 3 || l.CardsSupported != 3 {
		t.Errorf("precon = %+v", l)
	}
	if l := lists["Archidekt top 1000"]; l.Decks != 2 || l.DecksSupported != 0 || l.Cards != 4 || l.CardsSupported != 2 || l.Unresolved != 1 {
		t.Errorf("top 1000 = %+v", l)
	}
	if l := lists["precon (all Commander Deck products)"]; l.DiracDecks != 1 || l.DiracCards != 3 {
		t.Errorf("precon dirac = %+v", l)
	}
	if l := lists["Archidekt top 1000"]; l.DiracDecks != 0 || l.DiracCards != 1 {
		t.Errorf("top 1000 dirac = %+v", l)
	}
	if len(rep.Blockers) != 2 {
		t.Fatalf("blockers = %+v", rep.Blockers)
	}
	if b := rep.Blockers[0]; b.Card != "Shahrazad" || b.Decks != 2 || b.SoleDecks != 1 || !b.Paper || len(b.Missing) == 0 {
		t.Errorf("first blocker = %+v", b)
	}
	if b := rep.Blockers[1]; !b.Unresolved || b.Decks != 1 || b.SoleDecks != 0 {
		t.Errorf("second blocker = %+v", b)
	}
	if md := renderEDH(rep, 50); !strings.Contains(md, "| 1 | Shahrazad | 2 | 1 | yes |") {
		t.Errorf("render lacks the Shahrazad row:\n%s", md)
	}
}
