package effects

import (
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/state"
)

// A Forge filter spec is alternatives separated by "," (OR). Each alternative
// is a base type, optionally negated with a "non" prefix, followed by
// ".pred+pred+..." (AND).
//
// Unknown predicates never match. A filter that silently widens is how a rules
// engine quietly does the wrong thing, so the failure mode is "this card does
// nothing", which testing catches, rather than "this card does too much",
// which it does not.

type predFn func(g *state.Game, o *state.Object, you state.PlayerID, source state.ObjID) bool

var predicates = map[string]predFn{
	"YouCtrl": func(g *state.Game, o *state.Object, you state.PlayerID, _ state.ObjID) bool {
		return o.Controller == you
	},
	"YouDontCtrl": func(g *state.Game, o *state.Object, you state.PlayerID, _ state.ObjID) bool {
		return o.Controller != you
	},
	"OppCtrl": func(g *state.Game, o *state.Object, you state.PlayerID, _ state.ObjID) bool {
		return o.Controller != you
	},
	"YouOwn":    func(g *state.Game, o *state.Object, you state.PlayerID, _ state.ObjID) bool { return o.Owner == you },
	"OppOwn":    func(g *state.Game, o *state.Object, you state.PlayerID, _ state.ObjID) bool { return o.Owner != you },
	"Self":      func(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool { return o.ID == src },
	"Other":     func(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool { return o.ID != src },
	"tapped":    func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool { return o.Tapped },
	"untapped":  func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool { return !o.Tapped },
	"attacking": func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool { return o.IsAttacking },
	"blocking":  func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool { return isBlocking(g, o.ID) },
	"token":     func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool { return o.Card == nil },
	"!token":    func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool { return o.Card != nil },
	"Legendary": func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return hasType(o, "Legendary")
	},
	// Snow is a supertype used as a predicate in Count$Valid specs (Withering
	// Wisps' "Swamp.Snow+YouCtrl"). hasType already sees the Snow type word
	// (Types: Basic Snow Land Swamp), so the predicate is the same shape as
	// Legendary above; without it such a spec failed closed to zero, which is
	// why a computed ActivationLimit of "number of snow Swamps you control"
	// silently became 0.
	"Snow": func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return hasType(o, "Snow")
	},
	"nonLand": func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool { return !hasType(o, "Land") },
	"nonCreature": func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return !hasType(o, "Creature")
	},
	// nonBasic / nonBlack close the two most common non* predicates the
	// transaction target census reads (Wasteland's Land.nonBasic, an
	// Executioner's Capsule's Creature.nonBlack). A basic land carries the
	// "Basic" type word; nonBlack is a colour test, not a type test.
	"nonBasic": func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return !hasType(o, "Basic")
	},
	"nonBlack": func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return !strings.Contains(ColorsOf(o), "B")
	},
	"kicked": func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return o.CastFlags&state.FlagKicked != 0
	},
	"surged": func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return o.CastFlags&state.FlagSurged != 0
	},
	"NamedCard": func(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
		s := g.Obj(src)
		return s != nil && s.ChosenName != "" && o.Face() != nil && o.Face().Name == s.ChosenName
	},
	"ChosenType": func(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
		s := g.Obj(src)
		return s != nil && s.ChosenType != "" && hasType(o, s.ChosenType)
	},
}

// colorLetter maps a colour's English name to its WUBRG letter -- note Blue
// is "U", not "B" (col[:1] would collide with Black). Colour predicates
// (and their non<X> negations) read ColorsOf, not the face directly, so a
// Devoid card (effects.ColorsOf) matches no colour predicate, Green included.
var colorLetter = map[string]string{"White": "W", "Blue": "U", "Black": "B", "Red": "R", "Green": "G"}

func init() {
	for _, kw := range [...]string{"Flying", "Trample", "Deathtouch", "Lifelink",
		"Vigilance", "Reach", "Haste", "Indestructible", "First Strike", "Menace"} {
		k := kw
		predicates["with"+strings.ReplaceAll(k, " ", "")] = func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
			return o.Face() != nil && o.Face().HasKeyword(k)
		}
		predicates["without"+strings.ReplaceAll(k, " ", "")] = func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
			return o.Face() == nil || !o.Face().HasKeyword(k)
		}
	}
	// These read ColorsOf, not the face directly, so Devoid (effects.ColorsOf)
	// correctly stops a card from matching any colour predicate, Green included.
	for _, c := range [...]string{"White", "Blue", "Black", "Red", "Green"} {
		letter := colorLetter[c]
		predicates[c] = func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
			return strings.Contains(ColorsOf(o), letter)
		}
	}
	// StrictlyOther is Forge's other spelling of the same "not the source"
	// test Other already implements.
	predicates["StrictlyOther"] = predicates["Other"]
	// EquippedBy / EnchantedBy / AttachedBy: the candidate is the permanent
	// source is attached to (attachedBy below). Task 14 wires all three to the
	// same predicate -- Forge spells "attached to" three ways depending on
	// whether the source is Equipment, an Aura, or a generic script.
	predicates["EquippedBy"] = attachedBy
	predicates["EnchantedBy"] = attachedBy
	predicates["AttachedBy"] = attachedBy
}

