package main

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
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
//   - a Static$ True bookkeeping trigger with no description
//     (bookkeepingTrigger);
//   - abilities granted by another card or gained from one (GrantTriggerPush,
//     GainedAbilityPush, KeywordAbilityPush, a Clone's copied abilities):
//     they are not the card's own, so they credit nothing.
type abilitySlot struct {
	key  string // stable id: "f<face>/a<index>" or "f<face>/t<index>"
	desc string // human label for the report: API or trigger mode
	sa   *cards.SA
	mana bool
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
			out = append(out, abilitySlot{key: fmt.Sprintf("f%d/t%d", fi, i), desc: d, sa: t.Effect})
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

// useProbe is one game's engine-side observations the log cannot give back:
// every mana ability the engine resolved (rules.Engine.ManaAbilityHook) and
// every printed activated ability it offered a seat as a priority "ability"
// option. It is filled while the game plays (install, observe) and read by
// abilitiesUsed / abilitiesOffered afterwards.
type useProbe struct {
	mana    []probeRef
	offered []probeRef
}

// probeRef names an ability by its compiled pointer and the object it was
// activated from / offered on.
type probeRef struct {
	src state.ObjID
	sa  *cards.SA
}

// install arms the probe on a fresh engine (gbench.Hooks.Setup).
func (u *useProbe) install(e *rules.Engine) {
	e.ManaAbilityHook = func(_ state.PlayerID, source state.ObjID, sa *cards.SA) {
		u.mana = append(u.mana, probeRef{src: source, sa: sa})
	}
}

// observe records the pending priority decision's printed-ability offers. It
// runs from the drive loop's per-decision Guard, so it sees every decision
// before it is answered. Granted, keyword-granted and gained options (SVar,
// Keyword, GainedSource anchors) are not the source card's own abilities
// and record nothing. Offers repeat every priority window, so a ref already
// recorded this game is not appended again.
func (u *useProbe) observe(e *rules.Engine, seen map[probeRef]bool) {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return
	}
	for _, o := range d.Options {
		if o.Kind != "ability" || o.SVar != "" || o.Keyword != "" || o.GainedSource != 0 || o.Ability < 0 {
			continue
		}
		obj := e.G.Obj(o.Obj)
		if obj == nil {
			continue
		}
		pa, ok := obj.PileAbilityAt(o.Ability)
		if !ok || pa.SA == nil {
			continue
		}
		r := probeRef{src: o.Obj, sa: pa.SA}
		if seen[r] {
			continue
		}
		seen[r] = true
		u.offered = append(u.offered, r)
	}
}

// inventoryIndex maps every deck card's compiled ability pointers to the
// inventory slots they are.
func inventoryIndex(decks [][]*cards.Card) map[*cards.SA][]slotRef {
	bySA := map[*cards.SA][]slotRef{}
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
			}
		}
	}
	return bySA
}

// ownCardName is the object's own card name when it is a real card that is
// not a token, copy or copy-effect permanent (whose abilities are someone
// else's), else "".
func ownCardName(e *rules.Engine, id state.ObjID) string {
	o := e.G.Obj(id)
	if o == nil || o.Card == nil || o.IsToken || o.IsCopy || o.CopyFace != nil {
		return ""
	}
	return cardName(o.Card)
}

func creditKey(out map[string]map[string]bool, card, key string) {
	m := out[card]
	if m == nil {
		m = map[string]bool{}
		out[card] = m
	}
	m[key] = true
}

// creditRefs credits, for each probe ref whose source object is the card
// owning the referenced slot, that slot's key.
func creditRefs(e *rules.Engine, bySA map[*cards.SA][]slotRef, refs []probeRef, out map[string]map[string]bool) {
	for _, r := range refs {
		slots := bySA[r.sa]
		if len(slots) == 0 {
			continue
		}
		n := ownCardName(e, r.src)
		if n == "" {
			continue
		}
		for _, sr := range slots {
			if sr.card == n {
				creditKey(out, n, sr.slot.key)
			}
		}
	}
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
// are credited EXACTLY from the probe's engine hook (rules.Engine.
// ManaAbilityHook): the resolved ability's compiled pointer and its source
// object, whatever its cost (tap, sacrifice, life, none) and whatever colour
// choice it asked. The hook is harness-only and emits nothing, so the log
// and every chain head are unchanged. It replaced a Tap-then-ManaAdd log
// proxy that stopped scanning at the first decision, so it missed every
// ability that asks its colour between the Tap and the ManaAdd (every "one
// mana of any colour" source: Manalith, Darksteel Ingot, Mox Opal ...) and
// every mana ability without a {T} cost.
func abilitiesUsed(e *rules.Engine, decks [][]*cards.Card, probe *useProbe) map[string]map[string]bool {
	bySA := inventoryIndex(decks)
	used := map[string]map[string]bool{}
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Ability == nil || o.Card != nil || o.IsCopy || o.Source == 0 {
			continue
		}
		refs := bySA[o.Ability]
		if len(refs) == 0 {
			continue
		}
		owner := realCardName(e, o.Source)
		for _, r := range refs {
			if r.card == owner {
				creditKey(used, r.card, r.slot.key)
			}
		}
	}
	if probe != nil {
		creditRefs(e, bySA, probe.mana, used)
	}
	return used
}

// abilitiesOffered is the probe's offer record in abilitiesUsed's shape:
// which of each deck card's own activated abilities the engine offered its
// controller as a priority action at least once. A never-used key that was
// offered is a seat-policy gap (it was legal and never chosen); one never
// offered is an engine offer gap or a board the games never reached.
func abilitiesOffered(e *rules.Engine, decks [][]*cards.Card, probe *useProbe) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	if probe != nil {
		creditRefs(e, inventoryIndex(decks), probe.offered, out)
	}
	return out
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
