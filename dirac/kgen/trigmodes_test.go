package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// trigsOf returns the lowered triggers of a card, in ability order, of event ev.
func trigsOf(c *lCard, ev string) []*lTrigger {
	var out []*lTrigger
	for i := range c.Abilities {
		if tr := c.Abilities[i].Trigger; tr != nil && tr.Event == ev {
			out = append(out, tr)
		}
	}
	return out
}

func TestTrigModesLower(t *testing.T) {
	names := []string{"Shadow of the Goblin", "The Endstone", "Wan Shi Tong, Librarian", "Corpseberry Cultivator", "Curator of Sun's Creation"}
	by := map[string]*lCard{}
	for _, c := range lowerNamed(t, names) {
		by[c.Name] = c
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
	}
	for _, n := range names {
		if by[n] == nil {
			t.Fatalf("%s not lowered", n)
		}
	}
	gy, ex, lib, hand := int16(1<<kop.ZoneGraveyard), int16(1<<kop.ZoneExile), int16(1<<kop.ZoneLibrary), int16(1<<kop.ZoneHand)
	sh := trigsOf(by["Shadow of the Goblin"], "LandPlayed")
	if len(sh) != 2 || sh[0].N != gy|ex|lib || sh[1].N != hand {
		t.Fatalf("Shadow's LandPlayed lines: %+v, want N = Graveyard|Exile|Library then Hand", sh)
	}
	if es := trigsOf(by["The Endstone"], "LandPlayed"); len(es) != 1 || es[0].N != 0 || es[0].F == nil {
		t.Errorf("Endstone's LandPlayed line: %+v, want N = 0 with a filter", es)
	}
	if w := trigsOf(by["Wan Shi Tong, Librarian"], "SearchedLibrary"); len(w) != 1 || w[0].Whose != whoOpp {
		t.Errorf("Wan Shi Tong's SearchedLibrary line: %+v, want Whose = opponent", w)
	}
	if f := trigsOf(by["Corpseberry Cultivator"], "Forage"); len(f) != 1 || f[0].Whose != whoYou {
		t.Errorf("Corpseberry's Forage line: %+v, want Whose = you", f)
	}
	if d := trigsOf(by["Curator of Sun's Creation"], "Discover"); len(d) != 1 || d[0].Whose != whoYou || !d[0].OncePer {
		t.Errorf("Curator's Discover line: %+v, want Whose = you, once per turn", d)
	}
}

func TestTrigModesRefuse(t *testing.T) {
	readF := func(string) (*lFilter, error) { return nil, nil }
	for _, c := range []struct {
		mode string
		m    map[string]string
		want string
	}{
		{"SearchedLibrary", map[string]string{"Mode": "SearchedLibrary", "ValidPlayer": "You"}, "SearchOwnLibrary"},
		{"LandPlayed", map[string]string{"Mode": "LandPlayed", "ValidCard": "Land", "Origin": "Ante"}, "Origin$"},
		{"LandPlayed", map[string]string{"Mode": "LandPlayed", "ValidCard": "Land", "Origin": "Stack"}, "Origin$"},
		{"LandPlayed", map[string]string{"Mode": "LandPlayed", "ValidSA": "Spell"}, "ValidSA$"},
		{"Discover", map[string]string{"Mode": "Discover", "ValidCard": "Card"}, "ValidCard$"},
	} {
		err := (&lowerCtx{}).trigModes2(newParams(c.m), &lTrigger{}, c.mode, "", readF)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s %v: err %v, want a refusal naming %q", c.mode, c.m, err, c.want)
		}
	}
}
