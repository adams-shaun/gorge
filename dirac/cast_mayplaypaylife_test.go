package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// mayPlayPayLifeTable is the vanilla table plus a sorcery whose program
// grants the pay-life permission over the remembered set, so prepareGen arms
// the permission-kind walks.
func mayPlayPayLifeTable() *kop.Table {
	t := v.NewTable()
	t.Exprs = []kop.Expr{}
	t.Cards = append(t.Cards, kop.CardDef{
		Name:  "Test Pay Life Grant",
		Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpMayPlay, Obj: kop.RefRemembered, A: kop.Const(kop.DurEOT), B: kop.Const(kop.MayPlayPayLife), C: int16(kop.ZoneExile)},
		}}},
	})
	return t
}

// TestMayPlayPayLifeCast: a card the remembered grant carries
// MayPlayAltManaCost$ PayLife<ConvertedManaCost> for is cast for its mana
// value in life and nothing else. It is offered as a free may-play cast only
// while the life covers the mana value; the cast takes exactly that much life
// and spends the permission. A plain free grant over the same card is the
// control: it costs no life.
func TestMayPlayPayLifeCast(t *testing.T) {
	sc := newScenTable(t, mayPlayPayLifeTable(), Config{NoElide: true})
	s, k := sc.s, sc.k
	mv := int(k.t.Cards[v.ThinkTwice].Cost.MV())
	if mv != 2 {
		t.Fatalf("Think Twice has mana value %d, want 2", mv)
	}
	o := sc.obj(v.ThinkTwice, 0)
	s.place(o, kop.ZoneExile, false)
	s.g.rem[0], s.g.nrem = o, 1
	grant := func(b int) {
		s.g.nmay = 0
		in := kop.Instr{Op: kop.OpMayPlay, Obj: kop.RefRemembered, A: kop.Const(kop.DurEOT), B: kop.Const(b), C: int16(kop.ZoneExile)}
		if ok, done := s.execGen(k, &in); !ok || !done {
			t.Fatalf("OpMayPlay not handled (%v, %v)", ok, done)
		}
		if s.g.nmay != 1 {
			t.Fatalf("%d records, want 1", s.g.nmay)
		}
	}
	castMode := func() (mode uint8, ok bool) {
		var d Decision
		var av avail
		s.priorityGen(k, &d, 0, true, &av, false)
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
				return a.Mode(), true
			}
		}
		return 0, false
	}

	grant(kop.MayPlayPayLife)
	if !k.gen.anyMayKind || !s.g.may[0].payLife() || s.g.may[0].kind&kop.MayPlayFree == 0 {
		t.Fatalf("pay-life record %+v (anyMayKind %v)", s.g.may[0], k.gen.anyMayKind)
	}
	s.pl[0].life = int16(mv - 1)
	if _, ok := castMode(); ok {
		t.Fatalf("offered a cast owing %d life at %d life", mv, s.pl[0].life)
	}
	s.pl[0].life = int16(mv)
	mode, ok := castMode()
	if !ok || mode != castMayPlay|castFree {
		t.Fatalf("at exactly %d life the cast is offered %v with mode %#x, want a free may-play cast", mv, ok, mode)
	}

	s.pl[0].life = 20
	sc.do(t, AkCast, TCode(o), scAny)
	for i := 0; i < 8 && s.objs[o].zone != kop.ZoneStack; i++ {
		d := s.Step()
		if d.N == 0 {
			break
		}
		s.Apply(d.Options[0])
	}
	if s.objs[o].zone != kop.ZoneStack {
		t.Fatalf("the spell did not reach the stack (zone %d)", s.objs[o].zone)
	}
	if got := int(s.pl[0].life); got != 20-mv {
		t.Fatalf("life after the cast is %d, want %d (mana value %d paid)", got, 20-mv, mv)
	}
	if s.g.nmay != 0 {
		t.Fatalf("the permission was not spent (%d records)", s.g.nmay)
	}

	// control: the same card under a plain free record owes no life
	s2 := newScenTable(t, mayPlayPayLifeTable(), Config{NoElide: true})
	s, k = s2.s, s2.k
	o = s2.obj(v.ThinkTwice, 0)
	s.place(o, kop.ZoneExile, false)
	s.g.rem[0], s.g.nrem = o, 1
	grant(kop.MayPlayFree)
	if s.g.may[0].payLife() {
		t.Fatal("a plain free record reads as pay-life")
	}
	s.pl[0].life = 20
	s2.do(t, AkCast, TCode(o), scAny)
	for i := 0; i < 8 && s.objs[o].zone != kop.ZoneStack; i++ {
		d := s.Step()
		if d.N == 0 {
			break
		}
		s.Apply(d.Options[0])
	}
	if s.objs[o].zone != kop.ZoneStack || s.pl[0].life != 20 {
		t.Fatalf("plain free cast: zone %d life %d, want the stack and 20", s.objs[o].zone, s.pl[0].life)
	}
}