// attachedBy reports whether o is the permanent src is currently attached
// to. Affected$ Creature.EquippedBy on an Equipment's static matches exactly
// the equipped creature: source.AttachedTo is that creature's ID, and the
// candidate must be the object that field names. Only a battlefield source
// is a possible attachment (an Aura/Equipment that is not a permanent cannot
// be "attached" to anything), so a non-battlefield src matches no candidate.
func attachedBy(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
	s := g.Obj(src)
	return s != nil && s.AttachedTo == o.ID && s.Zone == state.ZBattlefield
}

// nonPredicate reports whether predicate p has the generic negation shape
// non<X>, and how to evaluate it. For <X> a colour name it returns that
// colour's WUBRG letter (isType=false); for <X> a type/supertype/subtype word
// in the corpus vocabulary it returns isType=true. ok is false for a p that is
// not a non<X> shape at all, or whose <X> is neither a colour nor a known type
// word -- the caller must treat that as an unknown predicate and fail closed,
// never as an always-true !hasType. The four legacy non* entries in `predicates`
// (nonLand/nonCreature/nonBasic/nonBlack) are matched there first and never
// reach this path, but this path reproduces their result exactly, so the
// handwritten entries could be deleted without changing behaviour.
func nonPredicate(p string) (x string, letter string, isType bool, ok bool) {
	x, has := strings.CutPrefix(p, "non")
	if !has || x == "" {
		return "", "", false, false
	}
	if l, is := colorLetter[x]; is {
		return x, l, false, true
	}
	if predicateTypeWords[x] {
		return x, "", true, true
	}
	return x, "", false, false
}

func hasType(o *state.Object, t string) bool {
	f := o.Face()
	if f == nil {
		return false
	}
	for _, x := range f.Types {
		if strings.EqualFold(x, t) {
			return true
		}
	}
	return false
}

func isBlocking(g *state.Game, id state.ObjID) bool {
	for i := range g.Objs {
		for _, b := range g.Objs[i].BlockedBy {
			if b == id {
				return true
			}
		}
	}
	return false
}

// noResolve is the resolver used whenever a caller has none of its own
// (MatchesSpec/MatchesSpecFrom, and the shape-only checks in
// UnknownPredicates/KnownPredicates below): every non-literal numeric RHS is
// a recognised shape that never matches, never a hard "unknown predicate".
func noResolve(string) (int32, bool) { return 0, false }

