package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const otawaraChannelRepro = "channel_paid_any_colour_land_prefix_budget_unmodelled"

// CR 602.2b, 601.2f: Otawara, Soaring City's Channel ({3}{U}, discard it, less
// {1} per legendary creature) costs {2}{U} beside a legendary creature, and the
// only blue its controller has is Starfleet Academy's {5},{T} any-colour
// ability, a paid source gorge's planner census cannot price. Whether gorge
// offers the Channel then rests on its one prefix search's 160-node budget: it
// withholds it with every land untapped (dr-mbc-abbb pair 58, decision 579)
// where the kernel offers it, and offers it two lands later where a kernel
// withhold keyed to the plain planner does not. No bounded kernel rule matches
// the search, so the card is refused by name and the recorded game still shows
// the divergence with it implicated.
func TestMBCOtawaraSoaringCityReprosLockstep(t *testing.T) {
	t.Parallel()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Otawara, Soaring City" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != otawaraChannelRepro {
			t.Fatalf("Otawara's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, otawaraChannelRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Otawara, Soaring City has no dirac/support/refused entry: its Channel funded by Starfleet Academy diverges at decision 579")
}
