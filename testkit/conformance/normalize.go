package conformance

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// nativeRaw is one read of the native wire, as the HTTP API served it.
type nativeRaw struct {
	Seats     [][]byte // GET view?seat=i, per seat
	Spectator []byte   // GET view
	Pending   []byte   // GET pending of the seat asked; nil at the end of the game
}

// noSelection is the id of an answer that chooses no options.
const noSelection = "none"

// maxSelections bounds a non-priority frame's Answers. Blockers over a full
// board have more legal declarations than can be listed, so a frame lists them
// smallest first, up to this many.
const maxSelections = 256

// maxSelectionTries bounds the candidates tried in reaching maxSelections.
const maxSelectionTries = 1 << 16

// normaliseNative turns one read of the native wire into a Frame. It may only
// remove what that wire documents as representation, and each thing it removes
// is named here:
//
//   - Object ids are renamed by first appearance, walking viewers, then seats,
//     then zones in view order, so two engines may number objects differently.
//   - A priority decision's Options, PaymentActions and the asked seat's
//     PotentialActions become one sorted set of play keys. A mana ability
//     (Option kind "activate", which at priority is only ever one) and concede
//     are not plays and are dropped. Whether a play is offered directly, with a
//     payment plan or after tapping mana is how an engine exposes mana, not
//     what it allows.
//   - A card off the battlefield loses attacking, tapped, summon_sick and
//     damage: gorge leaves them stale on an object that changes zones, and
//     nothing in the rules reads them there.
//   - label, prompt and text are display strings. They are left out of the
//     comparison, and must still be present on what a player is asked to answer.
//     An option naming no object keeps its label: a card name or a creature type
//     is offered as nothing else, so there the label is the choice.
//   - seq only orders the wire's own messages.
//
// A mana_payment window never reaches here: nativeSession.poll answers it.
func normaliseNative(raw nativeRaw) (Frame, *objectNames, error) {
	names := &objectNames{ids: map[state.ObjID]string{}}
	st := make(map[string]any, len(raw.Seats)+1)
	var views []map[string]any
	for i, b := range raw.Seats {
		v, err := decodeView(b)
		if err != nil {
			return Frame{}, nil, fmt.Errorf("seat %d view: %w", i, err)
		}
		st[seatName(i)] = v
		views = append(views, v)
	}
	spectator, err := decodeView(raw.Spectator)
	if err != nil {
		return Frame{}, nil, fmt.Errorf("spectator view: %w", err)
	}
	st["spectator"] = spectator
	views = append(views, spectator)

	for _, v := range views {
		names.nameZones(v)
	}
	for _, v := range views {
		names.rewrite(v)
		dropStaleBookkeeping(v)
		dropUncompared(v)
	}

	f := Frame{Key: "over", State: st}
	if raw.Pending == nil {
		return f, names, nil
	}
	var d decision.Decision
	if err := json.Unmarshal(raw.Pending, &d); err != nil {
		return Frame{}, nil, fmt.Errorf("pending decision: %w", err)
	}
	if d.Prompt == "" {
		return Frame{}, nil, fmt.Errorf("%s decision has no prompt", d.Kind)
	}
	for _, o := range d.Options {
		if o.Label == "" {
			return Frame{}, nil, fmt.Errorf("%s decision: option %d (%s) has no label", d.Kind, o.Index, o.Kind)
		}
	}
	f.Key = seatName(int(d.Player)) + "/" + string(d.Kind)
	if d.Kind != decision.KPriority {
		f.Answers = selections(&d)
		return f, names, nil
	}
	var asked struct {
		Players []struct {
			Potential []decision.PotentialAction `json:"potential_actions"`
		} `json:"players"`
	}
	if err := json.Unmarshal(raw.Seats[d.Player], &asked); err != nil {
		return Frame{}, nil, fmt.Errorf("seat %d view: %w", d.Player, err)
	}
	if f.Answers, err = playKeys(&d, asked.Players[d.Player].Potential, names); err != nil {
		return Frame{}, nil, err
	}
	askedView := views[d.Player]
	dv := askedView["decision"].(map[string]any)
	delete(dv, "options")
	delete(dv, "payment_actions")
	delete(askedView["players"].([]any)[d.Player].(map[string]any), "potential_actions")
	return f, names, nil
}

func seatName(i int) string { return "seat" + strconv.Itoa(i) }

func decodeView(b []byte) (map[string]any, error) {
	var v map[string]any
	err := json.Unmarshal(b, &v)
	return v, err
}

