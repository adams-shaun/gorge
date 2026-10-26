package rules

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runRememberScenario runs one oracle-scenario JSON against the corpus and
// fails the test with its transcript on any expectation mismatch.
func runRememberScenario(t *testing.T, raw string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var sc oracleScenario
	if err := json.Unmarshal([]byte(raw), &sc); err != nil {
		t.Fatal(err)
	}
	if fails, tr, _ := runOracleScenario(reg, sc); len(fails) > 0 {
		t.Fatalf("%s\n  transcript:\n    %s", strings.Join(fails, "\n"), strings.Join(tr, "\n    "))
	}
}

// TestSurveilRememberMovedReturnsCard pins RememberMoved$ (CR 607.2a):
// Enlightened Confidant's surveil remembers the card it binned, so the
// linked ChangeZoneAll returns it to hand when its mana value is at most the
// life gained this turn, and leaves a costlier one in the graveyard.
func TestSurveilRememberMovedReturnsCard(t *testing.T) {
	t.Parallel()
	const tmpl = `{"name":"confidant","setup":{"p0":{"battlefield":["Enlightened Confidant"],"hand":["Lightning Helix"],"library_top":["%s"],"library":["Forest","Forest"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Lightning Helix","mana":"RW","targets":["p1"]},{"op":"resolve"},
	  {"op":"pass_to","seat":0,"step":"end","active":"p0"},
	  {"op":"resolve","answers":[{"kind":"arrange","pick":[]}]}],
	 "expect":[%s]}`
	t.Run("cheap card returns to hand", func(t *testing.T) {
		raw := strings.Replace(strings.Replace(tmpl, "%s", "Grizzly Bears", 1), "%s",
			`{"card":"p0:Grizzly Bears","zone":"hand"},{"life":{"p0":23}}`, 1)
		runRememberScenario(t, raw)
	})
	t.Run("costly card stays in graveyard", func(t *testing.T) {
		raw := strings.Replace(strings.Replace(tmpl, "%s", "Craw Wurm", 1), "%s",
			`{"card":"p0:Craw Wurm","zone":"graveyard"}`, 1)
		runRememberScenario(t, raw)
	})
}

// TestManaSculptWizardAddsSpentMana pins RememberForCounter$: the countered
// spell is the delayed trigger's remembered object, so with a Wizard the
// next main phase adds {C} equal to the mana spent on it (4 for Hill Giant).
func TestManaSculptWizardAddsSpentMana(t *testing.T) {
	t.Parallel()
	runRememberScenario(t, `{"name":"mana-sculpt","setup":{"p0":{"battlefield":["Prodigal Sorcerer"],"hand":["Hill Giant","Mana Sculpt"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Hill Giant","mana":"RRRR"},
	  {"op":"cast","seat":0,"card":"p0:Mana Sculpt","mana":"UUU","targets":["p0:Hill Giant"]},{"op":"resolve"},
	  {"op":"pass_to","seat":0,"step":"main2","active":"p0","expect":[{"stack_size":1}]},{"op":"resolve"}],
	 "expect":[{"card":"p0:Hill Giant","zone":"graveyard"},{"pool":{"p0":"CCCC"}}]}`)
}

// TestFranticScapegoatPassesSuspicion pins RememberAltered$: the creature
// that was newly suspected is remembered, so the follow-up un-suspects
// Frantic Scapegoat itself.
func TestFranticScapegoatPassesSuspicion(t *testing.T) {
	t.Parallel()
	runRememberScenario(t, `{"name":"scapegoat","setup":{"p0":{"hand":["Frantic Scapegoat","Grizzly Bears"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Frantic Scapegoat","mana":"R"},{"op":"resolve"},
	  {"op":"cast","seat":0,"card":"p0:Grizzly Bears","mana":"GG"},{"op":"resolve","answers":[{"kind":"choose","pick":["p0:Grizzly Bears"]}]}],
	 "expect":[{"card":"p0:Grizzly Bears","keywords":["Menace"]},{"card":"p0:Frantic Scapegoat","no_keywords":["Menace"]}]}`)
}

