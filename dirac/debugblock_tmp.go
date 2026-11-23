package dirac

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TEMPORARY debug dump (delete before commit).
var DebugBlockDump bool

func (s *State) dumpBlockDebug(k *prep, d *Decision) {
	fmt.Printf("BLOCKDUMP def=%d active=%d anyLure=%v anyLureStat=%v anyMustBlock=%v\n",
		s.blockDef(), s.active, k.gen != nil && k.gen.anyLure, k.gen != nil && k.gen.anyLureStat, k.gen != nil && k.gen.anyMustBlock)
	for m := s.blockAtkM(k); m.any(); m = m.rest() {
		a := m.first()
		atk := s.perms[a].flags&pfAttacking != 0
		fmt.Printf("  atk sl=%d card=%q attacking=%v isLure=%v cardLure=%v mechLure=%v\n",
			a, k.t.Cards[s.cardAt(a)].Name, atk, s.isLure(k, a), k.t.Cards[s.cardAt(a)].Lure, s.mSl(k, mechLure, a))
	}
	s.eachStatic(k, func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatLure {
			return true
		}
		host := s.perms[st.sl].attached
		hostName := "none"
		if host != NoSlot {
			hostName = k.t.Cards[s.cardAt(host)].Name
		}
		for m := s.blockAtkM(k); m.any(); m = m.rest() {
			a := m.first()
			cond := s.statCond(k, st)
			mm := s.matchPermGen(k, ab.Static.Filter, a, st.ctrl, st.sl, layerFull)
			fmt.Printf("  lurestatic src=%q sl=%d ctrl=%d attached=%d(host=%q) cond=%v matchAtk sl=%d(%q)=%v\n",
				k.t.Cards[s.cardAt(st.sl)].Name, st.sl, st.ctrl, host, hostName, cond, a, k.t.Cards[s.cardAt(a)].Name, mm)
		}
		return true
	})
	for bm := s.ctrlM[s.blockDef()]; bm.any(); bm = bm.rest() {
		b := bm.first()
		fmt.Printf("  blk sl=%d card=%q ready=%v\n", b, k.t.Cards[s.cardAt(b)].Name, s.blockerReady(k, b))
	}
}
