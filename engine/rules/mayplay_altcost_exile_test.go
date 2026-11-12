package rules

// mayplay_altcost_exile_test.go pins the may-play alternative-cost class
// (CR 118.9 / 601.2b,f): a MayPlay static that carries
// MayPlayAltManaCost$ either OPENS the zone permission and is that
// permission's only price (Valgavoth, Terror Eater: "During your turn, you
// may play cards exiled with Valgavoth. If you cast a spell this way, pay
// life equal to its mana value rather than pay its mana cost"), or reprices
// a cast some other permission already makes (Warped Space: "Once each turn,
// you may pay {0} rather than pay the mana cost for a spell you cast from
// exile"). Both are observed as the offered cast and the price it charges.

import (
	"strings"
	"testing"
)

// valgavothScenario has p0 control Valgavoth and Shock p1's Grizzly Bears:
// Valgavoth's replacement exiles the dying card (a card p0 didn't control,
// going to an opponent's graveyard), so it sits in p1's exile "exiled with"
// Valgavoth. p1 also has an unrelated exiled card the setup placed. extra is
// appended after the expectation step.
func valgavothScenario(expect, extra string) string {
	return `{"name":"valgavoth-altcost","cr":["118.9","601.2b","601.2f"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Valgavoth, Terror Eater"],"hand":["Shock"]},` +
		`"p1":{"battlefield":["Grizzly Bears"],"exile":["Savannah Lions"]}},"steps":[` +
		`{"op":"cast","seat":0,"card":"p0:Shock","mana":"R","targets":["p1:Grizzly Bears"]},` +
		`{"op":"resolve"},` +
		`{"op":"mana","seat":0,"mana":"GG"},` +
		`{"op":"pass_to","seat":0,"decision":"priority","expect":[` + expect + `]}` + extra + `]}`
}

// offeredLabels lists the cast labels a snapshot offered for the named card.
func offeredCastLabels(s OracleSnapshot, card string) []string {
	var out []string
	for _, o := range s.Offered {
		if o.Kind == "cast" && strings.Contains(o.Label, card) {
			out = append(out, o.Label)
		}
	}
	return out
}

// TestValgavothCastsExiledWithCardForLifeOnly: the card Valgavoth exiled is
// offered ONLY at the life payment -- with {G}{G} floating the printed-cost
// cast would be affordable, yet it is not offered (CR 118.9: the permission's
// spell is cast "rather than pay its mana cost") -- and casting it costs 2
// life and no mana. The setup-placed exile card is not "exiled with"
// Valgavoth and is not offered at all.
func TestValgavothCastsExiledWithCardForLifeOnly(t *testing.T) {
	res := runInline(t, valgavothScenario(
		`{"offered":{"seat":0,"kind":"cast","card":"p1:Grizzly Bears","label":"alternative cost"},"want":true},`+
			`{"offered":{"seat":0,"kind":"cast","card":"p1:Savannah Lions"},"want":false}`,
		`,{"op":"cast","seat":0,"card":"p1:Grizzly Bears"},{"op":"resolve"},`+
			`{"op":"pass_to","seat":0,"decision":"priority","expect":[`+
			`{"life":{"p0":18,"p1":20}},{"pool":{"p0":"GG"}},`+
			`{"card":"p1:Grizzly Bears","zone":"battlefield"}]}`))
	var labels []string
	for _, s := range res.Snapshots {
		if l := offeredCastLabels(s, "Grizzly Bears"); len(l) > 0 {
			labels = l
			break
		}
	}
	if len(labels) != 1 || !strings.Contains(labels[0], "alternative cost") {
		t.Fatalf("Grizzly Bears cast offers = %q, want exactly the alternative-cost cast (no printed-cost offer)", labels)
	}
	final := res.Snapshots[len(res.Snapshots)-1]
	if bears, ok := snapPerm(final, "p1:Grizzly Bears"); !ok || bears.Controller != 0 || bears.Owner != 1 {
		t.Fatalf("cast Grizzly Bears = %+v (on battlefield %v), want controller 0, owner 1", bears, ok)
	}
}

// warpedSpaceScenario unlocks Warped Space by casting the door, with
// Misthollow Griffin (its own "You may cast this card from exile"
// permission) in p0's exile, then asserts the offers at p0's priority.
func warpedSpaceScenario(hand, expect, extra string) string {
	steps := `{"op":"pass_to","seat":0,"decision":"priority","expect":[` + expect + `]}` + extra
	if hand != "" {
		steps = `{"op":"cast","seat":0,"card":"p0:Warped Space","mana":"CCCCRR"},{"op":"resolve"},` + steps
	}
	return `{"name":"warped-space-altcost","cr":["118.9","601.2f"],"why":"inline",` +
		`"setup":{"p0":{"hand":[` + hand + `],"exile":["Misthollow Griffin"]},"p1":{}},"steps":[` + steps + `]}`
}

// TestWarpedSpacePaysZeroForExileCast: with Warped Space unlocked, the
// exile-castable Griffin is offered at {0} (cast_mode "alternative") with no
// mana floating, and resolves; without the door it is not castable at all.
func TestWarpedSpacePaysZeroForExileCast(t *testing.T) {
	runInline(t, warpedSpaceScenario(`"Charred Foyer // Warped Space"`,
		`{"offered":{"seat":0,"kind":"cast","card":"p0:Misthollow Griffin","label":"alternative cost"},"want":true}`,
		`,{"op":"cast","seat":0,"card":"p0:Misthollow Griffin","cast_mode":"alternative"},{"op":"resolve"},`+
			`{"op":"pass_to","seat":0,"decision":"priority","expect":[`+
			`{"card":"p0:Misthollow Griffin","zone":"battlefield"}]}`))
	runInline(t, warpedSpaceScenario("",
		`{"offered":{"seat":0,"kind":"cast","card":"p0:Misthollow Griffin"},"want":false}`, ""))
}
