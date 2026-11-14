//go:build dirac_mp

package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestChooseAttackersPod: at more than two seats each option is scored
// against its own defender's creatures and life (botpolicy's per-defender
// defBlockers): a 2/2 the 3/3 of seat 1 would eat swings at seat 2, whose
// board is empty; at equal risk the opponent closest to dying is pressed;
// the swarm subset counts only the defender's own untapped creatures.
func TestChooseAttackersPod(t *testing.T) {
	// opts lists defender-major, as gorge offers them: every attacker's
	// pair with seat 1, then every attacker's pair with seat 2, ...
	run := func(life []int32, others []creature, mine ...creature) map[dirac.Obj]uint8 {
		b := testBoard(0, life, append(append([]creature{}, others...), mine...)...)
		cs := &b.cs
		cs.n = 0
		for q := uint8(1); q < uint8(len(life)); q++ {
			for _, c := range mine {
				cs.opts[cs.n] = copt{src: c.obj, tgt: dirac.PlayerTarget0 + dirac.TCode(q), def: q}
				cs.n++
			}
		}
		cs.groupAttackers()
		cs.chooseAttackers(b, 0)
		out := map[dirac.Obj]uint8{}
		for _, oi := range cs.chosen[:cs.nCh] {
			out[cs.opts[oi].src] = cs.opts[oi].def
		}
		return out
	}
	// (a 0/4 stays home, so no blocker is held back)
	got := run([]int32{20, 20, 20}, []creature{cr(20, 1, 3, 3, 0)}, cr(1, 0, 2, 2, 0), cr(2, 0, 0, 4, 0))
	if len(got) != 1 || got[1] != 2 {
		t.Fatalf("2/2 with a 3/3 at seat 1 and seat 2 open: attacks %v, want 1 at seat 2", got)
	}
	got = run([]int32{20, 20, 9, 14}, nil, cr(1, 0, 2, 2, 0))
	if len(got) != 1 || got[1] != 2 {
		t.Fatalf("open boards, lives 20/9/14: attacks %v, want 1 at seat 2 (the lowest life)", got)
	}
	// three 2/2s against seat 1 at 2 life behind one untapped 1/1, which
	// blocks one; seat 2's 5/5 is not seat 1's blocker
	got = run([]int32{20, 2, 20}, []creature{cr(20, 1, 1, 1, 0), cr(21, 2, 5, 5, 0)},
		cr(1, 0, 2, 2, 0), cr(2, 0, 2, 2, 0), cr(3, 0, 2, 2, 0))
	if len(got) < 2 || got[1] != 1 || got[2] != 1 {
		t.Fatalf("swarm at seat 1 (2 life, one 1/1): attacks %v, want 1 and 2 at seat 1", got)
	}
}
