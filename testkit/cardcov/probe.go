package cardcov

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// Probe is one game's engine-side observations the log cannot give back:
// every mana ability the engine resolved (rules.Engine.ManaAbilityHook) and
// every printed activated ability it offered a seat as a priority "ability"
// option. It is filled while the game plays (Install, Observe) and read by
// Used / Offered afterwards.
type Probe struct {
	mana    []probeRef
	offered []probeRef
	seen    map[probeRef]bool
}

// probeRef names an ability by its compiled pointer and the object it was
// activated from / offered on.
type probeRef struct {
	src state.ObjID
	sa  *cards.SA
}

// Install arms the probe on a fresh engine (gbench.Hooks.Setup).
func (u *Probe) Install(e *rules.Engine) {
	e.ManaAbilityHook = func(_ state.PlayerID, source state.ObjID, sa *cards.SA) {
		u.mana = append(u.mana, probeRef{src: source, sa: sa})
	}
}

// Observe records the pending priority decision's printed-ability offers. It
// runs from the drive loop's per-decision Guard, so it sees every decision
// before it is answered. Granted, keyword-granted and gained options (SVar,
// Keyword, GainedSource anchors) are not the source card's own abilities
// and record nothing. Offers repeat every priority window, so a ref already
// recorded this game is not appended again.
func (u *Probe) Observe(e *rules.Engine) {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return
	}
	if u.seen == nil {
		u.seen = map[probeRef]bool{}
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
		if u.seen[r] {
			continue
		}
		u.seen[r] = true
		u.offered = append(u.offered, r)
	}
}

type slotRef struct {
	card string
	slot Slot
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
			for _, s := range Inventory(c) {
				bySA[s.SA] = append(bySA[s.SA], slotRef{card: n, slot: s})
			}
		}
	}
	return bySA
}

type modeRef struct {
	card string
	slot modeSlot
}

// modeIndex maps every deck card's modal ability pointers to their slots.
func modeIndex(decks [][]*cards.Card) map[*cards.SA][]modeRef {
	bySA := map[*cards.SA][]modeRef{}
	seenCard := map[string]bool{}
	for _, d := range decks {
		for _, c := range d {
			n := cardName(c)
			if seenCard[n] {
				continue
			}
			seenCard[n] = true
			for _, m := range modeSlots(c) {
				bySA[m.sa] = append(bySA[m.sa], modeRef{card: n, slot: m})
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
				creditKey(out, n, sr.slot.Key)
			}
		}
	}
}

// Used walks a finished game for which of its deck cards' own abilities were
// used, as card name -> set of keys.
//
// Activated and triggered abilities: every AbilityPush/TriggerPush mints a
// stack object whose Ability is the compiled SA pointer from the source
// face's Abilities/Triggers slice (engine/events/apply.go; the pointer identity
// engine/state/stackkind.go also relies on), and ability objects stay in the arena
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
// and every chain head are unchanged.
//
// Modes: each events.ModeChosen credits the ModeKeys keys of the modal
// ability its object is (a spell on the stack, or a trigger/ability object),
// for every mode whose label is among the event's chosen labels.
func Used(e *rules.Engine, decks [][]*cards.Card, p *Probe) map[string]map[string]bool {
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
				creditKey(used, r.card, r.slot.Key)
			}
		}
	}
	if p != nil {
		creditRefs(e, bySA, p.mana, used)
	}
	creditModes(e, decks, used)
	return used
}

// creditModes credits the mode keys of every ModeChosen in e's log.
func creditModes(e *rules.Engine, decks [][]*cards.Card, used map[string]map[string]bool) {
	var byMode map[*cards.SA][]modeRef
	for _, ev := range e.L.Events {
		if ev.Kind != events.ModeChosen || ev.Text == "" || ev.Counter == events.DilemmaReversed {
			continue
		}
		o := e.G.Obj(ev.Obj)
		if o == nil || o.IsToken || o.IsCopy {
			continue
		}
		var sa *cards.SA
		var owner string
		switch {
		case o.Ability != nil && o.Card == nil:
			sa, owner = o.Ability, realCardName(e, o.Source)
		case o.Card != nil:
			sa, owner = o.Face().SpellAbility(), ownCardName(e, ev.Obj)
		}
		if sa == nil || owner == "" {
			continue
		}
		if byMode == nil {
			byMode = modeIndex(decks)
		}
		chosen := "," + ev.Text + ","
		for _, r := range byMode[sa] {
			if r.card != owner {
				continue
			}
			for k, label := range r.slot.labels {
				if label != "" && strings.Contains(chosen, ","+label+",") {
					creditKey(used, owner, r.slot.modeKey(k))
				}
			}
		}
	}
}

// Offered is the probe's offer record in Used's shape: which of each deck
// card's own activated abilities the engine offered its controller as a
// priority action at least once. A never-used key that was offered is a
// seat-policy gap (it was legal and never chosen); one never offered is an
// engine offer gap or a board the games never reached.
func Offered(e *rules.Engine, decks [][]*cards.Card, p *Probe) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	if p != nil {
		creditRefs(e, inventoryIndex(decks), p.offered, out)
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

// RealCardName is realCardName for harnesses that name objects by card.
func RealCardName(e *rules.Engine, id state.ObjID) string { return realCardName(e, id) }
