package oracle

import "testing"

// CR 605.1a, 106.1: Three Tree City's {2},{T} ability adds mana equal to the
// number of creatures of its chosen type (here Splinter) its controller
// controls, and that is none, so it can't supply the {U} Otawara, Soaring City's
// channel ability needs ({3}{U}, one less for the Legendary Elas il-Kor).
// With no other blue source the channel isn't affordable and gorge offers only
// the pass; the kernel also offered the activation (decision 294 of 294;
// dr-mbc-abbb pair 20, autopay).
func TestMBCOtawaraSoaringCityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "channel_offer_zero_count_filter_land")
}
