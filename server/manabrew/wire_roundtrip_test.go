//go:build manabrew

package manabrew

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// TestWireRoundTripThenTranslate covers the pointer/
// value mismatch: server/protocol/manabrew's
// promptOutputCases (and promptInputCases) construct pointers so
// json.Unmarshal has an addressable target, but every consumer in this
// package type-switches on the VALUE (out.(mb.PassOutput), not
// out.(*mb.PassOutput)). A test that builds a mb.ClientResponse by hand and
// calls TranslateResponse in-process never exercises Decode at all, so it
// never caught this; this test goes all the way through Encode -> Decode ->
// TranslateResponse, the same path a real ManaBrew client's bytes take.
func TestWireRoundTripThenTranslate(t *testing.T) {
	t.Run("pass/priority", func(t *testing.T) {
		tr := New("table", 2, nil)
		v := smallView()
		d := newDec(7, 1, decision.KPriority,
			decision.Option{Index: 0, Kind: "cast", Label: "Cast Shock", Obj: 2},
			decision.Option{Index: 1, Kind: "pass", Label: "Pass priority"},
			decision.Option{Index: 2, Kind: "concede", Label: "Concede"})
		p := pendingFor(d, v)

		client := mb.ClientMessage{Value: mb.ClientResponse{
			Kind:     "response",
			PromptID: p.Prompt.PromptID,
			Action: mb.PromptOutput{
				Type:   "chooseAction",
				Output: mb.PromptOutputData{Value: mb.PassOutput{ExhaustStack: true}},
			},
		}}

		wire, err := mb.Encode(client)
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		var decoded mb.ClientMessage
		if unknown, err := mb.Decode(wire, &decoded); err != nil {
			t.Fatalf("Decode: %v", err)
		} else if len(unknown) != 0 {
			t.Fatalf("Decode reported unknown paths for a well-formed message: %v", unknown)
		}

		resp, ok := decoded.Value.(mb.ClientResponse)
		if !ok {
			t.Fatalf("decoded.Value is %T, want mb.ClientResponse", decoded.Value)
		}
		if _, ok := resp.Action.Output.Value.(mb.PassOutput); !ok {
			t.Fatalf("decoded output.Value is %T, want mb.PassOutput (a value, not a pointer)", resp.Action.Output.Value)
		}

		outcome := tr.TranslateResponse(decoded, p, 1)
		if outcome.Err != nil {
			t.Fatalf("unexpected error %s: %s", outcome.Err.Code, outcome.Err.Message)
		}
		if outcome.Intent == nil {
			t.Fatal("expected a pass intent")
		}
		if outcome.Policy == nil || !outcome.Policy.ExhaustStack {
			t.Fatalf("expected an exhaustStack policy, got %#v", outcome.Policy)
		}
		idx := passOptionIndex(d)
		if len(outcome.Intent.Choices) != 1 || outcome.Intent.Choices[0] != idx {
			t.Fatalf("intent choices = %v, want [%d]", outcome.Intent.Choices, idx)
		}
	})

	t.Run("boardTargets", func(t *testing.T) {
		tr := New("table", 2, nil)
		v := smallView()
		d := &decision.Decision{Seq: 12, Player: 1, Kind: decision.KTarget, Min: 1, Max: 1, Prompt: "Choose a target",
			TargetEffect: &decision.TargetEffect{API: "Destroy"},
			Options: []decision.Option{
				{Index: 0, Kind: "permanent", Label: "Bear", Obj: 2},
				{Index: 1, Kind: "player", Label: "Alice", Player: 0},
			}}
		p := pendingFor(d, v)

		client := mb.ClientMessage{Value: mb.ClientResponse{
			Kind:     "response",
			PromptID: p.Prompt.PromptID,
			Action: mb.PromptOutput{
				Type: "chooseBoardTargets",
				Output: mb.PromptOutputData{Value: mb.BoardTargetsDecision{
					Chosen: []mb.TargetRef{{Kind: mb.RefCard, ID: cardID(2)}},
				}},
			},
		}}

		wire, err := mb.Encode(client)
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		var decoded mb.ClientMessage
		if _, err := mb.Decode(wire, &decoded); err != nil {
			t.Fatalf("Decode: %v", err)
		}

		resp, ok := decoded.Value.(mb.ClientResponse)
		if !ok {
			t.Fatalf("decoded.Value is %T, want mb.ClientResponse", decoded.Value)
		}
		if _, ok := resp.Action.Output.Value.(mb.BoardTargetsDecision); !ok {
			t.Fatalf("decoded output.Value is %T, want mb.BoardTargetsDecision (a value, not a pointer)", resp.Action.Output.Value)
		}

		outcome := tr.TranslateResponse(decoded, p, 1)
		if outcome.Err != nil {
			t.Fatalf("unexpected error %s: %s", outcome.Err.Code, outcome.Err.Message)
		}
		if outcome.Intent == nil {
			t.Fatal("expected a target intent")
		}
		if len(outcome.Intent.Choices) != 1 || outcome.Intent.Choices[0] != 0 {
			t.Fatalf("intent choices = %v, want [0] (the Bear)", outcome.Intent.Choices)
		}
	})
}

