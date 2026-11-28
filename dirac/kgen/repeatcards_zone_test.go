package kgen

import (
	"strings"
	"testing"
)

// TestRepeatCardsZoneShapes: RepeatEach RepeatCards$ over a Zone$ other than
// the battlefield. The kernel's card walk (dirac eachCardList) reads
// battlefield slots only, so no card of the class lowers yet; the zone
// refusal now comes after the filter checks, so each card's RepeatEach node
// names its real blocker, and a zone list names the list.
func TestRepeatCardsZoneShapes(t *testing.T) {
	ratchetCards(t)
	for _, tc := range []struct{ card, want string }{
		{"Sylvan Library", `unknown property "DrawnThisTurn"`},
		{"Read the Runes", `RepeatCards$ "Card.IsRemembered" reads Remember`},
		{"Vault 21: House Gambit", `RepeatCards$ "Card.IsRemembered" reads Remember`},
		{"Scythe Specter", `RepeatEach cards Zone$ list "Battlefield,Graveyard,Exile,Library,Hand"`},
	} {
		c := lowerCorpusCard(t, tc.card)
		var rs []string
		for _, rf := range c.Refusals {
			rs = append(rs, rf.Reason)
		}
		got := strings.Join(rs, "; ")
		if len(rs) == 0 {
			t.Fatalf("%s: no refusal (the precondition: the card stays refused)", tc.card)
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: refusals %q, want one naming %q", tc.card, got, tc.want)
		}
		if strings.Contains(got, `RepeatEach cards Zone$ "`) {
			t.Errorf("%s: refusals %q still name the bare Zone$ class", tc.card, got)
		}
	}
	// a filter that lowers over a hand or exile walk keeps the narrower reason
	for _, z := range []string{"Hand", "Exile"} {
		got := strings.Join(lowerScript(t, "Probe Antics", repeatCardsProbe("RepeatCards$ Creature.OppCtrl | Zone$ "+z, "DB$ GainLife | LifeAmount$ 1")), "; ")
		if want := `RepeatEach cards Zone$ "` + z + `": the kernel walks battlefield cards only`; !strings.Contains(got, want) {
			t.Errorf("Zone$ %s: refusals %q, want %q", z, got, want)
		}
	}
}
