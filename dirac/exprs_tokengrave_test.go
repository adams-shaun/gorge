package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A creature token that died waits in the graveyard until the next state-based
// check ceases it; it is not a card, so "each creature card in your graveyard"
// (Moon-Vigil Adherents's second pass) does not count it (CR 111.7, 704.5d).
func TestGraveyardCountSkipsToken(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Burglar Rat", Count: 1}, {Name: "Swamp", Count: 8}})
	sc.k.t.Exprs = append(sc.k.t.Exprs, kop.Expr{Kind: kop.ExCountGraveyard})
	i := int16(len(sc.k.t.Exprs) - 1)
	count := func() int { return sc.s.expr(sc.k, i, evalCtx{src: NoObj}) }
	sc.grave(id["Burglar Rat"], 0, 1)
	if got := count(); got != 1 {
		t.Fatalf("one card in the graveyard counts %d, want 1", got)
	}
	tok := sc.s.newToken(id["Burglar Rat"], 0)
	sc.s.place(tok, kop.ZoneGraveyard, false)
	if got := count(); got != 1 {
		t.Fatalf("a dead token in the graveyard counts %d cards, want 1", got)
	}
}
