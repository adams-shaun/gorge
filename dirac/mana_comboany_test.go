package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestComboAnyMixStallClassIsUnmodelled: a Combo Any source beside another
// census-gap kind is a rule gap, not a capacity limit, and is named comboanymix.
func TestComboAnyMixStallClassIsUnmodelled(t *testing.T) {
	if got := StallClassOf(StallComboAnyMix); got != StallUnmodelled {
		t.Fatalf("StallClassOf(StallComboAnyMix) = %d, want StallUnmodelled", got)
	}
	if got := StallName(StallComboAnyMix); got != "comboanymix" {
		t.Fatalf("StallName(StallComboAnyMix) = %q, want comboanymix", got)
	}
}

// TestComboAnyShape: the Combo Any gap is an unrestricted {T} any-colour choice
// of a dynamic or multiple amount, and nothing nearby.
func TestComboAnyShape(t *testing.T) {
	axebane := kop.Ability{Kind: kop.AbMana, AFlags: kop.AfManaChoice | kop.AfManaDynamic, ManaColors: kop.ColAny, Cost: kop.Cost{Flags: kop.CostTap}}
	if !comboAnyMana(&axebane) {
		t.Fatal("an unrestricted dynamic {T} Combo Any is not a Combo Any gap")
	}
	two := axebane
	two.AFlags, two.ManaN = kop.AfManaChoice, 2
	if !comboAnyMana(&two) {
		t.Fatal("an unrestricted {T} Combo Any of two mana is not a Combo Any gap")
	}
	one := two
	one.ManaN = 1
	restr := axebane
	restr.ManaRestrict = 1
	paid := axebane
	paid.Cost.Flags |= kop.CostSacSelf
	one2 := axebane
	one2.AFlags = kop.AfManaChoice
	for name, a := range map[string]kop.Ability{"one mana": one, "restricted": restr, "sacrifice cost": paid, "constant one": one2} {
		if comboAnyMana(&a) {
			t.Errorf("%s is a Combo Any gap", name)
		}
	}
}

// TestComboAnyAnswerOrder: gorge's colour ask for n units lists the first 64
// per-unit combinations in odometer order (unit 0 the slowest digit), the
// answers naming a colour the play announces first, order kept inside each half.
func TestComboAnyAnswerOrder(t *testing.T) {
	var none, green [kop.NumManaKinds]bool
	green[kop.G] = true
	for r := 0; r < caAnswerCount(2); r++ {
		if got := caRank(2, &none, r); got != r {
			t.Fatalf("no announced colour: rank %d is answer %d, want odometer order", r, got)
		}
	}
	// two units, G announced: the answers with a G unit come first (W G, U G,
	// B G, R G, G W, G U, G B, G R, G G are the nine; the first is W G = digits 0,4)
	if got := caRank(2, &green, 0); caDigit(got, 2, 0) != 0 || caDigit(got, 2, 1) != 4 {
		t.Fatalf("first answer = %d, want W G", got)
	}
	named := 0
	for r := 0; r < caAnswerCount(2); r++ {
		i := caRank(2, &green, r)
		if caNames(i, 2, &green) {
			named++
			continue
		}
		if named != 9 {
			t.Fatalf("%d answers name G before the first that does not, want 9", named)
		}
		break
	}
	// three units: only the first 64 of 125 combinations are asked, so unit 0 is
	// never above U (0 or 1) and the leading units of a larger ask read W
	for r := 0; r < caAnswerCount(3); r++ {
		if i := caRank(3, &green, r); i >= maxCAAnswers {
			t.Fatalf("answer %d beyond the %d gorge asks", i, maxCAAnswers)
		}
	}
	if caDigit(63, 5, 0) != 0 || caDigit(63, 5, 1) != 0 {
		t.Fatal("a leading unit of a five-unit ask is not W")
	}
}
