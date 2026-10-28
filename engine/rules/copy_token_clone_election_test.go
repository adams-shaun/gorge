package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 614.12, 707.2: a token copy of a clone-like CARD (not of a clone
// permanent) has the card's "you may have this enter as a copy" replacement,
// so it makes the election and enters as the chosen copy. The election was
// posed and recorded on the token, but the token's own Clone line was never
// collected, so the body that applies it never ran.

// mimicVatCloneScenario: Clone copies p1's Silvercoat Lion, dies, is imprinted
// on Mimic Vat, then the Vat copies the exiled Clone card; lionPick answers
// the token's election.
func mimicVatCloneScenario(lionPick string) string {
	return `{"name":"mimic-vat-clone","cr":["614.12","707.2"],"why":"cr-g274","setup":{` +
		`"p0":{"hand":["Clone"],"battlefield":["Mimic Vat"]},"p1":{"battlefield":["Silvercoat Lion"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Clone","mana":"CCCU"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Silvercoat Lion"]}],"expect":[` +
		`{"count":{"seat":0,"zone":"battlefield","name":"Silvercoat Lion"},"eq":1}]},` +
		`{"op":"move","seat":0,"card":"p0:Silvercoat Lion","to":"graveyard"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"trigger_optional","pick":["yes"]}],"expect":[` +
		`{"count":{"seat":0,"zone":"exile","name":"Clone"},"eq":1},` +
		`{"count":{"seat":0,"zone":"battlefield","name":"Silvercoat Lion"},"eq":0}]},` +
		`{"op":"activate","seat":0,"card":"p0:Mimic Vat","mana":"CCC","ability_index":0},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p0:Clone"]},` +
		`{"kind":"choose","pick":["` + lionPick + `"]}]}],"expect":[` +
		`{"count":{"seat":0,"zone":"exile","name":"Clone"},"eq":1},`
}

func countClonePermanent(run *oracleRun) int {
	n := 0
	for _, ev := range run.e.L.Events {
		if ev.Kind == events.ClonePermanent {
			n++
		}
	}
	return n
}

func runCloneElection(t *testing.T, raw string) (*oracleRun, string) {
	t.Helper()
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	fails, tr, run := runOracleScenario(testutil.CorpusRegistry(t), sc)
	joined := strings.Join(tr, "\n")
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, joined)
	}
	return run, joined
}

// TestTokenCopyOfExiledCloneAppliesElection: the Vat's token copy of the
// exiled Clone card elects Silvercoat Lion and enters as a 2/2 Lion token;
// the Clone card stays in exile and one ClonePermanent is logged for it.
func TestTokenCopyOfExiledCloneAppliesElection(t *testing.T) {
	t.Parallel()
	run, _ := runCloneElection(t, mimicVatCloneScenario("p1:Silvercoat Lion")+
		`{"card":"p0:token:Silvercoat Lion","zone":"battlefield","pt":"2/2"}]}`)
	// The cast Clone's own copy is the first ClonePermanent; the token's is the second.
	if n := countClonePermanent(run); n != 2 {
		t.Fatalf("ClonePermanent events = %d, want 2 (cast + token)", n)
	}
}

// TestTokenCopyOfCloneCardDeclineEntersAsItself: declining the token's
// election enters it as the printed 0/0 Clone, which dies to SBA.
func TestTokenCopyOfCloneCardDeclineEntersAsItself(t *testing.T) {
	t.Parallel()
	run, _ := runCloneElection(t, mimicVatCloneScenario("Enter as itself")+
		`{"count":{"seat":0,"zone":"battlefield","name":"Silvercoat Lion"},"eq":0},`+
		`{"count":{"seat":0,"zone":"battlefield","name":"Clone"},"eq":0}]}`)
	if n := countClonePermanent(run); n != 1 {
		t.Fatalf("ClonePermanent events = %d, want 1 (the cast only)", n)
	}
}

// TestNahiriTokenCopyOfVisageBanditAppliesElection: Nahiri's [0] exiles Visage
// Bandit from the graveyard and copies it; the token elects p0's Silvercoat
// Lion and keeps Visage Bandit's Shapeshifter Rogue exception.
func TestNahiriTokenCopyOfVisageBanditAppliesElection(t *testing.T) {
	t.Parallel()
	raw := `{"name":"nahiri-visage","cr":["614.12"],"why":"cr-g274","setup":{` +
		`"p0":{"graveyard":["Visage Bandit"],"battlefield":["Nahiri, the Unforgiving","Silvercoat Lion"]}},` +
		`"steps":[{"op":"activate","seat":0,"card":"p0:Nahiri, the Unforgiving","ability_index":2,"targets":["p0:Visage Bandit"]},` +
		`{"op":"resolve","seat":0}],"expect":[` +
		`{"count":{"seat":0,"zone":"battlefield","name":"Silvercoat Lion"},"eq":2},` +
		`{"card":"p0:token:Silvercoat Lion","zone":"battlefield","pt":"2/2","types":["Creature","Shapeshifter","Rogue"]}]}`
	run, _ := runCloneElection(t, raw)
	if n := countClonePermanent(run); n != 1 {
		t.Fatalf("ClonePermanent events = %d, want 1", n)
	}
}
