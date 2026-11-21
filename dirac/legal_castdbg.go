package dirac

import (
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TEMPORARY debug harness (deleted before commit).
func castDbgOn() bool {
	return os.Getenv("DIRAC_CASTDBG") != ""
}

func (s *State) castDbgDump(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost, cav *avail) {
	fmt.Fprintf(os.Stderr, "CASTDBG card=%s cost=%+v pips=%v hyb=%v cav.total=%d n=%d life=%d realPool=%v forkDepth=%d\n",
		k.t.CardName(c), cost.Pips, pipMaskOf(cost), hybN(cost), cav.total, cav.n, cav.life, s.pl[SeatIx(p)].pool, s.forkDepthNow())
	for i := 0; i < 16; i++ {
		if cav.mask[i] != 0 || cav.cnt[i] > 0 {
			fmt.Fprintf(os.Stderr, "CASTDBG   unit mask=%02x n=%d\n", cav.mask[i], cav.cnt[i])
		}
	}
	fmt.Fprintf(os.Stderr, "CASTDBG   one=%v oneMask=%v oneAltMask=%v oneAltN=%v nOne=%d nModal=%d\n",
		cav.one, cav.oneMask, cav.oneAltMask, cav.oneAltN, cav.nOne, cav.nModal)
	fmt.Fprintf(os.Stderr, "CASTDBG   pain=%+v\n", cav.pain)
	fmt.Fprintf(os.Stderr, "CASTDBG   lifeGapMask=%v scriptGaps=%v tapsDef=%v restr=%v anyR=%v availX=%v uses.has(useLifeGap)=%v\n",
		s.lifeGapMask(k, p), s.scriptGaps(k, p), s.scriptTapsDefer(k, p), s.scriptRestr(k, p), s.scriptAnyR(k, p), s.g.availX, k.gen != nil && k.gen.uses.has(useLifeGap))
	fmt.Fprintf(os.Stderr, "CASTDBG   scriptFilt=%v anyFiltFirst=%v anyFilt=%v\n", s.scriptFilt(k, p), k.gen.anyFiltFirst, k.gen.anyFilt)
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		if k.gen != nil && k.gen.has(s.cardAt(b.first()), gfFilt) {
			fmt.Fprintf(os.Stderr, "CASTDBG   filt card=%s sl=%d\n", k.t.CardName(s.cardAt(b.first())), b.first())
		}
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		fmt.Fprintf(os.Stderr, "CASTDBG   perm card=%s sl=%d tapped=%v\n", k.t.CardName(s.cardAt(b.first())), b.first(), s.perms[b.first()].flags&pfTapped != 0)
	}
}

func init() {
	if v := os.Getenv("DIRAC_NODEBUDGET"); v != "" {
		n := 0
		for i := 0; i < len(v); i++ {
			n = n*10 + int(v[i]-'0')
		}
		DebugFiltNodeBudget = n
		DebugFiltMemoCheck = true
	}
}

func (s *State) prioTappedNames(k *prep, p uint8) []string {
	var out []string
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		if s.perms[b.first()].flags&pfTapped != 0 {
			out = append(out, k.t.CardName(s.cardAt(b.first())))
		}
	}
	return out
}

func (s *State) forkDepthNow() int {
	if s.pool == nil {
		return 0
	}
	return s.pool.depth
}