// objectNames is one frame's renaming of object ids. Id 0 means no object and
// keeps its value.
type objectNames struct {
	ids map[state.ObjID]string
}

func (n *objectNames) of(id state.ObjID) string {
	name, ok := n.ids[id]
	if !ok {
		name = "o" + strconv.Itoa(len(n.ids)+1)
		n.ids[id] = name
	}
	return name
}

// viewZones lists a player's card zones in view.PlayerView's order.
var viewZones = []string{"planar_deck", "library_top", "library", "hand", "battlefield", "graveyard", "exile", "command", "commanders"}

// eachZoneCard visits every card in v's zones: players in order, zones in
// viewZones order.
func eachZoneCard(v map[string]any, visit func(zone string, card map[string]any)) {
	players, _ := v["players"].([]any)
	for _, p := range players {
		pm, _ := p.(map[string]any)
		for _, zone := range viewZones {
			switch z := pm[zone].(type) {
			case []any:
				for _, c := range z {
					visit(zone, c.(map[string]any))
				}
			case map[string]any:
				visit(zone, z)
			}
		}
	}
}

// nameZones names everything in v's zones and on its stack, in order. An id
// that appears only as a reference, such as the source of an ability whose
// permanent has left, is named later by rewrite.
func (n *objectNames) nameZones(v map[string]any) {
	eachZoneCard(v, func(_ string, card map[string]any) { n.rename(card["id"]) })
	stack, _ := v["stack"].([]any)
	for _, s := range stack {
		n.rename(s.(map[string]any)["id"])
	}
}

// rename returns the name of id if id is a JSON number naming an object.
func (n *objectNames) rename(id any) any {
	f, ok := id.(float64)
	if !ok || f == 0 {
		return id
	}
	return n.of(state.ObjID(f))
}

// objectFields are the JSON fields of view.View and decision.Decision holding
// an object id or a list of them. On a stack entry "card" is a whole card, and
// a payment action's "id" is a string; rename leaves what is not a number.
var objectFields = map[string]bool{
	"id": true, "obj": true, "attacker": true, "battle": true, "source": true, "card": true,
	"attached_to": true, "paired": true, "blocked_by": true, "imprinted": true, "exiled_cards": true,
}

// rewrite replaces every object id under v with its name, walking keys in
// sorted order so that an id first seen here is named the same on both sides.
func (n *objectNames) rewrite(v any) {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			n.rewrite(e)
		}
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(x)) {
			switch val := x[k]; {
			case k == "token":
				x[k] = n.renameTag(val)
			case k == "group":
				x[k] = n.renameGroup(val)
			case k == "cmd_damage":
				x[k] = n.renameKeys(val)
			case !objectFields[k]:
				n.rewrite(val)
			default:
				switch ids := val.(type) {
				case []any:
					for i, e := range ids {
						ids[i] = n.rename(e)
					}
				case map[string]any:
					n.rewrite(ids)
				default:
					x[k] = n.rename(val)
				}
			}
		}
	}
}

// renameTag renames a card's display tag, which is "#" and its object id.
func (n *objectNames) renameTag(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(s, "#"), 10, 32)
	if err != nil {
		return v
	}
	return "#" + n.of(state.ObjID(id))
}

// renameGroup renames the object in an option's group where a group is per
// object. Other groups hold a name, a seat or a target index.
func (n *objectNames) renameGroup(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	prefix, rest, ok := strings.Cut(s, ":")
	if !ok || prefix != "blocker" && prefix != "payment" {
		return v
	}
	id, err := strconv.ParseUint(rest, 10, 32)
	if err != nil {
		return v
	}
	return prefix + ":" + n.of(state.ObjID(id))
}

// renameKeys renames the object ids keying a map.
func (n *objectNames) renameKeys(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := make(map[string]any, len(m))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		id, err := strconv.ParseUint(k, 10, 32)
		if err != nil {
			out[k] = m[k]
			continue
		}
		out[n.of(state.ObjID(id))] = m[k]
	}
	return out
}

// dropStaleBookkeeping removes the combat and damage fields from every card off
// the battlefield.
func dropStaleBookkeeping(v map[string]any) {
	strip := func(card map[string]any) {
		delete(card, "attacking")
		delete(card, "tapped")
		delete(card, "summon_sick")
		delete(card, "damage")
	}
	eachZoneCard(v, func(zone string, card map[string]any) {
		if zone != "battlefield" {
			strip(card)
		}
	})
	stack, _ := v["stack"].([]any)
	for _, s := range stack {
		if card, ok := s.(map[string]any)["card"].(map[string]any); ok {
			strip(card)
		}
	}
}

