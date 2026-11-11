package deck

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

func bannedKarakasDeck() File {
	return File{
		Name:      "WHITE",
		Commander: "Amalia",
		Cards: []Entry{
			{"Amalia", 1},
			{"Knight", 1},
			{"Guard", 1},
			{"Plains", 96},
			{"Karakas", 1},
		},
	}
}

func TestValidateCommanderRejectsBannedCard(t *testing.T) {
	r := commanderFixture(t)
	if err := legalMonoWhiteCommander().ValidateCommander(r); err != nil {
		t.Fatalf("precondition: legal deck rejected: %v", err)
	}
	k, _ := r.Lookup("Karakas")
	kn, _ := r.Lookup("Knight")
	if !IsCommanderBanned(k) || IsCommanderBanned(kn) {
		t.Fatalf("precondition: IsCommanderBanned(Karakas)=%v, (Knight)=%v", IsCommanderBanned(k), IsCommanderBanned(kn))
	}
	err := bannedKarakasDeck().ValidateCommander(r)
	if err == nil || !strings.Contains(err.Error(), `card "Karakas" is banned in Commander`) {
		t.Fatalf("got %v, want the Karakas ban line", err)
	}
}

func TestValidateCommanderRejectsBannedCommander(t *testing.T) {
	r := commanderFixture(t)
	f := File{Name: "GOLOS", Commander: "Golos, Tireless Pilgrim", Cards: []Entry{
		{"Golos, Tireless Pilgrim", 1}, {"Wastes", 99}}}
	err := f.ValidateCommander(r)
	if err == nil {
		t.Fatal("banned commander validated")
	}
	if !strings.Contains(err.Error(), `card "Golos, Tireless Pilgrim" is banned in Commander`) {
		t.Fatalf("missing ban line: %v", err)
	}
	if strings.Contains(err.Error(), "legendary creature") {
		t.Fatalf("eligibility check should pass: %v", err)
	}
}

func TestValidateCommanderBannedReportsAlongsideOtherViolations(t *testing.T) {
	r := commanderFixture(t)
	f := bannedKarakasDeck()
	f.Cards[3] = Entry{"Plains", 95}
	f.Cards = append(f.Cards, Entry{"Drake", 1})
	err := f.ValidateCommander(r)
	if err == nil {
		t.Fatal("deck validated")
	}
	for _, want := range []string{`card "Karakas" is banned in Commander`, `card "Drake" has colour identity`, "outside commander"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

func TestCommanderBannedNamesSortedUnique(t *testing.T) {
	n := CommanderBannedNames
	if !sort.StringsAreSorted(n) {
		t.Error("CommanderBannedNames not sorted")
	}
	seen := map[string]string{}
	for _, s := range n {
		k := cards.NormalizeName(s)
		if o, dup := seen[k]; dup {
			t.Errorf("%q duplicates %q", s, o)
		}
		seen[k] = s
	}
	if len(n) != 42 {
		t.Errorf("len = %d, want 42", len(n))
	}
}
