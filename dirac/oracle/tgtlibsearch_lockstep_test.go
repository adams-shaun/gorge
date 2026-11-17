package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// "Search target player's library" (kgen lower_zone.go targetLibrarySearch):
// Acquire and Bribery put an opponent's artifact or creature onto the
// battlefield under your control, Jester's Cap exiles three cards of any
// target player's library; you choose in each (gorge searchTargetedOwner).
var tgtLibSearchDecks = [2][]cardset.Entry{
	{
		{Name: "Bribery", Count: 4}, {Name: "Acquire", Count: 3}, {Name: "Jester's Cap", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Island", Count: 12}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Ornithopter", Count: 6}, {Name: "Jester's Cap", Count: 4},
		{Name: "Forest", Count: 12}, {Name: "Island", Count: 8},
	},
}

func TestChangeTypeValidTgtsLockstep(t *testing.T) {
	t.Parallel()
	lockstepRun(t, "tgtlibsearch", tgtLibSearchDecks, 80, true, nil)
}

// TestChangeTypeValidTgtsLockstepMutation: with the chooser moved to the
// searched library's owner (SrChooserYou dropped), some seed diverges.
func TestChangeTypeValidTgtsLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "tgtlibsearch", tgtLibSearchDecks, 80, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				for k := range prog {
					if prog[k].Op == kop.OpSearchEx {
						prog[k].C &^= int16(kop.SrChooserYou << 3) // the Sr flags ride C>>3 (kgen instr_search.go)
					}
				}
				abs[j].Prog = prog
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the searched library's owner choosing")
	}
}
