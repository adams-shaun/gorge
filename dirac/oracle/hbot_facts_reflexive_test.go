package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
)

// A reflexive trigger's kernel ability is latent (no gorge ability of its
// own); gorge ranks its targets by the ImmediateTrigger's Execute$ body, so the
// port's facts carry that body's damage on the latent ability: Boilerbilges
// Ripper's "when you do, 2 damage to any target" aimed at a creature it kills.
func TestHBotFactsReflexiveBody(t *testing.T) {
	c, _ := testCorpus(t)
	set := cardsettest.Synth(t, "std-dsk")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	f := HBotFactsSet(c.Reg, tbl, nil, set)
	seen := false
	for i := range tbl.Cards {
		if tbl.Cards[i].Name != "Boilerbilges Ripper" {
			continue
		}
		for j, a := range tbl.Cards[i].Abilities {
			if a.AFlags&kop.AfLatent == 0 {
				continue
			}
			seen = true
			chain := f.Cards[i].Abilities[j].Tgt
			if len(chain) != 1 || chain[0].API != hbot.ApiDamage || chain[0].Dmg != 2 {
				t.Errorf("latent ability %d targets %+v, want one 2-damage effect", j, chain)
			}
		}
	}
	if !seen {
		t.Fatal("Boilerbilges Ripper has no latent ability in the std-dsk pool")
	}
}
