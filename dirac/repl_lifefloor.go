package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplLifeReduced (gorge applyLifeReplacements' LifeReduced arm, the
// ReplaceWith$ ReduceLoss body): a battlefield R:Event$ LifeReduced |
// ValidPlayer$ You.lifeGE<n> | Result$ LT<n> | IsDamage$ True line (Ali from
// Cairo, Elderscale Wurm, Worship) floors the life a damage event would take
// from its controller's total: while they have n or more (the ValidPlayer$
// lifeGE gate), a loss that would take it below n takes it to n instead.
// Only the damage sites call lifeFloorTake (IsDamage$ True): a life cost is
// paid, never floored (CR 119.4). The floored amount is what loseLife
// commits, so a ReplLifeLossDouble doubler in the same table doubles the
// clamped loss -- gorge orders a floor beside a doubler on the CR 616.1 ask
// (it parks the competition; the kernel takes them in its scan order, and
// every floor-beside-doubler pod the focus pools avoid never ask). Floors
// beside floors commute (the tighter cap wins in either order, a met cap
// leaves the other inapplicable), so gorge never asks for them either
// (lifeReplacementsCommute). IsPresent$ is the generic gate (Worship), read
// as Repl.Cond.

// DebugLifeFloorOff skips the floor (the mutation: the kernel takes the loss
// gorge clamps).
var DebugLifeFloorOff bool

// lifeFloorTake is the damage sites' floor: the loss p's own battlefield
// ReplLifeReduced lines reduce n to (the ValidPlayer$ You.lifeGE<n> gate at
// the pre-loss total, the Result$ LT<n> rewrite below it). Only with prep's
// rb2LifeFloor.
//
//go:noinline
func (s *State) lifeFloorTake(k *prep, p uint8, n int) int {
	if n <= 0 || DebugLifeFloorOff {
		return n
	}
	g := k.gen
	life := int(s.pl[p].life)
	for m := s.statM.and(s.ctrlM[p]); m.any(); m = m.rest() {
		sl := m.first()
		if g.replBits2[s.cardAt(sl)]&rb2LifeFloor == 0 || s.genLost(k, sl) || s.faceDown(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			r := &cd.Abilities[i].Repl
			if cd.Abilities[i].Kind != kop.AbReplacement || r.Kind != kop.ReplLifeReduced {
				continue
			}
			floor := int(r.A)
			if life < floor || life-n >= floor {
				continue // the lifeGE gate, then a loss that never reaches the floor
			}
			if r.Cond != 0 && !s.condEval(k, r.Cond, evalCtx{src: s.perms[sl].obj, srcSl: sl, ctrl: s.perms[sl].ctrl}) {
				continue
			}
			n = life - floor // Result$ LT<floor>: the total stays at the floor
		}
	}
	return n
}

// loseLifeDmg is loseLife for a damage-caused loss: the damage floors apply
// first (IsDamage$ True), the commit and its triggers the ordinary loseLife
// (the floor's rewrite is the damage event's own, gorge foldDamage's
// LifeReduced arm; the LifeLost trigger still fires for the clamped amount,
// CR 119.3).
func (s *State) loseLifeDmg(k *prep, p uint8, n int) {
	if k.gen != nil && k.gen.anyRepl2&rb2LifeFloor != 0 {
		n = s.lifeFloorTake(k, p, n)
	}
	s.loseLife(k, p, n)
}
