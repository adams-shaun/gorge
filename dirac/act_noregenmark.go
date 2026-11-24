package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// OpMarkNoRegen (gap-effect-containers): the battlefield creature of Obj --
// Furnace Brood's targeted Effect "target creature can't be regenerated this
// turn" (CR 701.19c), the Effect-delivered Mode$ CantRegenerate static with
// no DealDamage carrying it -- takes the mechNoRegen slot mark
// (dirac/repl_noregen.go): its regeneration is refused until the mark's ends
// (the turn's cleanup, or its departure). gorge registers the restriction
// over the remembered target id and forgets it when the card moves
// (ForgetOnMoved$ Battlefield), so the mark is cleared at the permanent's
// departure like the damage-carried marks' (kop.DmgNoRegen's rider, CR
// 400.7: a new object loses it).
//
// The mech's detector: a table with an OpMarkNoRegen holds a mechNoRegen
// mask.
func init() {
	arenaDetect(mechNoRegen, func(t *kop.Table, in *kop.Instr) bool {
		return in.Op == kop.OpMarkNoRegen
	})
}

// execNoRegenMark runs OpMarkNoRegen (execOpGen): the creature of Obj takes
// the mark. Obj is the chain's own target or the remembered set's one card;
// a ref that resolves to nothing marks nothing.
//
//kmanifest:op OpMarkNoRegen
func (s *State) execNoRegenMark(k *prep, in *kop.Instr) (ok, done bool) {
	if in.Op != kop.OpMarkNoRegen {
		return true, false
	}
	if in.Obj == kop.RefRemembered && s.g.nrem > 1 {
		var objs [maxRem]Obj
		n := copy(objs[:], s.g.rem[:s.g.nrem])
		for i := 0; i < n; i++ {
			s.markNoRegen(k, objs[i])
		}
		return true, true
	}
	if o := s.objRefObj(in.Obj); o != NoObj {
		s.markNoRegen(k, o)
	}
	return true, true
}
