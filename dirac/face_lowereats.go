package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A face route's lowering whose sacrifice stacks a may-pay trigger. gorge
// lowers a face play's witness at priority before the cast, so a sacrificed
// Treasure's trigger (Furnace Celebration's "you may pay {2}") resolves with
// the witness's mana floating, the first option pays it, and the pool is no
// longer what the plan predicts: payexec ends the lowering and the cast is
// never made (oracle gorge.go lowerCompletes, "lower dead end"). The kernel
// pays inside the cast's payment window (CR 601.2g), so its trigger asks only
// after the cast has spent the mana; it withholds the play gorge never makes
// (bestowSacWitness, kw_bestow.go, is the bestow route's mirror).

// faceLowerEatsPool reports whether the witness paying the face cast of o at
// cost sacrifices a source whose trigger asks a mana may-pay the witness's
// production covers (counterPayEatsPool), for a cast at sorcery speed: an
// instant-speed cast is made with the trigger still on the stack.
func (s *State) faceLowerEatsPool(k *prep, p uint8, c CardID, cost *kop.ManaCost) bool {
	if k.cards[c].instant || s.castFlash(k, p, c) || !s.subs[kop.TrigSacrificed].any() {
		return false
	}
	tmp := s.fork()
	defer s.unfork()
	n0 := tmp.npend
	var pk planner
	pk.try = true
	tmp.tapForPK(k, p, cost, 0, NoObj, false, false, &pk)
	return pk.found && plannerConsumesSac(&pk) && tmp.counterPayEatsPool(k, n0, pk.needT)
}
