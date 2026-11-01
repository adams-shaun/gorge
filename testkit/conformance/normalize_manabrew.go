package conformance

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/server/manabrew/mbtest"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// manaBrewRaw is one read of the ManaBrew wire, as the transport served it.
type manaBrewRaw struct {
	Seats    [][]byte          // the state message, per seat
	Asked    int               // the seat Prompt was posed to
	Prompt   *mb.PromptMessage // nil at the end of the game
	GameOver [][]byte          // the terminal prompt, per seat, once delivered
}

// manaBrewAnswers maps a frame's answer keys to what a client sends.
type manaBrewAnswers map[string]mb.PromptOutputValue

// passKey is the id of passing priority.
const passKey = "pass"

// manaBrewSampleSeeds is how many seeded clients sample a prompt's answers.
// Combat declarations are exponential, and each option's reachability has a test
// of its own in server/manabrew/mbtest.
const manaBrewSampleSeeds = 8

// manaBrewDisplayKeys are display strings, at any depth.
var manaBrewDisplayKeys = []string{"label", "modeLabel", "description", "confirmLabel", "denyLabel", "oracle", "text"}

// manaBrewStaleKeys are the bookkeeping fields of a card, off the battlefield.
var manaBrewStaleKeys = []string{"isAttacking", "attackingPlayerId", "tapped", "summoningSick", "damage"}

// normaliseManaBrew turns one read of the ManaBrew wire into a Frame. It may only
// remove what that wire documents as representation, and each thing it removes
// is named here:
//
//   - Object ids, o<id> and s<id>, are renamed by first appearance, walking
//     seats, then zones in wire order, then the stack, so two engines may number
//     objects differently. A hidden card's h-<zone>-<owner>-<i> is positional and
//     stays.
//   - promptId is the decision's position in the event log, so an engine's own.
//   - A chooseAction's actions become Answers. Mana abilities are dropped, and an
//     opt- and a pay- action for one play are one answer: whether a play is
//     offered directly or by plan is how an engine exposes mana.
//   - A card off the battlefield loses isAttacking, attackingPlayerId, tapped,
//     summoningSick and damage: gorge leaves them stale on an object that changes
//     zones, and nothing in the rules reads them there.
//   - presentation.title and manaBrewDisplayKeys are display strings. They are
//     left out of the comparison, and must still be present on what a player is
//     asked to answer. A chooseFromSelection option keeps its label: the wire
//     names no object on one, so the label is the choice.
//
// An ack-only prompt and a payManaCost window the session can settle never reach
// here: the session answers them.
func normaliseManaBrew(raw manaBrewRaw) (Frame, manaBrewAnswers, error) {
	names := wireNames{}
	st := make(map[string]any, len(raw.Seats))
	views := make([]map[string]any, len(raw.Seats))
	for i, b := range raw.Seats {
		var msg struct {
			GameView map[string]any `json:"gameView"`
		}
		if err := json.Unmarshal(b, &msg); err != nil {
			return Frame{}, nil, fmt.Errorf("seat %d state: %w", i, err)
		}
		views[i] = msg.GameView
		st[seatName(i)] = views[i]
	}
	for _, v := range views {
		names.nameZones(v)
	}
	onBattlefield := map[string]bool{}
	for _, v := range views {
		eachWireCard(v, func(zone string, card map[string]any) {
			if id, _ := card["id"].(string); zone == string(mb.ZoneBattlefield) {
				onBattlefield[id] = true
			}
		})
	}
	for _, v := range views {
		dropManaBrewUncompared(v, onBattlefield)
		names.rewrite(v)
	}
	f := Frame{Key: "over", State: st}
	if raw.Prompt == nil {
		for i, b := range raw.GameOver {
			if b == nil {
				return Frame{}, nil, fmt.Errorf("seat %d: the game ended with no gameOver prompt", i)
			}
			var tree map[string]any
			if err := json.Unmarshal(b, &tree); err != nil {
				return Frame{}, nil, fmt.Errorf("seat %d gameOver: %w", i, err)
			}
			delete(tree, "promptId")
			views[i]["prompt"] = tree
		}
		return f, nil, nil
	}
	pm := *raw.Prompt
	if pm.Input.Value == nil {
		return Frame{}, nil, errors.New("a prompt with no input")
	}
	promptType := pm.Input.Value.PromptType()
	f.Key = seatName(raw.Asked) + "/" + promptType

	b, err := mb.Encode(mb.EngineMessage{Value: pm})
	if err != nil {
		return Frame{}, nil, fmt.Errorf("%s prompt: %w", promptType, err)
	}
	var tree map[string]any
	if err := json.Unmarshal(b, &tree); err != nil {
		return Frame{}, nil, fmt.Errorf("%s prompt: %w", promptType, err)
	}
	if err := checkPromptDisplay(promptType, tree); err != nil {
		return Frame{}, nil, err
	}
	delete(tree, "promptId")

	var answers manaBrewAnswers
	if in, ok := pm.Input.Value.(mb.ChooseActionInput); ok {
		if answers, err = chooseActionAnswers(in, names); err != nil {
			return Frame{}, nil, err
		}
		delete(tree["input"].(map[string]any), "actions")
	}
	dropManaBrewUncompared(tree, onBattlefield)
	names.rewrite(tree)
	views[raw.Asked]["prompt"] = tree
	if answers == nil {
		if answers, err = sampledAnswers(pm, names); err != nil {
			return Frame{}, nil, err
		}
	}
	f.Answers = slices.Sorted(maps.Keys(answers))
	return f, answers, nil
}