// numericPred handles the "<field><CMP><n>" family: powerLE2, cmcGE3, and so
// on, plus a right-hand side that is not a literal integer ("cmcEQY",
// "cmcEQChosen", "powerGEX"), which resolve looks up by name -- typically an
// {X} paid or a Chosen* value SpecContext.Resolve closes over. Returns
// ok=false when the token is not of this shape at all; ok=true with
// result=false when the shape is recognised but the RHS did not resolve, so
// a filter spec is either a hard "no" or "not this predicate", never a
// silent match.
func numericPred(name string, g *state.Game, o *state.Object, sc SpecContext) (result, ok bool) {
	resolve := sc.Resolve
	if resolve == nil {
		resolve = noResolve
	}
	// counters_<CMP><n>_<KIND>: a counter-kind comparison, e.g. counters_EQ0_P1P1
	// ("no +1/+1 counters", the Undying condition). Reads the object's current
	// counter count of KIND off the object it is applied to -- which for a
	// zone-change trigger is the LKI snapshot, so a "dies" condition sees what
	// the permanent had the moment it left the battlefield, not the reset state
	// Move leaves behind (events/apply.go's Move clears Counters).
	if strings.HasPrefix(name, "counters_") {
		rest := name[len("counters_"):]
		// Rest is "<CMP><n>_<KIND>"; at minimum "EQ0_A".
		if len(rest) < 4 {
			return false, false
		}
		cmp := rest[:2]
		numStr, kind, okSplit := strings.Cut(rest[2:], "_")
		if !okSplit || kind == "" {
			return false, false
		}
		n, err := strconv.Atoi(numStr)
		if err != nil {
			v, resolved := resolve(numStr)
			if !resolved {
				return false, true // recognised shape, unresolvable RHS never matches
			}
			n = int(v)
		}
		have := o.Counter(kind)
		target := int32(n)
		switch cmp {
		case "LE":
			return have <= target, true
		case "GE":
			return have >= target, true
		case "EQ":
			return have == target, true
		case "LT":
			return have < target, true
		case "GT":
			return have > target, true
		}
		return false, false
	}
	for _, field := range [...]string{"power", "toughness", "cmc"} {
		if !strings.HasPrefix(name, field) {
			continue
		}
		rest := name[len(field):]
		if len(rest) < 3 {
			return false, false
		}
		cmp, numStr := rest[:2], rest[2:]
		n, err := strconv.Atoi(numStr)
		if err != nil {
			v, resolved := resolve(numStr)
			if !resolved {
				return false, true // recognised shape, unresolvable RHS never matches
			}
			n = int(v)
		}
		f := o.Face()
		if f == nil {
			return false, true
		}
		var have int
		switch field {
		case "power":
			have = f.Power() + int(o.Counter("P1P1"))
		case "toughness":
			have = f.Toughness() + int(o.Counter("P1P1"))
		case "cmc":
			// CR 202.3e: {X} counts as its chosen value in a spell's mana
			// value. A caller that has the chosen X in hand (the CR 601.2e
			// post-announcement cast-illegality recheck) passes it through
			// SpecContext.ManaValue; otherwise the printed cost is used,
			// which counts an un-chosen X as 0, exactly as the offer-time
			// 601.3a check must.
			if sc.HasManaValue {
				have = int(sc.ManaValue)
			} else {
				have = int(parseCMC(f.ManaCost))
			}
		}
		switch cmp {
		case "LE":
			return have <= n, true
		case "GE":
			return have >= n, true
		case "EQ":
			return have == n, true
		case "LT":
			return have < n, true
		case "GT":
			return have > n, true
		}
		return false, false
	}
	return false, false
}

// parseCMC counts a mana cost's converted value without importing rules.
func parseCMC(cost string) int32 {
	cost = strings.NewReplacer("{", " ", "}", " ").Replace(cost)
	if strings.EqualFold(strings.TrimSpace(cost), "no cost") {
		return 0
	}
	var n int32
	for _, sym := range strings.Fields(cost) {
		if v, err := strconv.Atoi(sym); err == nil {
			n += int32(v)
			continue
		}
		if sym != "X" {
			n++
		}
	}
	return n
}

// matchesBase handles the base type, including a "non" prefix.
func matchesBase(g *state.Game, base string, o *state.Object) bool {
	if neg := strings.TrimPrefix(base, "non"); neg != base {
		return !matchesBase(g, neg, o)
	}
	switch base {
	case "Any", "Card":
		return true
	case "Permanent":
		return o.Zone == state.ZBattlefield
	case "Spell":
		return o.Zone == state.ZStack
	}
	return hasType(o, base)
}

// SpecContext carries the extra state a filter spec beyond MatchesSpec's
// three plain arguments needs: the perspective seat, the effect's source
// (CARDNAME/Self/Other/StrictlyOther/NamedCard/ChosenType are relative to it), and
// an optional resolver for a numeric predicate whose right-hand side is not a
// literal (an SVar name such as "Y" or "Chosen"). A nil Resolve leaves that
// family of RHS forever unresolvable -- MatchesSpec/MatchesSpecFrom's
// contract -- rather than guessing at what the name might mean.
type SpecContext struct {
	You     state.PlayerID
	Source  state.ObjID
	Resolve func(name string) (int32, bool)
	// ManaValue overrides the object's mana value for cmc predicates, with
	// HasManaValue set. It carries the CR 202.3e chosen-X effect: a caller
	// that has the chosen {X} passes the resulting mana value here so a
	// cmc restriction re-checked late (CR 601.2e) sees the spell as it is,
	// not as it was offered. Zero value with HasManaValue false is the
	// ordinary path (the printed cost, X as 0).
	ManaValue    int32
	HasManaValue bool
}

