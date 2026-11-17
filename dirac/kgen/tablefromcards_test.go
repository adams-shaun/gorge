package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestTableFromCards: TableFromCards lowers already-resolved cards (an inline
// cards.ParseBytes script, no corpus list or synthetic pool) into a pool table
// in this process, the same lowerPool/tableOf path kgen.Main's -card-list
// branch runs for real names, and the table's deck plays: the probe is
// playable, a matchup over its deck builds, and its lowering carries the
// captured-source sweep instruction.
func TestTableFromCards(t *testing.T) {
	probe, diags := cards.ParseBytes("tablefromcards_probe.txt", []byte(sweepCapGameProbe))
	if probe == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := probe.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	pd, err := TableFromCards(&cards.Registry{}, []*cards.Card{probe}, "tablefromcards")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	deck := []cardset.Entry{{Name: "Sweep Cap Probe", Count: 40}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatalf("matchup: %v", err)
	}
	supported := set.Supported()
	if len(da) == 0 {
		t.Fatal("the matchup assigned no card ids")
	}
	if !supported[da[0]] {
		t.Fatalf("the probe is not playable in its own table (supported %v)", supported)
	}
	id := da[0]
	if tbl.Cards[id].Name != "Sweep Cap Probe" {
		t.Fatalf("matchup card %d is %q, want the probe", id, tbl.Cards[id].Name)
	}
	saw := false
	for i := range tbl.Cards[id].Abilities {
		for _, in := range tbl.Cards[id].Abilities[i].Prog {
			if in.Op == kop.OpExt && in.A == kop.Const(int(kop.ExtSweepFrom)) {
				saw = true
			}
		}
	}
	if !saw {
		t.Fatal("the probe's lowering carries no ExtSweepFrom instruction")
	}
}
