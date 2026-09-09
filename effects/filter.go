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
	"Legendary": func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return hasType(o, "Legendary")
	},
	// Basic is a supertype used by the common hidden-library ChangeZone
	// filter Land.Basic (Evolving Wilds and the ramp/tutor family).
	"Basic": func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return hasType(o, "Basic")
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

// attachedToArg splits the space-bearing two-token predicate "AttachedTo <X>"
// into its argument and reports whether the argument is a single literal type
// or object class the base grammar (matchesBase) can answer from the object in
// hand. It returns false for any token that is not exactly this shape: a
// different predicate name, no space, an empty argument, an argument carrying
// a nested predicate ('.'/'+'/',' -- e.g. "AttachedTo Permanent.YouCtrl", a
// referent needing resolution-time context such as "AttachedTo Targeted", or
// a word that is neither an object class nor a corpus type word. Consuming
// tokens that are not this shape keeps the matcher and UnknownPredicates
// agreeing, because a token either becomes a wordAttachedTo classifier here or
// it does not -- there is no middle where one side sees it and the other does
// not.
func attachedToArg(p string) (string, bool) {
	name, arg, has := strings.Cut(p, " ")
	if !has || name != "AttachedTo" {
		return "", false
	}
	arg = strings.TrimSpace(arg)
	if arg == "" || strings.ContainsAny(arg, ".+,") {
		return "", false
	}
	switch arg {
	case "Card", "Permanent", "Spell":
		return arg, true
	}
	// "You" is in predicateTypeWords only because one card literally prints
	// `Types:Legendary Planeswalker You`, but every corpus `AttachedTo You`
	// (Witchbane Orb, Lynde) means a Curse attached to YOU THE PLAYER. This
	// engine cannot model that: state.Object.AttachedTo is an ObjID and a
	// player is not an object. Reading it as "attached to a permanent of type
	// You" would match nothing -- harmless on its own, but it would also lift
	// the token out of UnknownPredicates, and that is what the card-validation
	// pass uses to REFUSE a card it would otherwise misplay. Recognising it
	// would let those cards through while their curse test silently never
	// fires. It stays unknown, and stays refused.
	if arg == "You" {
		return "", false
	}
	if predicateTypeWords[arg] {
		return arg, true
	}
	return "", false
}

// wordKind classifies a predicate word that is neither in the `predicates`
// map nor a numeric predicate. It is the single classifier shared by the
// positive path in MatchesObjectCtx and by the generic non<X> negation in
// nonPredicate, so a word is either a recognised shape or it is not -- the
// matcher and UnknownPredicates cannot disagree about it.
type wordKind int

const (
	wordUnknown wordKind = iota
	wordColor
	wordType
	wordColorless
	wordMultiColor
	// The game/source-aware families. Each needs more than the object alone:
	// the game (for the active player and the commander list), the source
	// (for combat pairing), or the object's own zone/counters. They are
	// classified here so the matcher and UnknownPredicates cannot disagree
	// about whether a word is recognised, exactly as the type-word family is.
	wordInZoneStack
	wordActivePlayerCtrl
	wordHasCounters
	wordHistoric
	wordIsCommander
	wordBlockingSource
	wordBlockedBySource
	// The two-token space form "AttachedTo <X>": <X> is a literal type or
	// object class answerable from the object in hand (the base grammar).
	wordAttachedTo
)

