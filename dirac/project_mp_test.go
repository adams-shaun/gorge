//go:build dirac_mp

package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestProjectNSeats: at four seats Project gives every seat's library and
// hand cards a zone position and fills every seat's counts and life, and the
// per-seat getters read each seat's own value.
func TestProjectNSeats(t *testing.T) {
	const n = 4
	var decks [MaxSeats][]CardID
	for p := 0; p < n; p++ {
		decks[p] = vanilla.Decks[p%5]
	}
	s := new(State)
	s.InitN(vanilla.Table, decks, n, 7, Config{Mulligans: 1})
	for p := 0; p < n; p++ {
		pl := &s.pl[p]
		pl.life = int16(20 + p)
		pl.landsPlayed = uint8(p)
		pl.pool[0] = uint8(10 + p)
		pl.cmdCast = uint8(30 + p)
		pl.cmdDmg[(p+1)%n] = uint8(40 + p)
		pl.lost = p == 3
		s.g.poison[p] = uint8(1 + p)
		s.g.energy[p] = uint8(5 + p)
		s.g.room[p] = uint8(2 + p)
	}
	var pr Projection
	s.Project(&pr)
	for p := uint8(0); p < n; p++ {
		pl := &s.pl[p]
		for i := pl.libLo; i < pl.libHi; i++ {
			if got := pr.Objs[pl.lib[i]].ZoneIdx; got != i-pl.libLo {
				t.Fatalf("seat %d library card %d: position %d", p, i-pl.libLo, got)
			}
		}
		for i := 0; i < int(pl.handN); i++ {
			if got := pr.Objs[pl.hand[i]].ZoneIdx; got != uint8(i) {
				t.Fatalf("seat %d hand card %d: position %d", p, i, got)
			}
			if got := s.FeatZoneIndex(pl.hand[i]); got != uint8(i) {
				t.Fatalf("seat %d hand card %d: FeatZoneIndex %d", p, i, got)
			}
		}
		if pr.LibN[p] != uint8(pl.libLen()) || pr.HandN[p] != openingHand {
			t.Fatalf("seat %d: LibN %d HandN %d", p, pr.LibN[p], pr.HandN[p])
		}
		if pr.Life[p] != int16(20+p) || pr.Lands[p] != p || pr.Pool[p][0] != 10+p || pr.CmdCast[p] != 30+p || pr.CmdDmg[p][(p+1)%n] != 40+p {
			t.Fatalf("seat %d: life %d lands %d pool %d cmdcast %d cmddmg %v", p, pr.Life[p], pr.Lands[p], pr.Pool[p][0], pr.CmdCast[p], pr.CmdDmg[p])
		}
		if s.Poison(p) != int(1+p) || s.Energy(p) != int(5+p) || s.DungeonRoom(p) != 2+p {
			t.Fatalf("seat %d: poison %d energy %d room %d", p, s.Poison(p), s.Energy(p), s.DungeonRoom(p))
		}
		if pool, _ := s.PoolOf(p); pool[0] != 10+p {
			t.Fatalf("seat %d: PoolOf %v", p, pool)
		}
		if s.FeatLost(p) != (p == 3) || s.FeatCommanderCast(p) != 30+p || s.FeatCommanderDamage(p, (p+1)%n) != 40+p {
			t.Fatalf("seat %d: lost %v cmdcast %d cmddmg %d", p, s.FeatLost(p), s.FeatCommanderCast(p), s.FeatCommanderDamage(p, (p+1)%n))
		}
		if got := s.FeatZoneList(p, kop.ZoneHand, nil); len(got) != int(pl.handN) || got[0] != pl.hand[0] {
			t.Fatalf("seat %d: FeatZoneList %v", p, got)
		}
	}
}
