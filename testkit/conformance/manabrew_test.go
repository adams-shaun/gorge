package conformance

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/state"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// countingWire is ManaBrew, keeping the sessions it opens.
type countingWire struct {
	ManaBrew
	sessions []*ManaBrewSession
}

func (w *countingWire) Open(t testing.TB, e game.Engine, c game.Config) Session {
	s := w.ManaBrew.Open(t, e, c).(*ManaBrewSession)
	w.sessions = append(w.sessions, s)
	return s
}

func fdnConfig(t *testing.T, reg *cards.Registry, g int) game.Config {
	t.Helper()
	pair := fdnPairs[g%len(fdnPairs)]
	return game.Config{
		Seed:   uint64(1000 + g),
		Names:  []string{pair[0], pair[1]},
		Decks:  [][]*cards.Card{fdnDeck(t, reg, pair[0]), fdnDeck(t, reg, pair[1])},
		Tokens: reg.Tokens,
	}
}

func TestManaBrewConformanceGorgeSelf(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	var synthetic, windows atomic.Int64
	t.Run("games", func(t *testing.T) {
		for g := range selfCheckGames {
			t.Run(fmt.Sprintf("game%02d", g), func(t *testing.T) {
				t.Parallel()
				c := fdnConfig(t, reg, g)
				frames := 0
				random := Random(c.Seed)
				pick := func(f Frame) string {
					frames++
					return random(f)
				}
				w := &countingWire{}
				if d := Run(t, w, gorgegame.Engine{}, gorgegame.Engine{}, c, pick); d != nil {
					t.Fatal(d)
				}
				if frames < minFrames {
					t.Fatalf("the game finished after %d frames, want at least %d", frames, minFrames)
				}
				a, b := w.sessions[0], w.sessions[1]
				if a.Synthetic != b.Synthetic || a.Windows != b.Windows {
					t.Fatalf("one game, two counts: %d and %d synthetic, %d and %d windows", a.Synthetic, b.Synthetic, a.Windows, b.Windows)
				}
				synthetic.Add(int64(a.Synthetic))
				windows.Add(int64(a.Windows))
				t.Logf("%s vs %s: %d frames, %d synthetic prompts, %d windows", c.Names[0], c.Names[1], frames, a.Synthetic, a.Windows)
			})
		}
	})
	// These decks reveal and roll nothing, so they deliver no synthetic prompt:
	// TestManaBrewAcknowledgesSyntheticPrompts covers that.
	t.Logf("%d synthetic prompts, %d windows", synthetic.Load(), windows.Load())
	if windows.Load() == 0 {
		t.Error("no game posed a payManaCost window, so answering them went untested")
	}
}

// One deck reveals the defending player's top card on each attack, the other
// rolls a die each upkeep.
func TestManaBrewAcknowledgesSyntheticPrompts(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	deck := func(key string) []*cards.Card {
		var d []*cards.Card
		for _, name := range []string{key, "Mountain"} {
			c, ok := reg.Lookup(name)
			if !ok {
				t.Fatalf("%q is not in the corpus", name)
			}
			d = append(d, slices.Repeat([]*cards.Card{c}, 30)...)
		}
		return d
	}
	c := game.Config{Seed: 7, Names: []string{"guide", "kow"}, Decks: [][]*cards.Card{deck("Goblin Guide"), deck("Krazy Kow")}, Tokens: reg.Tokens}
	w := &countingWire{}
	if d := Run(t, w, gorgegame.Engine{}, gorgegame.Engine{}, c, Random(c.Seed)); d != nil {
		t.Fatal(d)
	}
	a, b := w.sessions[0].Synthetic, w.sessions[1].Synthetic
	t.Logf("%d synthetic prompts", a)
	if a == 0 || a != b {
		t.Fatalf("%d and %d synthetic prompts, want an equal count above 0", a, b)
	}
}

// wrongLife reports seat 1 one life too high.
type wrongLife struct{ gorgegame.Game }

func (g wrongLife) Board() *state.Game {
	b := *g.Game.Board()
	b.Players = slices.Clone(b.Players)
	b.Players[1].Life++
	return &b
}

// Fork keeps the stub on the copy a host projects a view from.
func (g wrongLife) Fork() game.Game { return wrongLife{g.Game.Fork().(gorgegame.Game)} }

// missingPlay offers no play at priority but passing and mana abilities.
type missingPlay struct{ gorgegame.Game }

