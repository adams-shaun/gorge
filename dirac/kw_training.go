package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// trainingGate: a Training trigger (kop.TrigNTraining; CR 702.149a,
// ) fires only when its source attacks with another attacker of
// strictly greater power, the whole declaration counted (gorge trigmatch
// AttacksMatches over DeclaredAttackers, derived power).
//
//go:noinline
func (s *State) trainingGate(k *prep, src Slot, atk []Slot) bool {
	pw, _ := s.pt(k, src)
	for _, a := range atk {
		if a == src {
			continue
		}
		if p, _ := s.pt(k, a); p > pw {
			return true
		}
	}
	return false
}

// subAmongTotal is the number of counters of the cost's kind on the
// permanents p controls matching its filter (the CostSubAmong offer gate;
// gorge SubCounterAmongUnits' total). Filter 0 is the source alone (the
// source-anchored RemoveAnyCounter<N/Kind/CARDNAME> form); CtrAll sums every
// kind (RemoveAnyCounter<N/Any/...>).
func (s *State) subAmongTotal(k *prep, p uint8, c *kop.Cost, src Slot) int {
	if c.Filter == 0 {
		if src == NoSlot {
			return 0
		}
		return permCtrN(s, &s.perms[src], int(c.Ctr))
	}
	n := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.matchPerm(k, c.Filter, sl, p, NoSlot) {
			n += permCtrN(s, &s.perms[sl], int(c.Ctr))
		}
	}
	return n
}

// permCtrN is p's counters of kind c, or every kind for CtrAll.
func permCtrN(s *State, p *perm, c int) int {
	if c != int(kop.CtrAll) {
		return p.ctrN(s, c)
	}
	n := int(p.p1)
	for i := range p.cn {
		if kop.IsCounterKind(int(p.ck[i])) {
			n += int(p.cn[i])
		}
	}
	if p.ck[ctrSlots-1] != 0 {
		n += s.ctrOverSum(p.obj, 0)
	}
	return n
}

// subAmongOptions poses the next counter of an ability's CostSubAmong
// removal (gorge amongCounterAsk): one option per permanent the payer
// controls matching the filter with a counter of the kind not yet chosen; a
// lone candidate takes the unit unasked. It reports whether a decision was
// posed; false once every counter is chosen.
func (s *State) subAmongOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if e.kind != seAbility {
		return false
	}
	a := s.buildAbility(k)
	if a == nil || a.Cost.Flags&kop.CostSubAmong == 0 {
		return false
	}
	c := uint8(a.Cost.Ctr)
	src := s.srcSlot()
	for {
		n := 0
		for i := 0; i < int(s.bld().pickN); i++ {
			if s.bld().pickKs[i]&pickSubCtr != 0 {
				n++
			}
		}
		if n >= int(a.Cost.N) {
			s.bld().pickK = 0
			return false
		}
		d.N = 0
		for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
			sl := m.first()
			if a.Cost.Filter == 0 && sl != src {
				continue
			}
			if a.Cost.Filter != 0 && !s.matchPerm(k, a.Cost.Filter, sl, e.ctrl, NoSlot) {
				continue
			}
			o := s.perms[sl].obj
			if c == uint8(kop.CtrAll) {
				if n := s.perms[sl].ctrLow(int(kop.CtrP1P1)); n > s.subCtrPicked(o, uint8(kop.CtrP1P1)) {
					d.add(MakeAction(AkChoose, TCode(o), 0, uint8(kop.CtrP1P1)))
				}
				for i := range s.perms[sl].ck {
					kind := s.perms[sl].ck[i]
					if kind == 0 { // a free slot, not a P1P1 counter
						continue
					}
					if kop.IsCounterKind(int(kind)) && s.perms[sl].ctrN(s, int(kind)) > s.subCtrPicked(o, kind) {
						d.add(MakeAction(AkChoose, TCode(o), 0, kind))
					}
				}
				for _, e := range s.ctrOverOf(o, 0) {
					if kop.IsCounterKind(int(e.kind)) && int(e.n) > s.subCtrPicked(o, e.kind) {
						d.add(MakeAction(AkChoose, TCode(o), 0, e.kind))
					}
				}
			} else if s.perms[sl].ctrN(s, int(c)) > s.subCtrPicked(o, c) {
				d.add(MakeAction(AkChoose, TCode(o), 0, c))
			}
		}
		if d.N == 0 {
			s.bld().pickK = 0 // the offer gate (subAmongTotal) never lets it start short
			return false
		}
		if d.N == 1 {
			s.recordSubCtr(Obj(d.Options[0].Source()), d.Options[0].Mode())
			continue
		}
		s.bld().pickK = pickSubCtr
		d.Kind, d.Sub, d.Player = DkChoose, ChCostSubCtr, e.ctrl
		return true
	}
}
