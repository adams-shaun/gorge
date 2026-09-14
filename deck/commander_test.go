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
		// ability stays colourless-identity — but CR 903.5d still binds what
		// their basic land TYPE could produce to the commander's identity:
		// Mountains are not free in a mono-white deck. Wastes has no basic
		// land subtype and produces only colourless, so it fits any deck.
		"Plains":   "Name:Plains\nTypes:Basic Land Plains\n",
		"Mountain": "Name:Mountain\nTypes:Basic Land Mountain\n",
		"Badlands": "Name:Badlands\nTypes:Land Swamp Mountain\nOracle:({T}: Add {B} or {R}.)\n",
		"Wastes":   "Name:Wastes\nTypes:Basic Land\nOracle:{T}: Add {C}.\n",
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
		// A legendary Vehicle: commander-eligible per CR 903.3(b) — Vehicles
		// are always printed with a power/toughness box.
		"Genesis Engine": "Name:Genesis Engine\nManaCost:2 W U\nTypes:Legendary Artifact Vehicle\nPT:8/8\n",
		// Legendary Spacecraft are commander-eligible only with a printed
		// power/toughness box (CR 903.3(c)); Spacecraft normally carry a
		// defense box instead. This fixture carries PT like the real
		// Hearthhull, the Worldseed (6/7) does.
		"Hearthhull": "Name:Hearthhull\nManaCost:1 B R G\nTypes:Legendary Artifact Spacecraft\nPT:6/7\n",
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

