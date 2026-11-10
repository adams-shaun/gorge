package kgen

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const testRoot = "../.."

func refusedOnDisk(t *testing.T) []cardset.RefusedEntry {
	t.Helper()
	entries, err := cardset.ReadRefused(os.DirFS(testRoot))
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestRefusedEntriesWellFormed(t *testing.T) {
	entries := refusedOnDisk(t)
	all := map[string]bool{}
	if len(entries) > 0 {
		b, err := os.ReadFile(testSetlistPath(t, "ALL"))
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range parseCardList(b) {
			all[n] = true
		}
	}
	for i := range entries {
		e := &entries[i]
		slug := cardset.SupportSlug(e.Card)
		if _, err := os.Stat(refusedPath(testRoot, e.Card)); err != nil {
			t.Errorf("%s: not at its slug %s: %v", e.Card, slug, err)
		}
		if !all[e.Card] {
			t.Errorf("%s: not a corpus card", e.Card)
		}
		if e.Class == "" || e.Why == "" {
			t.Errorf("%s: class %q, why %q", e.Card, e.Class, e.Why)
		}
		r := e.Repro
		if _, err := parseRefusedRepro(fmt.Sprintf("%s:%s:%d:%s", r.Pool, r.Surface, r.Pair, r.Policy)); err != nil {
			t.Errorf("%s: %v", e.Card, err)
		}
	}
}

func TestEvidenceAndRefusedAreDisjoint(t *testing.T) {
	s, err := readSupport(os.DirFS(testRoot))
	if err != nil {
		t.Fatal(err)
	}
	for card := range s.refused {
		if s.evidenced[card] {
			t.Errorf("%s is refused and has an evidence row", card)
		}
	}
}

// scriptText matches what only a Forge script holds: a "Key:" line of one, or the
// Key$ Value form of its abilities.
var scriptText = regexp.MustCompile(`(?m)^(Name|ManaCost|Types|PT|Loyalty|K|A|T|S|R|SVar|Oracle|AlternateMode|DeckHints|DeckHas):|\b[A-Z][A-Za-z]*\$`)

// supportScriptViolation says why a support file's bytes are not names and reasons.
func supportScriptViolation(name string, b []byte) string {
	if scriptText.Match(b) {
		return "holds Forge script text: " + string(scriptText.Find(b))
	}
	if strings.HasSuffix(name, ".txt") {
		for _, line := range strings.Split(string(b), "\n") {
			if len(line) > 150 {
				return "a line too long for a card name: " + line
			}
		}
	}
	return ""
}

func TestSupportHasNoScriptText(t *testing.T) {
	for _, dir := range []string{cardset.RefusedDir, cardset.EvidenceDir} {
		files, err := filepath.Glob(filepath.Join(testRoot, filepath.FromSlash(dir), "*"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if why := supportScriptViolation(f, b); why != "" {
				t.Errorf("%s: %s", f, why)
			}
		}
	}
	for _, c := range []struct {
		name, body string
		bad        bool
	}{
		{"a.txt", "Rockface Village\nCircle of Protection: Red\n", false},
		{"a.json", `{"card": "X", "why": "gorge offers cast after exile, kernel does not"}`, false},
		{"a.txt", "Name:Rockface Village\nManaCost:no cost\n", true},
		{"a.json", `{"why": "AB$ ChangeZone | Cost$ T"}`, true},
		{"a.txt", strings.Repeat("x", 151), true},
	} {
		if got := supportScriptViolation(c.name, []byte(c.body)) != ""; got != c.bad {
			t.Errorf("%q: violation %v, want %v", c.body, got, c.bad)
		}
	}
}

func TestRefuseRoundTrip(t *testing.T) {
	root := t.TempDir()
	ev := filepath.Join(root, filepath.FromSlash(cardset.EvidenceDir))
	if err := os.MkdirAll(ev, 0o755); err != nil {
		t.Fatal(err)
	}
	blb := filepath.Join(ev, "blb.txt")
	fdn := filepath.Join(ev, "fdn.txt")
	before := "Plains\nRockface Village\nSwamp\n"
	for _, f := range []string{blb, fdn} {
		body := before
		if f == fdn {
			body = "Plains\n"
		}
		if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repro, err := parseRefusedRepro("blb:autopay:710")
	if err != nil {
		t.Fatal(err)
	}
	path, err := refuseCard(root, "Rockface Village", "lockstep", "it differs", repro)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := cardset.ReadRefused(os.DirFS(root))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries %v, %v", entries, err)
	}
	e := entries[0]
	if !slices.Equal(e.Evidence, []string{"blb"}) || e.Repro.Seed != 1 || e.Repro.Policy != "random" {
		t.Errorf("entry %+v", e)
	}
	if b, _ := os.ReadFile(blb); string(b) != "Plains\nSwamp\n" {
		t.Errorf("the evidence row stayed: %q", b)
	}
	if _, err := refuseCard(root, "Rockface Village", "lockstep", "again", repro); err == nil {
		t.Error("refusing a refused card worked")
	}
	if err := unrefuseCard(root, "Rockface Village"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(blb); string(b) != before {
		t.Errorf("the evidence row did not come back: %q", b)
	}
	if b, _ := os.ReadFile(fdn); string(b) != "Plains\n" {
		t.Errorf("a list that never held the card changed: %q", b)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the entry stayed: %v", err)
	}
}

func TestRefusedReproParses(t *testing.T) {
	for _, bad := range []string{"", "blb", "blb:autopay", "blb:both:7", "blb:full:x", "blb:full:-1", ":full:1", "blb:full:1:smart", "blb:full:1:random:x"} {
		if _, err := parseRefusedRepro(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	r, err := parseRefusedRepro("std-dsk:full:17:explore")
	if err != nil || r != (cardset.RefusedRepro{Pool: "std-dsk", Surface: "full", Seed: 1, Pair: 17, Policy: "explore", Mulligans: 1}) {
		t.Errorf("%+v, %v", r, err)
	}
	r, err = parseRefusedRepro("std-dsk:autopay:3:random:0")
	if err != nil || r != (cardset.RefusedRepro{Pool: "std-dsk", Surface: "autopay", Seed: 1, Pair: 3, Policy: "random"}) {
		t.Errorf("%+v, %v", r, err)
	}
}

// lowerRefusals is c's refusals from lowering and building, without one by name: a
// fixture tests what a card lowers to, whether or not lockstep plays it yet.
func lowerRefusals(c *lCard) []refusal {
	return slices.DeleteFunc(slices.Clone(c.Refusals), func(r refusal) bool { return r.Node == supportNode })
}

// statusOf over real lowering: a card refused by name keeps its whole table entry,
// which is what lets a raw table play it, while a card that does not lower keeps its
// own reason.
func TestStatusOfRefusedKeepsItsAbilities(t *testing.T) {
	const name = "Lightning Bolt"
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	e := cardset.RefusedEntry{Card: name, Class: "lockstep", Why: "it differs"}
	s := &supportLists{refused: map[string]*cardset.RefusedEntry{name: &e}, evidenced: map[string]bool{name: true}}
	c := all[0]
	if st, why := statusOf(c, s); st != cardset.StatusSupported || why != "" {
		t.Errorf("before: %v, %q", st, why)
	}
	refuseByName(lw, s)
	if st, why := statusOf(c, s); st != cardset.StatusRefused || why != e.Reason() {
		t.Errorf("refused: %v, %q", st, why)
	}
	if len(bs[0].def.Abilities) == 0 {
		t.Error("a refused card lost its abilities, so a raw table cannot play it")
	}
	if cv := computeCoverage(p, lw); cv.cardsOK != 0 {
		t.Errorf("a refused card counts as accepted: %d", cv.cardsOK)
	}

	s.refused = map[string]*cardset.RefusedEntry{}
	s.evidenced = map[string]bool{}
	c.Refusals = nil
	if st, _ := statusOf(c, s); st != cardset.StatusUnverified {
		t.Errorf("no evidence: %v", st)
	}
	c.Token = true
	if st, _ := statusOf(c, s); st != cardset.StatusSupported {
		t.Errorf("a token with no evidence: %v", st)
	}
	c.Token = false
	c.Refusals = []refusal{{Card: name, Node: "card", Reason: "no"}}
	s.refused[name] = &e
	refuseByName(lw, s)
	if st, why := statusOf(c, s); st != cardset.StatusNotLowered || why != "not lowered: no" {
		t.Errorf("not lowered and refused: %v, %q", st, why)
	}
}

const supportDocPath = "dirac/docs/support.md"

// supportCounts is one row of the support page.
type supportCounts struct {
	list                                           string
	cards, lowered, refused, unverified, supported int
	fourSeat                                       int // supported and not two-seat only (CardDef.TwoSeat)
}

func countSupport(t *testing.T, cardsDir, list string, s *supportLists) supportCounts {
	t.Helper()
	p, err := loadCardList(testSetlistPath(t, list), cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatalf("%s: %v", list, err)
	}
	r := supportCounts{list: list, cards: len(lw.cards)}
	for _, c := range lw.cards {
		st, _ := statusOf(c, s)
		if st != cardset.StatusNotLowered {
			r.lowered++
		}
		switch st {
		case cardset.StatusRefused:
			r.refused++
		case cardset.StatusUnverified:
			r.unverified++
		case cardset.StatusSupported:
			r.supported++
			if !c.TwoSeat {
				r.fourSeat++
			}
		}
	}
	return r
}

// supportDoc renders the support page from this binary's lists and a lowering of
// every Standard list and the whole corpus.
func supportDoc(t *testing.T, cardsDir string) []byte {
	t.Helper()
	s, err := embeddedSupport()
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	b.WriteString("# Card support\n\n")
	fmt.Fprintf(&b, "Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.\n\n")
	b.WriteString("A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).\n\n")
	fmt.Fprintf(&b, "`cardset.EnforceEvidence` is %v", cardset.EnforceEvidence)
	if cardset.EnforceEvidence {
		b.WriteString(": a game plays supported cards only, and a deck holding an unverified or refused card is turned away")
	} else {
		b.WriteString(", so unverified cards still play")
	}
	b.WriteString(".\n\n| list | cards | lowered | refused | unverified | supported | 4-seat |\n|---|---:|---:|---:|---:|---:|---:|\n")
	codes, err := readCardList(filepath.Join(testRoot, standardListPath))
	if err != nil {
		t.Fatal(err)
	}
	for _, list := range append(codes, "ALL") {
		r := countSupport(t, cardsDir, list, s)
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d |\n", r.list, r.cards, r.lowered, r.refused, r.unverified, r.supported, r.fourSeat)
	}
	b.WriteString("\n`ALL` is the whole corpus; the rest are the Standard sets. `4-seat` counts the supported cards whose lowering holds at any seat count: a card whose lowering reads the one opponent of a two-player game (`kop.CardDef.TwoSeat`) is refused in a game of three or more seats (`StallTwoSeat`). It is a ceiling, not evidence: no 4-seat game is in the evidence runs.\n\n## What supported covers\n\n")
	b.WriteString("Evidence comes from the `fdn` and `blb` deck pools and one synthetic pool per Standard set: 1,500 pairs each, seed 1, the random policy, both surfaces, mulligans 0 and 1, every run passing `kfuzz -absolute`. A synthetic pool deals its decks from the cards it plays, so no pair is lost to a refused card; it skips only a pair the kernel refuses together, so it can play fewer games, and its cards met the floor in these:\n\n| pool | games per run |\n|---|---:|\n")
	games, err := os.ReadFile(filepath.Join(testRoot, "dirac", "support", "evidence-games.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range strings.Fields(strings.ReplaceAll(string(games), "\t", ",")) {
		pool, n, _ := strings.Cut(row, ",")
		fmt.Fprintf(&b, "| %s | %s |\n", pool, n)
	}
	b.WriteString("\nNothing else is covered. A card none of these runs reaches is unverified, whatever it does. A supported card has played clean in these games, not in every game: the standing sweep and census are separate runs, last measured at 164 and 244 divergent pairs, and are outside this claim.\n\n## Refused by name\n\n")
	entries, err := cardset.ReadRefused(os.DirFS(testRoot))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		b.WriteString("None.\n")
		return b.Bytes()
	}
	classes := map[string]int{}
	for _, e := range entries {
		classes[e.Class]++
	}
	fmt.Fprintf(&b, "%d cards, by what differed in the game that showed them. Each has a file in `dirac/support/refused` naming the card, the difference and the game.\n\n| what differed | cards |\n|---|---:|\n", len(entries))
	for _, c := range slices.Sorted(maps.Keys(classes)) {
		fmt.Fprintf(&b, "| %s | %d |\n", c, classes[c])
	}
	return b.Bytes()
}

func TestSupportDocIsCurrent(t *testing.T) {
	cardsDir := ratchetCards(t)
	got := supportDoc(t, cardsDir)
	file := path.Join(testRoot, supportDocPath)
	if os.Getenv("DIRAC_UPDATE_SUPPORT") == "1" {
		if err := os.WriteFile(file, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is stale: DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen rewrites it", supportDocPath)
	}
}