// The wire-shape goldens below pin each message gorge once sent in a shape
// ManaBrew's own protocol types refuse (the mbreal run: ManaBrew's real seat
// client, its prompts checked with the manabrew-protocol Rust types). gorge
// cannot import those types, so each expected shape is written by hand from
// the Rust source it cites (manabrew-rs/crates/manabrew-protocol/src/...),
// field for field: a required field is present even at its zero value, a
// list is never null, and an enum carries one of its declared values.

// wireValue encodes v the way the transport does and decodes it generically.
func wireValue(t *testing.T, v any) any {
	t.Helper()
	raw, err := mb.Encode(v)
	if err != nil {
		t.Fatalf("encode %T: %v", v, err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

// sameJSON fails unless got encodes to exactly the JSON value want spells.
func sameJSON(t *testing.T, what string, got any, want string) {
	t.Helper()
	var w any
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("%s: bad expectation: %v", what, err)
	}
	if g := wireValue(t, got); !reflect.DeepEqual(g, w) {
		gb, _ := json.Marshal(g)
		t.Fatalf("%s:\n got  %s\n want %s", what, gb, want)
	}
}

// F1, F6: prompts/common.rs AvailableAction = {id} + AvailableActionKind,
// internally tagged on type. cast{cardId, mode: PlayCardMode, label} with
// PlayCardMode itself tagged on type (normal, alternative{cost}, ...);
// activateAbility(ActivatableAbilityInfo) requires cardId, abilityIndex,
// description and isManaAbility, so ability 0 and a non-mana ability still
// carry them.
func TestWireShapeChooseActionActions(t *testing.T) {
	v := smallView()
	d := newDec(7, 0, decision.KPriority,
		decision.Option{Index: 0, Kind: "pass", Label: "Pass priority"},
		decision.Option{Index: 1, Kind: "cast", Label: "Cast Shock", Obj: 2},
		decision.Option{Index: 2, Kind: "play_land", Label: "Play Island", Obj: 1},
		decision.Option{Index: 3, Kind: "activate", Label: "{T}: Add {W}", Obj: 1},
		decision.Option{Index: 4, Kind: "ability", Label: "Scry 1", Obj: 6},
		decision.Option{Index: 5, Kind: "cast", Label: "Cast Bear (flashback)", Obj: 2, Mode: "flashback"},
		decision.Option{Index: 6, Kind: "cast", Label: "Cast Bear (kicked)", Obj: 2, Mode: "kicked"})
	msg := pendingMustBuild(t, d, v)
	sameJSON(t, "chooseAction actions", msg.Input.Value.(mb.ChooseActionInput).Actions, `[
		{"id":"opt-1","type":"cast","cardId":"o2","mode":{"type":"normal"},"label":"Cast Shock"},
		{"id":"opt-2","type":"cast","cardId":"o1","mode":{"type":"normal"},"label":"Play Island"},
		{"id":"opt-3","type":"activateAbility","cardId":"o1","abilityIndex":0,"description":"{T}: Add {W}",
		 "isManaAbility":true,"producedMana":[{"color":"W","amount":1}]},
		{"id":"opt-4","type":"activateAbility","cardId":"o6","abilityIndex":0,"description":"Scry 1","isManaAbility":false},
		{"id":"opt-5","type":"cast","cardId":"o2","mode":{"type":"alternative","cost":"flashback"},"label":"Cast Bear (flashback)"},
		{"id":"opt-6","type":"cast","cardId":"o2","mode":{"type":"normal"},"label":"Cast Bear (kicked)"}]`)
}

// F6, F7: prompts/common.rs PaymentAction = {id} + PaymentActionKind:
// activateManaAbility(ActivatableAbilityInfo) -- never activateAbility -- and
// undoMana{cardId}; there is no autofill member, so an offered auto-fill is
// the description and pay{auto:true} (prompts/pay_mana_cost.rs) answers it.
func TestWireShapePayManaCost(t *testing.T) {
	v := smallView()
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KChoose, Min: 1, Max: 1, Source: 2, Prompt: "Pay for Bear",
		ManaPayment: &decision.ManaPaymentWindow{Card: 2, Cost: decision.PaymentCost{Generic: 1}, Owed: decision.PaymentCost{Generic: 1}},
		Options: []decision.Option{
			{Index: 0, Kind: "mana", Obj: 1, ManaSymbol: "W", Label: "Add {W}"},
			{Index: 1, Kind: decision.OptAutoFill, Label: "Auto-fill: tap Island"},
			{Index: 2, Kind: decision.OptUndoTap, Obj: 1, Label: "Undo tapping Island"},
			{Index: 3, Kind: decision.OptCancelCast, Label: "Cancel cast"},
		}}
	msg, err := New("t", 1, nil).promptPayment(d, &v)
	if err != nil {
		t.Fatal(err)
	}
	in := msg.Input.Value.(mb.PayManaCostInput)
	sameJSON(t, "payManaCost actions", in.Actions, `[
		{"id":"opt-0","type":"activateManaAbility","cardId":"o1","abilityIndex":0,"description":"Add {W}",
		 "isManaAbility":true,"producedMana":[{"color":"W","amount":1}]},
		{"id":"opt-2","type":"undoMana","cardId":"o1"}]`)
	sameJSON(t, "payManaCost presentation", in.Presentation,
		`{"title":"Pay for Bear","description":"Auto-fill: tap Island","targets":[]}`)
}

// F2, F3, F4, the type split and cmc: game/mod.rs GameViewDto.dayTime is
// DayTime (neither|day|night), PlayerDto.status is PlayerStatus
// (playing|lost|conceded), and CardDto has color: String, cmc: i32 and
// separate types / subtypes / supertypes lists.
func TestWireShapeGameViewAndCard(t *testing.T) {
	v := smallView()
	v.Players[1].Lost = true
	v.Players[0].Battlefield = append(v.Players[0].Battlefield, view.CardView{ID: 7, Name: "Krenko",
		Printing: view.Printing{Name: "Krenko"}, Types: "Legendary Creature Goblin Warrior", ManaCost: "2 R R"},
		view.CardView{ID: 8, Name: "Cmd", Printing: view.Printing{Name: "Cmd"}, Types: "Creature Human", ManaCost: "W U/B"})
	gv := wireValue(t, New("t", 1, nil).gameView(v)).(map[string]any)
	if gv["dayTime"] != "neither" {
		t.Fatalf("dayTime %v, want neither", gv["dayTime"])
	}
	for i, want := range []string{"playing", "lost"} {
		if got := gv["players"].([]any)[i].(map[string]any)["status"]; got != want {
			t.Fatalf("players[%d].status %v, want %s", i, got, want)
		}
	}
	cards := map[string]map[string]any{}
	for _, z := range gv["zones"].([]any) {
		for _, c := range z.(map[string]any)["cards"].([]any) {
			card := c.(map[string]any)
			cards[card["id"].(string)] = card
		}
	}
	pick := func(id string, keys ...string) map[string]any {
		out := map[string]any{}
		for _, k := range keys {
			out[k] = cards[id][k]
		}
		return out
	}
	keys := []string{"color", "cmc", "types", "subtypes", "supertypes", "classLevels", "sagaChapters",
		"keywords", "choices", "counters"}
	sameJSON(t, "Krenko", pick("o7", keys...), `{"color":"R","cmc":4,"types":["Creature"],"subtypes":["Goblin","Warrior"],
		"supertypes":["Legendary"],"classLevels":[],"sagaChapters":[],"keywords":[],"choices":[],"counters":{}}`)
	sameJSON(t, "hybrid", pick("o8", "color", "cmc", "types", "subtypes"), `{"color":"WUB","cmc":2,"types":["Creature"],"subtypes":["Human"]}`)
	sameJSON(t, "Island", pick("o1", "color", "cmc", "types", "subtypes", "supertypes"),
		`{"color":"","cmc":0,"types":["Land"],"subtypes":["Island"],"supertypes":["Basic"]}`)
}

// F8, F10: prompts/choose_board_targets.rs chosenTargets is an i32 count and
// intent a TargetingIntent (game/mod.rs), which has no empty value.
func TestWireShapeChooseBoardTargets(t *testing.T) {
	d := &decision.Decision{Seq: 4, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1, Prompt: "Choose a card",
		Options: []decision.Option{{Index: 0, Kind: "permanent", Label: "Bear", Obj: 2}}}
	in := wireValue(t, pendingMustBuild(t, d, smallView()).Input).(map[string]any)
	sameJSON(t, "chooseBoardTargets", map[string]any{"chosenTargets": in["chosenTargets"], "intent": in["intent"],
		"hostile": in["hostile"], "candidates": in["candidates"]},
		`{"chosenTargets":0,"intent":"friendly","hostile":false,"candidates":[{"kind":"card","id":"o2","oracle":"Bear"}]}`)
}

// F9: a card the seat's view cannot show (a library card a scry offers) is
// built without most of its lists; game/mod.rs CardDto's Vec and BTreeMap
// fields reject null, so each is [] / {}.
func TestWireShapeUnseenCardListsAreEmpty(t *testing.T) {
	card := wireValue(t, New("t", 1, nil).optionCard(&view.View{}, decision.Option{Obj: 41, Label: "Opt"})).(map[string]any)
	for _, k := range []string{"types", "subtypes", "supertypes", "classLevels", "sagaChapters", "choices",
		"keywords", "attachmentIds", "mergedCardIds"} {
		if l, ok := card[k].([]any); !ok || len(l) != 0 {
			t.Errorf("%s = %#v, want []", k, card[k])
		}
	}
	if m, ok := card["counters"].(map[string]any); !ok || len(m) != 0 {
		t.Errorf("counters = %#v, want {}", card["counters"])
	}
	if card["color"] != "" {
		t.Errorf("color = %#v, want the empty string", card["color"])
	}
}

// F5: deck_dto.rs CardRulesSummary.color and Deck.color are strings; decoding
// still accepts the colour list gorge once modelled.
func TestWireShapeDeckColorDecodes(t *testing.T) {
	var d mb.Deck
	raw := `{"name":"d","color":"WU","editor":{"layout":{}},"coverCardFace":1,
		"cards":[{"identity":{"name":"A","setCode":"","cardNumber":"0"},"color":"","colorIdentity":[]},
		         {"identity":{"name":"B","setCode":"","cardNumber":"0"},"color":["G","W"]}]}`
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatalf("a ManaBrew deck did not decode: %v", err)
	}
	if d.Color != "WU" || d.Cards[0].Color != "" || d.Cards[1].Color != "GW" {
		t.Fatalf("colours %q %q %q", d.Color, d.Cards[0].Color, d.Cards[1].Color)
	}
}
