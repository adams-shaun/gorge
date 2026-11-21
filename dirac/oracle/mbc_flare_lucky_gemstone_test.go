package oracle

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
)

// mbcFlareSet is the table the recording's decks need: the game was dealt from
// dr-mbc-abbb with the enablers its -synth-with pools lent (Academy Ruins from
// std-trc, Prakhata Pillar-Bug from another), so no pool on disk holds all of
// them. The table is a test artifact, never committed (kgen's tables are
// corpus-derived); cardset.Raw plays every lowered card, a card refused after
// the recording included.
func mbcFlareSet(t *testing.T, cards [2][]string) cardset.Set {
	t.Helper()
	var names []string
	for _, d := range cards {
		names = append(names, d...)
	}
	slices.Sort(names)
	names = slices.Compact(names)
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Fatal(err)
	}
	d, err := kgen.SynthUnionTable(names, cardsDir, "dr-mbc-abbb")
	if err != nil {
		t.Fatal(err)
	}
	return cardset.Raw(cardset.NewDataSet(d))
}

// TestMBCFlareLuckyGemstoneReprosLockstep replays the recorded dr-mbc-abbb game
// in which p1 holds Evincar's Justice ({2}{B}{B}) with a lucky Gemstone Caverns
// (its luck counter, so the tap makes one mana of any colour) and Mana Flare on
// the battlefield. The Flare's extra is one mana of any colour the land could
// produce (CR 106.12a), so the Gemstone's tap funds 2 mana of one colour beside
// the Plains' {W}{W}, and the cast is offered in both engines (CR 605.4, gorge
// leads). The kernel's census dropped the cast: the extra was lost for the
// condition-split source (mana_condsplit.go, mana_tapextra.go).
func TestMBCFlareLuckyGemstoneReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "mbc_flare_lucky_gemstone.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	entries := func(names []string) []cardset.Entry {
		var es []cardset.Entry
		for _, n := range names {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
		return es
	}
	tbl, _, _, err := mbcFlareSet(t, r.Cards).Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(mbcFlareSet(t, r.Cards), tbl, Config{})(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	rc := r.RunConfig()
	script := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, script, rc)
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s\n gorge %v\n dirac %v\n %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions, res.Div.Diff)
	}
	if res.Stall != "" {
		t.Fatalf("kernel stalled: %s", res.Stall)
	}
}
