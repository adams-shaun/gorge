package oracle

import (
	"testing"
)

// Heaven's Gate cast at a Waterfall Aerialist (Ward {2}) with the caster's
// only open mana a Cinder Glade and a Mossfire Valley ("{1}, {T}: Add {R}{G}"):
// gorge's ward window taps the Glade, pays the filter's fee with its {R} and
// pays the ward with the filter's {R}{G} (decision 950 of pair 89, pool
// tail-23, surface autopay); the kernel priced the ward unpayable (the census
// counts no filter), tapped what it had and countered the spell, leaving
// {R}{G} floating.
func TestMBCWardStackReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ward_pays_through_first_only_filter_source")
}