// wireNames is one frame's renaming of object ids: the number each o<id> and
// s<id> carries becomes its order of appearance. A spell and its stack object
// share a number on the wire and keep sharing one.
type wireNames map[string]int

var wireObjectID = regexp.MustCompile(`^([os])([0-9]+)$`)

// rename returns s under its new number if s is an object id, or s.
func (n wireNames) rename(s string) string {
	m := wireObjectID.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	num, ok := n[m[2]]
	if !ok {
		num = len(n) + 1
		n[m[2]] = num
	}
	return fmt.Sprintf("%s%d", m[1], num)
}

// eachWireCard visits every card in v's zones, in wire order.
func eachWireCard(v map[string]any, visit func(zone string, card map[string]any)) {
	zones, _ := v["zones"].([]any)
	for _, z := range zones {
		zm, _ := z.(map[string]any)
		zone, _ := zm["zone"].(string)
		cards, _ := zm["cards"].([]any)
		for _, c := range cards {
			if card, ok := c.(map[string]any); ok {
				visit(zone, card)
			}
		}
	}
}

// nameZones names everything in v's zones and on its stack, in order. An id that
// appears only as a reference is named later by rewrite.
func (n wireNames) nameZones(v map[string]any) {
	eachWireCard(v, func(_ string, card map[string]any) {
		if id, ok := card["id"].(string); ok {
			n.rename(id)
		}
	})
	stack, _ := v["stack"].([]any)
	for _, s := range stack {
		if so, ok := s.(map[string]any); ok {
			if id, ok := so["id"].(string); ok {
				n.rename(id)
			}
		}
	}
}

// rewrite renames every object id under v, as a value or a map key, walking keys
// in sorted order so that an id first seen here is named the same on both sides.
func (n wireNames) rewrite(v any) any {
	switch x := v.(type) {
	case string:
		return n.rename(x)
	case []any:
		for i, e := range x {
			x[i] = n.rewrite(e)
		}
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(x)) {
			val := n.rewrite(x[k])
			if name := n.rename(k); name != k {
				delete(x, k)
				k = name
			}
			x[k] = val
		}
	}
	return v
}

