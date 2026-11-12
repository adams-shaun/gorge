package rules

// namor_chroma_test.go pins the ChromaSource count head end-to-end. Before the evaluator, `TriggeredCard$ChromaSource.Blue`
// answered zero, so Namor the Sub-Mariner's spell-cast trigger created no
// Merfolk, and `Count$ChromaSource.White` answered zero, so Light from
// Within's AffectedX added +0/+0. Both scenarios assert the OBSERVABLE
// effect (token, P/T), not the SVar number.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Divination is a noncreature spell with exactly one blue mana symbol
// ("2 U"), so Namor's trigger makes one 1/1 Merfolk Token; Namor itself is
// a Merfolk, so its characteristic-defining power becomes 2 (1 without the
// token).
const namorChromaScenario = `{"name":"namor-chroma","cr":["202.2","603.3"],"why":"inline","setup":{"p0":{"battlefield":["Namor the Sub-Mariner"],"hand":["Divination"],"library_top":["Island","Island"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Divination","mana":"CCU"},{"op":"resolve"}],"expect":[{"card":"p0:Namor the Sub-Mariner","zone":"battlefield","pt":"2/4"}]}`

func TestNamorChromaSourceTrigger(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(namorChromaScenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	// The power 2/4 expect alone would also pass if something else pumped
	// Namor, so count the token the trigger actually made: exactly one
	// Merfolk Token on p0's battlefield.
	last := res.Snapshots[len(res.Snapshots)-1]
	tokens := 0
	for _, p := range last.Permanents {
		if p.Token && p.Controller == 0 && p.Name == "Merfolk Token" {
			tokens++
		}
	}
	if tokens != 1 {
		t.Fatalf("Merfolk tokens = %d, want 1 (permanents: %+v)", tokens, last.Permanents)
	}
}

// Light from Within's AffectedX is Count$ChromaSource.White evaluated per
// affected creature, so Savannah Lions ("W") gets +1/+1 -> 3/2 while its
// blue-costed neighbour stays untouched. A source-anchoring regression (the
// static evaluating the Aura's own cost instead of each creature's) would
// read 2 white symbols and give +2/+2.
const lightFromWithinChromaScenario = `{"name":"light-from-within-chroma","cr":["202.2","613.4"],"why":"inline","setup":{"p0":{"battlefield":["Light from Within","Savannah Lions","Grizzly Bears"]}},"steps":[],"expect":[{"card":"p0:Savannah Lions","zone":"battlefield","pt":"3/2"},{"card":"p0:Grizzly Bears","zone":"battlefield","pt":"2/2"}]}`

func TestLightFromWithinChromaSourceStatic(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(lightFromWithinChromaScenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
}

// Fiery Bombardment's "{2}, Sacrifice a creature: deals damage equal to the
// number of red mana symbols in the sacrificed creature's mana cost" reads
// Sacrificed$ChromaSource.Red off the LKI capture: Raging Goblin ("R") is one
// red symbol, so p1 takes exactly 1. Before the evaluator the damage was 0 --
// and before the ManaCost capture it would still be 0 after the sacrifice's
// zone change.
const fieryBombardmentChromaScenario = `{"name":"fiery-bombardment-chroma","cr":["202.2","608.2h"],"why":"inline","setup":{"p0":{"battlefield":["Fiery Bombardment","Raging Goblin"],"library_top":["Island"]}},"steps":[{"op":"activate","seat":0,"card":"p0:Fiery Bombardment","mana":"CC","targets":["p1"],"answers":[{"kind":"choose","pick":["Raging Goblin"]}]},{"op":"resolve"}],"expect":[{"life":{"p1":19}},{"card":"p0:Raging Goblin","zone":"graveyard"}]}`

func TestFieryBombardmentChromaSourceSacrifice(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(fieryBombardmentChromaScenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
}
