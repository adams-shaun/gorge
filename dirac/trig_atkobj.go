package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The attacked planeswalker or battle of an attack trigger (OpAtkPWGuard C
// AtkPWHit: Fathom Fleet Swordjack, Raid Bombardment). gorge binds the
// DefenderObj as the trigger is queued (CR 603.3c) and deals the damage there
// after the attacker has left the battlefield (CR 608.2h), so the kernel keeps
// the object beside the trigger's defending seat.
//
// The object does not fit a trigger's bytes (pendingTrig and stackEntry have no
// spare byte and the State bar has no slack), so the declaration's attacked
// objects are interned in a small cold table, and the trigger's defSeat keeps
// the table index + 1 in its defSeatIxMask bits beside the seat and the
// defSeatObj tag (combat.go). The table is a per-combat record: every
// declaration starts it empty (fireAttacksGen) and the turn's end clears it,
// because an attack trigger is queued by its declaration and resolves before
// the combat ends. A table is kept only where some ability is
// OpAtkPWGuard C AtkPWHit (useAtkPWHit), so every other pool's bytes are
// unchanged. A fourth distinct attacked object in one declaration is not
// interned: its triggers keep the object tag without an index, which the guard
// stalls on as it did before the table.

// maxAtkObj is the attacked objects one declaration interns. The table is
// three records because the largest pool (edh, which holds Raid Bombardment)
// has six spare bytes in its cold buffer's last word and sits on the State bar
// (kernel_test.go maxStateBytes); defSeat's index bits hold up to fifteen.
const maxAtkObj = 3

var coldAtkObj = newCold[TCode]("atkObj", func(g *genPrep) int {
	if g.uses.has(useAtkPWHit) {
		return maxAtkObj
	}
	return 0
})

// atkObjCold is the table, or nil when the pool keeps none.
func (s *State) atkObjCold() []TCode { return coldAtkObj.of(s) }

// useAtkPWHit holds for a table with an ability program that is OpAtkPWGuard
// with C AtkPWHit.
var useAtkPWHit = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpAtkPWGuard && in.C&kop.AtkPWHit != 0 {
					return true
				}
			}
		}
	}
	return false
})

// atkObjReset empties the table.
func (s *State) atkObjReset() {
	c := s.atkObjCold()
	for i := range c {
		c[i] = 0
	}
}

// atkObjIntern is the table index + 1 of attacked object code, taking a record
// for a code not held yet; 0 when the pool keeps no table or it is full. A
// record is stored as code + 1, so 0 is a free one.
func (s *State) atkObjIntern(code TCode) uint8 {
	c := s.atkObjCold()
	for i := range c {
		switch c[i] {
		case code + 1:
			return uint8(i + 1)
		case 0:
			c[i] = code + 1
			return uint8(i + 1)
		}
	}
	return 0
}

// atkObjOf is the attacked object a defSeat byte names through the table, or
// NoTarget when it names none.
func (s *State) atkObjOf(d uint8) TCode {
	c := s.atkObjCold()
	ix := int(d&defSeatIxMask) >> defSeatIxShift
	if d&defSeatObj == 0 || ix == 0 || ix > len(c) || c[ix-1] == 0 {
		return NoTarget
	}
	return c[ix-1] - 1
}