// wordPredicate classifies a bare predicate word. key is the WUBRG letter for
// wordColor, the corpus type word for wordType, and empty for the others. An
// unrecognised word is wordUnknown: both sides must fail closed on it, never
// turn it into an always-true predicate.
func wordPredicate(p string) (wordKind, string) {
	if l, is := colorLetter[p]; is {
		return wordColor, l
	}
	switch p {
	case "Colorless":
		return wordColorless, ""
	case "MultiColor":
		return wordMultiColor, ""
	case "inZoneStack":
		return wordInZoneStack, ""
	case "ActivePlayerCtrl":
		return wordActivePlayerCtrl, ""
	case "HasCounters":
		return wordHasCounters, ""
	case "Historic":
		return wordHistoric, ""
	case "IsCommander":
		return wordIsCommander, ""
	case "blockingSource":
		return wordBlockingSource, ""
	case "blockedBySource":
		return wordBlockedBySource, ""
	}
	// The two-token space form "AttachedTo <X>": the whole "AttachedTo
	// Creature" token survives the spec splitter (a space is not a ',' '.'
	// or '+' delimiter), so it arrives here intact. The argument must be a
	// single literal type or object class the base grammar can answer from
	// the object in hand; a referent that needs resolution-time context
	// (AttachedTo Targeted) or a nested predicate (AttachedTo
	// Permanent.YouCtrl) stays wordUnknown and fails closed.
	if arg, ok := attachedToArg(p); ok {
		return wordAttachedTo, arg
	}
	if predicateTypeWords[p] {
		return wordType, p
	}
	return wordUnknown, ""
}

// wordMatches reports whether an object satisfies a positively-evaluated
// classifier from wordPredicate. Colorless is "no colour at all" and
// MultiColor "more than one colour", both read off ColorsOf rather than the
// face directly -- so a Devoid card (CR 702.114, which ColorsOf already
// implements) is Colorless, which is the whole point of Devoid. The
// game/source-aware families read the live game, the object's own zone or
// counters, and the effect's source (for combat pairing and commander
// membership).
func wordMatches(kind wordKind, key string, g *state.Game, o *state.Object, you state.PlayerID, source state.ObjID) bool {
	switch kind {
	case wordColor:
		return strings.Contains(ColorsOf(o), key)
	case wordType:
		return hasType(o, key)
	case wordColorless:
		return ColorsOf(o) == ""
	case wordMultiColor:
		return len(ColorsOf(o)) > 1
	case wordInZoneStack:
		// Forge's inZoneStack: the object is a spell or ability currently on
		// the stack (a spell carries its card face; an ability object has
		// Card == nil, but both have Zone == ZStack).
		return o.Zone == state.ZStack
	case wordActivePlayerCtrl:
		// Forge's ActivePlayerCtrl: the object is controlled by the active
		// player -- the seat whose turn it is, g.Active.
		return o.Controller == g.Active
	case wordHasCounters:
		// Forge's HasCounters: the object has at least one counter of any
		// kind on it.
		return len(o.Counters) > 0
	case wordHistoric:
		// Forge's Historic: artifact, legendary, or Saga (the reminder text
		// on the Historic keyword).
		return hasType(o, "Artifact") || hasType(o, "Legendary") || hasType(o, "Saga")
	case wordIsCommander:
		// Forge's IsCommander: the object is one of a seat's commanders.
		// The commander list lives on the Players at genesis.
		for i := range g.Players {
			for _, c := range g.Players[i].Commanders {
				if c == o.ID {
					return true
				}
			}
		}
		return false
	case wordBlockingSource:
		// Forge's blockingSource: the object is a creature blocking the
		// source. BlockedBy is recorded on the attacked object, so the
		// source's BlockedBy names its blockers; this object is one of them.
		s := g.Obj(source)
		return s != nil && containsID(s.BlockedBy, o.ID)
	case wordBlockedBySource:
		// Forge's blockedBySource: the object is being blocked by the source
		// -- the source is one of THIS object's blockers.
		return containsID(o.BlockedBy, source)
	case wordAttachedTo:
		// Forge's AttachedTo <X>: this object (an Aura or Equipment) is
		// attached to something, and the permanent it is attached to (its
		// own AttachedTo id) satisfies the base <X>. An unattached object
		// (AttachedTo == 0), or one whose attachment is gone, matches
		// nothing. This is the two-token counterpart of attachedBy, which
		// reads the SOURCE's AttachedTo to find what the source attaches
		// to; here we read the candidate object's own AttachedTo.
		if o.AttachedTo == 0 {
			return false
		}
		a := g.Obj(o.AttachedTo)
		if a == nil {
			return false
		}
		return matchesBase(g, key, a)
	}
	return false
}

