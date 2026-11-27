package rules

// altcost_xgrant_scope_test.go pins the battlefield-only, controller-relative
// scope of Kentaro, the Smiling Cat's XAlternative$ Count$CardManaCost grant
// (ValidSA$ Spell.Samurai | ValidPlayer$ You | Cost$ X): the no-EffectZone$
// static is a battlefield continuous effect (CR 113.6) granting Samurai
// spells its CONTROLLER casts, never its own card's cast from hand and never
// an opponent's Samurai.

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestAltCostXGrantOnlyOnTheControllersOtherSpells(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	run := func(sc string) []string {
		t.Helper()
		res, err := RunOracleScenarioJSON(reg, []byte(sc))
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return res.Fails
	}
	offered := func(seat int, card string, want bool) string {
		w := "false"
		if want {
			w = "true"
		}
		s := strconv.Itoa(seat)
		return `{"offered":{"seat":` + s + `,"kind":"cast","card":"p` + s + `:` + card +
			`","label":"(alternative cost)"},"want":` + w + `}`
	}
	// Kentaro in hand: its static is a battlefield grant to other spells, not
	// a self-carried alternative, so it must not pay for Kentaro's own cast
	// (X is Kentaro's mana value, 2, which the RR pool would pay).
	inHand := `{"name":"xgrant-hand","cr":["113.6","118.9"],"why":"inline",` +
		`"setup":{"p0":{"hand":["Kentaro, the Smiling Cat"],"mana":"RR"}},` +
		`"steps":[{"op":"mana","seat":0,"mana":"R","expect":[` +
		offered(0, "Kentaro, the Smiling Cat", false) + `]}]}`
	if fails := run(inHand); len(fails) != 0 {
		t.Errorf("Kentaro in hand granted its own alternative cast: %v", fails)
	}
	// Precondition: the same static on the battlefield does grant the
	// controller's Samurai spell (Hand of Honor, mana value 2, paid by RR),
	// so the not-offered assertion above is about the battlefield-only guard
	// and not a grant that never fires.
	onBattlefield := `{"name":"xgrant-battlefield","cr":["113.6","118.9"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Kentaro, the Smiling Cat"],"hand":["Hand of Honor"],"mana":"RR"}},` +
		`"steps":[{"op":"mana","seat":0,"mana":"R","expect":[` +
		offered(0, "Hand of Honor", true) + `]}]}`
	if fails := run(onBattlefield); len(fails) != 0 {
		t.Errorf("Kentaro on the battlefield did not grant the controller's Samurai spell: %v", fails)
	}
	// ValidPlayer$ You: Kentaro under p0 at turn 2 (p1's turn) must not grant
	// p1's Samurai spell even though p1's RR pool pays the {X}.
	opponentsSpell := `{"name":"xgrant-opponent","turn":2,"cr":["113.6","118.9"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Kentaro, the Smiling Cat"]},` +
		`"p1":{"hand":["Hand of Honor"],"mana":"RR"}},` +
		`"steps":[{"op":"mana","seat":1,"mana":"R","expect":[` +
		offered(1, "Hand of Honor", false) + `]}]}`
	if fails := run(opponentsSpell); len(fails) != 0 {
		t.Errorf("an opponent's Samurai spell took Kentaro's alternative: %v", fails)
	}
}
