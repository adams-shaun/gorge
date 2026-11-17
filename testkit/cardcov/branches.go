package cardcov

import (
	"fmt"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Branch keys are reporting only: they say which choice a game made on a
// card (an X, an alternative or additional cost, an optional trigger, a
// response, a copy ...), never whether the card is supported. They are kept
// apart from Keys so a persistent inventory of Keys does not move.
//
// Key forms: "f<f>/s" (the spell ability resolved), "<k>/x0" and "<k>/x+"
// (X was 0 or positive), "cast/alt:<flag>" and "cast/alt:none" (an
// alternative or additional cost paid, or none of a card's costs), "<t>/opt:y"
// and "<t>/opt:n" (an optional trigger accepted or declined), "ix:resp",
// "ix:copy", "ix:blink", "ix:ctrl" and "ix:mp".

// BranchInv is what a card owes the branch keys: facts read off the compiled
// card by BranchInventory, or written out by hand in a test.
type BranchInv struct {
	HasX        bool     // the card's mana cost has X
	AltFlags    []string // cast-flag names (events.FlagsString) of its alternative and additional costs
	OptTriggers []string // inventory keys ("f<f>/t<i>") of its optional triggers
	SpellFaces  []int    // faces with a spell ability, ascending
	Branches    []string // the owed branch keys, sorted
}

// altCostKeywords maps a keyword to the cast flag its payment sets. A keyword
// absent here is not an alternative or additional cost the cast log can show.
var altCostKeywords = map[string]uint64{
	"Kicker": state.FlagKicked, "Multikicker": state.FlagMultikicked, "Flashback": state.FlagFlashback,
	"Evoke": state.FlagEvoked, "Dash": state.FlagDashed, "Overload": state.FlagOverloaded,
	"Warp": state.FlagWarped, "Buyback": state.FlagBuyback, "Harmonize": state.FlagHarmonize,
	"Suspend": state.FlagSuspend, "Escape": state.FlagEscaped, "Surge": state.FlagSurged,
	"Miracle": state.FlagMiracle, "Bestow": state.FlagBestowed, "Foretell": state.FlagForetold,
	"Replicate": state.FlagReplicated, "Conspire": state.FlagConspired, "Mutate": state.FlagMutated,
	"Squad": state.FlagSquadPaid, "Offspring": state.FlagOffspringPaid, "Jump-start": state.FlagJumpstart,
	"Blitz": state.FlagBlitzed, "Mayhem": state.FlagMayhem, "Bargain": state.FlagBargained,
	"Impending": state.FlagImpending, "Cleave": state.FlagCleaved, "Prototype": state.FlagPrototype,
	"Morph": state.FlagMorphed, "Megamorph": state.FlagMegamorphed, "Disguise": state.FlagDisguised,
	"Sneak": state.FlagSneaked, "Web-slinging": state.FlagWebSlinged, "Gift": state.FlagPromisedGift,
	"Aftermath": state.FlagAftermath, "Fuse": state.FlagFused,
}

// BranchInventory reads c's owed branches (see BranchInv). Only a face's own
// spell ability, keywords, mana cost and non-bookkeeping optional triggers
// are read: nothing is derived from a granted or copied ability.
func BranchInventory(c *cards.Card) BranchInv {
	var bi BranchInv
	seen := map[string]bool{}
	owe := func(k string) {
		if !seen[k] {
			seen[k] = true
			bi.Branches = append(bi.Branches, k)
		}
	}
	for fi, f := range c.Faces {
		if f == nil {
			continue
		}
		x := strings.Contains(f.ManaCost, "X")
		if sa := f.SpellAbility(); sa != nil {
			bi.SpellFaces = append(bi.SpellFaces, fi)
			k := fmt.Sprintf("f%d/s", fi)
			owe(k)
			owe("ix:resp")
			if x {
				bi.HasX = true
				owe(k + "/x0")
				owe(k + "/x+")
			}
		}
		for _, kw := range f.Keywords {
			name, _, _ := strings.Cut(kw, ":")
			if bit, ok := altCostKeywords[strings.TrimSpace(name)]; ok {
				bi.AltFlags = append(bi.AltFlags, events.FlagsString(bit))
			}
		}
		for i, t := range f.Triggers {
			if t.Effect == nil || bookkeepingTrigger(t) {
				continue
			}
			owe("ix:resp")
			if t.ParamStr(cards.PKOptionalDecider) != "" {
				k := fmt.Sprintf("f%d/t%d", fi, i)
				bi.OptTriggers = append(bi.OptTriggers, k)
				owe(k + "/opt:y")
				owe(k + "/opt:n")
			}
		}
		for _, a := range f.Abilities {
			if a != nil && a.Kind == "AB" {
				owe("ix:resp")
			}
		}
	}
	bi.AltFlags = sortedUnique(bi.AltFlags)
	for _, a := range bi.AltFlags {
		owe("cast/alt:" + a)
	}
	if len(bi.AltFlags) > 0 {
		owe("cast/alt:none")
	}
	sort.Strings(bi.Branches)
	return bi
}

// spellKey is the key of the card's spell ability: its first spell face's,
// or face 0 when the inventory names none.
func spellKey(bi BranchInv) string {
	if len(bi.SpellFaces) > 0 {
		return fmt.Sprintf("f%d/s", bi.SpellFaces[0])
	}
	return "f0/s"
}

func sortedUnique(in []string) []string {
	sort.Strings(in)
	out := in[:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// castMarkerFlags are the CastInfo flags whose Amount is not X (a marker's 1,
// a list's length, or a mana total): engine/events' castInfoMarkerFlags and
// the provenance counts that ride a flag of their own.
const castMarkerFlags = state.FlagConspired | state.FlagTeamworkPaid | state.FlagConvoked |
	state.FlagManaSourcesSpent | state.FlagAddsCounters | state.FlagAddsKeywords |
	state.FlagManaColorSpent | state.FlagManaExpendCast | state.FlagManaSpent | state.FlagManaSnowSpent |
	state.FlagManaTreasureSpent | state.FlagManaCaveSpent | state.FlagManaDesertSpent |
	state.FlagManaArtifactSpent | state.FlagMultikicked | state.FlagReplicated | state.FlagConverged

// plainTriggerAmount bounds a TriggerPush Amount read as a face-0 trigger
// index: a face-packed or left-copy amount is far larger.
const plainTriggerAmount = 1 << 10

// stackEntry is one spell or ability on the stack, as far as the log shows.
type stackEntry struct {
	obj       state.ObjID // the spell's object; 0 for an ability (its object id is not logged)
	name      string      // the card it belongs to
	responded bool        // something was put above it
}

// castRec is the cast of one spell in progress: its flags and X.
type castRec struct {
	name  string
	flags uint64
	x     int32
	hasX  bool
}

// Branches reads the branch keys of one game's event log and intents. It is
// pure: no engine call, and nothing ranged into an order that reaches the
// output. owner maps an object to the card name it counts for ("" credits
// nothing: tokens, copies, unknown objects); inv gives that card's owed facts.
// seats is the game's seat count (ix:mp: a card used in a game of 3+ seats).
//
// Approximations, all reporting only: the optional-trigger decline is
// attributed to the object of the event just before the DecisionAsk (the log
// names no source for it), a resolved ability is matched to the top stack
// entry (its object id is not logged), and ix:ctrl credits a control change
// away from the controller the card first entered play or was cast under,
// when a trigger or activation of it is then pushed.
func Branches(evs []events.Event, intents []decision.Intent, seats int, owner func(state.ObjID) string, inv func(name string) BranchInv) map[string]map[string]int {
	out := map[string]map[string]int{}
	credit := func(name, key string) {
		if name == "" {
			return
		}
		if out[name] == nil {
			out[name] = map[string]int{}
		}
		out[name][key]++
	}
	fired := map[string]bool{}
	var stack []stackEntry
	casts := map[state.ObjID]*castRec{}
	endCast := func(id state.ObjID) {
		r := casts[id]
		if r == nil {
			return
		}
		delete(casts, id)
		bi := inv(r.name)
		if bi.HasX && r.hasX {
			k := spellKey(bi)
			if r.x == 0 {
				credit(r.name, k+"/x0")
			} else {
				credit(r.name, k+"/x+")
			}
		}
		if len(bi.AltFlags) > 0 {
			paid := false
			for _, part := range strings.Split(events.FlagsString(r.flags), ",") {
				for _, a := range bi.AltFlags {
					if part == a {
						credit(r.name, "cast/alt:"+a)
						paid = true
					}
				}
			}
			if !paid {
				credit(r.name, "cast/alt:none")
			}
		}
	}
	home := map[state.ObjID]state.PlayerID{}
	away := map[state.ObjID]bool{}
	left := map[state.ObjID]bool{}
	optAt := optionalAsks(evs, intents)
	// castOrder fixes the order cast records end in at the log's close, so
	// no map is ranged into the result.
	var castOrder []state.ObjID

	push := func(e stackEntry) {
		for i := range stack {
			stack[i].responded = true
		}
		stack = append(stack, e)
	}
	pop := func(id state.ObjID) (stackEntry, bool) {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].obj == id && id != 0 {
				e := stack[i]
				stack = append(stack[:i], stack[i+1:]...)
				return e, true
			}
		}
		if n := len(stack); n > 0 && stack[n-1].obj == 0 {
			e := stack[n-1]
			stack = stack[:n-1]
			return e, true
		}
		return stackEntry{}, false
	}
	for i, ev := range evs {
		switch ev.Kind {
		case events.PutOnStack:
			if ev.To != state.ZStack {
				continue
			}
			name := owner(ev.Obj)
			if name == "" {
				// no card of the decks (a token or copy spell): it is still
				// something put above whatever is below it
				push(stackEntry{obj: ev.Obj})
				continue
			}
			endCast(ev.Obj)
			casts[ev.Obj] = &castRec{name: name}
			castOrder = append(castOrder, ev.Obj)
			fired[name] = true
			if _, ok := home[ev.Obj]; !ok {
				home[ev.Obj] = ev.Player
			}
			push(stackEntry{obj: ev.Obj, name: name})
		case events.CastInfo:
			r := casts[ev.Obj]
			if r == nil {
				continue
			}
			f := events.FlagsFrom(ev.Counter)
			r.flags |= f
			if f&castMarkerFlags == 0 {
				r.x, r.hasX = ev.Amount, true
			}
		case events.AbilityPush, events.TriggerPush:
			name := owner(ev.Obj)
			if name == "" {
				// the source is no real card (a token, a copy): the entry
				// still sits on the stack, so a later resolve pops it
				push(stackEntry{})
				continue
			}
			fired[name] = true
			push(stackEntry{name: name})
			if away[ev.Obj] {
				credit(name, "ix:ctrl")
			}
			if ev.Kind == events.TriggerPush {
				creditOptional(optAt, i, ev, name, inv, credit)
			}
		case events.Resolve:
			if e, ok := pop(ev.Obj); ok {
				if bi := inv(e.name); e.obj != 0 && len(bi.SpellFaces) > 0 {
					credit(e.name, spellKey(bi))
				}
				if e.responded {
					credit(e.name, "ix:resp")
				}
			}
		case events.DecisionAsk:
			// a declined optional trigger leaves no push: attribute it to the
			// object of the event before the ask, when that card has exactly
			// one optional trigger
			if yes, ok := optAt[i]; ok && !yes && i > 0 {
				name := owner(evs[i-1].Obj)
				if ot := inv(name).OptTriggers; name != "" && len(ot) == 1 {
					credit(name, ot[0]+"/opt:n")
				}
			}
		case events.StackCopy:
			for _, e := range stack {
				if e.obj != 0 && e.obj == ev.Obj {
					credit(e.name, "ix:copy")
				}
			}
		case events.MoveZone:
			if ev.From == state.ZStack {
				pop(ev.Obj)
			}
			switch {
			case ev.From == state.ZBattlefield && ev.To != state.ZBattlefield:
				left[ev.Obj] = true
				delete(away, ev.Obj)
				delete(home, ev.Obj)
			case ev.To == state.ZBattlefield:
				if left[ev.Obj] {
					credit(owner(ev.Obj), "ix:blink")
					delete(left, ev.Obj)
				}
				if _, ok := home[ev.Obj]; !ok {
					home[ev.Obj] = ev.Player
				}
			}
		case events.ControlChange:
			if h, ok := home[ev.Obj]; ok {
				away[ev.Obj] = ev.Player != h
			}
		case events.LandPlayed:
			if name := owner(playedLand(evs, i)); name != "" {
				fired[name] = true
			}
		}
	}
	for _, id := range castOrder {
		endCast(id)
	}
	if seats >= 3 {
		for name := range fired {
			credit(name, "ix:mp")
		}
	}
	return out
}

// optionalAsks maps the index of each DecisionAsk of a KTriggerOptional the
// intents answered to whether the answer was yes. The intent's Seq is the
// event count when the decision was posed, so it is the index of the
// DecisionAsk it posed.
func optionalAsks(evs []events.Event, intents []decision.Intent) map[int]bool {
	m := map[int]bool{}
	for _, in := range intents {
		if in.Seq >= uint64(len(evs)) || len(in.Choices) != 1 {
			continue
		}
		ev := evs[in.Seq]
		if ev.Kind == events.DecisionAsk && ev.Text == string(decision.KTriggerOptional) {
			m[int(in.Seq)] = in.Choices[0] == 0
		}
	}
	return m
}

// creditOptional credits the opt:y of a TriggerPush that an accepted
// optional-trigger ask (a DecisionAsk within the few events before it) led to.
func creditOptional(optAt map[int]bool, i int, ev events.Event, name string, inv func(string) BranchInv, credit func(name, key string)) {
	if ev.Amount < 0 || ev.Amount >= plainTriggerAmount {
		return
	}
	key := fmt.Sprintf("f0/t%d", ev.Amount)
	for _, k := range inv(name).OptTriggers {
		if k == key {
			for j := i - 1; j >= 0 && j >= i-3; j-- {
				if yes, ok := optAt[j]; ok && yes {
					credit(name, key+"/opt:y")
					return
				}
			}
		}
	}
}
