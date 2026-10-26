package rules

// cast_flash_self_static_zone_test.go pins CR 113.6 for a card's own
// CastWithFlash static: without an EffectZone$ it is a battlefield static, so
// Radagast of Rhosgobel / Captain Mar-Vell grant flash to spells only while
// on the battlefield, never to themselves from hand; an `EffectZone$ All`
// "this spell" grant (Illusion Spinners, CR 113.6e) still works from hand.
//
// Offers are gated on the pool (the walk refuses an unaffordable cast), so
// each scenario seeds the pool with a "mana" op at p0's priority in p1's turn.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func runFlashZoneScenario(t *testing.T, raw string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
}

// flashZoneToP0Priority walks to p1's upkeep and has p1 pass, so p0 holds
// priority there with pool mana; expect is checked at that decision.
func flashZoneToP0Priority(mana, expect string) string {
	return `{"op":"pass_to","seat":0,"step":"upkeep","active":"p1"},{"op":"pass","seat":1},` +
		`{"op":"mana","seat":0,"mana":"` + mana + `","expect":[` + expect + `]}`
}

func flashZoneOffered(card string, want bool) string {
	w := "false"
	if want {
		w = "true"
	}
	return `{"offered":{"seat":0,"kind":"cast","card":"` + card + `"},"want":` + w + `}`
}

func TestCastWithFlashSelfStaticDefaultsToBattlefield(t *testing.T) {
	t.Run("in hand", func(t *testing.T) {
		runFlashZoneScenario(t, `{"name":"radagast-hand","cr":["113.6"],"why":"inline","setup":{"p0":{"hand":["Radagast of Rhosgobel","Grizzly Bears"]}},"steps":[`+
			flashZoneToP0Priority("GGGG", flashZoneOffered("p0:Radagast of Rhosgobel", false)+","+flashZoneOffered("p0:Grizzly Bears", false))+`]}`)
	})
	t.Run("on battlefield", func(t *testing.T) {
		runFlashZoneScenario(t, `{"name":"radagast-bf","cr":["113.6"],"why":"inline","setup":{"p0":{"battlefield":["Radagast of Rhosgobel"],"hand":["Grizzly Bears","Runeclaw Bear"]}},"steps":[`+
			flashZoneToP0Priority("GGGG", flashZoneOffered("p0:Grizzly Bears", true)+","+flashZoneOffered("p0:Runeclaw Bear", true))+`,`+
			`{"op":"cast","seat":0,"card":"p0:Grizzly Bears"},{"op":"resolve"},`+
			`{"op":"pass","seat":1,"expect":[`+flashZoneOffered("p0:Runeclaw Bear", false)+`]}]}`)
	})
}

// p1 casts Lightning Bolt in its own upkeep, so p0 holds priority with an
// opponent's spell cast this turn: the condition of Mar-Vell's static holds,
// yet only a Mar-Vell on the battlefield grants flash.
func TestCaptainMarVellInHandHasNoFlash(t *testing.T) {
	bolt := `{"op":"pass_to","seat":0,"step":"upkeep","active":"p1"},` +
		`{"op":"cast","seat":1,"card":"p1:Lightning Bolt","mana":"R","targets":["p0"]},{"op":"pass","seat":1},`
	t.Run("in hand", func(t *testing.T) {
		runFlashZoneScenario(t, `{"name":"marvell-hand","cr":["113.6"],"why":"inline","setup":{"p0":{"hand":["Captain Mar-Vell, Space-Born","Grizzly Bears"]},"p1":{"hand":["Lightning Bolt"]}},"steps":[`+
			bolt+`{"op":"mana","seat":0,"mana":"WWWWW","expect":[`+
			flashZoneOffered("p0:Captain Mar-Vell, Space-Born", false)+","+flashZoneOffered("p0:Grizzly Bears", false)+`]}]}`)
	})
	t.Run("on battlefield", func(t *testing.T) {
		runFlashZoneScenario(t, `{"name":"marvell-bf","cr":["113.6"],"why":"inline","setup":{"p0":{"battlefield":["Captain Mar-Vell, Space-Born"],"hand":["Grizzly Bears"]},"p1":{"hand":["Lightning Bolt"]}},"steps":[`+
			bolt+`{"op":"mana","seat":0,"mana":"GGGG","expect":[`+flashZoneOffered("p0:Grizzly Bears", true)+`]}]}`)
	})
}

func TestCastWithFlashEffectZoneAllSelfStillWorks(t *testing.T) {
	t.Run("with a Faerie", func(t *testing.T) {
		runFlashZoneScenario(t, `{"name":"spinners-faerie","cr":["113.6e"],"why":"inline","setup":{"p0":{"battlefield":["Faerie Miscreant"],"hand":["Illusion Spinners"]}},"steps":[`+
			flashZoneToP0Priority("UUUUU", flashZoneOffered("p0:Illusion Spinners", true))+`]}`)
	})
	t.Run("without a Faerie", func(t *testing.T) {
		runFlashZoneScenario(t, `{"name":"spinners-nofaerie","cr":["113.6e"],"why":"inline","setup":{"p0":{"hand":["Illusion Spinners"]}},"steps":[`+
			flashZoneToP0Priority("UUUUU", flashZoneOffered("p0:Illusion Spinners", false))+`]}`)
	})
}
