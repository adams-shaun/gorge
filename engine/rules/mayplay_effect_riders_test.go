package rules

// mayplay_effect_riders_test.go pins the two riders an Effect-delivered
// may-play grant shares with the printed route (cr-g219): MayPlayAltManaCost$
// (Inside Information's "pay life equal to its mana value rather than pay its
// mana cost") and a grant over the top card of the library with its
// MayLookAt$ look (The Belligerent, Gwenom, Remorseless). Every other unread
// key on an Effect grant (RaiseCost$) keeps the loud Note and offers nothing.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

func countNotes(run *oracleRun, sub string) int {
	n := 0
	for _, ev := range run.e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, sub) {
			n++
		}
	}
	return n
}

// TestInsideInformationPaysLifeForExiledSpell: Inside Information with X = 2
// exiles Doom Blade and a Forest from p1's library; the exiled instant is
// offered ONLY at the life payment (no mana floats), costs 2 life and
// resolves; the exiled land is playable through the same Effect grant.
func TestInsideInformationPaysLifeForExiledSpell(t *testing.T) {
	run := runInlineOracle(t, `{"name":"inside-information-altcost","cr":["118.9","601.2b"],"why":"inline",
	"setup":{"p0":{"hand":["Inside Information"]},"p1":{"battlefield":["Grizzly Bears"],"library_top":["Doom Blade","Forest"]}},
	"steps":[
	 {"op":"cast","seat":0,"card":"p0:Inside Information","mana":"BBBB","targets":["p1"],
	  "answers":[{"kind":"choose","pick":["X = 2"]}]},
	 {"op":"resolve"},
	 {"op":"pass_to","seat":0,"decision":"priority","expect":[
	   {"card":"p1:Doom Blade","zone":"exile"},
	   {"offered":{"seat":0,"kind":"cast","card":"p1:Doom Blade","label":"alternative cost"},"want":true},
	   {"offered":{"seat":0,"kind":"play","card":"p1:Forest"},"want":true},
	   {"life":{"p0":20,"p1":20}}]},
	 {"op":"cast","seat":0,"card":"p1:Doom Blade","targets":["p1:Grizzly Bears"]},
	 {"op":"resolve"},
	 {"op":"pass_to","seat":0,"decision":"priority","expect":[
	   {"life":{"p0":18,"p1":20}},
	   {"card":"p1:Grizzly Bears","zone":"graveyard"}]}]}`)
	if n := countNotes(run, "continuous effect Continuous unimplemented"); n != 0 {
		t.Fatalf("the Effect grant fell to the unimplemented Note %d time(s)", n)
	}
}

// TestBelligerentPlaysTopOfLibrary: The Belligerent's attack trigger lets p0
// look at the top card of their library and play lands and cast spells from
// it this turn. The top land is offered as play_land and the look reads true
// this turn; on the next turn (after the Forest is drawn, Mountain is the
// top) neither holds (the Effect ended at cleanup).
func TestBelligerentPlaysTopOfLibrary(t *testing.T) {
	runInlineOracle(t, `{"name":"belligerent-top","cr":["401.5","305.1"],"why":"inline",
	"setup":{"p0":{"battlefield":["The Belligerent","Grizzly Bears","Grizzly Bears"],"library_top":["Forest","Mountain"]},"p1":{}},
	"steps":[
	 {"op":"pass_to","seat":0,"step":"main1","decision":"priority","expect":[
	   {"look_at_library_top":{"p0":false}},
	   {"offered":{"seat":0,"kind":"play","card":"p0:Forest"},"want":false}]},
	 {"op":"activate","seat":0,"card":"p0:The Belligerent","ability":"Crew 3"},
	 {"op":"attack","seat":0,"defender":"p1","attackers":["p0:The Belligerent"]},
	 {"op":"resolve"},
	 {"op":"pass_to","seat":0,"step":"main2","active":"p0","expect":[
	   {"look_at_library_top":{"p0":true}},
	   {"offered":{"seat":0,"kind":"play","card":"p0:Forest"},"want":true}]},
	 {"op":"pass_to","seat":0,"step":"main1","active":"p0","expect":[
	   {"look_at_library_top":{"p0":false}},
	   {"offered":{"seat":0,"kind":"play","card":"p0:Mountain"},"want":false}]}]}`)
}

