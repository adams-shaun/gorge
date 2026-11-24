package kgen

import "testing"

// TestTokenCRiderBitsDisjoint pins that no two Token lInstr C rider bits share a
// value (a shared bit made one rider read as another once branches merged).
func TestTokenCRiderBitsDisjoint(t *testing.T) {
	bits := map[string]int{
		"2": 2, "4": 4, "8": 8, "16": 16, "32": 32, "64": 64, "128": 128,
		"256": 256, "512": 512, "1024": 1024, "2048": 2048,
		"tokListBit": tokListBit, "tokRemSrcBit": tokRemSrcBit,
		"tokSacYourUpkeepBit": tokSacYourUpkeepBit,
	}
	seen := map[int]string{}
	for name, v := range bits {
		if v&(v-1) != 0 {
			t.Errorf("%s = %d is not a single bit", name, v)
		}
		if o, dup := seen[v]; dup {
			t.Errorf("%s and %s share C bit %d", name, o, v)
		}
		seen[v] = name
	}
}