// dropManaBrewUncompared removes display strings at any depth, and the
// bookkeeping of every object not in onBattlefield. A chooseFromSelection's
// options keep their labels.
func dropManaBrewUncompared(v any, onBattlefield map[string]bool) {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			dropManaBrewUncompared(e, onBattlefield)
		}
	case map[string]any:
		for _, k := range manaBrewDisplayKeys {
			delete(x, k)
		}
		if p, ok := x["presentation"].(map[string]any); ok {
			delete(p, "title")
		}
		if id, _ := x["id"].(string); wireObjectID.MatchString(id) && !onBattlefield[id] {
			for _, k := range manaBrewStaleKeys {
				delete(x, k)
			}
		}
		selection := x["type"] == mb.ChooseFromSelectionInput{}.PromptType()
		for k, e := range x {
			if !selection || k != "options" {
				dropManaBrewUncompared(e, onBattlefield)
				continue
			}
			options, _ := e.([]any)
			for _, o := range options {
				option, _ := o.(map[string]any)
				label := option["label"]
				dropManaBrewUncompared(option, onBattlefield)
				option["label"] = label
			}
		}
	}
}

// checkPromptDisplay fails on a prompt a player could not read: no title, or an
// option with no label.
func checkPromptDisplay(promptType string, tree map[string]any) error {
	input, _ := tree["input"].(map[string]any)
	if p, ok := input["presentation"].(map[string]any); ok {
		if title, _ := p["title"].(string); title == "" {
			return fmt.Errorf("%s prompt has no title", promptType)
		}
	}
	if promptType == (mb.ChooseFromSelectionInput{}).PromptType() {
		options, _ := input["options"].([]any)
		for i, o := range options {
			if label, _ := o.(map[string]any)["label"].(string); label == "" {
				return fmt.Errorf("%s prompt: option %d has no label", promptType, i)
			}
		}
	}
	return nil
}

// chooseActionAnswers is passing, plus one answer per non-mana action. A cast is
// cast/<card>/<mode>/<n>, with n its ordinal among that card's plays of that
// mode, since a pay- action carries no mode; an opt- and a pay- action with the
// same key are one answer, sent as the pay- action. An ability is
// ability/<card>/<index>.
func chooseActionAnswers(in mb.ChooseActionInput, names wireNames) (manaBrewAnswers, error) {
	answers := manaBrewAnswers{passKey: mb.PassOutput{}}
	for _, pay := range []bool{true, false} {
		plays := map[string]int{}
		for _, a := range in.Actions {
			if strings.HasPrefix(a.ID, "pay-") != pay || a.IsManaAbility {
				continue
			}
			var key string
			switch a.Type {
			case "cast":
				if a.Label == "" {
					return nil, fmt.Errorf("action %s has no label", a.ID)
				}
				base := fmt.Sprintf("cast/%s/%s", names.rename(a.CardID), a.Mode)
				key = fmt.Sprintf("%s/%d", base, plays[base])
				plays[base]++
			case "activateAbility":
				if a.Description == "" {
					return nil, fmt.Errorf("action %s has no description", a.ID)
				}
				key = fmt.Sprintf("ability/%s/%d", names.rename(a.CardID), a.AbilityIndex)
			default:
				return nil, fmt.Errorf("action %s has type %q", a.ID, a.Type)
			}
			if _, dup := answers[key]; !dup {
				answers[key] = mb.ActOutput{ActionID: a.ID}
			}
		}
	}
	return answers, nil
}

// sampledAnswers is what the seeded clients and the first-legal client answer pm,
// keyed by the answer's JSON with object ids renamed. The wire cannot pre-validate
// every answer, so the game can still refuse one.
func sampledAnswers(pm mb.PromptMessage, names wireNames) (manaBrewAnswers, error) {
	clients := []*mbtest.MockClient{mbtest.NewFirstLegalClient()}
	for s := range manaBrewSampleSeeds {
		clients = append(clients, mbtest.NewSeededRandomClient(uint64(s+1)))
	}
	answers := manaBrewAnswers{}
	for _, c := range clients {
		msg, err := c.Answer(pm)
		if err != nil {
			return nil, err
		}
		out := msg.Value.(mb.ClientResponse).Action.Output.Value
		b, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		var tree any
		if err := json.Unmarshal(b, &tree); err != nil {
			return nil, err
		}
		if b, err = json.Marshal(names.rewrite(tree)); err != nil {
			return nil, err
		}
		answers[string(b)] = out
	}
	return answers, nil
}
