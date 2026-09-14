package cards

import (
	"slices"
	"testing"
)

func TestSplitKeywordList(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{"ampersand list", "Vigilance & Lifelink", []string{"Vigilance", "Lifelink"}},
		{"empty", "", nil},
		{"parameter comma", "Protection:Spell.Instant,Spell.Sorcery:instant spells and from sorcery spells", []string{"Protection:Spell.Instant,Spell.Sorcery:instant spells and from sorcery spells"}},
		{"parameter comma static", "OnlyUntapChosen:Artifact,Creature,Land", []string{"OnlyUntapChosen:Artifact,Creature,Land"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SplitKeywordList(tc.in); !slices.Equal(got, tc.want) {
				t.Errorf("SplitKeywordList(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestKeywordParam(t *testing.T) {
	c, _ := ParseBytes("k.txt", []byte("Name:K\nTypes:Creature\nK:Kicker:B\nK:Flash\nK:Flashback:Sac<1/Creature>\nK:Protection from blue\nOracle:x\n"))
	f := c.Faces[0]
	for head, want := range map[string]string{"Kicker": "B", "Flashback": "Sac<1/Creature>", "Flash": "", "Protection from blue": ""} {
		if got, ok := f.KeywordParam(head); !ok || got != want {
			t.Errorf("%s: %q %v", head, got, ok)
		}
	}
	if _, ok := f.KeywordParam("Delve"); ok {
		t.Error("absent keyword reported present")
	}
}
