package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// payLifeTwoScenario is a pay-life library static in force `emblems` times
// (Gwenom's emblem, copied by Peter Parker's Camera) and Think Twice on top.
func payLifeTwoScenario(t *testing.T, emblems uint8) (*scen, Obj) {
	t.Helper()
	sc := newScenTable(t, playTopTable(kop.StMayPayLife), Config{NoElide: true})
	o := playTopPutOnTop(sc, v.ThinkTwice)
	n := uint8(len(sc.k.t.Emblems))
	for i := uint8(0); i < emblems; i++ {
		sc.s.g.emb[0][i] = n
	}
	sc.s.g.nemb[0] = emblems
	sc.s.pl[0].life = 20
	return sc, o
}

// payLifeCasts is the cast options of o in the priority decision.
func payLifeCasts(sc *scen, o Obj) (casts []Action) {
	var d Decision
	var av avail
	sc.s.priorityGen(sc.k, &d, 0, true, &av, false)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
			casts = append(casts, a)
		}
	}
	return casts
}

// TestPlayFromTopEmblemTwoPayLife: two pay-life statics in force and no other
// permission offer one cast each (gorge lists one AltCostIndex per Effect,
// CR 118.9) instead of stalling; one static keeps the single target-0 cast.
func TestPlayFromTopEmblemTwoPayLife(t *testing.T) {
	t.Run("one static keeps the single cast", func(t *testing.T) {
		sc, o := payLifeTwoScenario(t, 1)
		got := payLifeCasts(sc, o)
		if len(got) != 1 || got[0].Target() != 0 || got[0].Mode()&castMayPlay == 0 {
			t.Fatalf("one static offered %v, want one may-play cast at target 0", got)
		}
		if sc.s.Stalled() {
			t.Fatalf("stalled (%s)", StallName(sc.s.StallReason()))
		}
	})
	t.Run("two statics offer one alternative each", func(t *testing.T) {
		sc, o := payLifeTwoScenario(t, 2)
		s := sc.s
		mv := int(sc.k.t.Cards[v.ThinkTwice].Cost.MV())
		got := payLifeCasts(sc, o)
		if len(got) != 2 || got[0].Target() != 1 || got[1].Target() != 2 || got[0].Mode() != got[1].Mode() ||
			got[0].Mode()&castMayPlay == 0 {
			t.Fatalf("two statics offered %v, want two may-play casts at targets 1 and 2", got)
		}
		if s.Stalled() {
			t.Fatalf("stalled (%s)", StallName(s.StallReason()))
		}
		sc.do(t, AkCast, uint8(o), 2)
		for i := 0; i < 8 && s.objs[o].zone != kop.ZoneStack; i++ {
			d := s.Step()
			if d.N == 0 {
				break
			}
			s.Apply(d.Options[0])
		}
		if s.objs[o].zone != kop.ZoneStack {
			t.Fatalf("the spell did not reach the stack (zone %v)", s.objs[o].zone)
		}
		if got := int(s.pl[0].life); got != 20-mv {
			t.Fatalf("life after the cast is %d, want %d", got, 20-mv)
		}
	})
}
