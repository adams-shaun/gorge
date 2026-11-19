package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPeekHandRefusesOtherRevealArms: the Hand arm is the remember write of a
// RevealHand (act_peekreveal.go peekHand), and it must not silently overwrite
// another reveal arm's flag. opPeek assigns ki.C once per arm (MayReveal,
// Hand, Public), so without the guard a CS carrying Hand beside MayReveal or
// Public would lower as though the other intent were absent. Only the exact
// Hand+Remember shape lowers; a remember-less Hand and a Hand beside any other
// arm refuse, fail closed (no corpus card has those shapes today).
func TestPeekHandRefusesOtherRevealArms(t *testing.T) {
	for _, tc := range []struct {
		name string
		fl   map[string]string
		want string // "" = lowered
	}{
		{"hand remember", map[string]string{"Hand": "", "Remember": ""}, ""},
		{"hand without remember", map[string]string{"Hand": ""}, "without Remember"},
		{"hand with may reveal", map[string]string{"Hand": "", "Remember": "", "MayReveal": ""}, "MayReveal"},
		{"hand with public", map[string]string{"Hand": "", "Remember": "", "Public": ""}, "Public"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ic := &instrCtx{fl: tc.fl}
			err := ic.opPeek()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("opPeek(%v) = %v, want lowered", tc.fl, err)
				}
				if ic.ki.C&kop.PeekHand == 0 {
					t.Fatalf("opPeek(%v) C = %d, want the PeekHand bit", tc.fl, ic.ki.C)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("opPeek(%v) = %v, want an error containing %q", tc.fl, err, tc.want)
			}
		})
	}
}
