package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The gap-copyspell slice in lockstep (dirac/kgen/lower_effect_copy.go,
// lower_copyspell.go; dirac/copy_spellnonlegendary.go): NonLegendary$ spell
// copies of legendary creature spells (Jackal, Double Major, The Sixth
// Doctor, Storm of Saruman; The Clone Saga's chapter III names a card from
// the whole corpus, which this pool-only table cannot offer), Meletis Charlatan's copy for the
// target spell's controller, Wild Ricochet's copy of its retargeted spell, and
// the Chain of Acid / Chain of Smog may-copy asked of the target's
// controller. Two-seat games and four-seat pods.
var (
	copyNonLegDeck = []cardset.Entry{
		{Name: "Jackal, Genius Geneticist", Count: 6}, {Name: "Double Major", Count: 6},
		{Name: "The Sixth Doctor", Count: 3}, {Name: "Storm of Saruman", Count: 3},
		{Name: "Azusa, Lost but Seeking", Count: 4}, {Name: "Kira, Great Glass-Spinner", Count: 3},
		{Name: "Llanowar Elves", Count: 6}, {Name: "Forest", Count: 8}, {Name: "Island", Count: 8},
	}
	copyURDeck = []cardset.Entry{
		{Name: "Meletis Charlatan", Count: 6}, {Name: "Wild Ricochet", Count: 6},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 4}, {Name: "Opt", Count: 4},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 7},
	}
	copyBGDeck = []cardset.Entry{
		{Name: "Chain of Smog", Count: 8}, {Name: "Chain of Acid", Count: 6},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8},
	}
)

// copiesSeen counts the stack spell copies a game has made so far.
func copiesSeen() func(g *state.Game) int {
	var cur *state.Game
	seen := map[state.ObjID]bool{}
	return func(g *state.Game) int {
		if g != cur {
			cur, seen = g, map[state.ObjID]bool{}
		}
		for p := range g.Players {
			for _, id := range g.Zone(state.ZStack, state.PlayerID(p)) {
				if o := g.Obj(id); o != nil && o.IsCopy {
					seen[id] = true
				}
			}
		}
		return len(seen)
	}
}

func TestCopySpellNonLegendaryLockstep(t *testing.T) {
	t.Parallel()
	d := copyNonLegDeck
	names := []string{"Jackal, Genius Geneticist", "Double Major", "The Sixth Doctor", "Storm of Saruman"}
	tally := func(g *state.Game) int { // the stripped copies on the battlefield
		n := 0
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if o := g.Obj(id); o != nil && o.CopyNonLegendary {
					n++
				}
			}
		}
		return n
	}
	two := cedhPodRun(t, "copynonleg", [][]cardset.Entry{d, d}, names, 40, tally)
	four := cedhPodRun(t, "copynonleg", [][]cardset.Entry{d, d, d, d}, names, 16, tally)
	t.Logf("non-legendary copies on the battlefield at game end: %d in two-seat games, %d in pods", two, four)
	if two < 5 || four < 5 {
		t.Fatalf("too little evidence: %d two-seat, %d four-seat (want 5 each)", two, four)
	}
}

func TestCopySpellControllerLockstep(t *testing.T) {
	t.Parallel()
	names := []string{"Meletis Charlatan", "Wild Ricochet", "Chain of Smog", "Chain of Acid"}
	two := cedhPodRun(t, "copyctrl", [][]cardset.Entry{copyURDeck, copyBGDeck}, names, 40, copiesSeen())
	four := cedhPodRun(t, "copyctrl", [][]cardset.Entry{copyURDeck, copyBGDeck, copyURDeck, copyBGDeck}, names, 16, copiesSeen())
	t.Logf("spell copies made: %d in two-seat games, %d in pods", two, four)
	if two < 10 || four < 10 {
		t.Fatalf("too little evidence: %d two-seat, %d four-seat (want 10 each)", two, four)
	}
}
