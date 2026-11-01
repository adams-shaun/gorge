package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// kbargainTestTable is vanilla's table plus extra card defs, with a generated
// table shape so prep.gen is set (mintSpellCopy reads it).
func kbargainTestTable(extra ...kop.CardDef) (*kop.Table, []CardID) {
	tb := *v.Table
	tb.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tb.Exprs = make([]kop.Expr, 1)
	ids := make([]CardID, 0, len(extra))
	for _, d := range extra {
		ids = append(ids, CardID(len(tb.Cards)))
		tb.Cards = append(tb.Cards, d)
	}
	return &tb, ids
}

// TestBargainOfferNoSurface exercises the kernel's own legal set: with no
// gorge surface (Config{}), casualtyOffers offers the bargained cast whenever
// a candidate exists, independent of the floating pool. The lockstep's two
// Config parities both set a surface, so this branch has no other test.
func TestBargainOfferNoSurface(t *testing.T) {
	tb, ids := kbargainTestTable(kop.CardDef{
		Name: "Test Bargain", Cost: kop.ManaCost{Generic: 1}, Types: kop.TSorcery, Bargain: true,
	})
	bargain := ids[0]
	sc := newScenTable(t, tb, Config{})
	s := sc.s
	k := sc.k

	spell := sc.hand(bargain, 0)
	if s.bargainOffer(k, 0, bargain) {
		t.Fatal("bargainOffer without a candidate")
	}
	var d Decision
	s.bargainCands(k, 0, &d)
	if d.N != 0 {
		t.Fatalf("bargainCands without a candidate: %d options", d.N)
	}
	if !s.casualtyOffers(0, spell) {
		t.Fatal("casualtyOffers with no surface: want the cast offered without a pool")
	}

	art := sc.perm(v.PerilousMyr, 0) // an artifact candidate
	if !s.bargainOffer(k, 0, bargain) {
		t.Fatal("bargainOffer with an artifact candidate")
	}
	d.N = 0
	s.bargainCands(k, 0, &d)
	if d.N != 1 || Obj(d.Options[0].Source()) != art {
		t.Fatalf("bargainCands: %d options, want the artifact %d", d.N, art)
	}

	// A gorge surface prices the plain cast from the floating pool: an empty
	// pool lists no bargained cast even with a candidate.
	sc2 := newScenTable(t, tb, Config{GorgeAutopaySurface: true})
	spell2 := sc2.hand(bargain, 0)
	sc2.perm(v.PerilousMyr, 0)
	if sc2.s.casualtyOffers(0, spell2) {
		t.Fatal("casualtyOffers under an empty-pool surface: want the cast hidden")
	}
}

// TestBargainCopyDropsFlag is the CR 707.10 guard: a copy of a bargained
// spell is never cast and carries no FlagBargained, while a kicked spell's
// copy keeps sfKicked (the ordinary copy behaviour).
func TestBargainCopyDropsFlag(t *testing.T) {
	tb, ids := kbargainTestTable(
		kop.CardDef{Name: "Test Bargain Spell", Types: kop.TSorcery, Bargain: true},
		kop.CardDef{Name: "Test Kicked Spell", Types: kop.TSorcery},
	)
	bargain, kicked := ids[0], ids[1]
	sc := newScenTable(t, tb, Config{})
	s := sc.s
	for _, tc := range []struct {
		name string
		card CardID
		want bool // the copy keeps sfKicked
	}{
		{"bargained", bargain, false},
		{"kicked", kicked, true},
	} {
		o := sc.hand(tc.card, 0)
		s.stack[0] = stackEntry{kind: seSpell, obj: o, card: tc.card, ctrl: 0, flags: sfKicked}
		s.sp = 1
		if !s.mintSpellCopy(sc.k, 0, 0) {
			t.Fatalf("%s: mintSpellCopy failed", tc.name)
		}
		if got := s.stack[0].flags&sfKicked != 0; got != tc.want {
			t.Errorf("%s copy keeps sfKicked = %v, want %v", tc.name, got, tc.want)
		}
		s.sp = 0
	}
}
