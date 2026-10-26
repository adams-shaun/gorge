package rules

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func mustPermanentBaseScenario(t *testing.T, raw string) oracleScenario {
	t.Helper()
	var sc oracleScenario
	if err := json.Unmarshal([]byte(raw), &sc); err != nil {
		t.Fatal(err)
	}
	return sc
}

// runPermanentBaseScenario decodes raw and runs it as an oracle scenario.
func runPermanentBaseScenario(t *testing.T, raw string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc := mustPermanentBaseScenario(t, raw)
	if fails, tr, _ := runOracleScenario(reg, sc); len(fails) > 0 {
		t.Fatalf("%s\n  transcript:\n    %s", strings.Join(fails, "\n"), strings.Join(tr, "\n    "))
	}
}

// TestDiscardValidPermanentBaseMatchesHandCards pins CR 110.4a on the Discard
// reader: Solve for Disappointment's DiscardValid$ Permanent.nonLand reads the
// opponent's hand, where a bare Permanent base is the permanent-card type
// test. Only the Bears are offered (Forest is a land, Lightning Bolt is not a
// permanent card), and they are discarded.
func TestDiscardValidPermanentBaseMatchesHandCards(t *testing.T) {
	t.Parallel()
	runPermanentBaseScenario(t, solveScenario("Grizzly Bears"))
}

// TestKellanCastsPermanentFromHand pins the Play reader: Kellan, the Kid's
// Valid$ Permanent.YouOwn+cmcLEX with ValidZone$ Hand offers the Bears (mana
// value 2, the flashed-back Think Twice's value) and casts them for free.
func TestKellanCastsPermanentFromHand(t *testing.T) {
	t.Parallel()
	runPermanentBaseScenario(t, kellanScenario("Grizzly Bears"))
}

// TestMoonshadowRemovesCounterOnMilledPermanentCard pins the ChangesZoneAll
// trigger's live branch: Moonshadow (Origin$ Any) sees a permanent card put
// into its owner's graveyard from the library or the hand, and not an instant.
func TestMoonshadowRemovesCounterOnMilledPermanentCard(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, card, from string
		want             int32
	}{
		{"milled creature card", "Grizzly Bears", "library", 5},
		{"milled instant card", "Lightning Bolt", "library", 6},
		{"discarded creature card", "Grizzly Bears", "hand", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"name":"moonshadow","setup":{"p0":{"battlefield":["Moonshadow"],"` + tc.from + `":["` + tc.card + `"]}},
			 "steps":[{"op":"move","card":"p0:` + tc.card + `","to":"graveyard"},{"op":"resolve"}],
			 "expect":[{"card":"p0:Moonshadow","counters":{"M1M1":` + string(rune('0'+tc.want)) + `}}]}`
			runPermanentBaseScenario(t, raw)
		})
	}
}

// solveScenario is Solve for Disappointment against p1's hand {Grizzly Bears,
// Forest, Lightning Bolt}, the caster naming pick; the Bears must be the card
// discarded.
func solveScenario(pick string) string {
	return `{"name":"solve","setup":{"p0":{"hand":["Solve for Disappointment"]},
	 "p1":{"hand":["Grizzly Bears","Forest","Lightning Bolt"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Solve for Disappointment","mana":"CB","targets":["p1"]},
	  {"op":"resolve","answers":[{"kind":"any","pick":["p1:` + pick + `"]}]}],
	 "expect":[{"card":"p1:Grizzly Bears","zone":"graveyard"},{"card":"p1:Forest","zone":"hand"},{"card":"p1:Lightning Bolt","zone":"hand"}]}`
}

// kellanScenario is Kellan, the Kid with a flashed-back Think Twice (mana
// value 2), the caster naming pick from a hand of {Grizzly Bears, Lightning
// Bolt}; the Bears must land on the battlefield.
func kellanScenario(pick string) string {
	return `{"name":"kellan","setup":{"p0":{"battlefield":["Kellan, the Kid"],"graveyard":["Think Twice"],"hand":["Grizzly Bears","Lightning Bolt"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Think Twice","cast_mode":"flashback","mana":"CCU"},
	  {"op":"pass","seat":0},
	  {"op":"resolve","answers":[{"kind":"any","pick":["yes"]},{"kind":"any","pick":["p0:` + pick + `"]}]}],
	 "expect":[{"card":"p0:Grizzly Bears","zone":"battlefield"},{"card":"p0:Lightning Bolt","zone":"hand"}]}`
}

// collectorScenario is Resourceful Collector's end-step ChooseCard over a
// graveyard of {Grizzly Bears, Lightning Bolt}: the random nonland... permanent
// card is the Bears, which becomes a Food; the Bolt does not.
const collectorScenario = `{"name":"collector","setup":{"p0":{"battlefield":["Resourceful Collector"],"graveyard":["Grizzly Bears","Lightning Bolt"]}},
	 "steps":[{"op":"pass_to","step":"end","active":"p0"},{"op":"resolve"}],
	 "expect":[{"card":"p0:Grizzly Bears","types":["Food"]},{"card":"p0:Lightning Bolt","no_types":["Food"]}]}`

// kamiScenario is Kami of Transmutation's AnimateAll over a hand of {Grizzly
// Bears, Lightning Bolt}: the Bears become an artifact, the Bolt does not.
const kamiScenario = `{"name":"kami","setup":{"p0":{"hand":["Kami of Transmutation","Grizzly Bears","Lightning Bolt"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Kami of Transmutation","mana":"CW"},{"op":"resolve"},
	  {"op":"resolve"}],
	 "expect":[{"card":"p0:Grizzly Bears","types":["Artifact"]},{"card":"p0:Lightning Bolt","no_types":["Artifact"]}]}`

// grolnokScenario is Grolnok, the Omnivore's ChangesZone trigger (Origin$
// Library, Destination$ Graveyard): a Bears put into the graveyard from the
// library is exiled, a Lightning Bolt stays.
const grolnokScenario = `{"name":"grolnok","setup":{"p0":{"battlefield":["Grolnok, the Omnivore"],"library":["Grizzly Bears","Lightning Bolt"]}},
	 "steps":[{"op":"move","card":"p0:Grizzly Bears","to":"graveyard"},{"op":"resolve"},
	  {"op":"move","card":"p0:Lightning Bolt","to":"graveyard"}],
	 "expect":[{"card":"p0:Grizzly Bears","zone":"exile"},{"card":"p0:Lightning Bolt","zone":"graveyard"}]}`

// moonshadowScenario is Moonshadow's ChangesZoneAll trigger (Origin$ Any): a
// Bears milled from the library removes a -1/-1 counter, a Lightning Bolt
// removes none.
const moonshadowScenario = `{"name":"moonshadow","setup":{"p0":{"battlefield":["Moonshadow"],"library":["Grizzly Bears","Lightning Bolt"]}},
	 "steps":[{"op":"move","card":"p0:Grizzly Bears","to":"graveyard"},{"op":"resolve"},
	  {"op":"move","card":"p0:Lightning Bolt","to":"graveyard"}],
	 "expect":[{"card":"p0:Moonshadow","counters":{"M1M1":5}}]}`
