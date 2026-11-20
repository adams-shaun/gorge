package oracle

import "testing"

// CR 106.6 with CR 605.3b: Galazeth Prismari's artifacts make red mana "spend
// ... only to cast an instant or sorcery spell". Tectonic Break (a sorcery, so
// the mana pays its cost) is cast with only that red floating beside an
// untapped Shadowblood Ridge, whose "{1}, {T}: Add {B}{R}" is an activated mana
// ability. The restricted red cannot pay the filter's {1}, so X = 1 is not
// payable and the announcement offers X = 0 alone. The kernel's window bound
// let the restricted mana fund the filter's fee and offered X = 1 too
// (decision 458 of 458).
func TestMBCTectonicBreakReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restr_pool_not_filter_fee_x")
}
