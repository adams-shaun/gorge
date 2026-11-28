package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// permTgtCands appends to out, from n, the permanents spec t can target and
// returns the new length, in the order gorge's target ask offers them: seat
// 0's battlefield first, then each later seat's, each in entry order (gorge
// legalTargetCandidates walks Zone(ZBattlefield, q) for q in AliveFrom(0)).
// Slot order is not that order, because slots are lowest-free and reused. The
// order picks the target wherever a line is scripted: a scripted line answers
// an ask with its first posed option (scriptStep), so a prefix tap's targeted
// trigger (C.A.M.P.'s counter, CR 603.3d) lands on the permanent gorge names.
func (s *State) permTgtCands(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, out *[MaxOptions]TCode, n int) int {
	var hit slotMask
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.targetOK(k, t, p, src, self, TCode(s.perms[sl].obj)) {
			hit.set(sl)
		}
	}
	var ord [MaxPerm]Slot
	for _, sl := range ord[:s.listenerOrder(hit, &ord)] {
		out[n] = TCode(s.perms[sl].obj)
		n++
	}
	return n
}