// nonPredicate reports whether predicate p has the generic negation shape
// non<X>, and how to evaluate it: the classifier to negate and its key. For
// <X> a colour name it is wordColor (with the WUBRG letter); for <X> a
// type/supertype/subtype word in the corpus vocabulary it is wordType; for
// <X> Colorless it is wordColorless (so nonColorless is "has at least one
// colour"). The caller negates by evaluating wordMatches and inverting. ok is
// false for a p that is not a non<X> shape at all, or whose <X> is none of a
// colour, a known type word, or Colorless -- the caller must treat that as an
// unknown predicate and fail closed, never as an always-true !hasType. Only
// wordColor / wordType / wordColorless negate; a nonMultiColor / nonChosenCard
// remains unknown. The four legacy non* entries in `predicates`
// (nonLand/nonCreature/nonBasic/nonBlack) are matched there first and never
// reach this path, but this path reproduces their result exactly, so the
// handwritten entries could be deleted without changing behaviour.
func nonPredicate(p string) (kind wordKind, key string, ok bool) {
	x, has := strings.CutPrefix(p, "non")
	if !has || x == "" {
		return wordUnknown, "", false
	}
	kind, key = wordPredicate(x)
	switch kind {
	case wordColor, wordType, wordColorless:
		return kind, key, true
	}
	return wordUnknown, "", false
}

// positiveRecognised reports whether a predicate token p is a recognised
// positive-evaluation shape: an entry in the `predicates` map, a numeric
// <field><CMP><n> predicate, a generic non<X> negation whose <X> is a
// recognised classifier, or a wordPredicate classifier word. It is the single
// recognition source shared by the evaluator (matchPositive) and by
// UnknownPredicates, so the matcher and the census cannot disagree about
// whether a word is recognised. An unrecognised word is "the engine does not
// know", never "true" -- that is the fail-closed contract.
func positiveRecognised(p string) bool {
	if _, _, ok := controlReferent(p); ok {
		return true
	}
	if _, ok := predicates[p]; ok {
		return true
	}
	if _, ok := numericPred(p, nil, &state.Object{}, SpecContext{}); ok {
		return true
	}
	if _, _, ok := nonPredicate(p); ok {
		return true
	}
	if kind, _ := wordPredicate(p); kind != wordUnknown {
		return true
	}
	return false
}

// recognisedPredicate reports whether a predicate token p is recognised by
// this build at all, including a leading '!'. A !<X> is recognised exactly
// when <X> is a recognised positive-evaluation predicate, by the same
// resolution the positive path uses. A second '!' (!!X) is never a
// recognised shape -- the double negation is not part of this grammar, so it
// fails closed like any unknown.
func recognisedPredicate(p string) bool {
	if positiveRecognised(p) {
		return true
	}
	if x, has := strings.CutPrefix(p, "!"); has && x != "" {
		return positiveRecognised(x)
	}
	return false
}

// matchPositive evaluates a recognised positive-evaluation predicate token p
// to its boolean. ok is false for an unknown token OR an unbound trigger
// referent. The latter remains a recognised grammar shape for the census, but
// cannot be negated into a match when its resolution context is absent.
func matchPositive(g *state.Game, p string, o *state.Object, sc SpecContext) (result, ok bool) {
	if op, ref, recognised := controlReferent(p); recognised {
		return matchControlReferent(g, o, sc, op, ref)
	}
	if fn, ok := predicates[p]; ok {
		return fn(g, o, sc.You, sc.Source), true
	}
	if res, ok := numericPred(p, g, o, sc); ok {
		return res, true
	}
	if nkind, nkey, ok := nonPredicate(p); ok {
		// non<X> is the negation of a recognised classifier: the object
		// matches when the positive classifier does not.
		return !wordMatches(nkind, nkey, g, o, sc.You, sc.Source), true
	}
	if kind, key := wordPredicate(p); kind != wordUnknown {
		return wordMatches(kind, key, g, o, sc.You, sc.Source), true
	}
	return false, false
}