// dropUncompared removes label, prompt, text and seq at any depth, but for the
// label of a decision option naming no object.
func dropUncompared(v any) {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			dropUncompared(e)
		}
	case map[string]any:
		delete(x, "label")
		delete(x, "prompt")
		delete(x, "text")
		delete(x, "seq")
		for k, e := range x {
			if k != "options" {
				dropUncompared(e)
				continue
			}
			options, _ := e.([]any)
			for _, o := range options {
				option, _ := o.(map[string]any)
				label, named := option["label"]
				dropUncompared(option)
				if _, object := option["obj"]; named && !object {
					option["label"] = label
				}
			}
		}
	}
}

// playKey identifies a play at a priority decision.
func playKey(kind string, obj state.ObjID, ability int, mode string, names *objectNames) string {
	object := "-"
	if obj != 0 {
		object = names.of(obj)
	}
	return fmt.Sprintf("%s/%s/%d/%s", kind, object, ability, mode)
}

// isManaAbility reports whether a priority option taps for mana.
func isManaAbility(o decision.Option) bool { return o.Kind == "activate" }

// isPlay reports whether a priority option is a play rather than a mana ability
// or a concession.
func isPlay(o decision.Option) bool { return !isManaAbility(o) && o.Kind != "concede" }

func optionKey(o decision.Option, names *objectNames) string {
	return playKey(o.Kind, o.Obj, o.Ability, o.Mode, names)
}

func paymentKey(a decision.PaymentAction, names *objectNames) string {
	return playKey("cast", a.Cast.Object, 0, a.Cast.CastMode(), names)
}

// playKeys is the sorted set of plays a priority decision offers.
func playKeys(d *decision.Decision, potential []decision.PotentialAction, names *objectNames) ([]string, error) {
	var keys []string
	for _, o := range d.Options {
		if isPlay(o) {
			keys = append(keys, optionKey(o, names))
		}
	}
	for _, a := range d.PaymentActions {
		if a.Label == "" {
			return nil, fmt.Errorf("payment action %s has no label", a.ID)
		}
		keys = append(keys, paymentKey(a, names))
	}
	for _, p := range potential {
		if p.Label == "" {
			return nil, fmt.Errorf("potential %s of object %d has no label", p.Kind, p.Obj)
		}
		keys = append(keys, playKey(p.Kind, p.Obj, p.Ability, p.Mode, names))
	}
	slices.Sort(keys)
	return slices.Compact(keys), nil
}

// selections lists the option-index selections d accepts, sorted by id, taking
// the smallest first when they exceed maxSelections. decision.Validate sees
// only the fields the wire publishes, so the game can still refuse one. Order
// matters only to a trigger order or an arrangement.
func selections(d *decision.Decision) []string {
	ordered := d.Kind == decision.KTriggerOrder || d.Kind == decision.KArrange
	var out []string
	tries := 0
	accept := func(choices []int) {
		in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
		if d.Validate(in) == nil {
			out = append(out, selectionID(choices))
		}
	}
	var choose func(choices []int, size, from int)
	choose = func(choices []int, size, from int) {
		if len(out) >= maxSelections || tries >= maxSelectionTries {
			return
		}
		if len(choices) == size {
			tries++
			accept(choices)
			return
		}
		for i := from; i < len(d.Options); i++ {
			if !d.Repeatable && slices.Contains(choices, i) {
				continue
			}
			next := i + 1
			switch {
			case ordered:
				next = 0
			case d.Repeatable:
				next = i
			}
			choose(append(choices, i), size, next)
		}
	}
	if d.AllowNone && d.Min > 0 {
		accept([]int{})
	}
	largest := d.Max
	if !d.Repeatable {
		largest = min(largest, len(d.Options))
	}
	for size := d.Min; size <= largest; size++ {
		choose(make([]int, 0, size), size, 0)
	}
	slices.Sort(out)
	return out
}

func selectionID(choices []int) string {
	if len(choices) == 0 {
		return noSelection
	}
	parts := make([]string, len(choices))
	for i, c := range choices {
		parts[i] = strconv.Itoa(c)
	}
	return strings.Join(parts, ",")
}

// parseSelection is selectionID's inverse.
func parseSelection(id string) ([]int, error) {
	if id == noSelection {
		return []int{}, nil
	}
	parts := strings.Split(id, ",")
	choices := make([]int, len(parts))
	for i, p := range parts {
		c, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("answer %q: %w", id, err)
		}
		choices[i] = c
	}
	return choices, nil
}
