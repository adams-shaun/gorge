package deck

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// chooseABackgroundFixture is commanderFixture plus the authored cards the
// Choose-a-Background (CR 702.124k) pair tests need. The scripts are inline.
func chooseABackgroundFixture(t *testing.T) *cards.Registry {
	t.Helper()
	r := commanderFixture(t)
	scripts := map[string]string{
		"Campaign Captain":     "Name:Campaign Captain\nManaCost:1 R\nTypes:Legendary Creature Human Warrior\nK:Choose a Background\n",
		"Rival Captain":        "Name:Rival Captain\nManaCost:1 G\nTypes:Legendary Creature Human Warrior\nK:Choose a Background\n",
		"Wandering Background": "Name:Wandering Background\nManaCost:1 W\nTypes:Legendary Enchantment Background\n",
		"Second Background":    "Name:Second Background\nManaCost:1 W\nTypes:Legendary Enchantment Background\n",
		"Unpaired Legendary":   "Name:Unpaired Legendary\nManaCost:W\nTypes:Legendary Creature Human\n",
		"Back Background":      "Name:Back Background\nManaCost:1 W\nTypes:Legendary Enchantment\nAlternateMode:DoubleFaced\nOracle:x\nALTERNATE\nName:Back Background Back\nTypes:Legendary Enchantment Background\nOracle:x\n",
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

// chooseABackgroundDeck is a 100-card deck: the commanders, a white Knight
// (legal only via the Background's W), a red Orc (legal only via the
// Captain's R) and Plains for the rest.
func chooseABackgroundDeck(cmdrs ...string) File {
	f := File{Name: "BG", Commanders: cmdrs}
	for _, c := range cmdrs {
		f.Cards = append(f.Cards, Entry{c, 1})
	}
	f.Cards = append(f.Cards, Entry{"Knight", 1}, Entry{"Orc", 1}, Entry{"Plains", 100 - len(cmdrs) - 2})
	return f
}

func hasBackgroundSubtype(f *cards.Face) bool {
	for _, ty := range f.Types {
		if strings.EqualFold(ty, "Background") {
			return true
		}
	}
	return false
}

// TestValidateCommanderChooseABackgroundPair pins CR 702.124k at the deck
// layer: a Choose-a-Background card plus a legendary Background enchantment is
// a legal two-commander designation in either order, the deck identity is the
// union, and every other shape stays rejected.
func TestValidateCommanderChooseABackgroundPair(t *testing.T) {
	r := chooseABackgroundFixture(t)
	captain, _ := r.Lookup("Campaign Captain")
	bg, _ := r.Lookup("Wandering Background")

	// Preconditions: the Background is not eligible alone (so the pair gate is
	// what admits it), the keyword is on the captain only, and the two
	// identities differ (so the union is what admits Knight and Orc).
	if IsCommanderEligible(bg) {
		t.Fatal("fixture drift: Wandering Background is commander-eligible alone")
	}
	if !hasChooseABackground(captain) || hasChooseABackground(bg) {
		t.Fatal("fixture drift: keyword must be on the captain only")
	}
	if captain.ColourIdentity() == bg.ColourIdentity() {
		t.Fatal("fixture drift: the union test needs two different identities")
	}

	if err := chooseABackgroundDeck("Campaign Captain", "Wandering Background").ValidateCommander(r); err != nil {
		t.Fatalf("Choose a Background deck rejected: %v", err)
	}
	if err := chooseABackgroundDeck("Wandering Background", "Campaign Captain").ValidateCommander(r); err != nil {
		t.Fatalf("Choose a Background deck rejected with the pair reversed: %v", err)
	}

	// Lone Background: still illegal.
	lone := File{Name: "LONE", Commanders: []string{"Wandering Background"},
		Cards: []Entry{{"Wandering Background", 1}, {"Plains", 99}}}
	err := lone.ValidateCommander(r)
	if err == nil || !strings.Contains(err.Error(), "legendary creature") || !strings.Contains(err.Error(), "Wandering Background") {
		t.Fatalf("lone Background: want the single-commander eligibility error, got %v", err)
	}

	// Precondition: the subtype is only on Back Background's back face.
	back, _ := r.Lookup("Back Background")
	if len(back.Faces) != 2 || hasBackgroundSubtype(back.Faces[0]) || !hasBackgroundSubtype(back.Faces[1]) {
		t.Fatal("fixture drift: Back Background must carry the Background subtype on its back face only")
	}

	for _, tc := range []struct{ a, b string }{
		{"Campaign Captain", "Unpaired Legendary"},
		{"Campaign Captain", "Rival Captain"},
		{"Wandering Background", "Second Background"},
		{"Campaign Captain", "Back Background"},
	} {
		f := File{Name: "BAD", Commanders: []string{tc.a, tc.b},
			Cards: []Entry{{tc.a, 1}, {tc.b, 1}, {"Plains", 98}}}
		err := f.ValidateCommander(r)
		if err == nil || !strings.Contains(err.Error(), "not a legal commander pair") {
			t.Fatalf("%s + %s: want a pair rejection, got %v", tc.a, tc.b, err)
		}
	}
}
