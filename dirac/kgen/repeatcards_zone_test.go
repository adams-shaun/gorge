package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRepeatCardsZoneShapes: RepeatEach RepeatCards$ over a Zone$ other than
// the battlefield. The kernel's card walk (dirac eachCardList) reads
// battlefield slots only; the one hand shape that lowers is Sylvan Library's
// Card.ChosenCard over Zone$ Hand, a plain loop over the resolution's chosen
// cards (lower_repeatchosenhand.go). The zone refusal comes after the filter
// checks, so each other card's RepeatEach node names its real blocker, and a
// zone list names the list.
func TestRepeatCardsZoneShapes(t *testing.T) {
	ratchetCards(t)
	sylvanLibraryLowers(t)
	for _, tc := range []struct{ card, want string }{
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

// sylvanLibraryLowers: Sylvan Library lowers, its RepeatEach a plain ordered
// loop over the chosen cards (RefChosen) whose body is the PayLife<4> unless
// frame around the hand pick that puts a chosen card back.
func sylvanLibraryLowers(t *testing.T) {
	t.Helper()
	c := lowerCorpusCard(t, "Sylvan Library")
	for _, rf := range c.Refusals {
		t.Fatalf("Sylvan Library: refused %q", rf.Reason)
	}
	var prog []lInstr
	for _, a := range c.Abilities {
		for i := range a.Prog {
			if a.Prog[i].Op == "RepeatEachCard" {
				prog = a.Prog[i:]
			}
		}
	}
	if len(prog) < 3 {
		t.Fatalf("Sylvan Library: no RepeatEachCard with a two-instruction body")
	}
	h := prog[0]
	if h.Obj.Kind != rChosen || h.C != kop.RepeatPlain|kop.RepeatEachOrder || h.Skip != 2 {
		t.Errorf("Sylvan Library: header %+v, want Obj Chosen, C RepeatPlain|RepeatEachOrder, skip 2", h)
	}
	if prog[1].Op != "UnlessPaid" || prog[1].CS != "PayLife" || prog[2].Op != "ChooseMove" || prog[2].F == nil {
		t.Errorf("Sylvan Library: body %s %s, want UnlessPaid PayLife then ChooseMove", prog[1].Op, prog[2].Op)
	}
}
