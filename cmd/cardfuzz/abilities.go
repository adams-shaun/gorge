package main

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

// abilitySlot is one ability of a card that play can "use".
//
// The inventory (abilityInventory) is every activated ability (an AB entry of
// Face.Abilities, mana abilities included) and every triggered ability (a
// Face.Triggers entry), on every face of the card (DFC back faces, split
// halves, adventures, flip/meld faces). Keyword-expanded abilities count:
// cards/keywords.go's expandKeywords compiles keywords such as Cycling,
// Equip, Ward or Annihilator into ordinary Abilities/Triggers entries at load,
// and those are real activations/triggers the fuzzer should reach.
//
// Excluded, deliberately:
//   - the spell ability itself (Kind SP -- every face's spell, including an
//     adventure or split half): casting is measured by cov.Cast;
//   - statics (Face.Statics), replacement effects (Face.Repls) and keywords
//     rules reads directly without expanding (Flying, Flash, Kicker,
//     Flashback, Protection, Evoke ...): nothing is "used" in the log;
//   - abilities granted by another card or gained from one (GrantTriggerPush,
//     GainedAbilityPush, KeywordAbilityPush, a Clone's copied abilities):
//     they are not the card's own, so they credit nothing.
type abilitySlot struct {
	key      string // stable id: "f<face>/a<index>" or "f<face>/t<index>"
	desc     string // human label for the report: API or trigger mode
	sa       *cards.SA
	mana     bool
	produced string // Produced$ of a mana ability
}

func isManaAPI(api string) bool { return api == "Mana" || api == "ManaReflected" }

// abilityInventory lists c's usable abilities in face, then abilities-before-
// triggers, then index order. Keys index the compiled face slices, so they are
// stable for a given corpus pin.
func abilityInventory(c *cards.Card) []abilitySlot {
	var out []abilitySlot
	for fi, f := range c.Faces {
		if f == nil {
			continue
		}
		for i, a := range f.Abilities {
			if a == nil || a.Kind != "AB" {
				continue
			}
			s := abilitySlot{key: fmt.Sprintf("f%d/a%d", fi, i), desc: a.API, sa: a, mana: isManaAPI(a.API)}
			if kw := a.Params["Keyword"]; kw != "" {
				s.desc += "/" + kw
			}
			if s.mana {
				s.produced = a.Params["Produced"]
			}
			out = append(out, s)
		}
		for i, t := range f.Triggers {
			if t.Effect == nil {
				continue
			}
			d := t.Mode
			if kw := t.Params["Keyword"]; kw != "" {
				d += "/" + kw
			}
			out = append(out, abilitySlot{key: fmt.Sprintf("f%d/t%d", fi, i), desc: d, sa: t.Effect})
		}
	}
	return out
}

// abilityKeys is abilityInventory's keys (already in a stable order).
func abilityKeys(c *cards.Card) []string {
	inv := abilityInventory(c)
	if len(inv) == 0 {
		return nil
	}
	out := make([]string, len(inv))
	for i, s := range inv {
		out[i] = s.key
	}
	return out
}

// abilityDescs maps key -> label for the report.
func abilityDescs(c *cards.Card) map[string]string {
	m := map[string]string{}
	for _, s := range abilityInventory(c) {
		m[s.key] = s.desc
	}
	return m
}

type slotRef struct {
	card string
	slot abilitySlot
}

// producesColour reports how a mana ability's Produced$ relates to one added
// mana letter: exact (the letter is one of its tokens), wild (it names a
// choice -- Any, Chosen, Combo Any, ManaReflected's empty -- that can make any
// colour) or neither.
func producesColour(produced string, colour byte) (exact, wild bool) {
	toks := strings.Fields(produced)
	if len(toks) == 0 {
		return false, true
	}
	for _, t := range toks {
		switch {
		case len(t) == 1 && t[0] == colour:
			exact = true
		case t == "Combo" || (len(t) == 1 && strings.ContainsRune("WUBRGC", rune(t[0]))):
		default:
			wild = true
		}
	}
	return exact, wild
}

