package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// feeUnitCol is the colour a running fee unit's production takes once its fee
// is paid (planPayFee). The fee units run last, on the pool the plan's other
// units filled (runOrder), and the fee's generic spends the mana the cost's
// pips claim only when nothing else floats (takeFeeGeneric's second pass, CR
// 601.2h). The colour pass chose the fee unit's colour before that, counting
// the eaten mana as paying its pip: Orb of Dragonkind's {1} paid with a
// Mountain's {R} leaves Lathliss, Dragon Queen's {1}{R} short of its {R}, and
// gorge's plan makes the Orb's mana red. So a colour the cost's pips are now
// short of, and the unit makes, replaces a colour they are not short of; a
// plan that leaves no pip short keeps the colour pass's choice. made is the
// restricted mana the earlier fee units floated beside the pool (State.rb),
// which the pool no longer shows.
func (s *State) feeUnitCol(p uint8, pk *planner, ty *utype, col int, made *[kop.NumManaKinds]int) int {
	if ty.fee == 0 && ty.feeCol == 0 || ty.dist || pk.anyColor {
		return col
	}
	pool := &s.pl[p].pool
	short := func(c int) bool { return int(pk.cpips[c]) > int(pool[c])+made[c] }
	if !short(col) {
		for c := 0; c < kop.NumManaKinds; c++ {
			if ty.mask&(1<<c) != 0 && short(c) {
				col = c
				break
			}
		}
	}
	if ty.rb {
		made[col] += int(ty.n)
	}
	return col
}
