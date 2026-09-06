package deck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// commanderFixture builds a small synthetic registry of commander-shaped cards
// so the validator tests need no corpus. The scripts are authored here, not
// copied from Forge.
func commanderFixture(t *testing.T) *cards.Registry {
	t.Helper()
	r := cards.NewRegistry()
	scripts := map[string]string{
		// Basic lands carry no printed mana ability (the corpus grants the
		// {T}: add {colour} intrinsic after derive), so a script that names no
		// ability stays colourless-identity and exempt from the singleton rule.
		"Mountain": "Name:Mountain\nTypes:Basic Land Mountain\n",
		"Island":   "Name:Island\nTypes:Basic Land Island\n",
		// Amalia is the white legendary-commander: identity {W}.
		"Amalia":       "Name:Amalia\nManaCost:W\nTypes:Legendary Creature Scout\nOracle:Amalia is a commander.\n",
		"Knight":       "Name:Knight\nManaCost:W\nTypes:Creature Knight\nOracle:Knight.\n",
		"Guard":        "Name:Guard\nManaCost:W\nTypes:Creature Soldier\nOracle:Guard.\n",
		"Drake":        "Name:Drake\nManaCost:U\nTypes:Creature Drake\nOracle:Drake.\n",
		"Orc":          "Name:Orc\nManaCost:R\nTypes:Creature Orc\nOracle:Orc.\n",
		"Black Knight": "Name:Black Knight\nManaCost:B\nTypes:Creature Knight\nOracle:Black Knight.\n",
		// A planeswalker that says it can be your commander: eligible by the
		// Oracle phrase even though it is not a legendary creature.
		"Isahara": "Name:Isahara\nManaCost:3\nTypes:Planeswalker\nOracle:Isahara can be your commander.\n",
	}
	for name, src := range scripts {
		c, diags := cards.ParseBytes("fixture.txt", []byte(src))
		if len(diags) > 0 {
			t.Fatalf("fixture %s parse: %v", name, diags)
		}
		r.Add(c)
	}
	return r
}

// legalMonoWhiteCommander builds a CR-903.4/903.5 legal 100-card mono-white
// deck: a legendary-commander plus two other white cards and ninety-seven
// Mountains (whose repeated copies sit under the basic-land exemption).
func legalMonoWhiteCommander() File {
	return File{
		Name:      "WHITE",
		Commander: "Amalia",
		Cards: []Entry{
			{"Amalia", 1},
			{"Knight", 1},
			{"Guard", 1},
			{"Mountain", 97},
		},
	}
}

func TestValidateCommanderConstructedDeckUntouched(t *testing.T) {
	r := commanderFixture(t)
	// A constructed deck with no commander designation and any card count is
	// not subject to any Commander rule: three Mountain copies, no commander.
	f := File{Cards: []Entry{{"Mountain", 3}}}
	if err := f.ValidateCommander(r); err != nil {
		t.Fatalf("constructed deck should validate as nil, got %v", err)
	}
}

func TestValidateCommanderAcceptsLegalDeck(t *testing.T) {
	r := commanderFixture(t)
	if err := legalMonoWhiteCommander().ValidateCommander(r); err != nil {
		t.Fatalf("legal commander deck rejected: %v", err)
	}
}

func TestValidateCommanderRejectsWrongCount(t *testing.T) {
	r := commanderFixture(t)
	f := legalMonoWhiteCommander()
	// Kick one Mountain out: 99 cards is not a Commander deck.
	f.Cards = []Entry{{"Amalia", 1}, {"Knight", 1}, {"Guard", 1}, {"Mountain", 96}}
	if err := f.ValidateCommander(r); err == nil || !strings.Contains(err.Error(), "99 cards") {
		t.Fatalf("want a 99-card error, got %v", err)
	}
}

func TestValidateCommanderRejectsNonSingleton(t *testing.T) {
	r := commanderFixture(t)
	f := legalMonoWhiteCommander()
	// Two Knights, but keep 100 total by dropping a Mountain.
	f.Cards = []Entry{{"Amalia", 1}, {"Knight", 2}, {"Mountain", 97}}
	err := f.ValidateCommander(r)
	if err == nil || !strings.Contains(err.Error(), "Knight") || !strings.Contains(err.Error(), "singleton") {
		t.Fatalf("want a Knight singleton error, got %v", err)
	}
}

func TestValidateCommanderAllowsBasicLandDuplicates(t *testing.T) {
	r := commanderFixture(t)
	f := legalMonoWhiteCommander()
	// Mountain is a basic land: its 97 copies must not trip the singleton rule.
	f.Cards = []Entry{{"Amalia", 1}, {"Knight", 1}, {"Guard", 1}, {"Mountain", 4}, {"Island", 93}}
	if err := f.ValidateCommander(r); err != nil {
		t.Fatalf("basic-land duplicates wrongly rejected: %v", err)
	}
}

func TestValidateCommanderRejectsOutsideColourIdentity(t *testing.T) {
	r := commanderFixture(t)
	f := legalMonoWhiteCommander()
	// Swap a Mountain for a blue Drake: identity {U} is outside {W}.
	f.Cards = []Entry{{"Amalia", 1}, {"Knight", 1}, {"Guard", 1}, {"Mountain", 96}, {"Drake", 1}}
	err := f.ValidateCommander(r)
	if err == nil || !strings.Contains(err.Error(), "Drake") || !strings.Contains(err.Error(), "blue") {
		t.Fatalf("want a Drake colour-identity error, got %v", err)
	}
}