// legalMonoWhiteCommander builds a CR-903.4/903.5/903.5d legal 100-card
// mono-white deck: a legendary-commander plus two other white cards and
// ninety-seven Plains (whose repeated copies sit under the basic-land
// exemption, and whose Plains type could produce only white — CR 903.5d).
func legalMonoWhiteCommander() File {
	return File{
		Name:      "WHITE",
		Commander: "Amalia",
		Cards: []Entry{
			{"Amalia", 1},
			{"Knight", 1},
			{"Guard", 1},
			{"Plains", 97},
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
	// Kick one Plains out: 99 cards is not a Commander deck.
	f.Cards = []Entry{{"Amalia", 1}, {"Knight", 1}, {"Guard", 1}, {"Plains", 96}}
	if err := f.ValidateCommander(r); err == nil || !strings.Contains(err.Error(), "99 cards") {
		t.Fatalf("want a 99-card error, got %v", err)
	}
}

func TestValidateCommanderRejectsNonSingleton(t *testing.T) {
	r := commanderFixture(t)
	f := legalMonoWhiteCommander()
	// Two Knights, but keep 100 total by dropping a Plains.
	f.Cards = []Entry{{"Amalia", 1}, {"Knight", 2}, {"Plains", 97}}
	err := f.ValidateCommander(r)
	if err == nil || !strings.Contains(err.Error(), "Knight") || !strings.Contains(err.Error(), "singleton") {
		t.Fatalf("want a Knight singleton error, got %v", err)
	}
}

func TestValidateCommanderAllowsBasicLandDuplicates(t *testing.T) {
	r := commanderFixture(t)
	f := legalMonoWhiteCommander()
	// Plains is a basic land: its 97 copies must not trip the singleton rule,
	// whether they sit on one row or several.
	f.Cards = []Entry{{"Amalia", 1}, {"Knight", 1}, {"Guard", 1}, {"Plains", 4}, {"Plains", 93}}
	if err := f.ValidateCommander(r); err != nil {
		t.Fatalf("basic-land duplicates wrongly rejected: %v", err)
	}
}

// TestValidateCommanderRejectsBasicLandTypeProduction pins CR 903.5d: a card
// with a basic land type may be in the deck only if every colour it could
// produce is in the commander's identity. A basic land's colour IDENTITY is
// empty (its mana ability is subtype-granted, not printed rules text), so
// without this check Mountains validated under any commander — the fixture
// held 97 of them under a white commander before the rule existed.
func TestValidateCommanderRejectsBasicLandTypeProduction(t *testing.T) {
	r := commanderFixture(t)
	// Mountain produces red: outside a white commander's identity.
	f := legalMonoWhiteCommander()
	f.Cards = []Entry{{"Amalia", 1}, {"Knight", 1}, {"Guard", 1}, {"Plains", 96}, {"Mountain", 1}}
	err := f.ValidateCommander(r)
	if err == nil || !strings.Contains(err.Error(), "Mountain") || !strings.Contains(err.Error(), "red") || !strings.Contains(err.Error(), "903.5d") {
		t.Fatalf("want a Mountain could-produce error, got %v", err)
	}
	// Badlands has no Basic supertype, so it is a singleton row, but its basic
	// land types (Swamp, Mountain) could produce black and red: the 903.5d
	// rule is what keeps the typed nonbasics (the corpus's snow duals, Dryad
	// Arbor) in matching decks, since their parenthesised mana ability is not
	// in the identity.
	f2 := legalMonoWhiteCommander()
	f2.Cards = []Entry{{"Amalia", 1}, {"Knight", 1}, {"Guard", 1}, {"Plains", 96}, {"Badlands", 1}}
	err = f2.ValidateCommander(r)
	if err == nil || !strings.Contains(err.Error(), "Badlands") || !strings.Contains(err.Error(), "903.5d") {
		t.Fatalf("want a Badlands could-produce error, got %v", err)
	}
	// Wastes produce only colourless, which is not a colour: they fit any deck.
	f3 := legalMonoWhiteCommander()
	f3.Cards = []Entry{{"Amalia", 1}, {"Knight", 1}, {"Guard", 1}, {"Plains", 96}, {"Wastes", 1}}
	if err := f3.ValidateCommander(r); err != nil {
		t.Fatalf("Wastes wrongly rejected: %v", err)
	}
}

func TestValidateCommanderRejectsOutsideColourIdentity(t *testing.T) {
	r := commanderFixture(t)
	f := legalMonoWhiteCommander()
	// Swap a Mountain for a blue Drake: identity {U} is outside {W}.
	f.Cards = []Entry{{"Amalia", 1}, {"Knight", 1}, {"Guard", 1}, {"Plains", 96}, {"Drake", 1}}
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
		Cards:     []Entry{{"Amalia", 1}, {"Orc", 2}, {"Plains", 40}},
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
// non-creature other than a Spacecraft — artifact, land, enchantment or
// sorcery — is not a legal commander unless its Oracle says it can be. Karakas is a legendary land
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

func TestValidateCommanderAcceptsLegendarySpacecraft(t *testing.T) {
	r := commanderFixture(t)
	f := File{Name: "SPACECRAFT", Commander: "Hearthhull", Cards: []Entry{{"Hearthhull", 1}, {"Wastes", 99}}}
	if err := f.ValidateCommander(r); err != nil {
		t.Fatalf("legendary Spacecraft commander rejected: %v", err)
	}
}

// TestValidateCommanderRejectsPTLessSpacecraftCommander pins the other half
// of CR 903.3's Spacecraft carve-out: a legendary Spacecraft WITHOUT a
// printed power/toughness box — it carries a defense box instead — is not a
// legal commander. The Eternity Elevator is the corpus's real example.
func TestValidateCommanderRejectsPTLessSpacecraftCommander(t *testing.T) {
	blob := corpusCardScript(t, "t/the_eternity_elevator.txt")
	c, diags := cards.ParseBytes("elevator.txt", blob)
	if len(diags) > 0 {
		t.Fatalf("The Eternity Elevator parse: %v", diags)
	}
	// Pin the face first: a legendary Spacecraft with NO PT field.
	f := c.Faces[0]
	if !f.IsLegendary() || !f.IsSpacecraft() || f.PT != "" {
		t.Fatalf("fixture drift: The Eternity Elevator parsed as Types %v PT %q; want a Legendary Spacecraft with no power/toughness box", f.Types, f.PT)
	}
	if IsCommanderEligible(c) {
		t.Fatal("IsCommanderEligible(The Eternity Elevator) = true: a Spacecraft with no power/toughness box is not a legal commander (CR 903.3)")
	}
}

// TestIsCommanderEligibleAcceptsLegendaryVehicle pins CR 903.3(b): a Vehicle
// card is commander-eligible — Vehicles are always printed with a
// power/toughness box. Shorikai, Genesis Engine is the corpus's real example.
func TestIsCommanderEligibleAcceptsLegendaryVehicle(t *testing.T) {
	blob := corpusCardScript(t, "s/shorikai_genesis_engine.txt")
	c, diags := cards.ParseBytes("shorikai.txt", blob)
	if len(diags) > 0 {
		t.Fatalf("Shorikai parse: %v", diags)
	}
	f := c.Faces[0]
	if !f.IsLegendary() || !f.IsVehicle() || f.PT == "" {
		t.Fatalf("fixture drift: Shorikai parsed as Types %v PT %q; want a Legendary Vehicle with a power/toughness box", f.Types, f.PT)
	}
	if !IsCommanderEligible(c) {
		t.Fatal("IsCommanderEligible(Shorikai, Genesis Engine) = false: a legendary Vehicle is a legal commander (CR 903.3)")
	}
	// And a fixture Vehicle validates as a commander end to end.
	r := commanderFixture(t)
	deck := File{Name: "VEH", Commander: "Genesis Engine", Cards: []Entry{{"Genesis Engine", 1}, {"Wastes", 99}}}
	if err := deck.ValidateCommander(r); err != nil {
		t.Fatalf("legendary Vehicle commander rejected: %v", err)
	}
}

func TestValidateCommanderAcceptsOracleMarkedCommander(t *testing.T) {
	r := commanderFixture(t)
	// Isahara is a planeswalker, not a legendary creature, but says it can be
	// your commander in its Oracle: it must be accepted as commander. Its
	// colour identity is empty ({3} cost), so the rest of the deck must be
	// colourless too — hence only basic Wastes beside it (a Plains could
	// produce white, which CR 903.5d binds to the commander's identity).
	f := File{Name: "IS", Commander: "Isahara", Cards: []Entry{{"Isahara", 1}, {"Wastes", 99}}}
	if err := f.ValidateCommander(r); err != nil {
		t.Fatalf("oracle-marked commander rejected: %v", err)
	}
	// But the same card without the marker is not eligible.
	f.Commander = "Drake"
	if err := f.ValidateCommander(r); err == nil || !strings.Contains(err.Error(), "legendary creature") {
		t.Fatalf("want a not-eligible error for non-marked planeswalker, got %v", err)
	}
}

// TestValidateCommanderPTLessSpacecraftErrorText pins the eligibility error
// text a validator caller surfaces: a legendary Vehicle and a Spacecraft with
// a power/toughness box are inside CR 903.3's list, so the message must name
// the full legal set, not only the creature half.
func TestValidateCommanderPTLessSpacecraftErrorText(t *testing.T) {
	r := commanderFixture(t)
	f := File{Name: "E", Commander: "Drake", Cards: []Entry{{"Drake", 1}, {"Wastes", 99}}}
	err := f.ValidateCommander(r)
	if err == nil || !strings.Contains(err.Error(), "legendary creature") || !strings.Contains(err.Error(), "Vehicle") || !strings.Contains(err.Error(), "power/toughness") {
		t.Fatalf("want the full CR 903.3 eligibility error, got %v", err)
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

// TestFileCommanderIndex pins the shared commander-index resolution: the
// flat position of f.Commander's card in the count-expanded deck, whatever
// row it sits on. This is the index rules.Config.Commanders wants (genesis
// moves that object to the command zone) and the one resolution gorged and
// the engine's own commander tests both use, so a deck that does not print
// its commander first must resolve exactly as a deck that does. m39 moved
// the one copy of this scan out of rules/commander_decks_test.go into the
// deck package so the product and the engine cannot disagree.
func TestFileCommanderIndex(t *testing.T) {
	// First row, ordinary row, duplicate-count rows before the commander.
	if got := (File{Name: "x", Commander: "Zed", Cards: []Entry{
		{"Zed", 1},         // 0 — it is the first entry
		{"A", 2}, {"B", 3}, // 2, 5 — counts accumulate past row 0
		{"C", 1}, // 9
	}}).CommanderIndex(); got != 0 {
		t.Fatalf("first-row commander index = %d, want 0", got)
	}
	if got := (File{Name: "x", Commander: "Zed", Cards: []Entry{
		{"A", 2}, {"Zed", 1}, {"B", 3},
	}}).CommanderIndex(); got != 2 {
		t.Fatalf("mid-deck commander index = %d, want 2 (the two A copies before it)", got)
	}
	if got := (File{Name: "x", Commander: "Zed", Cards: []Entry{
		{"A", 1}, {"B", 1}, {"C", 1},
	}}).CommanderIndex(); got != 3 {
		t.Fatalf("absent commander index = %d, want 3 (the count-expanded length: the caller validates first)", got)
	}
	if got := (File{Name: "none"}).CommanderIndex(); got != 0 {
		t.Fatalf("commander-less File index = %d, want 0 (never called by anyone who checked Commander first)", got)
	}
}
