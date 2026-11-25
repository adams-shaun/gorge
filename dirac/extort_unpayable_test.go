package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExtortAsksOnlyWhenThePipIsPayable is the dirac half of gorge's
// TestExtortAsksOnlyWhenThePipIsPayable (CR 118.3): the Extort election is
// posed only when the plain pool holds a W or B, so a seat with neither
// resolves the trigger with no ask, and a restricted-only W is not a pip.
func TestExtortAsksOnlyWhenThePipIsPayable(t *testing.T) {
	prog := []kop.Instr{{Op: kop.OpExt, A: kop.Const(int(kop.ExtExtort))}}
	for _, tc := range []struct {
		name  string
		setup func(*State)
		want  bool
	}{
		{"empty pool", func(*State) {}, false},
		{"only red", func(s *State) { s.pl[0].pool[kop.R] = 2 }, false},
		{"restricted white only", func(s *State) {
			s.rb[0][0] = rbatch{kind: 1}
			s.rb[0][0].n[kop.W] = 1
		}, false},
		{"white", func(s *State) { s.pl[0].pool[kop.W] = 1 }, true},
		{"black", func(s *State) { s.pl[0].pool[kop.B] = 1 }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := czScen(t, prog)
			s := sc.s
			s.pl[0].pool = [kop.NumManaKinds]uint8{}
			tc.setup(s)
			done := s.exec(sc.k, &prog[0])
			asked := !done && s.res.ask == raGenExtort
			if asked != tc.want {
				t.Fatalf("Extort ask posed = %v (exec done %v, res.ask %d), want %v", asked, done, s.res.ask, tc.want)
			}
			if tc.want {
				var d Decision
				s.resolveOptionsGen(sc.k, &d)
				if d.Sub != ChGenPayMode || d.N != 2 {
					t.Fatalf("election sub %d n %d, want the pay/decline pair", d.Sub, d.N)
				}
			}
		})
	}
}
