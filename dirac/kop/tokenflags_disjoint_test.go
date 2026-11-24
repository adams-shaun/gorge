package kop

import "testing"

// TestTokenFlagBitsDisjoint: the OpCreateTokenEx B.N flags that share the
// uint16 image are pairwise disjoint. TokAttackPlayer once took bit 14, which
// TokImprintObj already owns, so every player-only attacking token also read
// as an imprinted one.
func TestTokenFlagBitsDisjoint(t *testing.T) {
	flags := []struct {
		name string
		bit  uint16
	}{
		{"TokTapped", TokTapped}, {"TokRemember", TokRemember}, {"TokRememberObj", TokRememberObj},
		{"TokSacAtEnd", TokSacAtEnd}, {"TokExileAtEnd", TokExileAtEnd}, {"TokAttach", TokAttach},
		{"TokAttacking", TokAttacking}, {"TokAttackAsk", TokAttackAsk},
		{"TokAttackPlayer", TokAttackPlayer}, {"TokImprintObj", TokImprintObj}, {"TokRememberSrc", TokRememberSrc},
	}
	var seen uint16
	owner := map[uint16]string{}
	for _, f := range flags {
		if f.bit == 0 || f.bit&(f.bit-1) != 0 {
			t.Errorf("%s = %#x is not a single bit", f.name, f.bit)
			continue
		}
		if seen&f.bit != 0 {
			t.Errorf("%s = %#x collides with %s", f.name, f.bit, owner[f.bit])
		}
		seen |= f.bit
		owner[f.bit] = f.name
	}
}
