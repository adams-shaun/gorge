package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFeatCtrChoiceRankIsPrintedOrder: the counter-kind pick's options sit in
// kernel kind order, but a kernel bot answers gorge's first offer, the first
// listed kind, so FeatCtrChoiceRank reports the printed position of each
// offered kind: in the resolving ask and in the as-enters ask, and -1 with no
// such ask pending or for a kind the list lacks.
func TestFeatCtrChoiceRankIsPrintedOrder(t *testing.T) {
	check := func(t *testing.T, s *State, d Decision) {
		t.Helper()
		if d.Kind != DkChoose || d.Sub != ChRemCtrKind {
			t.Fatalf("decision kind %d sub %d, want the counter-kind pick", d.Kind, d.Sub)
		}
		sorted := true
		for i := 1; i < int(d.N); i++ {
			sorted = sorted && d.Options[i-1].Mode() < d.Options[i].Mode()
		}
		if !sorted || pccKinds[0] == kop.CounterKind(d.Options[0].Mode()) {
			t.Fatalf("options %v: want kernel kind order apart from the printed order %v", pccModes(t, d), pccKinds)
		}
		for want, k := range pccKinds {
			if got := s.FeatCtrChoiceRank(uint8(k)); got != want {
				t.Errorf("rank of kind %d = %d, want %d", k, got, want)
			}
		}
		if got := s.FeatCtrChoiceRank(uint8(kop.CtrLifelink)); got != -1 {
			t.Errorf("rank of an unlisted kind = %d, want -1", got)
		}
	}
	t.Run("resolving", func(t *testing.T) {
		f := newPccFix(t)
		sc := f.sc
		if got := sc.s.FeatCtrChoiceRank(uint8(pccKinds[0])); got != -1 {
			t.Fatalf("rank with no ask pending = %d, want -1", got)
		}
		host := sc.perm(f.host, 0)
		sc.do(t, AkActivate, TCode(host), scAny)
		sc.do(t, AkTarget, scAny, TCode(host))
		sc.do(t, AkPass, scAny, scAny)
		sc.do(t, AkPass, scAny, scAny)
		check(t, sc.s, sc.s.Step())
	})
	t.Run("enters", func(t *testing.T) {
		f := newPccFix(t)
		sc := f.sc
		sc.perm(f.enter, 0)
		check(t, sc.s, sc.s.Step())
	})
}
