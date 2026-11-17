package cardcov

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// Slot is one ability of a card that play can "use".
//
// The inventory (Inventory) is every activated ability (an AB entry of
// Face.Abilities, mana abilities included) and every triggered ability (a
// Face.Triggers entry), on every face of the card (DFC back faces, split
// halves, adventures, flip/meld faces). Keyword-expanded abilities count:
// engine/cards/keywords.go's expandKeywords compiles keywords such as Cycling,
// Equip, Ward or Annihilator into ordinary Abilities/Triggers entries at load,
// and those are real activations/triggers the fuzzer should reach.
//
// Excluded, deliberately:
//   - the spell ability itself (Kind SP -- every face's spell, including an
//     adventure or split half): casting is measured by Game.Cast;
//   - statics (Face.Statics), replacement effects (Face.Repls) and keywords
//     rules reads directly without expanding (Flying, Flash, Kicker,
//     Flashback, Protection, Evoke ...): nothing is "used" in the log;
//   - a Static$ True bookkeeping trigger with no description
//     (bookkeepingTrigger);
//   - abilities granted by another card or gained from one (GrantTriggerPush,
//     GainedAbilityPush, KeywordAbilityPush, a Clone's copied abilities):
//     they are not the card's own, so they credit nothing.
type Slot struct {
	Key  string // stable id: "f<face>/a<index>" or "f<face>/t<index>"
	Desc string // human label for the report: API or trigger mode
	SA   *cards.SA
	Mana bool
}

func isManaAPI(api string) bool { return api == "Mana" || api == "ManaReflected" }

// cardName is the card's front-face name, the key every map here is by.
func cardName(c *cards.Card) string { return c.Faces[0].Name }

// Inventory lists c's usable abilities in face, then abilities-before-
// triggers, then index order. Keys index the compiled face slices, so they are
// stable for a given corpus pin.
func Inventory(c *cards.Card) []Slot {
	var out []Slot
	for fi, f := range c.Faces {
		if f == nil {
			continue
		}
		for i, a := range f.Abilities {
			if a == nil || a.Kind != "AB" {
				continue
			}
			s := Slot{Key: fmt.Sprintf("f%d/a%d", fi, i), Desc: a.API, SA: a, Mana: isManaAPI(a.API)}
			if kw := a.Params["Keyword"]; kw != "" {
				s.Desc += "/" + kw
			}
			out = append(out, s)
		}
		for i, t := range f.Triggers {
			if t.Effect == nil || bookkeepingTrigger(t) {
				continue
			}
			d := t.Mode
			if kw := t.Params["Keyword"]; kw != "" {
				d += "/" + kw
			}
			out = append(out, Slot{Key: fmt.Sprintf("f%d/t%d", fi, i), Desc: d, SA: t.Effect})
		}
	}
	return out
}

// bookkeepingTrigger reports a Forge script's internal state-tracking
// trigger: a Static$ True line with no TriggerDescription$ (the DBForget /
// DBCleanup riders that clear a Remembered/Imprinted card when it leaves
// exile or its source leaves play -- Chrome Mox, Myr Welder, Isochron
// Scepter ...). It is no printed ability of the card, and it fires only on
// the incidental zone change it tidies up after, so counting it would hold
// the card below "full" for no ability a player could ever use. A Static$
// True TapsForMana trigger is a real triggered mana ability (CR 605.1b) and
// always has a description, so it stays.
func bookkeepingTrigger(t cards.Trigger) bool {
	return strings.EqualFold(strings.TrimSpace(t.Params["Static"]), "True") &&
		strings.TrimSpace(t.Params["TriggerDescription"]) == "" && t.Mode != "TapsForMana"
}

// Keys is Inventory's keys (already in a stable order); nil when none.
func Keys(c *cards.Card) []string {
	inv := Inventory(c)
	if len(inv) == 0 {
		return nil
	}
	out := make([]string, len(inv))
	for i, s := range inv {
		out[i] = s.Key
	}
	return out
}

// Descs maps key -> label for the report.
func Descs(c *cards.Card) map[string]string {
	m := map[string]string{}
	for _, s := range Inventory(c) {
		m[s.Key] = s.Desc
	}
	return m
}

// modeSlot is one modal ability of a card: a face's spell ability or an
// inventory slot whose API is Charm, with the label of each mode.
type modeSlot struct {
	key    string // "f<face>/s" or the inventory slot's key
	sa     *cards.SA
	labels []string // labels[k] is mode k's label, as rules' modeLabels names it
}

func (m modeSlot) modeKey(k int) string { return fmt.Sprintf("%s/m%d", m.key, k) }

// modeSlots lists c's modal abilities: each face's Charm spell ability, then
// each Charm slot of the inventory.
func modeSlots(c *cards.Card) []modeSlot {
	var out []modeSlot
	add := func(key string, sa *cards.SA, svars map[string]string) {
		modes := effects.CharmOf(sa).Modes
		labels := make([]string, len(modes))
		for k, name := range modes {
			labels[k] = effects.CharmModeLabel(cards.ResolveSVar(svars, name), name)
		}
		out = append(out, modeSlot{key: key, sa: sa, labels: labels})
	}
	for fi, f := range c.Faces {
		if f == nil {
			continue
		}
		if sa := f.SpellAbility(); sa != nil && sa.API == "Charm" {
			add(fmt.Sprintf("f%d/s", fi), sa, f.SVars)
		}
	}
	inv := Inventory(c)
	for fi, f := range c.Faces {
		if f == nil {
			continue
		}
		for _, s := range inv {
			if s.SA.API == "Charm" && strings.HasPrefix(s.Key, fmt.Sprintf("f%d/", fi)) {
				add(s.Key, s.SA, f.SVars)
			}
		}
	}
	return out
}

// ModeKeys is c's per-mode keys, kept apart from Keys so a persistent
// inventory of Keys does not move: "f<face>/s/m<k>" for each mode k of a face
// whose spell ability is a Charm, and "<slotkey>/m<k>" for each mode of an
// inventory slot that is one.
func ModeKeys(c *cards.Card) []string {
	var out []string
	for _, m := range modeSlots(c) {
		for k := range m.labels {
			out = append(out, m.modeKey(k))
		}
	}
	return out
}

// IsModeKey reports whether key is a ModeKeys key rather than a Keys one.
func IsModeKey(key string) bool {
	i := strings.LastIndexByte(key, '/')
	if i < 0 || len(key) < i+3 || key[i+1] != 'm' {
		return false
	}
	for _, r := range key[i+2:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
