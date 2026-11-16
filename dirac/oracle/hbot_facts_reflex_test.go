package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
)

// TestHBotFactsReflexiveBody: a "when you do" body (Boilerbilges Ripper's
// 2 damage to any target, after the sacrifice) is a latent kernel ability with
// no gorge ability index; its TargetEffect facts come from the ImmediateTrigger's
// Execute$ chain, so the port ranks the body's target as gorge's bot does.
func TestHBotFactsReflexiveBody(t *testing.T) {
	c, _ := testCorpus(t)
	set := cardsettest.Synth(t, "std-dsk")
	if _, files, err := SynthPool(c.Reg, set, 1, SynthDecks); err != nil {
		t.Fatal(err)
	} else {
		c.AddDecks(files)
	}
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	f := HBotFactsSet(c.Reg, tbl, nil, set)
	found := false
	for i := range tbl.Cards {
		if tbl.Cards[i].Name != "Boilerbilges Ripper" {
			continue
		}
		for j, ka := range tbl.Cards[i].Abilities {
			if ka.AFlags&kop.AfLatent == 0 {
				continue
			}
			found = true
			tg := f.Cards[i].Abilities[j].Tgt
			if len(tg) != 1 || tg[0].API != hbot.ApiDamage || tg[0].Dmg != 2 {
				t.Errorf("Boilerbilges Ripper's reflexive body facts = %+v, want one 2-damage effect", tg)
			}
		}
	}
	if !found {
		t.Fatal("Boilerbilges Ripper has no latent ability in std-dsk: the test cannot see the body")
	}
}
