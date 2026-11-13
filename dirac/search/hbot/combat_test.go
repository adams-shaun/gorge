package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// testBoard is a board holding just creatures (obj, ctrl, power, toughness,
// keywords) and the life totals, one per seat: what the combat policy reads.
func testBoard(me uint8, life []int32, cres ...creature) *board {
	b := &board{me: me, n: uint8(len(life))}
	copy(b.life[:], life)
	for i, c := range cres {
		b.cre[i] = c
		b.creX[c.obj] = int16(i + 1)
	}
	b.nCre = len(cres)
	return b
}

func cr(obj dirac.Obj, ctrl uint8, p, t int32, kw kop.KwMask) creature {
	return creature{obj: obj, ctrl: ctrl, power: p, tough: t, kw: kw}
}

func TestBlockCombat(t *testing.T) {
	var cs combatScratch
	a := cr(10, 0, 3, 3, kop.KwFirstStrike)
	bl := cr(1, 1, 2, 2, 0)
	cs.team[0] = &bl
	if aDead := cs.blockCombat(&a, cs.team[:1]); aDead || !cs.dead[0] {
		t.Fatalf("3/3 first strike vs 2/2: attacker dead %v, blocker dead %v; want false, true", aDead, cs.dead[0])
	}
	dt := cr(2, 1, 1, 1, kop.KwDeathtouch)
	cs.team[0] = &dt
	if aDead := cs.blockCombat(&a, cs.team[:1]); aDead || !cs.dead[0] {
		t.Fatalf("first strike kills the deathtoucher first: attacker dead %v", aDead)
	}
	a2 := cr(11, 0, 3, 3, 0)
	if aDead := cs.blockCombat(&a2, cs.team[:1]); !aDead || !cs.dead[0] {
		t.Fatalf("3/3 vs 1/1 deathtouch: both die, got attacker dead %v blocker dead %v", aDead, cs.dead[0])
	}
}

// TestChooseBlockers: the cheapest block that kills the attacker and is
// not a trade down; a menace attacker gets the support blocker too.
func TestChooseBlockers(t *testing.T) {
	b := testBoard(1, []int32{20, 20},
		cr(10, 0, 2, 2, 0), cr(11, 0, 2, 2, kop.KwMenace),
		cr(1, 1, 3, 3, 0), cr(2, 1, 1, 1, 0), cr(3, 1, 4, 4, 0))
	cs := &b.cs
	cs.n = 0
	for _, bl := range []dirac.Obj{1, 2, 3} {
		for _, a := range []dirac.Obj{10, 11} {
			cs.opts[cs.n] = copt{src: bl, tgt: uint8(a)}
			cs.n++
		}
	}
	cs.groupBlockAttackers()
	cs.chooseBlockers(b, 1)
	got := map[[2]int]bool{}
	for _, oi := range cs.chosen[:cs.nCh] {
		got[[2]int{int(cs.opts[oi].src), int(cs.opts[oi].tgt)}] = true
	}
	// attackers are equal power: the first-listed (10) is processed first
	// and takes the cheapest killer (3/3); the menace 2/2 then gets the
	// 4/4 with the 1/1 as support
	want := map[[2]int]bool{{1, 10}: true, {3, 11}: true, {2, 11}: true}
	if len(got) != len(want) {
		t.Fatalf("blocks %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Fatalf("blocks %v, want %v", got, want)
		}
	}
}

// TestChooseAttackers: a creature the defender kills for less than it is
// worth stays home (AR3); a flyer nothing can block attacks (AR2); a
// lethal one attacks whatever the board (AR7).
func TestChooseAttackers(t *testing.T) {
	run := func(life int32, mine ...creature) []dirac.Obj {
		cres := append([]creature{cr(20, 1, 3, 3, 0)}, mine...)
		b := testBoard(0, []int32{20, life}, cres...)
		cs := &b.cs
		cs.n = 0
		for _, c := range mine {
			cs.opts[cs.n] = copt{src: c.obj, tgt: dirac.PlayerTarget0 + 1, def: 1}
			cs.n++
		}
		cs.groupAttackers()
		cs.chooseAttackers(b, 0)
		var out []dirac.Obj
		for _, oi := range cs.chosen[:cs.nCh] {
			out = append(out, cs.opts[oi].src)
		}
		return out
	}
	if got := run(20, cr(1, 0, 2, 2, 0)); len(got) != 0 {
		t.Fatalf("2/2 into an untapped 3/3: attacked with %v", got)
	}
	if got := run(20, cr(1, 0, 2, 2, kop.KwFlying), cr(2, 0, 1, 1, 0)); len(got) != 1 || got[0] != 1 {
		t.Fatalf("flyer and a 1/1 home: attacked with %v, want [1]", got)
	}
	if got := run(2, cr(1, 0, 2, 2, 0), cr(2, 0, 1, 1, 0)); len(got) != 1 || got[0] != 1 {
		t.Fatalf("lethal 2/2 at 2 life: attacked with %v, want [1] (the 1/1 is vetoed)", got)
	}
}
