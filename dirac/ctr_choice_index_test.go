package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A bot answering a PutCounterChoice ask in gorge's order takes the first
// listed kind, but the kernel sorts the options: FeatCounterChoiceIndex gives
// each offered kind its printed index, in both forms, and -1 outside the ask.
func TestPutCounterChoiceFeatIndex(t *testing.T) {
	check := func(t *testing.T, s *State) {
		t.Helper()
		d := s.Step()
		if d.Kind != DkChoose || d.Sub != ChRemCtrKind {
			t.Fatalf("decision kind %d sub %d, want the counter-kind pick", d.Kind, d.Sub)
		}
		first := d.Options[0].Mode()
		if first == uint8(pccKinds[0]) {
			t.Fatal("the first sorted option is the first printed kind: the test cannot tell the orders apart")
		}
		for i, k := range pccKinds {
			if got := s.FeatCounterChoiceIndex(uint8(k)); got != i {
				t.Errorf("kind %d: index %d, want its printed index %d", k, got, i)
			}
		}
		if got := s.FeatCounterChoiceIndex(uint8(kop.CtrLoyalty)); got != -1 {
			t.Errorf("an unlisted kind has index %d, want -1", got)
		}
	}
	t.Run("resolving", func(t *testing.T) {
		f := newPccFix(t)
		sc := f.sc
		host := sc.perm(f.host, 0)
		sc.do(t, AkActivate, TCode(host), scAny)
		sc.do(t, AkTarget, scAny, TCode(host))
		sc.do(t, AkPass, scAny, scAny)
		sc.do(t, AkPass, scAny, scAny)
		check(t, sc.s)
	})
	t.Run("enters", func(t *testing.T) {
		f := newPccFix(t)
		f.sc.perm(f.enter, 0)
		check(t, f.sc.s)
	})
	t.Run("no ask", func(t *testing.T) {
		f := newPccFix(t)
		if got := f.sc.s.FeatCounterChoiceIndex(uint8(pccKinds[0])); got != -1 {
			t.Errorf("index %d with no ask pending, want -1", got)
		}
	})
}
