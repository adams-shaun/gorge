package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestL5ControlStaticOlderSourceMoves: CR 613.7. Two control statics on one
// host, the older Aura attached first. When the older Aura's controller
// changes, its "You control enchanted creature" wants the host for the new
// controller; that want must not register at the end of the record list,
// above the newer static (gorge staticGrantNewer). The host stays with the
// newer Aura.
func TestL5ControlStaticOlderSourceMoves(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sameStamp bool
	}{
		{"older source has an earlier stamp", false},
		{"equal stamps tie-break on the source slot", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := newL5(t)
			s, k := l.sc.s, l.sc.k
			w := l.sc.perm(l.id["Ambush Wolf"], 0)
			a0 := l.sc.perm(l.id["Witness Protection"], 0)
			a1 := l.sc.perm(l.id["Witness Protection"], 0)
			sl0, sl1 := l.sc.slot(a0), l.sc.slot(a1)
			if sl0 >= sl1 {
				t.Fatalf("setup: older Aura slot %d is not below the newer %d", sl0, sl1)
			}
			s.attachTo(k, sl0, l.sc.slot(w))
			s.attachTo(k, sl1, l.sc.slot(w))
			if tc.sameStamp {
				s.lstamp[sl1] = s.lstamp[sl0]
			}
			if s.layerTs(sl0) > s.layerTs(sl1) || (s.layerTs(sl0) == s.layerTs(sl1) && sl0 > sl1) {
				t.Fatalf("setup: a0 stamp %d slot %d is not the older, a1 stamp %d slot %d",
					s.layerTs(sl0), sl0, s.layerTs(sl1), sl1)
			}
			s.sba(k)
			if got := l.ctrl(w); got != 0 {
				t.Fatalf("setup: host controller %d, want 0", got)
			}
			if got := l.records(); got != 2 {
				t.Fatalf("setup: %d control records, want both statics", got)
			}
			// p1 steals the older Aura (in the engine oracle: a Confiscate on
			// it). Its static now wants the host for p1, but the newer Aura
			// is still live and keeps it.
			l.gain(a0, 1, kop.DurPermanent, NoObj)
			if got := l.ctrl(a0); got != 1 {
				t.Fatalf("setup: older Aura controller %d, want 1", got)
			}
			s.sba(k)
			if got := l.ctrl(w); got != 0 {
				t.Fatalf("older Aura's controller moved: host controller %d, want 0 (the newer static's)", got)
			}
		})
	}
}
