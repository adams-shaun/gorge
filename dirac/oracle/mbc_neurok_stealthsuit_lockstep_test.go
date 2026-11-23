package oracle

import "testing"

// Neurok Stealthsuit's lockstep repros, one row per cause:
//   - neurok_stealthsuit_modes: its second option at a modes decision: gorge
//     offers two modes where dirac offers one (pool tail-34-c000, pair 5,
//     decision 243).
//   - equip_unpriced_census_under_manabarbs: at a main-phase priority decision
//     with every land under the opponent's Manabarbs beside an Unknown Shores,
//     gorge's potential walk leaves the Equip {1} census_incomplete (as it does
//     both hand casts and Rakdos Riteknife's Equip {2}) and offers only the
//     {U}{U} attach, while dirac also offers the Equip (pool tail-34-c000,
//     pair 3, decision 1229).
func TestMBCNeurokStealthsuitReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"neurok_stealthsuit_modes",
		"equip_unpriced_census_under_manabarbs",
	} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
