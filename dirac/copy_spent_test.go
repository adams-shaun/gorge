package dirac

import "github.com/adams-shaun/gorge/dirac/kop"
import "testing"

// TestSpellCopyHasNoManaSpent: a copy of a spell is never cast (CR 707.10),
// so "the amount of mana spent to cast this spell" reads 0 on it even when
// the original was cast for 5 (Molten Note copied by Mica, Reader of Ruins).
func TestSpellCopyHasNoManaSpent(t *testing.T) {
	tb, ids := kbargainTestTable(kop.CardDef{Name: "Test Spend Spell", Types: kop.TSorcery})
	sc := newScenTable(t, tb, Config{})
	s := sc.s
	o := sc.hand(ids[0], 0)
	s.stack[0] = stackEntry{kind: seSpell, obj: o, card: ids[0], ctrl: 0, trigAmt: 5, x: 2}
	s.sp = 1
	if got := s.spentOfSpell(&s.stack[0]); got != 5 {
		t.Fatalf("the original's spend is %d, want 5", got)
	}
	if !s.mintSpellCopy(sc.k, 0, 0) {
		t.Fatal("mintSpellCopy failed")
	}
	if got := s.spentOfSpell(&s.stack[0]); got != 0 {
		t.Errorf("the copy's mana spent is %d, want 0", got)
	}
	if s.stack[0].x != 2 {
		t.Errorf("the copy's X is %d, want the original's 2", s.stack[0].x)
	}
}
