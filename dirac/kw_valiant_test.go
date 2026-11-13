package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// valiantTarget runs checkWard for a stack entry controlled by p that
// targets o (n times in its target slots) and returns the pending triggers.
func valiantTarget(sc *scen, p uint8, o Obj, n int) int {
	sc.s.npend = 0
	e := stackEntry{kind: seSpell, obj: NoObj, ctrl: p, nt: uint8(n)}
	for i := 0; i < n; i++ {
		e.tgt[i] = TCode(o)
	}
	sc.s.checkWard(sc.k, &e)
	return int(sc.s.npend)
}

// TestValiantFirstTimeByYou: Seedglaive Mentor's Valiant (gorge
// valiantFirstTimeTargetedByYouThisTurn) fires on the first targeting this
// turn by a spell or ability its controller controls: a second targeting
// that turn fires nothing, a spell naming it twice fires once, an
// opponent's targeting never fires nor disarms it, and a new turn rearms
// it. A new object (zone change) has no mark.
func TestValiantFirstTimeByYou(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Seedglaive Mentor", Count: 2}, {Name: "Plains", Count: 8}})
	sm := id["Seedglaive Mentor"]
	ab := histAb(t, sc, sm, kop.TrigBecomesTarget)
	if a := sc.k.t.Cards[sm].Abilities[ab].Trig; a.N != kop.TrigNValiant || a.Whose != kop.WhoseYou || a.Flags&kop.TfSelf == 0 {
		t.Fatalf("Seedglaive Mentor's trigger %+v is not the Valiant shape", a)
	}
	o := sc.perm(sm, 0)
	steps := []struct {
		p    uint8
		n    int
		want int
		what string
	}{
		{1, 1, 0, "the opponent targets it"},
		{0, 1, 1, "its controller targets it first (after the opponent)"},
		{0, 1, 0, "its controller targets it again this turn"},
		{1, 1, 0, "the opponent targets it again"},
	}
	for _, st := range steps {
		if got := valiantTarget(sc, st.p, o, st.n); got != st.want {
			t.Fatalf("%s: %d triggers, want %d", st.what, got, st.want)
		}
	}
	sc.s.newTurnGen()
	if got := valiantTarget(sc, 0, o, 2); got != 1 {
		t.Fatalf("next turn, one spell naming it twice: %d triggers, want 1", got)
	}
	if got := valiantTarget(sc, 0, o, 1); got != 0 {
		t.Fatalf("next turn, second targeting: %d triggers, want 0", got)
	}
	// another Mentor is another object with its own mark
	o2 := sc.perm(sm, 0)
	if got := valiantTarget(sc, 0, o2, 1); got != 1 {
		t.Fatalf("a second Mentor's first targeting: %d triggers, want 1", got)
	}
	// an opponent's Mentor ignores player 0's targeting
	o3 := sc.perm(sm, 1)
	if got := valiantTarget(sc, 0, o3, 1); got != 0 {
		t.Fatalf("targeting the opponent's Mentor: %d triggers, want 0", got)
	}
	if got := valiantTarget(sc, 1, o3, 1); got != 1 {
		t.Fatalf("its own controller targets the opponent's Mentor: %d triggers, want 1", got)
	}
}
