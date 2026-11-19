package oracle

import "testing"

// TestMBCMinamoSchoolAtWaterSEdgeReprosLockstep: at a priority decision with
// only fee'd any-colour mana sources beside the untap land (pool dr-mbc-abbb,
// pair 1, seed 609670), the kernel offered Minamo, School at Water's Edge's
// "{U}, {T}: Untap target legendary permanent" although gorge offers nothing:
// the {U} is payable only through Starfleet Academy's "Federation — {5}, {T}"
// chain (two creature types among a lone Mesmeric Fiend, the {5} paid from the
// other five lands), and gorge's non-mana activation offer gate prices the
// printed cost over the plain potential bound only — a filter-funded script
// never reaches a non-mana activation offer.
func TestMBCMinamoSchoolAtWaterSEdgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_script_funds_nonmana_activation")
}
