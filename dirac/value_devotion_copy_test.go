package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestDevotionCountsACopiedManaCost: CR 707.2 makes a copy acquire the copied
// card's copiable mana cost, and CR 700.5's devotion counts the symbols in
// the mana costs of the permanents the player controls. A red Hill Giant
// that copied the white Serra Angel ({3}{W}{W}) therefore adds two white
// symbols to its controller's devotion.
func TestDevotionCountsACopiedManaCost(t *testing.T) {
	sc := newScen(t, Config{})
	sl := sc.slot(sc.perm(v.HillGiant, 0))
	printed := sc.k.t.Cards[v.HillGiant].Cost.Pips
	if printed[0] != 0 {
		t.Fatalf("precondition: Hill Giant has %d white pips", printed[0])
	}
	copied := sc.k.t.Cards[v.SerraAngel].Cost.Pips
	if copied[0] == 0 {
		t.Fatal("precondition: Serra Angel has no white pips")
	}
	if got := sc.s.devotion(sc.k, kop.ColW, 0); got != 0 {
		t.Fatalf("precondition: the uncopied Hill Giant contributes %d white devotion, want 0", got)
	}
	setCopy(sc, sl, v.SerraAngel)
	if got := sc.s.cardAt(sl); got != v.SerraAngel {
		t.Fatalf("precondition: the permanent shows card %d, want the copied Serra Angel %d", got, v.SerraAngel)
	}
	if got, want := sc.s.devotion(sc.k, kop.ColW, 0), int(copied[0]); got != want {
		t.Fatalf("devotion to white with a copied Serra Angel = %d, want %d", got, want)
	}
}
