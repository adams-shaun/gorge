package oracle

import "github.com/adams-shaun/gorge/dirac"

// The combat and history words of kgen's filter-word table
// (dirac/kgen/filter_words_combat.go), each a kop.FilterWord literal in a
// PredProg: IsGoaded, firstTurnControlled and its '!' negation, the
// entered-from-graveyard word, and the '!' negation of kicked.
func init() {
	for _, w := range []string{"Card.IsGoaded", "Card.firstTurnControlled", "Card.!firstTurnControlled"} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomBattlefield})
	}
	// gorge answers kicked on a spell or a permanent and false anywhere else
	RegisterFilterAtom(FilterAtom{Name: "fw:Card.!kicked", GorgeSpec: "Card.!kicked", Ctx: AtomAnyZone})
	// the entered ledger is kept only for a table that reads it
	RegisterFilterAtom(FilterAtom{Name: "fw:Card.ThisTurnEnteredFrom_Graveyard", GorgeSpec: "Card.ThisTurnEnteredFrom_Graveyard", Ctx: AtomAnyZone, Needs: dirac.BankEntered})
}