// MatchesObjectCtx applies one Forge filter spec to an object VALUE rather
// than to a live-game id -- the same grammar (base type plus .A+B+...
// predicate conjunction, alternatives ORed on ",") MatchesSpecCtx applies to
// `g.Obj(id)`, but with the object handed in, so a caller can match against
// something that is not (or is no longer) reachable by id: the last-known-
// information snapshot a zone-change trigger holds (effects.Ctx.LKI, CR
// 603.10), or a card that has since left the battlefield. An IsCopy object
// that has left the stack (CR 707.10h: a copy that changes zones ceases to
// exist) never matches anything regardless of spec.
func MatchesObjectCtx(g *state.Game, spec string, o *state.Object, sc SpecContext) bool {
	if o == nil {
		return false
	}
	if o.IsCopy && o.Zone != state.ZStack {
		return false
	}
	resolve := sc.Resolve
	if resolve == nil {
		resolve = noResolve
	}
	for _, alt := range strings.Split(spec, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		base, rest, _ := strings.Cut(alt, ".")
		if base == "CARDNAME" {
			// CR 201.5: a self-reference means this object, not another
			// object with the same name. Without a source, fail closed.
			if sc.Source == 0 || o.ID != sc.Source {
				continue
			}
		} else if !matchesBase(g, base, o) {
			continue
		}
		all := true
		for _, p := range strings.Split(rest, "+") {
			if p == "" {
				continue
			}
			if fn, ok := predicates[p]; ok {
				if !fn(g, o, sc.You, sc.Source) {
					all = false
					break
				}
				continue
			}
			if res, ok := numericPred(p, g, o, sc); ok {
				if !res {
					all = false
					break
				}
				continue
			}
			// Generic non<X> negation. An unknown <X> (neither a colour nor a
			// type word, e.g. nonFrobnicate) is unknown, so it fails closed --
			// the alternative, !hasType, would always match and silently widen
			// the filter.
			if x, letter, isType, ok := nonPredicate(p); ok {
				if (isType && hasType(o, x)) || (!isType && strings.Contains(ColorsOf(o), letter)) {
					all = false
					break
				}
				continue
			}
			all = false // unknown predicate: never match
			break
		}
		if all {
			return true
		}
	}
	return false
}

// MatchesSpecCtx is MatchesSpec/MatchesSpecFrom's full form: the same
// grammar as MatchesObjectCtx, applied to the object g.Obj(id) names.
func MatchesSpecCtx(g *state.Game, spec string, id state.ObjID, sc SpecContext) bool {
	o := g.Obj(id)
	if o == nil {
		return false
	}
	return MatchesObjectCtx(g, spec, o, sc)
}

// MatchesSpecFrom is MatchesSpecCtx with an explicit source object, which the
// CARDNAME base and Self/Other predicates are relative to, and no numeric-RHS
// resolver.
func MatchesSpecFrom(g *state.Game, spec string, id state.ObjID, you state.PlayerID, source state.ObjID) bool {
	return MatchesSpecCtx(g, spec, id, SpecContext{You: you, Source: source})
}

// MatchesSpec reports whether an object matches a Forge filter spec.
func MatchesSpec(g *state.Game, spec string, id state.ObjID, you state.PlayerID) bool {
	return MatchesSpecFrom(g, spec, id, you, 0)
}

// MatchesPlayerSpec is the player-side filter: You, Opponent, Player.
// Unknown qualifiers fail closed so restrictions and triggers are not widened.
func MatchesPlayerSpec(g *state.Game, spec string, p, you state.PlayerID) bool {
	for _, alt := range strings.Split(spec, ",") {
		base, qualifier, qualified := strings.Cut(strings.TrimSpace(alt), ".")
		switch base {
		case "Player", "Any":
			if !qualified {
				return true
			}
			switch qualifier {
			case "You":
				if p == you {
					return true
				}
			case "Opponent", "Other":
				if p != you {
					return true
				}
			}
		case "You":
			if !qualified && p == you {
				return true
			}
		case "Opponent", "Other":
			if !qualified && p != you {
				return true
			}
		}
	}
	return false
}

// UnknownPredicates lists tokens in a spec this build does not implement. The
// card-validation pass uses it to refuse cards it would otherwise misplay.
func UnknownPredicates(spec string) []string {
	var out []string
	for _, alt := range strings.Split(spec, ",") {
		_, rest, _ := strings.Cut(strings.TrimSpace(alt), ".")
		for _, p := range strings.Split(rest, "+") {
			if p == "" {
				continue
			}
			if _, ok := predicates[p]; ok {
				continue
			}
			if _, ok := numericPred(p, nil, &state.Object{}, SpecContext{}); ok {
				continue
			}
			if _, _, _, ok := nonPredicate(p); ok {
				continue
			}
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// KnownPredicates lists the predicates this build implements, in sorted order.
func KnownPredicates() []string {
	out := make([]string, 0, len(predicates))
	for k := range predicates {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