// TestGwenomTopCardCostsLife: Gwenom's attack trigger lets p0 cast spells from
// the top of their library this turn, paying life equal to the spell's mana
// value instead of its mana cost: the top Doom Blade (mana value 2) is offered
// at the alternative cost only, costs 2 life and no mana.
func TestGwenomTopCardCostsLife(t *testing.T) {
	runInlineOracle(t, `{"name":"gwenom-top","cr":["118.9","401.5"],"why":"inline",
	"setup":{"p0":{"battlefield":["Gwenom, Remorseless"],"library_top":["Doom Blade","Forest"]},"p1":{"battlefield":["Grizzly Bears"]}},
	"steps":[
	 {"op":"attack","seat":0,"defender":"p1","attackers":["p0:Gwenom, Remorseless"]},
	 {"op":"resolve"},
	 {"op":"pass_to","seat":0,"decision":"priority","expect":[
	   {"look_at_library_top":{"p0":true}},
	   {"offered":{"seat":0,"kind":"cast","card":"p0:Doom Blade","label":"alternative cost"},"want":true},
	   {"life":{"p0":20}}]},
	 {"op":"cast","seat":0,"card":"p0:Doom Blade","targets":["p1:Grizzly Bears"]},
	 {"op":"resolve"},
	 {"op":"pass_to","seat":0,"decision":"priority","expect":[
	   {"life":{"p0":18}},
	   {"card":"p1:Grizzly Bears","zone":"graveyard"}]}]}`)
}

// TestEffectGrantUnreadRiderStillRefused: an Effect-delivered grant carrying a
// rider this build does not read (Osteomancer Adept's RaiseCost$ Forage) keeps
// the loud Note and offers nothing -- admitting the alternative-cost and
// library-top keys widened nothing else.
func TestEffectGrantUnreadRiderStillRefused(t *testing.T) {
	run := runInlineOracle(t, `{"name":"effect-grant-raisecost-refused","cr":["601.2f"],"why":"inline",
	"setup":{"p0":{"battlefield":["Osteomancer Adept"],"graveyard":["Grizzly Bears"]},"p1":{}},
	"steps":[
	 {"op":"activate","seat":0,"card":"p0:Osteomancer Adept","ability_index":0},
	 {"op":"resolve"},
	 {"op":"pass_to","seat":0,"decision":"priority","expect":[
	   {"offered":{"seat":0,"kind":"cast","card":"p0:Grizzly Bears"},"want":false}]}]}`)
	if n := countNotes(run, "continuous effect Continuous unimplemented"); n == 0 {
		t.Fatal("the RaiseCost$ Effect grant did not hit the unimplemented Note: precondition for the refusal check")
	}
}

// TestHamaWaterbendCastsExiledCard: Hama, the Bloodbender's enter trigger mills
// p1 and exiles a noncreature nonland card from their graveyard; for as long as
// p0 controls Hama, on p0's turn the exiled Shock is offered at its waterbend
// alternative cost (Waterbend<ConvertedManaCost>, priced by altCostParse), and
// only at it: no printed-cost cast is offered.
func TestHamaWaterbendCastsExiledCard(t *testing.T) {
	run := runInlineOracle(t, `{"name":"hama-waterbend","cr":["118.9","701.67"],"why":"inline",
	"setup":{"p0":{"hand":["Hama, the Bloodbender"]},"p1":{"library_top":["Shock","Grizzly Bears","Forest"]}},
	"steps":[
	 {"op":"cast","seat":0,"card":"p0:Hama, the Bloodbender","mana":"BBBBB","targets":[]},
	 {"op":"resolve","answers":[{"kind":"choose","pick":["Yes"]},{"kind":"choose","pick":["Shock"]}]},
	 {"op":"resolve"},
	 {"op":"mana","seat":0,"mana":"R"},
	 {"op":"pass_to","seat":0,"decision":"priority","expect":[
	   {"offered":{"seat":0,"kind":"cast","card":"p1:Shock","label":"alternative cost"},"want":true}]}]}`)
	if n := countNotes(run, "continuous effect Continuous unimplemented"); n != 0 {
		t.Fatalf("Hama's grant fell to the unimplemented Note %d time(s)", n)
	}
}
