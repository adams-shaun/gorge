package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// playTopTable is the vanilla table plus one end-of-turn emblem holding a
// library MayPlayFrom static (The Belligerent / Gwenom's Effect: "you may
// play cards from the top of your library this turn"); flags are the static's.
func playTopTable(flags kop.StaticFlags) *kop.Table {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // a generated table: prepareGen keeps g.gen
	tbl.Emblems = append(append([]kop.Emblem{}, v.Table.Emblems...), kop.Emblem{
		UntilEOT: true,
		Abilities: []kop.Ability{{Kind: kop.AbStatic,
			Static: kop.Static{Kind: kop.StatMayPlayFrom, Zone: kop.ZoneLibrary, StFlags: flags}}},
	})
	return &tbl
}

// playTopOffers is the library cards player 0 is offered to play or cast.
func playTopOffers(sc *scen, cards ...Obj) (offered []Obj) {
	var d Decision
	var av avail
	sc.s.priorityGen(sc.k, &d, 0, true, &av, false)
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != AkPlayLand && a.Kind() != AkCast {
			continue
		}
		for _, o := range cards {
			if Obj(a.Source()) == o {
				offered = append(offered, o)
			}
		}
	}
	return offered
}

func playTopPutOnTop(sc *scen, c CardID) Obj {
	o := sc.obj(c, 0)
	sc.s.objs[o].zone = kop.ZoneLibrary
	sc.s.pl[0].libTop(o)
	return o
}