func (g missingPlay) Pending() *decision.Decision {
	d := g.Game.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return d
	}
	c := *d
	c.PaymentActions = nil
	c.Options = slices.DeleteFunc(slices.Clone(d.Options), isPlayToHide)
	return &c
}

func isPlayToHide(o decision.Option) bool { return o.Kind != "pass" && isPlay(o) }

// stubEngine is gorge with its games wrapped.
type stubEngine struct {
	gorgegame.Engine
	wrap func(gorgegame.Game) game.Game
}

func (e stubEngine) New(c game.Config) (game.Game, error) {
	g, err := e.Engine.New(c)
	if err != nil {
		return nil, err
	}
	return e.wrap(g.(gorgegame.Game)), nil
}

func TestManaBrewCatchesAWrongLife(t *testing.T) {
	c := fdnConfig(t, testutil.CorpusRegistry(t), 0)
	stub := stubEngine{wrap: func(g gorgegame.Game) game.Game { return wrongLife{g} }}
	d := Run(t, ManaBrew{}, gorgegame.Engine{}, stub, c, First())
	if d == nil {
		t.Fatal("a moved life total produced no diff")
	}
	if want := "$.state.seat0.players[1].life"; d.Path != want {
		t.Fatalf("diff at %s, want %s", d, want)
	}
}

func TestManaBrewCatchesAMissingAction(t *testing.T) {
	c := fdnConfig(t, testutil.CorpusRegistry(t), 0)
	stub := stubEngine{wrap: func(g gorgegame.Game) game.Game { return missingPlay{g} }}
	d := Run(t, ManaBrew{}, gorgegame.Engine{}, stub, c, First())
	if d == nil {
		t.Fatal("a play taken off the offer produced no diff")
	}
	if !strings.HasPrefix(d.Path, "$.answers[") {
		t.Fatalf("diff at %s, want one in the answers", d)
	}
}

