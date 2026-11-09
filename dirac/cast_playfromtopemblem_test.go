package dirac

import (
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

	t.Run("pay life beside a second permission stalls", func(t *testing.T) {
		tbl := playTopTable(0) // a plain library permission, walked first
		pl := playTopTable(kop.StMayPayLife)
		tbl.Emblems = append(tbl.Emblems, pl.Emblems[len(pl.Emblems)-1])
		sc := newScenTable(t, tbl, Config{NoElide: true})
		s, k := sc.s, sc.k
		o := playTopPutOnTop(sc, v.ThinkTwice)
		n := uint8(len(k.t.Emblems))
		s.g.emb[0][0], s.g.emb[0][1], s.g.nemb[0] = n-1, n, 2
		s.pl[0].life = 20
		if s.Stalled() {
			t.Fatalf("stalled (%s) before the offer", StallName(s.StallReason()))
		}
		if got := playTopOffers(sc, o); len(got) != 1 {
			t.Fatalf("offered %v, want the joined offer of the two permissions: the precondition", got)
		}
		if s.StallReason() != StallPayLifeShared {
			t.Fatalf("stall reason %q, want %q: the joined offer would pay neither mana nor life", StallName(s.StallReason()), StallName(StallPayLifeShared))
		}
	})

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
