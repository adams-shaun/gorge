package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const svogthosRepro = "svogthos_green_only_from_candleguide_filter_search_unmodelled"

// CR 602.2b, 605.3a: Svogthos, the Restless Tomb's {3}{B}{G} animate is payable
// when the only green source is Crossroads Candleguide's "{2}: Add one mana of
// any color", and the kernel offers it. gorge's potential-play prefix search
// (potentialPrefixScript, 160 nodes) tries the colour ask's B answer before
// its G one, both being announced pips, and runs out of budget before it
// reaches G, so gorge offers no activation (census_incomplete, measured at
// found=false limited=true on a Candleguide, three Islands, four Swamps and
// the land). Mirroring that search's budget for a two-mana repeatable fee is
// over the bounded-code line, so the card is refused by name and the recorded
// game (tail-71 pair 100, seed 7; first filed against tail-70 pair 4 before a
// refusal moved the pool's buckets) still shows the divergence with it
// implicated.
func TestMBCSvogthosTheRestlessTombReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Svogthos, the Restless Tomb" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != svogthosRepro {
			t.Fatalf("Svogthos's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, svogthosRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Svogthos, the Restless Tomb has no dirac/support/refused entry: its animate over a Candleguide-only green is offered by the kernel alone")
}
