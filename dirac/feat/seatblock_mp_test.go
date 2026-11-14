//go:build dirac_mp

package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// fourSeatCommander is a four-seat Commander game whose seats' decks differ
// in length (so every seat's library count differs at the root), each
// with a Grizzly Bears commander at deck index 0.
func fourSeatCommander(seed uint64) *dirac.State {
	var decks [dirac.MaxSeats][]dirac.CardID
	var cmds [dirac.MaxSeats][2]int16
	for p := 0; p < 4; p++ {
		d := append([]dirac.CardID{vanilla.GrizzlyBears}, vanilla.Decks[p][:40-3*p]...)
		decks[p] = d
		cmds[p] = [2]int16{0, -1}
	}
	s := new(dirac.State)
	s.InitN(vanilla.Table, decks, 4, seed, dirac.Config{NoElide: true, Commander: true, Commanders: cmds})
	return s
}

// seatBlock is relative seat r's ten-value block of st.
func seatBlock(st *State, r int) [10]float32 {
	var b [10]float32
	copy(b[:], st.Dense[28+r*10:28+r*10+10])
	return b
}

// cmdBlock is relative seat r's Commander block of st.
func cmdBlock(st *State, r int) [commanderBlockWidth]float32 {
	var b [commanderBlockWidth]float32
	copy(b[:], st.Dense[denseCmd0+r*commanderBlockWidth:denseCmd0+(r+1)*commanderBlockWidth])
	return b
}

func clip1(x float32) float32 {
	if x > 1 {
		return 1
	}
	return x
}

// TestFourSeatDenseBlocksRelative: at four seats every viewer's dense
// vector fills all four seat blocks and all four Commander blocks, in turn
// order from the viewer. Each block holds its own seat's life, hand,
// library and battlefield counts and its own Commander columns (casts,
// on-field, left-the-game, and damage taken from each other seat in turn
// order from it); relative seat r of viewer v is relative seat r-1 of
// viewer v+1; and at the root the four blocks are pairwise distinct (the
// library counts differ), so no seat's block stands in for another's.
func TestFourSeatDenseBlocksRelative(t *testing.T) {
	e := NewEncoder(NewPool(vanilla.Table, make([]CardSpec, len(vanilla.Table.Cards))))
	e.Reserve()
	var objs []dirac.Obj
	var castSeen, dmgSeen, lostSeen, checked int
	for seed := uint64(1); seed <= 3; seed++ {
		s := fourSeatCommander(seed)
		if !s.FeatCommanderGame() || s.NumSeats() != 4 {
			t.Fatalf("setup: Commander %v, %d seats", s.FeatCommanderGame(), s.NumSeats())
		}
		pol := dirac.NewRandomPolicy(seed)
		var d dirac.Decision
		for i := 0; i < 4000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if i == 0 || i%7 == 0 {
				var blocks [4][4][10]float32
				var cblocks [4][4][commanderBlockWidth]float32
				for viewer := uint8(0); viewer < 4; viewer++ {
					st := e.EncodeStateFor(s, &d, viewer)
					for r := 0; r < 4; r++ {
						p := (viewer + uint8(r)) % 4
						b := seatBlock(st, r)
						blocks[viewer][r] = b
						objs = s.FeatBattlefield(p, objs[:0])
						want := [4]float32{float32(s.Life(int(p))), float32(s.HandSize(p)), float32(s.LibrarySize(p)), float32(len(objs))}
						if got := [4]float32{b[0], b[1], b[2], b[4]}; got != want {
							t.Fatalf("seed %d step %d viewer %d block %d (life, hand, library, battlefield) = %v, want seat %d's %v",
								seed, i, viewer, r, got, p, want)
						}
						c := cmdBlock(st, r)
						cblocks[viewer][r] = c
						var wc [commanderBlockWidth]float32
						for k := 1; k < 4; k++ {
							wc[denseCmdDamage0+k-1] = clip1(float32(s.FeatCommanderDamage(p, (p+uint8(k))%4)) / 21)
						}
						wc[denseCmdCasts] = clip1(float32(s.FeatCommanderCast(p)) / 4)
						if cmd := s.FeatCommander(p); cmd != dirac.NoObj && e.po(cmd).Zone == kop.ZoneBattlefield {
							wc[denseCmdOnField] = 1
						}
						if s.FeatLost(p) {
							wc[denseCmdLost] = 1
						}
						if c != wc {
							t.Fatalf("seed %d step %d viewer %d Commander block %d = %v, want seat %d's %v", seed, i, viewer, r, c, p, wc)
						}
						if r > 0 && c[denseCmdCasts] > 0 {
							castSeen++
						}
						if r > 0 && (c[0] > 0 || c[1] > 0 || c[2] > 0) {
							dmgSeen++
						}
						if c[denseCmdLost] > 0 {
							lostSeen++
						}
					}
				}
				for viewer := 0; viewer < 4; viewer++ {
					next := (viewer + 1) % 4
					for r := 1; r < 4; r++ {
						if blocks[viewer][r] != blocks[next][r-1] || cblocks[viewer][r] != cblocks[next][r-1] {
							t.Fatalf("seed %d step %d: viewer %d's relative seat %d is not viewer %d's relative seat %d",
								seed, i, viewer, r, next, r-1)
						}
					}
				}
				if i == 0 {
					for r := 0; r < 4; r++ {
						for q := r + 1; q < 4; q++ {
							if blocks[0][r] == blocks[0][q] {
								t.Fatalf("seed %d root: relative seats %d and %d have the same block %v", seed, r, q, blocks[0][r])
							}
						}
					}
				}
				checked++
			}
			s.Apply(pol.Pick(&d))
		}
	}
	t.Logf("%d positions checked; another seat's block showed commander casts %d times, commander damage %d, a lost seat %d",
		checked, castSeen, dmgSeen, lostSeen)
	if castSeen == 0 {
		t.Fatal("precondition: no other seat ever cast its commander, so the Commander columns were never exercised")
	}
}
