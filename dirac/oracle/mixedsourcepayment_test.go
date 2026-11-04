package oracle

import "testing"

// A cast from exile through Zero, Incorporeal Sensate's permission costs {2}
// more (CR 118.8, 601.2f). Cha'DIch Investigator ({4}{B}) is then a 7-mana
// cast that the printed sources (two Forests, two Swamps, Xenobotanist and
// Cardassia Prime) cannot pay and Rok-Tahk's granted {G}{G} on every creature
// can. gorge's prefix search floats Xenobotanist's mana and U.S.S. Discovery's
// {G}{G} before the cast; the kernel priced the same search without the
// surcharge, found the printed sources enough, and tapped Cardassia Prime and
// Photosynthetic Physician at the cast instead (full surface).
func TestMixedSourcePaymentFullLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-trk", []stdRepro{
		{"granted_prefix_prices_may_play_surcharge", false, false}, // CR 118.8, 601.2f: the prefix funds the cast's {2} surcharge too, so the same permanents are tapped
	})
}
