package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const lotusBlossomAnyColourSacManaRepro = "lotus_blossom_any_colour_counter_scaled_sacrifice_mana_unmodelled"

// CR 605.1a with CR 106.4: Lotus Blossom's "{T}, Sacrifice this artifact: Add X
// mana of any one color, where X is the number of petal counters on it" is a
// counter-scaled sacrifice source, which gorge's prefix search activates to
// price a cast from the floating mana. With one petal and a Mountain gorge
// offers Lotus Blossom and Novellamental ({1}{U}, the Blossom's colour chosen)
// at decision 86 (tail-74-c000 pair 5). The kernel once modelled the shape for
// one fixed colour only and offered neither, so the card was refused; it now
// prices an any-colour counter-scaled sacrifice source for the colour the cast
// needs (dirac/mana_sacctr.go, be4db3171d), and the recorded game replays clean
// on a raw table of its own decks' cards, its whole script served.
func TestMBCLotusBlossomReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", lotusBlossomAnyColourSacManaRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), cardset.Raw(printedDelveSet(t, r.Cards)), r, pol)
	if pol.next != len(r.Actions) || pol.Skipped != 0 {
		t.Fatalf("recording served %d of %d actions (%d skipped): its game path has drifted", pol.next, len(r.Actions), pol.Skipped)
	}
	if !res.Clean() {
		t.Fatalf("Lotus Blossom's recorded game no longer replays clean: div %+v, stall %q", res.Div, res.Stall)
	}
}