// A cast through a pay- action is answered by the session: the next frame has the
// spell on the stack, and no frame poses the window.
func TestManaBrewWindowIsAnsweredBySession(t *testing.T) {
	c := fdnConfig(t, testutil.CorpusRegistry(t), 0)
	s := ManaBrew{}.Open(t, gorgegame.Engine{}, c).(*ManaBrewSession)
	preferred := func(key string) int {
		switch {
		case strings.HasPrefix(key, "cast/") && !strings.Contains(key, "/play/"):
			return 0
		case strings.HasPrefix(key, "cast/"):
			return 1
		case key == passKey:
			return 2
		}
		return 3
	}
	for range 2000 {
		f, more := s.Next()
		if !more {
			t.Fatal("the game ended before any cast")
		}
		if strings.HasSuffix(f.Key, "/payManaCost") {
			t.Fatalf("a frame posed the window: %s", f.Key)
		}
		if s.Windows > 0 {
			view, _ := f.State["seat0"].(map[string]any)
			if stack, _ := view["stack"].([]any); len(stack) == 0 {
				t.Fatalf("frame %s after the window: an empty stack", f.Key)
			}
			return
		}
		key := slices.MinFunc(f.Answers, func(a, b string) int { return preferred(a) - preferred(b) })
		if err := s.Answer(key); err != nil {
			t.Logf("%s refused: %v", key, err)
			if err := s.Answer(f.Answers[len(f.Answers)-1]); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Fatal("no cast posed a window in 2000 frames")
}

// manaBrewState is a seat's state with the two given object ids: a hidden card
// and a card with stale bookkeeping in hand, a card on the battlefield, a stack
// object, and references to them.
func manaBrewState(inHand, onField int) []byte {
	return []byte(fmt.Sprintf(`{"kind":"state","gameView":{
		"gameId":"t/1","turn":3,"step":"main1","activePlayerId":"player-0","priorityPlayerId":"player-0",
		"combatAssignments":[{"attackerId":"o%[2]d","blockerIds":["o%[1]d"]}],
		"players":[{"id":"player-0","life":20,"commanderDamage":{"o%[2]d":2},"commanderCasts":{"o%[1]d":1}}],
		"zones":[
			{"zone":"hand","ownerId":"player-0","count":2,"cards":[
				{"id":"h-hand-0-0"},
				{"id":"o%[1]d","text":"Deal three.","tapped":true,"isAttacking":true,"attackingPlayerId":"player-1","summoningSick":true,"damage":1,"keywords":["haste"]}]},
			{"zone":"battlefield","ownerId":"player-0","count":1,"cards":[
				{"id":"o%[2]d","text":"Big.","tapped":true,"isAttacking":true,"attackingPlayerId":"player-1","summoningSick":true,"damage":1,"counters":{"P1P1":1}}]}],
		"stack":[{"id":"s%[1]d","sourceId":"o%[1]d","text":"Deal three.","label":"Bolt","modeLabel":"m","oracle":"o","description":"d","confirmLabel":"c","denyLabel":"e",
			"targets":[{"kind":"card","id":"o%[2]d"}]}]}}`, inHand, onField))
}

func manaBrewPrompt(in mb.PromptInputData) *mb.PromptMessage {
	return &mb.PromptMessage{Kind: "prompt", AgentPrompt: mb.AgentPrompt{PromptID: 99, DecidingPlayerID: "player-0", Input: mb.PromptInput{Value: in}}}
}

func manaBrewActions(inHand, onField int) mb.ChooseActionInput {
	return mb.ChooseActionInput{Actions: []mb.AvailableAction{
		{ID: "opt-2", Type: "activateAbility", CardID: fmt.Sprintf("o%d", onField), Description: "Tap for mana", IsManaAbility: true},
		{ID: "opt-3", Type: "activateAbility", CardID: fmt.Sprintf("o%d", onField), AbilityIndex: 1, Description: "Pump"},
		{ID: "opt-4", Type: "cast", CardID: fmt.Sprintf("o%d", inHand), Label: "Cast it"},
		{ID: "pay-mc1", Type: "cast", CardID: fmt.Sprintf("o%d", inHand), Label: "Cast it, paying"},
	}}
}

func manaBrewSelection(title, first string) mb.ChooseFromSelectionInput {
	return mb.ChooseFromSelectionInput{
		PromptBase: mb.PromptBase{Presentation: mb.PromptPresentation{Title: title, Description: "Pick one.", Text: "Long text."}},
		Options:    []mb.SelectionOption{{Label: first, Weight: 1}, {Label: "Elf", Weight: 1}},
		MinTotal:   1, MaxTotal: 1,
	}
}

func manaBrewRead(state []byte, in mb.PromptInputData) manaBrewRaw {
	return manaBrewRaw{Seats: [][]byte{state, state}, Prompt: manaBrewPrompt(in)}
}

func TestManaBrewNormaliseRenamesTokens(t *testing.T) {
	norm := func(a, b int) Frame {
		f, _, err := normaliseManaBrew(manaBrewRead(manaBrewState(a, b), manaBrewActions(a, b)))
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	low, high := norm(5, 9), norm(4000, 12)
	if !reflect.DeepEqual(low, high) {
		t.Fatalf("an object id survived normalising:\n%s\n%s", frameJSON(t, low), frameJSON(t, high))
	}
	if want := []string{"ability/o2/1", "cast/o1//0", passKey}; !slices.Equal(low.Answers, want) {
		t.Fatalf("answers %v, want %v", low.Answers, want)
	}
	view := low.State["seat0"].(map[string]any)
	got := map[string]any{
		"hidden":   view["zones"].([]any)[0].(map[string]any)["cards"].([]any)[0].(map[string]any)["id"],
		"attacker": view["combatAssignments"].([]any)[0].(map[string]any)["attackerId"],
		"blocker":  view["combatAssignments"].([]any)[0].(map[string]any)["blockerIds"].([]any)[0],
		"stack":    view["stack"].([]any)[0].(map[string]any)["id"],
		"source":   view["stack"].([]any)[0].(map[string]any)["sourceId"],
		"target":   view["stack"].([]any)[0].(map[string]any)["targets"].([]any)[0].(map[string]any)["id"],
		"damage":   view["players"].([]any)[0].(map[string]any)["commanderDamage"],
		"casts":    view["players"].([]any)[0].(map[string]any)["commanderCasts"],
	}
	want := map[string]any{
		"hidden": "h-hand-0-0", "attacker": "o2", "blocker": "o1", "stack": "s1", "source": "o1", "target": "o2",
		"damage": map[string]any{"o2": 2.0}, "casts": map[string]any{"o1": 1.0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("renamed references %v, want %v", got, want)
	}
}

func TestManaBrewNormaliseKeepsWhatAPromptOffers(t *testing.T) {
	norm := func(in mb.PromptInputData) map[string]any {
		f, _, err := normaliseManaBrew(manaBrewRead(manaBrewState(5, 9), in))
		if err != nil {
			t.Fatal(err)
		}
		return frameTree(f)
	}
	base := norm(manaBrewSelection("Pick", "Goblin"))
	if path, va, vb, found := firstDifference(base, norm(manaBrewSelection("Choose", "Goblin")), "$"); found {
		t.Errorf("a title survived normalising: %s: %v, %v", path, va, vb)
	}
	path, _, _, found := firstDifference(base, norm(manaBrewSelection("Pick", "Merfolk")), "$")
	if want := "$.state.seat0.prompt.input.options[0].label"; !found || path != want {
		t.Errorf("a different option diffed at %q, want %q", path, want)
	}
	bounds := manaBrewSelection("Pick", "Goblin")
	bounds.MaxTotal = 2
	path, _, _, found = firstDifference(base, norm(bounds), "$")
	if want := "$.state.seat0.prompt.input.maxTotal"; !found || path != want {
		t.Errorf("a different bound diffed at %q, want %q", path, want)
	}
}

// removedKeys names the map keys present in before and gone from after. An object
// id keying a map is renamed, not removed.
func removedKeys(before, after any) []string {
	var out []string
	switch x := before.(type) {
	case []any:
		y, _ := after.([]any)
		for i := range min(len(x), len(y)) {
			out = append(out, removedKeys(x[i], y[i])...)
		}
	case map[string]any:
		y, _ := after.(map[string]any)
		for k, e := range x {
			kept, ok := y[k]
			switch {
			case ok:
				out = append(out, removedKeys(e, kept)...)
			case !wireObjectID.MatchString(k):
				out = append(out, k)
			}
		}
	}
	return out
}

// TestManaBrewNormaliserLines holds the normaliser to its documented removals,
// and shows each line changes nothing a player can act on.
func TestManaBrewNormaliserLines(t *testing.T) {
	const a, b = 5, 9
	st := manaBrewState(a, b)
	frame := func(raw manaBrewRaw) (Frame, error) {
		f, _, err := normaliseManaBrew(raw)
		return f, err
	}

	t.Run("removes exactly the table", func(t *testing.T) {
		var removed []string
		for _, in := range []mb.PromptInputData{manaBrewActions(a, b), manaBrewSelection("Pick", "Goblin"),
			mb.ChooseBooleanInput{PromptBase: mb.PromptBase{Presentation: mb.PromptPresentation{Title: "Sure?"}}, ConfirmLabel: "Yes", DenyLabel: "No"}} {
			raw := manaBrewRead(st, in)
			f, err := frame(raw)
			if err != nil {
				t.Fatal(err)
			}
			var view struct {
				GameView any `json:"gameView"`
			}
			if err := json.Unmarshal(st, &view); err != nil {
				t.Fatal(err)
			}
			removed = append(removed, removedKeys(view.GameView, f.State["seat1"])...)
			msg, err := mb.Encode(mb.EngineMessage{Value: *raw.Prompt})
			if err != nil {
				t.Fatal(err)
			}
			var prompt any
			if err := json.Unmarshal(msg, &prompt); err != nil {
				t.Fatal(err)
			}
			removed = append(removed, removedKeys(prompt, f.State["seat0"].(map[string]any)["prompt"])...)
		}
		slices.Sort(removed)
		want := []string{"actions", "attackingPlayerId", "confirmLabel", "damage", "denyLabel", "description", "isAttacking",
			"label", "modeLabel", "oracle", "promptId", "summoningSick", "tapped", "text", "title"}
		slices.Sort(want)
		if got := slices.Compact(removed); !slices.Equal(got, want) {
			t.Fatalf("removed %v, want %v", got, want)
		}
	})

	selection := func() manaBrewRaw { return manaBrewRead(st, manaBrewSelection("Pick", "Goblin")) }
	replace := func(old, new string) func(*manaBrewRaw) {
		return func(raw *manaBrewRaw) {
			if !strings.Contains(string(raw.Seats[0]), old) {
				t.Fatalf("the fixture has no %q", old)
			}
			s := []byte(strings.Replace(string(raw.Seats[0]), old, new, 1))
			raw.Seats = [][]byte{s, s}
		}
	}
	action := func(edit func(*mb.ChooseActionInput)) func(*manaBrewRaw) {
		return func(raw *manaBrewRaw) {
			in := manaBrewActions(a, b)
			edit(&in)
			raw.Prompt = manaBrewPrompt(in)
		}
	}
	dropAction := func(id string) func(*mb.ChooseActionInput) {
		return func(in *mb.ChooseActionInput) {
			in.Actions = slices.DeleteFunc(in.Actions, func(a mb.AvailableAction) bool { return a.ID == id })
		}
	}
	const handCard = `"damage":1,"keywords"`
	for _, tc := range []struct {
		name   string
		base   func() manaBrewRaw
		mutate func(*manaBrewRaw)
		want   string // the first differing path; empty if the frames must agree
	}{
		{"promptId", selection, func(raw *manaBrewRaw) { raw.Prompt.PromptID = 7000 }, ""},
		{"opt- alone", func() manaBrewRaw { return manaBrewRead(st, manaBrewActions(a, b)) }, action(dropAction("pay-mc1")), ""},
		{"pay- alone", func() manaBrewRaw { return manaBrewRead(st, manaBrewActions(a, b)) }, action(dropAction("opt-4")), ""},
		{"no mana ability", func() manaBrewRaw { return manaBrewRead(st, manaBrewActions(a, b)) }, action(dropAction("opt-2")), ""},
		{"a play missing", func() manaBrewRaw { return manaBrewRead(st, manaBrewActions(a, b)) }, action(dropAction("opt-3")), "$.answers[0]"},
		{"tapped in hand", selection, replace(`"tapped":true,"isAttacking":true,"attackingPlayerId":"player-1","summoningSick":true,"damage":1,"keywords"`, `"tapped":false,"isAttacking":true,"attackingPlayerId":"player-1","summoningSick":true,"damage":1,"keywords"`), ""},
		{"attacking in hand", selection, replace(`"isAttacking":true,"attackingPlayerId":"player-1","summoningSick":true,"damage":1,"keywords"`, `"isAttacking":false,"summoningSick":true,"damage":1,"keywords"`), ""},
		{"sick in hand", selection, replace(`"summoningSick":true,"damage":1,"keywords"`, `"summoningSick":false,"damage":1,"keywords"`), ""},
		{"damage in hand", selection, replace(handCard, `"damage":7,"keywords"`), ""},
		{"damage on the battlefield", selection, replace(`"damage":1,"counters"`, `"damage":7,"counters"`), "$.state.seat0.zones[1].cards[0].damage"},
		{"tapped on the battlefield", selection, replace(`"text":"Big.","tapped":true`, `"text":"Big.","tapped":false`), "$.state.seat0.zones[1].cards[0].tapped"},
		{"card text", selection, replace(`"text":"Big."`, `"text":"Small."`), ""},
		{"stack label", selection, replace(`"label":"Bolt"`, `"label":"Shock"`), ""},
		{"priority player stays", selection, replace(`"priorityPlayerId":"player-0"`, `"priorityPlayerId":"player-1"`), "$.state.seat0.priorityPlayerId"},
		{"keywords stay", selection, replace(`"keywords":["haste"]`, `"keywords":["flying"]`), "$.state.seat0.zones[0].cards[1].keywords[0]"},
		{"life stays", selection, replace(`"life":20`, `"life":21`), "$.state.seat0.players[0].life"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, err := frame(tc.base())
			if err != nil {
				t.Fatal(err)
			}
			raw := tc.base()
			tc.mutate(&raw)
			got, err := frame(raw)
			if err != nil {
				t.Fatal(err)
			}
			path, va, vb, found := firstDifference(frameTree(base), frameTree(got), "$")
			if tc.want == "" && found {
				t.Fatalf("diff at %s: %v, %v", path, va, vb)
			}
			if tc.want != "" && (!found || path != tc.want) {
				t.Fatalf("diff at %q, want %q", path, tc.want)
			}
		})
	}

	for name, in := range map[string]mb.PromptInputData{
		"no title":        manaBrewSelection("", "Goblin"),
		"no option label": manaBrewSelection("Pick", ""),
		"no action label": mb.ChooseActionInput{Actions: []mb.AvailableAction{{ID: "pay-mc1", Type: "cast", CardID: "o5"}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := frame(manaBrewRead(st, in)); err == nil {
				t.Fatal("a prompt with no display string passed")
			}
		})
	}
}
