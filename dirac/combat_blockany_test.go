package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

const (
	baBear CardID = 0 // 2/2
	baWall CardID = 1 // 9/9, CanBlockAny$ True
	baA3   CardID = 2 // 3/3
	baA4   CardID = 3 // 4/4
	baTwo  CardID = 4 // 7/7, CanBlockAmount$ 1
)

func baScen(t *testing.T, cfg Config) *scen {
	t.Helper()
	tbl := &kop.Table{
		Cards: []kop.CardDef{
			{Name: "Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
			{Name: "Wall", Types: kop.TCreature, Power: 9, Toughness: 9, ExtraBlocks: kop.ExtraBlocksAny},
			{Name: "Three", Types: kop.TCreature, Power: 3, Toughness: 3},
			{Name: "Four", Types: kop.TCreature, Power: 4, Toughness: 4},
			{Name: "Twins", Types: kop.TCreature, Power: 7, Toughness: 7, ExtraBlocks: 1},
		},
		Filters: []kop.Filter{{}},
		Conds:   []kop.Cond{},
	}
	sc := newScenTableLib(t, tbl, baBear, cfg)
	if sc.k.gen == nil || !sc.k.gen.uses.has(useBlockAny) || !sc.k.gen.anyExtraBlock {
		t.Fatal("the table is not a generated one with useBlockAny")
	}
	if len(sc.s.xblkRecs()) != maxXblkAny {
		t.Fatalf("xblk records = %d, want %d", len(sc.s.xblkRecs()), maxXblkAny)
	}
	return sc
}

// splitOf finds the option index of the division want of p damage.
func splitOf(t *testing.T, p int, want ...uint8) uint8 {
	t.Helper()
	var amt [MaxPerm]uint8
	for i := 0; i < splits(p, len(want)); i++ {
		SplitAt(p, len(want), i, amt[:])
		if string(amt[:len(want)]) == string(want) {
			return uint8(i)
		}
	}
	t.Fatalf("no split %v of %d", want, p)
	return 0
}

// TestBlockAnyBlocksEveryAttacker: a CanBlockAny$ blocker is offered each
// attacker it has not blocked yet, however many it already blocks, while a
// CanBlockAmount$ 1 blocker stops at two.
func TestBlockAnyBlocksEveryAttacker(t *testing.T) {
	sc := baScen(t, Config{NoElide: true})
	atk := []Obj{sc.perm(baBear, 0), sc.perm(baBear, 0), sc.perm(baA3, 0), sc.perm(baA4, 0)}
	wall, tw := sc.perm(baWall, 1), sc.perm(baTwo, 1)
	xbAttack(t, sc, atk...)
	for i, a := range atk {
		d := sc.s.Step()
		for _, b := range atk[i:] {
			if !xbOffered(d, wall, b) {
				t.Fatalf("after %d blocks the Wall is not offered attacker %d: %v", i, b, d.Options[:d.N])
			}
		}
		for _, b := range atk[:i] {
			if xbOffered(d, wall, b) {
				t.Fatalf("after %d blocks the Wall is offered attacker %d again", i, b)
			}
		}
		if i >= 2 && xbOffered(d, tw, a) {
			t.Fatalf("after %d blocks the Twins is offered attacker %d", i, a)
		}
		sc.do(t, AkBlock, TCode(wall), TCode(a))
		if i == 1 {
			sc.do(t, AkBlock, TCode(tw), TCode(a))
			sc.do(t, AkBlock, TCode(tw), TCode(atk[0]))
		}
	}
	b := sc.slot(wall)
	var l [MaxPerm]Slot
	if n := sc.s.blockedLive(b, &l); n != 4 {
		t.Fatalf("the Wall blocks %d attackers, want 4", n)
	}
	for _, a := range atk {
		if !sc.s.blocks(b, sc.slot(a)) {
			t.Fatalf("blocks(wall, %d) is false", a)
		}
	}
	if got := sc.s.blockProjected(b); got == NoSlot || sc.s.perms[got].obj != atk[0] {
		t.Fatalf("projected block %v, want the lowest instance %d", got, atk[0])
	}
}

// TestBlockAnyDivisionThreeWays: the Wall's controller divides its damage
// among three attackers in ts order, every split offered, and each attacker is
// dealt its share (CR 510.1d).
func TestBlockAnyDivisionThreeWays(t *testing.T) {
	for _, tc := range []struct {
		name   string
		split  [3]uint8
		deaths [3]bool // the 2/2, the 3/3, the 4/4
	}{
		{"lethal to each", [3]uint8{2, 3, 4}, [3]bool{true, true, true}},
		{"all on the first", [3]uint8{9, 0, 0}, [3]bool{true, false, false}},
		{"all on the last", [3]uint8{0, 0, 9}, [3]bool{false, false, true}},
		{"the middle short", [3]uint8{2, 2, 5}, [3]bool{true, false, true}},
	} {
		sc := baScen(t, Config{NoElide: true})
		a2, a3, a4 := sc.perm(baBear, 0), sc.perm(baA3, 0), sc.perm(baA4, 0)
		wall := sc.perm(baWall, 1)
		xbAttack(t, sc, a2, a3, a4)
		for _, a := range []Obj{a4, a2, a3} { // declared out of ts order: recipients still go by ts
			sc.do(t, AkBlock, TCode(wall), TCode(a))
		}
		sc.do(t, AkDone, scAny, scAny)
		d := xbToDivide(t, sc)
		if want := splits(9, 3); d.Player != 1 || int(d.N) != want || d.Options[0].Source() != TCode(uint8(wall)) {
			t.Fatalf("%s: division player %d, N %d, source %d; want player 1, N %d, source %d", tc.name, d.Player, d.N, d.Options[0].Source(), want, wall)
		}
		sc.s.Apply(MakeAction(AkChoose, TCode(wall), 0, splitOf(t, 9, tc.split[0], tc.split[1], tc.split[2])))
		sc.passUntil(t, kop.StepMain2)
		for i, a := range []Obj{a2, a3, a4} {
			if sc.onBF(a) == tc.deaths[i] {
				t.Fatalf("%s: attacker %d alive=%v, want dead=%v", tc.name, a, sc.onBF(a), tc.deaths[i])
			}
		}
		if sc.onBF(wall) {
			t.Fatalf("%s: the Wall took 9 and lives", tc.name)
		}
	}
}

// TestBlockAnyFallbackDivision: with the ask off the damage is lethal to each
// attacker in ts order and the rest to the last (gorge bandBlockerAssignments).
func TestBlockAnyFallbackDivision(t *testing.T) {
	sc := baScen(t, Config{NoElide: true, AutoDivide: true})
	a2, a3, a4 := sc.perm(baBear, 0), sc.perm(baA3, 0), sc.perm(baA4, 0)
	wall := sc.perm(baWall, 1)
	sc.combat(t, []Obj{a2, a3, a4}, [][2]Obj{{wall, a2}, {wall, a3}, {wall, a4}})
	if sc.onBF(a2) || sc.onBF(a3) || sc.onBF(a4) {
		t.Fatalf("2/2 alive=%v 3/3 alive=%v 4/4 alive=%v, want all dead (2, 3, then 4)", sc.onBF(a2), sc.onBF(a3), sc.onBF(a4))
	}
}

// TestBlockAnyAttackerLeavesPromotes: the first attacker of three leaves and
// the next declared becomes the first; the Wall still blocks the other two.
func TestBlockAnyAttackerLeavesPromotes(t *testing.T) {
	sc := baScen(t, Config{NoElide: true})
	a1, a2, a3 := sc.perm(baBear, 0), sc.perm(baA3, 0), sc.perm(baA4, 0)
	wall := sc.perm(baWall, 1)
	xbAttack(t, sc, a1, a2, a3)
	for _, a := range []Obj{a1, a2, a3} {
		sc.do(t, AkBlock, TCode(wall), TCode(a))
	}
	b, s1, s2, s3 := sc.slot(wall), sc.slot(a1), sc.slot(a2), sc.slot(a3)
	sc.s.forgetBlocked(b, s1)
	var l [MaxPerm]Slot
	if sc.s.perms[b].blocking != s2 || !sc.s.blocks(b, s3) || sc.s.blocks(b, s1) || sc.s.blockedLive(b, &l) != 2 {
		t.Fatalf("after the first left: blocking %d, blocks 3rd %v, blocks 1st %v", sc.s.perms[b].blocking, sc.s.blocks(b, s3), sc.s.blocks(b, s1))
	}
	if sc.s.blockKey(b, s2) >= sc.s.blockKey(b, s3) {
		t.Fatalf("declaration order lost: keys %d, %d", sc.s.blockKey(b, s2), sc.s.blockKey(b, s3))
	}
	sc.s.releaseBlocked(b)
	if len(sc.s.xblkRecs()) == 0 || sc.s.xblkOf(b) != NoSlot {
		t.Fatal("releaseBlocked left a record")
	}
	if sc.s.perms[s2].nblock != 0 || sc.s.perms[s3].nblock != 0 {
		t.Fatalf("nblock after release: %d, %d", sc.s.perms[s2].nblock, sc.s.perms[s3].nblock)
	}
}

// TestBlockAnyTableFullStalls: a blocker that needs a record when the table is
// full stops at StallExtraBlockCap while another attacker is blockable.
func TestBlockAnyTableFullStalls(t *testing.T) {
	sc := baScen(t, Config{NoElide: true})
	a1, a2 := sc.perm(baBear, 0), sc.perm(baBear, 0)
	wall := sc.perm(baWall, 1)
	xbAttack(t, sc, a1, a2)
	sc.do(t, AkBlock, TCode(wall), TCode(a1))
	for i := range sc.s.xblkRecs() { // other blockers hold every record
		sc.s.xblkRecs()[i] = xblkRec{b: 100, a: 101, ord: 1}
	}
	if d := sc.s.Step(); xbOffered(d, wall, a2) || sc.s.StallReason() != StallExtraBlockCap {
		t.Fatalf("stall %d, offered %v; want StallExtraBlockCap and no pair", sc.s.StallReason(), xbOffered(d, wall, a2))
	}
}
