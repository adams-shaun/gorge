package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 614.1c / 122.6: every counter a permanent enters with is put by ONE
// event, whichever source supplies it (the moving effect's WithCountersType$,
// the card's own etbCounter, a lent Metallic Mimic rider, an Effect-created
// "enters with an additional counter" body), so a "that many plus one"
// AddCounter replacement (Hardened Scales, Kami of Whispered Hopes) applies
// once to the whole put.

func runEntryOneEvent(t *testing.T, raw string) (*oracleRun, []string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	return run, transcript
}

// counterEventsNaming counts the logged P1P1 CounterChange events on a
// battlefield object of the given name.
func counterEventsNaming(run *oracleRun, name string) int {
	n := 0
	for _, ev := range run.e.L.Events {
		if ev.Kind != events.CounterChange || ev.Counter != "P1P1" {
			continue
		}
		if o := run.e.G.Obj(ev.Obj); o != nil && o.Zone == state.ZBattlefield && o.Face() != nil && o.Face().Name == name {
			n++
		}
	}
	return n
}

// TestEntryCountersRequestedAndRiderAreOnePut: Vigor Mortis's requested
// counter and Metallic Mimic's lent rider are one put under Hardened Scales
// (1+1 requested, +1 rider, one +1 modifier = 3), logged as one event.
func TestEntryCountersRequestedAndRiderAreOnePut(t *testing.T) {
	t.Parallel()
	run, tr := runEntryOneEvent(t, `{"name":"x","cr":["614.1c"],"why":"x","setup":{"p0":{`+
		`"hand":["Metallic Mimic","Vigor Mortis"],"battlefield":["Hardened Scales","Forest","Forest","Swamp","Swamp"],`+
		`"graveyard":["Silvercoat Lion"]}},"steps":[{"op":"resolve","seat":0},`+
		`{"op":"cast","seat":0,"card":"p0:Metallic Mimic","mana":"GG"},`+
		`{"op":"pass_to","seat":0,"step":"upkeep","active":"p1","answers":[{"kind":"any","pick":["Cat"],"seat":0}]},`+
		`{"op":"pass_to","seat":0,"step":"upkeep","active":"p0"},{"op":"pass_to","seat":0,"step":"main1","active":"p0"},`+
		`{"op":"resolve","seat":0},`+
		`{"op":"cast","seat":0,"card":"p0:Vigor Mortis","mana":"GGBB","targets":["p0:Silvercoat Lion"]},`+
		`{"op":"pass_to","seat":0,"step":"begin-combat","active":"p0"},{"op":"resolve","seat":0}],`+
		`"expect":[{"count":{"seat":0,"zone":"battlefield","name":"Metallic Mimic"},"eq":1},`+
		`{"card":"p0:Silvercoat Lion","counters":{"P1P1":3}},{"card":"p0:Silvercoat Lion","pt":"5/5"}]}`)
	if got := counterEventsNaming(run, "Silvercoat Lion"); got != 1 {
		t.Fatalf("CounterChange P1P1 events on the Lion = %d, want exactly 1\n%s", got, strings.Join(tr, "\n"))
	}
}

const entryOneEventKami = `"Kami of Whispered Hopes"`

func recommissionScenario(extraBF, target string) string {
	return `{"name":"x","cr":["614.1c"],"why":"x","setup":{"p0":{"hand":["Recommission"],` +
		`"graveyard":["` + target + `"],"battlefield":[` + extraBF + `,"Plains","Plains"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Recommission","mana":"CW","targets":["p0:` + target + `"]},` +
		`{"op":"resolve","seat":0}],` +
		`"expect":[{"count":{"seat":0,"zone":"battlefield","name":"` + target + `"},"eq":1},` +
		`{"card":"p0:` + target + `","counters":{"P1P1":%d}}]}`
}

func recommissionWant(extraBF, target string, want string) string {
	return strings.Replace(recommissionScenario(extraBF, target), "%d", want, 1)
}

// TestEntryCountersEffectBorneBodyIsOnePut: Recommission's Effect-borne
// "additional +1/+1 counter" and the Drone's own etbCounter are one put under
// Kami: (1 + 1) + 1 = 3, not two puts of 2.
func TestEntryCountersEffectBorneBodyIsOnePut(t *testing.T) {
	t.Parallel()
	run, tr := runEntryOneEvent(t, recommissionWant(entryOneEventKami, "Dockworker Drone", "3"))
	if got := counterEventsNaming(run, "Dockworker Drone"); got != 1 {
		t.Fatalf("CounterChange P1P1 events on the Drone = %d, want exactly 1\n%s", got, strings.Join(tr, "\n"))
	}
}

// TestEntryCountersEffectBorneAndStaticRiderAreOnePut: Recommission's body
// and Thunderous Velocipede's static rider onto a counterless Lion are one
// put under Kami: 1 + 1 + 1 = 3.
func TestEntryCountersEffectBorneAndStaticRiderAreOnePut(t *testing.T) {
	t.Parallel()
	runEntryOneEvent(t, recommissionWant(entryOneEventKami+`,"Thunderous Velocipede"`, "Silvercoat Lion", "3"))
}

// TestEntryCountersDoublerUnchanged: a doubler is unaffected by the merge
// (2a + 2b = 2(a+b)): Doubling Season on Recommission's Drone is 2 x (1+1) = 4.
func TestEntryCountersDoublerUnchanged(t *testing.T) {
	t.Parallel()
	runEntryOneEvent(t, recommissionWant(`"Doubling Season"`, "Dockworker Drone", "4"))
}