// abilitiesUsed walks a finished game for which of its deck cards' own
// abilities were used, as card name -> set of inventory keys.
//
// Activated and triggered abilities: every AbilityPush/TriggerPush mints a
// stack object whose Ability is the compiled SA pointer from the source
// face's Abilities/Triggers slice (events/apply.go; the pointer identity
// state/stackkind.go also relies on), and ability objects stay in the arena
// after they resolve. So the walk is over the arena, not the log: an ability
// object whose Ability is a known slot's SA is a use of that slot, credited
// only when its Source (walked through tokens/copies by name) is the card
// that owns the slot -- a Clone's copied ability, a stack copy and a granted
// body (a fresh ResolveSVar SA) credit nothing.
//
// Mana abilities never use the stack and ManaAdd carries no source, so they
// are detected by proxy: a Tap of the source followed (before the next Tap,
// priority, decision or stack push) by a positive ManaAdd credits the
// source card's mana abilities whose Produced$ names that colour (else those
// with a wildcard Produced$, else all of them). An ability carrying an
// activation limit also logs an exact ManaActivate marker (flat pile index),
// which credits that slot directly. A mana ability with no tap cost and no
// limit (a sacrifice outlet like Ashnod's Altar) is therefore never seen.
func abilitiesUsed(e *rules.Engine, decks [][]*cards.Card) map[string]map[string]bool {
	bySA := map[*cards.SA][]slotRef{}
	mana := map[string][]abilitySlot{}
	seenCard := map[string]bool{}
	for _, d := range decks {
		for _, c := range d {
			n := cardName(c)
			if seenCard[n] {
				continue
			}
			seenCard[n] = true
			for _, s := range abilityInventory(c) {
				bySA[s.sa] = append(bySA[s.sa], slotRef{card: n, slot: s})
				if s.mana {
					mana[n] = append(mana[n], s)
				}
			}
		}
	}
	used := map[string]map[string]bool{}
	credit := func(card, key string) {
		m := used[card]
		if m == nil {
			m = map[string]bool{}
			used[card] = m
		}
		m[key] = true
	}
	name := func(id state.ObjID) string { return realCardName(e, id) }

	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Ability == nil || o.Card != nil || o.IsCopy || o.Source == 0 {
			continue
		}
		refs := bySA[o.Ability]
		if len(refs) == 0 {
			continue
		}
		owner := name(o.Source)
		for _, r := range refs {
			if r.card == owner {
				credit(r.card, r.slot.key)
			}
		}
	}

	// ownCard is the source's own card name when the object is a real card
	// that is not a token, copy or copy-effect permanent (whose abilities are
	// someone else's).
	ownCard := func(id state.ObjID) string {
		o := e.G.Obj(id)
		if o == nil || o.Card == nil || o.IsToken || o.IsCopy || o.CopyFace != nil {
			return ""
		}
		return cardName(o.Card)
	}
	evs := e.L.Events
	for i, ev := range evs {
		switch ev.Kind {
		case events.ManaActivate:
			if len(ev.IDs) > 0 {
				continue // a gained (foreign) mana ability
			}
			n := ownCard(ev.Obj)
			if n == "" {
				continue
			}
			if pa, ok := e.G.Obj(ev.Obj).PileAbilityAt(int(ev.Amount)); ok {
				for _, r := range bySA[pa.SA] {
					if r.card == n {
						credit(n, r.slot.key)
					}
				}
			}
		case events.Tap:
			n := ownCard(ev.Obj)
			slots := mana[n]
			if len(slots) == 0 {
				continue
			}
			colour := byte(0)
		scan:
			for j := i + 1; j < len(evs) && j <= i+8; j++ {
				switch evs[j].Kind {
				case events.ManaAdd:
					if evs[j].Amount > 0 {
						colour = 'C'
						if ctr := evs[j].Counter; ctr != "" {
							colour = ctr[len(ctr)-1]
						}
						break scan
					}
				case events.Tap, events.Priority, events.DecisionAsk, events.DecisionMade,
					events.PutOnStack, events.AbilityPush, events.TriggerPush, events.StepChange:
					break scan
				}
			}
			if colour == 0 {
				continue
			}
			var exact, wild []string
			for _, s := range slots {
				ex, wd := producesColour(s.produced, colour)
				if ex {
					exact = append(exact, s.key)
				} else if wd {
					wild = append(wild, s.key)
				}
			}
			keys := exact
			if len(keys) == 0 {
				keys = wild
			}
			if len(keys) == 0 {
				for _, s := range slots {
					keys = append(keys, s.key)
				}
			}
			for _, k := range keys {
				credit(n, k)
			}
		}
	}
	return used
}

// realCardName walks an object's Source chain (tokens, copies, ability
// objects) to the real card it came from, or "".
func realCardName(e *rules.Engine, id state.ObjID) string {
	for hops := 0; hops < 3; hops++ {
		o := e.G.Obj(id)
		if o == nil {
			return ""
		}
		if o.Card != nil && !o.IsToken && !o.IsCopy {
			return cardName(o.Card)
		}
		if o.Source == 0 || o.Source == id {
			return ""
		}
		id = o.Source
	}
	return ""
}
