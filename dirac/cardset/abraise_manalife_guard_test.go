package cardset

import "testing"

// TestAbRaiseGuardManaLifeGrant: a mana-abilities' life raiser (Thran Portal)
// is refused beside a card whose grant is a mana ability (AbManaGrants, the
// granted view prices no raise) and accepted beside one that grants no mana
// ability (absent from AbManaGrants).
func TestAbRaiseGuardManaLifeGrant(t *testing.T) {
	mk := func(grants []int) dataSet {
		return dataSet{d: &PoolData{
			Names:           []string{"Portal", "Lantern", "Equipper"},
			AbRaisers:       []int{0},
			AbRaiseTypes:    []uint16{0},
			AbRaisePick:     []bool{false},
			AbRaiseManaLife: []bool{true},
			AbManaGrants:    grants,
			AbManaAny:       []int{0},
			AbManaAnyTypes:  []uint16{0},
		}}
	}
	if err := mk([]int{1}).abRaiseGuard([]int{0, 1}); err == nil {
		t.Error("a mana-life raiser beside a mana grant was not refused")
	}
	if err := mk([]int{1}).abRaiseGuard([]int{0, 2}); err != nil {
		t.Errorf("a mana-life raiser beside a non-mana grant card refused: %v", err)
	}
	if err := mk(nil).abRaiseGuard([]int{0, 1, 2}); err != nil {
		t.Errorf("a mana-life raiser beside no mana grant refused: %v", err)
	}
}