// matchPredicate evaluates a predicate token in a filter conjunction,
// including the leading-'!' negation. ok is false for an unknown shape or an
// unbound trigger referent, so the caller must fail closed. A !<X> negates the positive evaluation of <X>; when
// <X> is itself not recognised, !<X> is unknown too -- the negation of "I do
// not know" is not "yes".
func matchPredicate(g *state.Game, p string, o *state.Object, sc SpecContext) (result, ok bool) {
	if x, has := strings.CutPrefix(p, "!"); has {
		if x == "" {
			return false, false
		}
		r, rek := matchPositive(g, x, o, sc)
		if !rek {
			return false, false
		}
		return !r, true
	}
	return matchPositive(g, p, o, sc)
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
	TriggerContext
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
			// matchPredicate evaluates every recognised shape -- the predicates
			// map, a numeric predicate, a generic non<X> negation, a
			// wordPredicate classifier word, and a leading-'!' negation of any
			// of those -- to a boolean. An unrecognised token (ok == false) is
			// unknown, so it fails closed: never an always-true fallback, which
			// would silently widen the filter instead of showing up as a
			// missing action.
			res, ok := matchPredicate(g, p, o, sc)
			if !ok || !res {
				all = false
				break
			}
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

// SearchStatesQuality reports whether a search's card filter (a ChangeType
// spec) states a QUALITY of the cards to be found -- a card type, subtype,
// colour, name, or any other characteristic that narrows what counts -- rather
// than only a quantity. CR 701.23b lets a player searching a hidden zone for
// cards with a stated quality decline to find (even if a matching card is
// present), while CR 701.23d requires a player searching only for a quantity
// ("a card", "three cards") to find that many, or as many as the zone holds.
//
// A spec is quantity-only when every comma alternative is a bare card clause:
// the universal `Card`/`Any` base with only possession/control predicates
// (YouOwn, YouCtrl, ...) and no type/subtype/colour/name restriction. Any
// alternative whose base names a type/identity other than `Card`/`Any`, or
// that carries any predicate other than a possession/control word, states a
// quality. This is a property of the FILTER, deliberately independent of
// Forge's `Mandatory$` parameter, which is recorded in AGENTS.md as
// deliberately unread and is a different thing.
func SearchStatesQuality(spec string) bool {
	for _, alt := range strings.Split(spec, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		base, rest, _ := strings.Cut(alt, ".")
		if base != "Card" && base != "Any" {
			return true
		}
		for _, p := range strings.Split(rest, "+") {
			if p == "" {
				continue
			}
			if !possessionPredicate(p) {
				return true
			}
		}
	}
	return false
}

// possessionPredicate reports whether a predicate word restricts only who owns
// or controls the card, not what the card is -- so a `Card.YouOwn` search is
// still a bare quantity search under CR 701.23d. Every other predicate word is
// a quality clause, so it fails closed to "states quality" (the conservative
// direction: it preserves 701.23b's fail-to-find allowance rather than making
// a stated-quality search mandatory).
func possessionPredicate(p string) bool {
	switch p {
	case "YouOwn", "YouCtrl", "YouControl", "YourControl", "YouControlled",
		"OppOwn", "OppCtrl", "OpponentOwns", "OpponentControls":
		return true
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
			// recognisedPredicate is the single classifier the matcher
			// (matchPredicate) and this census walk share, so a token is
			// either recognised by both or unknown to both -- including a
			// leading-'!' negation, which is recognised only when its inner
			// word is.
			if recognisedPredicate(p) {
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
