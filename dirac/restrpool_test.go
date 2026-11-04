package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRestrPoolActivation: restricted mana floating beside an untapped land
// and Furnace Whelp. Mana the ability may not spend neither funds the offer nor
// removes it (CR 106.6; gorge reads the pool through pay.PlanPool): the Mountain
// pays the {R} on every surface, and with the Mountain tapped nothing does.
func TestRestrPoolActivation(t *testing.T) {
	for _, c := range []struct {
		name string
		cfg  uint8
	}{
		{"native", 0},
		{"gorge lowering", cfgGorgeLowering},
		{"gorge autopay", cfgGorgeAutopay},
	} {
		sc, id := restrSourceScen(t, "Furnace Whelp", "Mountain")
		mtn := sc.perm(id["Mountain"], 0)
		whelp := sc.perm(id["Furnace Whelp"], 0)
		sc.s.cfg |= c.cfg
		if d := sc.s.Step(); !actOffered(&d, whelp) {
			t.Fatalf("%s: Whelp's {R} activation not offered with no restricted mana floating", c.name)
		}
		sc.s.setRestr(0, 1, kop.W, 1)
		if d := sc.s.Step(); !actOffered(&d, whelp) {
			t.Fatalf("%s: Whelp's {R} activation not offered from the Mountain with restricted mana floating", c.name)
		}
		sc.s.perms[sc.slot(mtn)].flags |= pfTapped
		if d := sc.s.Step(); actOffered(&d, whelp) {
			t.Fatalf("%s: Whelp's {R} activation offered from restricted mana alone", c.name)
		}
	}
}
