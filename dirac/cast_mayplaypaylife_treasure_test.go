package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMayPlayPayLifeWithTreasureTap: the pay-life mark and the Treasure-tap
// mark of one cast are independent. A cast that both taps a Treasure source
// and goes through the pay-life permission keeps both and pays the mana value
// once; a cast through a plain free permission that taps a Treasure source
// owes no life.
func TestMayPlayPayLifeWithTreasureTap(t *testing.T) {
	run := func(t *testing.T, b int) (sc *scen, o Obj, mv int) {
		sc = newScenTable(t, mayPlayPayLifeTable(), Config{NoElide: true})
		s, k := sc.s, sc.k
		mv = int(k.t.Cards[v.ThinkTwice].Cost.MV())
		o = sc.obj(v.ThinkTwice, 0)
		s.place(o, kop.ZoneExile, false)
		s.g.rem[0], s.g.nrem = o, 1
		in := kop.Instr{Op: kop.OpMayPlay, Obj: kop.RefRemembered, A: kop.Const(kop.DurEOT), B: kop.Const(b), C: int16(kop.ZoneExile)}
		if ok, done := s.execGen(k, &in); !ok || !done {
			t.Fatalf("OpMayPlay not handled (%v, %v)", ok, done)
		}
		s.pl[0].life = 20
		sc.do(t, AkCast, TCode(o), scAny)
		sc.perm(v.GrizzlyBears, 0)
		// a pool spend that takes a Treasure's unit marks the build (treSync)
		s.g.poolTre[0][kop.G] = 1
		s.pl[0].pool[kop.G] = 0
		s.treSync(0)
		if s.bld().bfl&bflTre == 0 {
			t.Fatalf("the Treasure spend was not recorded (bfl %#x)", s.bld().bfl)
		}
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
		return sc, o, mv
	}
	t.Run("pay life", func(t *testing.T) {
		sc, _, mv := run(t, kop.MayPlayPayLife)
		if got := int(sc.s.pl[0].life); got != 20-mv {
			t.Fatalf("life %d after a Treasure-tapping pay-life cast, want %d", got, 20-mv)
		}
	})
	t.Run("plain free", func(t *testing.T) {
		sc, _, _ := run(t, kop.MayPlayFree)
		if got := sc.s.pl[0].life; got != 20 {
			t.Fatalf("life %d after a Treasure-tapping free cast, want 20 (the Treasure mark read as pay-life)", got)
		}
	})
}
