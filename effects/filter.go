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
	"nonLand": func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool { return !hasType(o, "Land") },
	"nonCreature": func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return !hasType(o, "Creature")
	},
}

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
	for _, c := range [...]string{"White", "Blue", "Black", "Red", "Green"} {
		col := c
		predicates[col] = func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
			return o.Face() != nil && strings.Contains(o.Face().Colors, col[:1])
		}
	}
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

// numericPred handles the "<field><CMP><n>" family: powerLE2, cmcGE3, and so
// on. Returns ok=false when the token is not of that shape.
func numericPred(name string, g *state.Game, o *state.Object) (result, ok bool) {
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
			return false, false // "cmcLEX" and friends need Count$; Task 17
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
			have = int(parseCMC(f.ManaCost))
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
	case "Any", "Card", "Permanent":
		return true
	case "Spell":
		return o.Zone == state.ZStack
	}
	return hasType(o, base)
}

// MatchesSpec reports whether an object matches a Forge filter spec.
func MatchesSpec(g *state.Game, spec string, id state.ObjID, you state.PlayerID) bool {
	return MatchesSpecFrom(g, spec, id, you, 0)
}

// MatchesSpecFrom is MatchesSpec with an explicit source object, which the
// Self and Other predicates are relative to.
func MatchesSpecFrom(g *state.Game, spec string, id state.ObjID, you state.PlayerID, source state.ObjID) bool {
	o := g.Obj(id)
	if o == nil {
		return false
	}
	for _, alt := range strings.Split(spec, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		base, rest, _ := strings.Cut(alt, ".")
		if !matchesBase(g, base, o) {
			continue
		}
		all := true
		for _, p := range strings.Split(rest, "+") {
			if p == "" {
				continue
			}
			if fn, ok := predicates[p]; ok {
				if !fn(g, o, you, source) {
					all = false
					break
				}
				continue
			}
			if res, ok := numericPred(p, g, o); ok {
				if !res {
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

// MatchesPlayerSpec is the player-side filter: You, Opponent, Player.
func MatchesPlayerSpec(g *state.Game, spec string, p, you state.PlayerID) bool {
	for _, alt := range strings.Split(spec, ",") {
		switch base, _, _ := strings.Cut(strings.TrimSpace(alt), "."); base {
		case "Player", "Any":
			return true
		case "You":
			if p == you {
				return true
			}
		case "Opponent":
			if p != you {
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
			if _, ok := numericPred(p, nil, &state.Object{}); ok {
				continue
			}
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func KnownPredicates() []string {
	out := make([]string, 0, len(predicates))
	for k := range predicates {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
