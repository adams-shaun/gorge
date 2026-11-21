package oracle

import "testing"

// An activation the grant search prices from granted mana is no menu option
// of gorge's. At decision 1149 of pool tail-26 (pair 35, surface autopay)
// p1's Wishful Merfolk activation {1}{U} is priced by neither the floating
// pool nor the kernel's menu mirrors: the only blue source is Henge of
// Ramos's repeatable net-negative fee gap "{2},{T}: one mana of any color",
// whose fee no printed source funds, so gorge's payment planner (pay/board.go
// PaymentAbility holds printed abilities only) and the kernel's census and
// filter script both leave the activation off the list. The grant search,
// though, funds Henge's fee with two granted Elf taps ("Elves you control
// have '{T}: Add {B}'" under A-Tyvar Kell) before it — mana gorge's planner
// never witnesses — and the script route offered the activation anyway. The
// filter-only script prices the lines the planner does; the grant search
// prices no activation (options options, lead gorge, implicates Wishful
// Merfolk).
func TestMBCWishfulMerfolkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_prefix_menu_absent")
}