func TestValidateCommanderNamesEveryOffender(t *testing.T) {
	r := commanderFixture(t)
	// Two violations at once — count and identity — must both be named.
	f := File{
		Name:      "BAD",
		Commander: "Amalia",
		Cards:     []Entry{{"Amalia", 1}, {"Orc", 2}, {"Mountain", 40}},
	}
	err := f.ValidateCommander(r)
	if err == nil {
		t.Fatal("want errors")
	}
	if !strings.Contains(err.Error(), "Orc") || !strings.Contains(err.Error(), "43 cards") {
		t.Fatalf("want Orc and 43-card errors, got %v", err)
	}
}

// corpusCardScript reads one pinned corpus script by its cardsfolder path so
// an eligibility test can run on real Forge data — a real card's Types:/Oracle:
// — rather than a hand-built face (the m35 fix1 hole was exactly a test set
// whose faces were all hand-built around the half of the condition the author
// was thinking about). It Skips on a clean checkout with no .cards/ corpus,
// the same contract cards/corpus_test.go sets.
func corpusCardScript(t *testing.T, rel string) []byte {
	t.Helper()
	blob, err := os.ReadFile(filepath.Join("..", ".cards", "cardsfolder", rel))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("corpus not fetched; run `make fetch-cards compile-cards`")
		}
		t.Fatalf("reading corpus script %s: %v", rel, err)
	}
	return blob
}

// TestIsCommanderEligibleRejectsLegendaryNonCreature pins the half of CR
// 903.3's eligibility condition the other tests never exercised: a legendary
// non-creature — artifact, land, enchantment or sorcery — is not a legal
// commander unless its Oracle says it can be. Karakas is a legendary land
// from the pinned corpus. Deleting `&& f.IsCreature()` from IsCommanderEligible
// makes this test fail by name.
func TestIsCommanderEligibleRejectsLegendaryNonCreature(t *testing.T) {
	blob := corpusCardScript(t, "k/karakas.txt")
	c, diags := cards.ParseBytes("karakas.txt", blob)
	if len(diags) > 0 {
		t.Fatalf("Karakas parse: %v", diags)
	}
	if len(c.Faces) != 1 {
		t.Fatalf("Karakas has %d faces, want 1", len(c.Faces))
	}
	// Pin the face predicates on the real corpus data first: the eligibility
	// assertion below is only meaningful if Karakas really is a legendary
	// non-creature (a corpus-pin drift that grows a creature type or a
	// commander phrase on it is a fixture problem, not a pass).
	f := c.Faces[0]
	if !f.IsLegendary() || f.IsCreature() || !f.IsLand() {
		t.Fatalf("fixture drift: Karakas parsed as Types %v; want a Legendary Land with no creature type", f.Types)
	}
	if IsCommanderEligible(c) {
		t.Fatal("IsCommanderEligible(Karakas) = true: a legendary land is not a legal commander (CR 903.3)")
	}
}

func TestValidateCommanderRejectsNonEligibleCommander(t *testing.T) {
	r := commanderFixture(t)
	f := legalMonoWhiteCommander()
	f.Commander = "Knight" // a plain creature, not legendary, no prose marker
	if err := f.ValidateCommander(r); err == nil || !strings.Contains(err.Error(), "legendary creature") {
		t.Fatalf("want a not-a-legendary-creature error, got %v", err)
	}
}

func TestValidateCommanderAcceptsOracleMarkedCommander(t *testing.T) {
	r := commanderFixture(t)
	// Isahara is a planeswalker, not a legendary creature, but says it can be
	// your commander in its Oracle: it must be accepted as commander. Its
	// colour identity is empty ({3} cost), so the rest of the deck must be
	// colourless too — hence only basic Mountains beside it.
	f := File{Name: "IS", Commander: "Isahara", Cards: []Entry{{"Isahara", 1}, {"Mountain", 99}}}
	if err := f.ValidateCommander(r); err != nil {
		t.Fatalf("oracle-marked commander rejected: %v", err)
	}
	// But the same card without the marker is not eligible.
	f.Commander = "Drake"
	if err := f.ValidateCommander(r); err == nil || !strings.Contains(err.Error(), "legendary creature") {
		t.Fatalf("want a not-eligible error for non-marked planeswalker, got %v", err)
	}
}

func TestParseReadsCommanderField(t *testing.T) {
	raw := `{"name":"W","commander":"Amalia","cards":[{"name":"Amalia","count":1},{"name":"Mountain","count":99}]}`
	f, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if f.Commander != "Amalia" {
		t.Fatalf("Commander = %q, want Amalia", f.Commander)
	}
	// And a file without the field parses with an empty Commander, unchanged.
	legacy := `{"name":"Legacy","cards":[{"name":"Mountain","count":2}]}`
	f2, err := Parse([]byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if f2.Commander != "" || len(f2.Cards) != 1 {
		t.Fatalf("legacy file misparsed: %+v", f2)
	}
}
