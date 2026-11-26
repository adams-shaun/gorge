package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Two filter words and one count head in lockstep with gorge, on one fixture
// pool (fixtures/manacolour):
//
//   - canProduceManaColor <Colour> (kgen fwArms, kernel
//     FwCanProduceManaColor over cardProducible; gorge predicates
//     "canProduceManaColor <Colour>" over producibleSymbols): Break the
//     Ice's target "land that is snow or could produce {C}", Obsidian
//     Charmaw's {1} less per opponent's land that could produce {C}. The
//     other deck plays Wastes beside Plains and Swamps.
//   - attackingYouOrYourPW (kgen cmpAttackingYou PredN 1, kernel
//     attackingYouOrPW; gorge attackingYouOrYourPWLKI): Soul Snare's target.
//   - Count$ValidAll over an attacking spec (kgen validAllBattlefieldOnly:
//     the battlefield Count$Valid fold): Mangara, the Diplomat's and Tomik,
//     Wielder of Law's "two or more attacking you" intervening-if.

var manaColourWatch = map[string]bool{
	"Break the Ice": true, "Obsidian Charmaw": true, "Soul Snare": true,
	"Mangara, the Diplomat": true, "Tomik, Wielder of Law": true,
}

var manaColourBRG = []cardset.Entry{
	{Name: "Break the Ice", Count: 7}, {Name: "Obsidian Charmaw", Count: 6}, {Name: "Hill Giant", Count: 6},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 3},
}

var manaColourWBC = []cardset.Entry{
	{Name: "Soul Snare", Count: 6}, {Name: "Mangara, the Diplomat", Count: 5}, {Name: "Tomik, Wielder of Law", Count: 5},
	{Name: "Walking Corpse", Count: 4}, {Name: "Plains", Count: 9}, {Name: "Swamp", Count: 4}, {Name: "Wastes", Count: 7},
}

var manaColourDecks = []fixtureDecks{{manaColourBRG, manaColourWBC}, {manaColourWBC, manaColourBRG}}

func TestManaColourLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "manacolour", manaColourDecks, manaColourWatch, 160, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	for _, n := range []string{"Break the Ice", "Obsidian Charmaw", "Soul Snare", "Mangara, the Diplomat", "Tomik, Wielder of Law"} {
		if run.casts[n] < 10 {
			t.Errorf("%s cast %d times", n, run.casts[n])
		}
	}
	// a colourless ask that also accepts white: every Plains could produce {C}
	mut := runFixtureLockstep(t, "manacolour", manaColourDecks, manaColourWatch, 160, false, func(tbl *kop.Table) {
		tbl.FilterProgs = append([]kop.FilterProg(nil), tbl.FilterProgs...)
		n := 0
		for i := range tbl.FilterProgs {
			lits := append([]kop.FLit(nil), tbl.FilterProgs[i].Lits...)
			for j := range lits {
				if lits[j].W == kop.FwCanProduceManaColor {
					lits[j].Arg |= int16(kop.ColW)
					n++
				}
			}
			tbl.FilterProgs[i].Lits = lits
		}
		if n == 0 {
			t.Fatal("precondition: no table literal is FwCanProduceManaColor")
		}
	})
	mut.requireDivergence(t, "canProduceManaColor Colorless widened to white")
	// the attacking-you-or-your-planeswalker predicate dropped: any attacker
	mut = runFixtureLockstep(t, "manacolour", manaColourDecks, manaColourWatch, 160, false, func(tbl *kop.Table) {
		tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
		n := 0
		for i := range tbl.Filters {
			if f := &tbl.Filters[i]; f.Pred == kop.PredAttackingYou && f.PredN == 1 {
				f.Pred, f.PredN = kop.PredNone, 0
				n++
			}
		}
		if n == 0 {
			t.Fatal("precondition: no table filter is attackingYouOrYourPW")
		}
	})
	mut.requireDivergence(t, "attackingYouOrYourPW dropped to attacking")
}