// TestPlayFromTopEmblem: an emblem's library permission offers the player's
// current top card only: a card under it is not offered, and once the top
// card leaves the library (a draw) the card beneath it is. With the pay-life
// flag a spell on top is offered free of mana while the life covers its mana
// value, and the cast takes that much life.
func TestPlayFromTopEmblem(t *testing.T) {
	t.Run("top card only, follows a draw", func(t *testing.T) {
		sc := newScenTable(t, playTopTable(0), Config{NoElide: true})
		s := sc.s
		under := playTopPutOnTop(sc, v.Plains)
		top := playTopPutOnTop(sc, v.Forest)
		if got := playTopOffers(sc, top, under); len(got) != 0 {
			t.Fatalf("offered %v with no emblem in force, want nothing: the precondition for the offers below", got)
		}
		s.g.emb[0][0], s.g.nemb[0] = uint8(len(s.k().t.Emblems)), 1 // the table's last emblem
		if got := playTopOffers(sc, top, under); len(got) != 1 || got[0] != top {
			t.Fatalf("offered %v, want only the top card %d (not %d)", got, top, under)
		}
		s.draw(0)
		if s.objs[top].zone != kop.ZoneHand {
			t.Fatalf("the draw did not take the top card (zone %v)", s.objs[top].zone)
		}
		if got := playTopOffers(sc, top, under); len(got) != 1 || got[0] != under {
			t.Fatalf("after the draw offered %v, want the new top card %d", got, under)
		}
	})

	t.Run("pay life", func(t *testing.T) {
		sc := newScenTable(t, playTopTable(kop.StMayPayLife), Config{NoElide: true})
		s, k := sc.s, sc.k
		mv := int(k.t.Cards[v.ThinkTwice].Cost.MV())
		o := playTopPutOnTop(sc, v.ThinkTwice)
		s.g.emb[0][0], s.g.nemb[0] = uint8(len(k.t.Emblems)), 1
		s.pl[0].life = int16(mv - 1)
		if got := playTopOffers(sc, o); len(got) != 0 {
			t.Fatalf("offered a cast owing %d life at %d life", mv, s.pl[0].life)
		}
		s.pl[0].life = 20
		if got := playTopOffers(sc, o); len(got) != 1 {
			t.Fatalf("not offered at 20 life with no mana: %v", got)
		}
		sc.do(t, AkCast, uint8(o), 0xff)
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

	t.Run("pay life beside a second permission offers both casts", func(t *testing.T) {
		sc, o := payLifeBeside(t, 0, 2)
		s, k := sc.s, sc.k
		mv := int(k.t.Cards[v.ThinkTwice].Cost.MV())
		if mv == 0 || s.objs[o].zone != kop.ZoneLibrary {
			t.Fatalf("precondition: Think Twice mana value %d in zone %v", mv, s.objs[o].zone)
		}
		if got := payLifeCastModes(sc, o); len(got) != 2 || got[0] != castMayPlay || got[1] != castMayPlay|castFree {
			t.Fatalf("cast modes %v, want the plain cast %d then the life alternative %d", got, castMayPlay, castMayPlay|castFree)
		}
		if s.Stalled() {
			t.Fatalf("stalled (%s)", StallName(s.StallReason()))
		}
		sc, o = payLifeBeside(t, 0, 0) // no lands: the mana cast can't be paid
		if got := payLifeCastModes(sc, o); len(got) != 1 || got[0] != castMayPlay|castFree {
			t.Fatalf("with no mana offered %v, want only the life alternative", got)
		}
		sc, o = payLifeBeside(t, 0, 2)
		sc.s.pl[0].life = int16(mv - 1)
		if got := payLifeCastModes(sc, o); len(got) != 1 || got[0] != castMayPlay {
			t.Fatalf("at %d life offered %v, want only the plain cast", mv-1, got)
		}
	})

	t.Run("pay life beside a second permission: each cast pays its own cost", func(t *testing.T) {
		for _, mode := range []uint8{castMayPlay | castFree, castMayPlay} {
			sc, o := payLifeBeside(t, 0, 2)
			s := sc.s
			mv := int(sc.k.t.Cards[v.ThinkTwice].Cost.MV())
			if tapped, floating := sc.tappedAndFloating(0); tapped != 0 || floating != 0 {
				t.Fatalf("precondition: %d tapped, %d floating", tapped, floating)
			}
			payLifeCast(t, sc, o, mode)
			payLifeBuild(t, sc, o)
			if s.Stalled() {
				t.Fatalf("mode %d stalled (%s)", mode, StallName(s.StallReason()))
			}
			tapped, floating := sc.tappedAndFloating(0)
			if mode&castFree != 0 && (int(s.pl[0].life) != 20-mv || tapped != 0) {
				t.Fatalf("life alternative: life %d (want %d), %d lands tapped (want 0)", s.pl[0].life, 20-mv, tapped)
			}
			if mode&castFree == 0 && (s.pl[0].life != 20 || tapped+floating < mv) {
				t.Fatalf("plain cast: life %d (want 20), %d tapped %d floating (want %d mana spent)", s.pl[0].life, tapped, floating, mv)
			}
		}
	})

	for _, flags := range []kop.StaticFlags{kop.StWithoutManaCost, kop.StMayPayLife} {
		t.Run(fmt.Sprintf("pay life beside a free or second pay-life permission stalls (flags %d)", flags), func(t *testing.T) {
			sc, o := payLifeBeside(t, flags, 0)
			s := sc.s
			if s.Stalled() {
				t.Fatalf("stalled (%s) before the offer", StallName(s.StallReason()))
			}
			if got := payLifeCastModes(sc, o); len(got) != 1 || got[0] != castMayPlay|castFree {
				t.Fatalf("offered %v, want the one joined free cast: the precondition", got)
			}
			if s.StallReason() != StallPayLifeShared {
				t.Fatalf("stall reason %q, want %q: two of gorge's casts share one mode", StallName(s.StallReason()), StallName(StallPayLifeShared))
			}
		})
	}

	t.Run("pay-life canon follows the payable life", func(t *testing.T) {
		sc := newScenTable(t, playTopTable(kop.StMayPayLife), Config{NoElide: true})
		s, k := sc.s, sc.k
		mv := int(k.t.Cards[v.ThinkTwice].Cost.MV())
		o := playTopPutOnTop(sc, v.ThinkTwice)
		s.g.emb[0][0], s.g.nemb[0] = uint8(len(k.t.Emblems)), 1
		s.pl[0].life = 20
		if !s.FeatMayPayLifeCast(0, o) {
			t.Fatalf("not a pay-life cast at 20 life: the precondition")
		}
		s.pl[0].life = int16(mv - 1)
		if s.FeatMayPayLifeCast(0, o) {
			t.Fatalf("a pay-life cast at %d life owing %d", s.pl[0].life, mv)
		}
	})
}

// payLifeBeside is a scenario with Think Twice on player 0's library top, a
// library permission with flags walked first and a pay-life one beside it,
// both in force, life 20 and n Islands.
func payLifeBeside(t *testing.T, flags kop.StaticFlags, n int) (*scen, Obj) {
	t.Helper()
	tbl := playTopTable(flags)
	pl := playTopTable(kop.StMayPayLife)
	tbl.Emblems = append(tbl.Emblems, pl.Emblems[len(pl.Emblems)-1])
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.lands(0, v.Island, n)
	o := playTopPutOnTop(sc, v.ThinkTwice)
	k := uint8(len(sc.k.t.Emblems))
	sc.s.g.emb[0][0], sc.s.g.emb[0][1], sc.s.g.nemb[0] = k-1, k, 2
	sc.s.pl[0].life = 20
	return sc, o
}

// payLifeCastModes is the modes of the casts of o player 0 is offered, in order.
func payLifeCastModes(sc *scen, o Obj) (modes []uint8) {
	var d Decision
	var av avail
	sc.s.priorityGen(sc.k, &d, 0, true, &av, false)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
			modes = append(modes, a.Mode())
		}
	}
	return modes
}

// payLifeBuild steps the cast of o until the spell is on the stack.
func payLifeBuild(t *testing.T, sc *scen, o Obj) {
	t.Helper()
	for i := 0; i < 8 && sc.s.objs[o].zone != kop.ZoneStack; i++ {
		d := sc.s.Step()
		if d.N == 0 {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[o].zone != kop.ZoneStack {
		t.Fatalf("the spell did not reach the stack (zone %v)", sc.s.objs[o].zone)
	}
}

// payLifeCast applies player 0's cast of o with mode at the next decision.
func payLifeCast(t *testing.T, sc *scen, o Obj, mode uint8) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o && a.Mode() == mode {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("no cast of %d with mode %d among %v", o, mode, d.Options[:d.N])
}
