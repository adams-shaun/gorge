package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPreventCombatDamageFrom: a source granted KwPreventCombatDamageFrom
// (Maze of Ith's Effect: gorge's bodyless Prevent$ ValidSource$
// IsRemembered) deals no combat damage to a creature or a player, gains
// no lifelink; another source's damage to the same creature stays.
func TestPreventCombatDamageFrom(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a, b, c := sc.perm(fc, 0), sc.perm(fc, 0), sc.perm(fc, 1)
	sa, sb, scl := sc.slot(a), sc.slot(b), sc.slot(c)
	sc.s.perms[sa].eotKw |= kop.KwPreventCombatDamageFrom
	sc.s.perms[sa].atk = TCode(PlayerTarget0 + 1)
	var r dmgRec
	add := func(src, dst Slot, n int16) {
		if dst == NoSlot {
			r.toPl[1] += n
			r.dealtPl[src] += n
		} else {
			r.toPerm[dst] += n
		}
		r.dealt[src] += n
		r.note(src, dst, n)
	}
	add(sa, scl, 2)
	add(sb, scl, 1)
	add(sa, NoSlot, 3)
	was := sc.k.gen.anyPrevFrom
	sc.k.gen.anyPrevFrom = true
	defer func() { sc.k.gen.anyPrevFrom = was }()
	sc.s.preventCombatGen(sc.k, &r)
	if r.toPerm[scl] != 1 || r.toPl[1] != 0 || r.dealt[sa] != 0 || r.dealtPl[sa] != 0 || r.dealt[sb] != 1 {
		t.Fatalf("toPerm %d toPl %d dealt(a) %d dealtPl(a) %d dealt(b) %d", r.toPerm[scl], r.toPl[1], r.dealt[sa], r.dealtPl[sa], r.dealt[sb])
	}
}

// TestFog: after OpFog no combat damage is dealt this turn (gorge
// damageStep's fogActive); the next turn clears it.
func TestFog(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 4}})
	s := sc.s
	a := sc.perm(id["Savannah Lions"], 0)
	b := sc.perm(id["Savannah Lions"], 1)
	s.exec(sc.k, &kop.Instr{Op: kop.OpFog})
	life := s.pl[1].life
	sc.combat(t, []Obj{a}, nil)
	if s.pl[1].life != life {
		t.Fatalf("fogged: the defender went from %d to %d life", life, s.pl[1].life)
	}
	s.newTurnGen()
	if s.g.fog {
		t.Fatal("the fog outlived the turn")
	}
	_ = b
}