// TestWinnowingClearsRememberedPerPlayer pins ClearRemembered$ on the card
// RepeatEach: it is a read key (no "unread parameter" Note) and each
// player's loop body compares against that player's own chosen creature
// only. p0's choice (Grizzly Bears) shares a type with p1's Runeclaw Bear,
// but p1's own choice (Hill Giant) does not, so the iteration imprints the
// Runeclaw Bear for the sacrifice; p0's Hill Giant is imprinted likewise
// although p1 chose a Hill Giant. The imprinted creatures are asserted from
// the Imprint events, and SacrificeAll ValidCards$ Creature.IsImprinted
// consumes them: they end in the graveyard while the chosen ones stay.
func TestWinnowingClearsRememberedPerPlayer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	var sc oracleScenario
	if err := json.Unmarshal([]byte(`{"name":"winnowing","setup":{"p0":{"battlefield":["Grizzly Bears","Hill Giant"],"hand":["Winnowing"]},"p1":{"battlefield":["Runeclaw Bear","Hill Giant"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Winnowing","mana":"WWWWWW"},
	  {"op":"resolve","answers":[{"kind":"choose","pick":["p0:Grizzly Bears"]},{"kind":"choose","pick":["p1:Hill Giant"]}]}],
	 "expect":[{"card":"p0:Grizzly Bears","zone":"battlefield"},{"card":"p0:Hill Giant","zone":"graveyard"},
	  {"card":"p1:Hill Giant","zone":"battlefield"},{"card":"p1:Runeclaw Bear","zone":"graveyard"}]}`), &sc); err != nil {
		t.Fatal(err)
	}
	fails, tr, r := runOracleScenario(reg, sc)
	if len(fails) > 0 {
		t.Fatalf("%s\n  transcript:\n    %s", strings.Join(fails, "\n"), strings.Join(tr, "\n    "))
	}
	var imprinted [][]state.ObjID
	for _, ev := range r.e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unread parameter") {
			t.Errorf("RepeatEach left a parameter unread: %q", ev.Text)
		}
		if ev.Kind == events.Imprint && len(ev.IDs) > 0 {
			imprinted = append(imprinted, ev.IDs)
		}
	}
	want := [][]state.ObjID{{r.refs["p0:Hill Giant"]}, {r.refs["p1:Runeclaw Bear"]}}
	if want[0][0] == 0 || want[1][0] == 0 || !reflect.DeepEqual(imprinted, want) {
		t.Errorf("imprinted per iteration = %v, want %v (p0:Hill Giant, then p1:Runeclaw Bear)", imprinted, want)
	}
}

// TestAzogIndestructibleStillAmasses pins AlwaysRemember$ (CR 607.2a): the
// destroy of an indestructible creature does nothing, but the target is
// still the linked object, so its controller still amasses by its power.
func TestAzogIndestructibleStillAmasses(t *testing.T) {
	t.Parallel()
	runRememberScenario(t, `{"name":"azog","setup":{"p0":{"hand":["Azog, Moria's Ruin"]},"p1":{"battlefield":["Darksteel Colossus"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Azog, Moria's Ruin","mana":"BBB"},{"op":"resolve","answers":[{"kind":"target","pick":["p1:Darksteel Colossus"]}]}],
	 "expect":[{"card":"p1:Darksteel Colossus","zone":"battlefield"},{"count":{"seat":1,"zone":"battlefield","name":"Goblin Army","token":true},"eq":1}]}`)
}

// TestKellanDrawsWhenTargetSurvives pins AlwaysRemember$ on Kellan,
// Inquisitive Prodigy: "if you controlled that permanent, draw a card" does
// not depend on the destroy succeeding, so your own indestructible artifact
// still draws.
func TestKellanDrawsWhenTargetSurvives(t *testing.T) {
	t.Parallel()
	runRememberScenario(t, `{"name":"kellan","setup":{"p0":{"battlefield":["Kellan, Inquisitive Prodigy","Darksteel Ingot"]}},
	 "steps":[{"op":"attack","seat":0,"attackers":["p0:Kellan, Inquisitive Prodigy"],"defender":"p1","answers":[{"kind":"target","pick":["p0:Darksteel Ingot"]}]},
	  {"op":"resolve"}],
	 "expect":[{"card":"p0:Darksteel Ingot","zone":"battlefield"},{"hand_size":{"p0":1}}]}`)
}

// TestIllTimedExplosionDamageIsGreatestDiscardedMV pins RememberDiscarded$ on
// the unless-discard: the reflexive trigger's remembered set is the discarded
// cards, so X is the greatest mana value among them (6), and declining the
// discard deals no damage.
func TestIllTimedExplosionDamageIsGreatestDiscardedMV(t *testing.T) {
	t.Parallel()
	const tmpl = `{"name":"explosion","setup":{"p0":{"hand":["Ill-Timed Explosion","Craw Wurm","Llanowar Elves"],"library":["Forest","Forest"]},"p1":{"battlefield":["Hill Giant"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Ill-Timed Explosion","mana":"UUUR"},
	  {"op":"resolve","answers":[%s]}],
	 "expect":[{"card":"p1:Hill Giant","zone":"%s"}]}`
	t.Run("discard two", func(t *testing.T) {
		runRememberScenario(t, strings.Replace(strings.Replace(tmpl, "%s",
			`{"kind":"modes","pick":["yes"]},{"kind":"choose","pick":["p0:Craw Wurm","p0:Llanowar Elves"]}`, 1), "%s", "graveyard", 1))
	})
	t.Run("decline", func(t *testing.T) {
		runRememberScenario(t, strings.Replace(strings.Replace(tmpl, "%s",
			`{"kind":"modes","pick":["no"]}`, 1), "%s", "battlefield", 1))
	})
}
