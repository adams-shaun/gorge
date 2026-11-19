package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHStaticsBSupported pins the edh-staticsB class: each card below was
// blocked from full Commander-deck support only by an unregistered static
// mode (CantTransform, CantAttach, CantDiscard, MaxCounter, ManaBurn). With
// the coverage report's shape gate set, every one must now be counted
// playable. Hogaak, Arisen Necropolis (stat:ManaRestriction: "You can't spend
// mana to cast this spell") is not in the list: its convoke/delve-only payment
// is a separate ticket, and the mode's other carriers (Myr Superion,
// Imperiosaur) need per-source pool provenance gorge does not track.
func TestEDHStaticsBSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	for _, name := range []string{
		"Immerwolf",
		"Bound by Moonsilver",
		"O-Naginata",
		"Konda's Banner",
		"Tamiyo, Collector of Tales",
		"Rasputin Dreamweaver",
		"Yurlok of Scorch Thrash",
		"Hogaak, Arisen Necropolis",
	} {
		c, ok := r.Lookup(name)
		if !ok {
			t.Errorf("corpus missing %s", name)
			continue
		}
		if m := r.Unsupported(c, sup); len(m) != 0 {
			t.Errorf("%s unsupported: %v", name, m)
		}
	}
}
