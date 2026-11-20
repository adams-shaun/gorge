package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// protectionFormCards are the paper cards the protection forms ticket
// (gap4-05) lowers: a fixed Protection effect, a granted Protection:<spec>
// with an exception field, a granted protection from a creature host, and a
// quality the kernel's filter reads off the source.
var protectionFormCards = []string{
	"Black Ward", "Blue Ward", "Crimson Acolyte", "Devoted Caretaker",
	"Feline Sovereign", "Goblin Wizard", "Green Ward", "Grotesque Hybrid",
	"Guildscorn Ward", "Haytham Kenway", "Hell-Bent Raider", "Hexdrinker", "Hungry Lynx",
	"Keeper of Kookus", "Masked Gorgon", "Obsidian Acolyte",
	"Pristine Angel", "Red Ward", "Spare from Evil", "Spectra Ward", "Tattoo Ward",
	"Tel-Jilad Defiance", "Tower of the Magistrate", "White Ward", "Willow Priestess",
}

// protectionFormDeferred are the ticket's cards that stay refused, each by a
// reason that names the missing piece (a substring of the refusal).
var protectionFormDeferred = map[string]string{
	"Apostle's Blessing":           `Protection Choices$ "AnyColor,Artifact"`,
	"Jeweled Spirit":               `Protection Choices$ "AnyColor,Artifact"`,
	"Razor Barrier":                `Protection Choices$ "AnyColor,Artifact"`,
	"Giver of Runes":               `Protection Choices$ "AnyColor,colorless"`,
	"Pippin, Guard of the Citadel": `Protection Choices$ "CardType"`,
	"Floating Shield":              `Protection Gains$ "ChosenColor"`,
	"Opal Titan":                   `Protection Duration$`,
	"Crypsis":                      `Protection:Creature.OppCtrl`,
	"Lord of the Nazgûl":           `Protection:Card.IsRingbearer`,
	"Pledge of Loyalty":            `Card.ColorsYouCtrl`,
	"Riders of Gavony":             `Protection:Creature.ChosenType`,
}

func lowerProtectionFormCards(t *testing.T, names []string) map[string]*lCard {
	t.Helper()
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*lCard{}
	for _, c := range lowerPool(p).cards {
		byName[c.Name] = c
	}
	return byName
}

// TestProtectionFormsLower: none of the lowered cards keeps a refusal (the
// census rows kw:Protection, Protection Gains$ and Protection Choices$ are
// gone for them), and each deferred card is still refused by the reason that
// names what is missing, so lowering one means moving it between the lists.
func TestProtectionFormsLower(t *testing.T) {
	names := append([]string(nil), protectionFormCards...)
	for n := range protectionFormDeferred {
		names = append(names, n)
	}
	byName := lowerProtectionFormCards(t, names)
	for _, name := range protectionFormCards {
		c := byName[name]
		if c == nil {
			t.Errorf("%s: not lowered", name)
			continue
		}
		for _, r := range c.Refusals {
			t.Errorf("%s: %s: %s", name, r.Node, r.Reason)
		}
	}
	for name, want := range protectionFormDeferred {
		c := byName[name]
		if c == nil {
			t.Errorf("%s: not lowered", name)
			continue
		}
		var got []string
		for _, r := range c.Refusals {
			got = append(got, r.Node+": "+r.Reason)
		}
		if !strings.Contains(strings.Join(got, "; "), want) {
			t.Errorf("%s: want a refusal naming %q, got %q", name, want, got)
		}
	}
}
