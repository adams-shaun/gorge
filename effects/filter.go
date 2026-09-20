package effects

import (
	"iter"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/state"
)

// A Forge filter spec is alternatives separated by "," (OR). Each alternative
// is a base type, optionally negated with a "non" prefix, followed by
// ".pred+pred+..." (AND). A raw comma in a named<Name>/notnamed<Name>
// argument is part of the printed name when what follows is not another
// filter alternative; filterAlternatives owns that ambiguity in one place.
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
	"token":     func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool { return o.IsToken },
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
		return s != nil && s.ChosenName != "" && sharesName(o, s.ChosenName)
	},
	"ChosenType": func(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
		s := g.Obj(src)
		return s != nil && s.ChosenType != "" && hasType(o, s.ChosenType)
	},
	"IsNotChosenType": func(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
		s := g.Obj(src)
		return s != nil && s.ChosenType != "" && !hasType(o, s.ChosenType)
	},
	// An object records this association in events.Apply when an effect moves
	// it to exile with moveZoneEvent. Both spellings use the same tracked
	// provenance; LKI refinements are outside this narrow association.
	"ExiledWithSource": func(_ *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
		return src != 0 && o.ExiledWith == src
	},
	"ExiledWithSourceLKI": func(_ *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
		return src != 0 && o.ExiledWith == src
	},
	// escaped is the CastFlags provenance of an escape cast (CR 702.42a): the
	// "sacrifice it unless it escaped" ETB family reads it through
	// Card.Self+escaped (Kroxa, Uro, Phlage), as do the escape-with-counters
	// replacement ValidCard$ specs. A card never escape-cast never matches.
	"escaped": func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return o.CastFlags&state.FlagEscaped != 0
	},
	// wasCastFromGraveyard is the CastFlags provenance of a GRAVEYARD-ORIGIN
	// cast (CR 601.2b): any of FlagFlashback, FlagHarmonize or FlagEscaped.
	// The same bit test the Count$wasCastFromGraveyard branch head shares
	// (effects/count.go) and its compiled twin mirrors
	// (effects/compiled_predicate.go's predicateTermWasCastFromGraveyard).
	// Ash Zealot's "whenever a player casts a spell from a graveyard"
	// ValidCard$ reads it at spellCastMatches time — the deferred cast
	// trigger fires after payCast's CastInfo, so the bit is already stamped
	// — as do River Kelpie's draws and Laquatus's Disdain's counter. A card
	// never so cast never matches.
	"wasCastFromGraveyard": func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return o.CastFlags&(state.FlagFlashback|state.FlagHarmonize|state.FlagEscaped) != 0
	},
	// notExertedThisTurn is CR 702.100a's offer gate (task exert1): the
	// object has NOT been exerted this turn. The event-backed read is
	// events.Apply's Exert fold (state.Object.ExertedThisTurn). Combat
	// Celebrant's `IsPresent$ Creature.Self+notExertedThisTurn` is the
	// corpus's one carrier; the predicate is a recognised-shape entry (the
	// compiled predicate layer marks an unlisted term `maybe` and falls
	// through to this textual oracle, so no twin term is owed), and
	// UnknownPredicates classifies it through the same predicates map, so
	// the census and the matcher cannot disagree.
	"notExertedThisTurn": func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return !o.ExertedThisTurn
	},
}

// colorLetter maps a colour's English name to its WUBRG letter -- note Blue
// is "U", not "B" (col[:1] would collide with Black). Colour predicates
// (and their non<X> negations) read ColorsOf, not the face directly, so a
// Devoid card (effects.ColorsOf) matches no colour predicate, Green included.
var colorLetter = map[string]string{"White": "W", "Blue": "U", "Black": "B", "Red": "R", "Green": "G"}

func init() {
	for _, kw := range [...]string{"Flying", "Trample", "Deathtouch", "Lifelink",
		"Vigilance", "Reach", "Haste", "Indestructible", "First Strike", "Menace",
		"Flanking", "Horsemanship"} {
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
	// ExiledWithEffectSource is the Effect-delivered spelling of the same
	// exiled-by-this-source provenance: the effect's source card is what
	// exiled the candidate (Opposition Agent/Valki-style MayPlay grants name
	// it), the same tracked ExiledWith field ExiledWithSource reads.
	predicates["ExiledWithEffectSource"] = predicates["ExiledWithSource"]
	// EquippedBy / EnchantedBy / AttachedBy: the candidate is the permanent
	// source is attached to (attachedBy below). Task 14 wires all three to the
	// same predicate -- Forge spells "attached to" three ways depending on
	// whether the source is Equipment, an Aura, or a generic script.
	predicates["EquippedBy"] = attachedBy
	predicates["EnchantedBy"] = attachedBy
	predicates["AttachedBy"] = attachedBy
	// CanEnchantEquippedBy: the candidate card could legally be attached to
	// the creature the resolving source attaches to -- Mantle of the
	// Ancients' "return ... Aura and/or Equipment cards that could be
	// attached to enchanted creature" (ValidTgts$
	// Aura.CanEnchantEquippedBy+YouOwn,Equipment.CanEnchantEquippedBy+YouOwn)
	// and Holy Avenger's "put an Aura card from your hand onto the
	// battlefield attached to it" (ChangeType$ Aura.CanEnchantEquippedBy),
	// the two corpus carriers. The referent creature is the source itself
	// when the source is a creature (Holy Avenger's equipped creature fires
	// the trigger), else the permanent the source is attached to (Mantle's
	// bearer). An Aura candidate matches when the bearer still satisfies the
	// candidate's K:Enchant spec -- the same test the CR 704.5m SBA runs
	// (rules/attach.go auraStillMatchesEnchant); an Equipment candidate when
	// the bearer is a creature (CR 704.5n); anything else admits nothing. A
	// source with no referent (gone, or an unattached non-creature) and an
	// Enchant spec this filter cannot evaluate both fail closed inside the
	// filter, never over-offering an attachment the SBA would just sweep.
	predicates["CanEnchantEquippedBy"] = canEnchantEquippedBy
	// equipped / enchanted: the IS-side counterpart of the pair above -- the
	// candidate itself carries the attachment. Auriok Steelshaper's IsPresent$
	// Card.Self+equipped ("as long as CARDNAME is equipped") reads the first;
	// the corpus also spells the Aura case +enchanted (21 files carrying a
	// bare +enchanted, e.g. Krond the Dawn-Clad's IsPresent$
	// Card.Self+enchanted). The state the attach path maintains: some
	// battlefield permanent whose face carries the kind's type word names the
	// candidate in its AttachedTo.
	predicates["equipped"] = func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return hasAttachmentOfKind(g, o.ID, "Equipment")
	}
	predicates["enchanted"] = func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return hasAttachmentOfKind(g, o.ID, "Aura")
	}
	predicates["Enchanted"] = predicates["enchanted"]
	// Soulbond's "PairedWith" and "Paired" predicates (CR 702.103): the
	// Affected$ spec `Creature.PairedWith` names the creature a source is
	// paired with, and `Creature.Self+Paired` names the source itself when it
	// is paired. PairedWith reads source.Paired (the source is the effect's
	// own permanent).
	predicates["Paired"] = func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
		return o.Paired != 0
	}
	predicates["PairedWith"] = func(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
		s := g.Obj(src)
		return s != nil && s.Paired == o.ID && o.Zone == state.ZBattlefield
	}
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

// canEnchantEquippedBy is the CanEnchantEquippedBy predicate body; see the
// registration above for the spelling's carriers and the referent rule.
func canEnchantEquippedBy(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
	s := g.Obj(src)
	if s == nil {
		return false
	}
	// The resolving source may be the Face-less ability/trigger wrapper a
	// TriggerPush minted (the placement ask's SpecContext.Source is that
	// wrapper, rules/trigger_queue.go pushTrigger) -- its Source field names
	// the permanent that carries the ability (Ruling T20-b). Unwrap before
	// reading the creature/attach referent, or Mantle's own placement ask
	// would see an unattached Face-less object and admit nothing.
	if s.Face() == nil && s.Source != 0 {
		if real := g.Obj(s.Source); real != nil {
			s = real
		}
	}
	bearer := s
	if !hasType(bearer, "Creature") {
		bearer = g.Obj(s.AttachedTo)
		if bearer == nil {
			return false
		}
	}
	return attachableTo(g, o, bearer)
}

// attachableTo reports whether the (possibly off-battlefield) card o could
// legally be attached to the battlefield permanent bearer: an Aura when the
// bearer satisfies its K:Enchant spec, an Equipment when the bearer is a
// creature (CR 704.5n), anything else never. Evaluated from the candidate's
// own controller seat (the Aura's YouCtrl is the Aura controller's), the same
// seat the CR 704.5m SBA's auraStillMatchesEnchant test uses.
func attachableTo(g *state.Game, o *state.Object, bearer *state.Object) bool {
	if o == nil || bearer == nil || bearer.Zone != state.ZBattlefield {
		return false
	}
	f := o.Face()
	if f == nil {
		return false
	}
	switch {
	case hasType(o, "Aura"):
		param, ok := f.KeywordParam("Enchant")
		if !ok || strings.TrimSpace(param) == "" {
			return true
		}
		spec, _, _ := strings.Cut(param, ":")
		return MatchesSpecFrom(g, strings.TrimSpace(spec), bearer.ID, o.Controller, o.ID)
	case hasType(o, "Equipment"):
		bf := bearer.Face()
		return bf != nil && bf.IsCreature()
	}
	return false
}

// hasAttachmentOfKind reports whether any battlefield permanent whose face
// carries the type word kind names id in its AttachedTo -- the state the
// equip/attach path maintains (rules/attach_test.go pins the SBA that
// detaches on death, so a dead Equipment never counts). The scan walks the
// deterministic AliveFrom/zone slices, never a map, so it is replay-safe as
// a filter predicate. A candidate itself off the battlefield (a graveyard
// card a hidden search spec asks about) is still eligible as the attachment
// TARGET read: AttachedTo only ever names the bearer, so the scan alone
// decides.
func hasAttachmentOfKind(g *state.Game, id state.ObjID, kind string) bool {
	for _, p := range g.AliveFrom(0) {
		for _, sid := range g.Zone(state.ZBattlefield, p) {
			s := g.Obj(sid)
			if s == nil || s.AttachedTo != id {
				continue
			}
			if s.Face() != nil && hasType(s, kind) {
				return true
			}
		}
	}
	return false
}

// sharesTypeArg splits the space-bearing two-token predicates
// "sharesCardTypeWith <X>" and "sharesCreatureTypeWith <X>" and classifies
// their shared referent. The referent is a resolution-time object list: the
// remembered set (RememberedCard — its first card entry, Braids's "a
// permanent that shares a card type with it" — Remembered, RememberedLKI),
// the triggering card (TriggeredCard/TriggeredCardLKICopy, Heirloom
// Blade's "a creature card that shares a creature type with it"), the
// resolution's targets (Targeted), or the source itself (Self). The
// predicate NAME is returned alongside the referent so the dispatch can
// tell the CARD-type and CREATURE-type readings apart. A referent with no
// live binding — and any other <X>, including a nested predicate — is
// unrecognised: the token stays unknown and the spec fails closed, never
// widened.
func sharesTypeArg(p string) (name, arg string, ok bool) {
	name, arg, ok = strings.Cut(p, " ")
	if !ok || (name != "sharesCardTypeWith" && name != "sharesCreatureTypeWith") {
		return "", "", false
	}
	arg = strings.TrimSpace(arg)
	if arg == "" || strings.ContainsAny(arg, ".+,!") {
		return "", "", false
	}
	switch arg {
	case "RememberedCard", "Remembered", "RememberedLKI", "TriggeredCard",
		"TriggeredCardLKICopy", "Targeted", "Self":
		return name, arg, true
	}
	return "", "", false
}

// sharesTypeReferents resolves the SHARED referent switch of the
// sharesCardTypeWith/sharesCreatureTypeWith family into the live objects it
// names (empty = an unbound referent; both callers fail closed on that), so
// the two readings can never disagree about which objects <X> names.
func sharesTypeReferents(sc SpecContext, ref string) []state.Target {
	var ts []state.Target
	switch ref {
	case "RememberedCard":
		for _, t := range sc.Remembered {
			if !t.IsPlayer {
				ts = append(ts, t)
				break // the FIRST card entry, per Forge's RememberedCard
			}
		}
	case "Remembered", "RememberedLKI":
		for _, t := range sc.Remembered {
			if !t.IsPlayer {
				ts = append(ts, t)
			}
		}
	case "TriggeredCard", "TriggeredCardLKICopy":
		if sc.TriggerCard != 0 {
			ts = append(ts, state.Target{Obj: sc.TriggerCard})
		}
	case "Targeted":
		ts = sc.ResolutionTargets
	case "Self":
		if sc.Source != 0 {
			ts = append(ts, state.Target{Obj: sc.Source})
		}
	}
	return ts
}

// sharesCardTypeWith reports whether o shares at least one CARD type with
// any object the referent names (Forge Card.sharesCardTypeWith: an
// intersection over the card types — Artifact, Creature, Enchantment, Land,
// Planeswalker, Battle — not supertypes or subtypes). The referent object
// is read live from the game, so a remembered card in the graveyard still
// answers from its own face (CR 603.10's LKI reading applies to
// power/toughness/counters, not types). An unbound referent matches
// nothing — fail closed, never widened.
func sharesCardTypeWith(g *state.Game, o *state.Object, sc SpecContext, ref string) bool {
	for _, t := range sharesTypeReferents(sc, ref) {
		if t.IsPlayer {
			continue
		}
		r := g.Obj(t.Obj)
		if r == nil {
			continue
		}
		for _, cardType := range []string{"Artifact", "Battle", "Creature", "Enchantment", "Land", "Planeswalker"} {
			if hasType(o, cardType) && hasType(r, cardType) {
				return true
			}
		}
	}
	return false
}

// sharesCreatureTypeWith reports whether o shares at least one CREATURE
// subtype with any object the referent names (Forge
// Card.sharesCreatureTypeWith: an intersection over the creature subtypes —
// Heirloom Blade's "a creature card that shares a creature type with it").
// The candidate's subtypes are read context-aware (hasTypeCtx: layer grants
// and Changeling reach it); the referent's own subtypes are read from its
// live face exactly like sharesCardTypeWith's card-type read (hasType,
// which handles Changeling on the referent's side too). An unbound referent
// matches nothing — fail closed, never widened.
func sharesCreatureTypeWith(g *state.Game, o *state.Object, sc SpecContext, ref string) bool {
	for _, t := range sharesTypeReferents(sc, ref) {
		if t.IsPlayer {
			continue
		}
		r := g.Obj(t.Obj)
		if r == nil || r.Face() == nil {
			continue
		}
		for _, word := range r.Face().Types {
			if !CreatureTypeWords(word) {
				continue
			}
			if hasTypeCtx(o, word, sc) && hasType(r, word) {
				return true
			}
		}
	}
	return false
}

// attachedToArg splits the space-bearing two-token predicate "AttachedTo <X>"
// into its argument and reports whether the argument is (a) a single literal
// type or object class the base grammar (matchesBase) can answer from the
// object in hand, or (b) the dotted two-token form "AttachedTo <class>.<qual>"
// whose qualifier is evaluated against the attached object itself (the
// counterpart of the adjacent enchantedByArg's <Type>.<qual>). The dotted
// allowlist is exactly YouCtrl — the only measured qualifier (Umbra Mystic's
// "Aura.AttachedTo Permanent.YouCtrl" grant; 6 occurrences / 5 files). <class>
// keeps the bare form's object-class / type-word validation, so
// "Player.EnchantedBy" (the 2 curse occurrences) fails naturally: a player is
// neither an object class nor a type word. It returns false for any token that
// is not one of these shapes: a different predicate name, no space, an empty
// argument, an argument carrying a nested predicate ('+'/','), a dotted
// qualifier outside the allowlist, a referent needing resolution-time context
// such as "AttachedTo Targeted", or a word that is neither an object class nor
// a corpus type word. Consuming tokens that are not these shapes keeps the
// matcher and UnknownPredicates agreeing, because a token either becomes a
// wordAttachedTo classifier here or it does not -- there is no middle where
// one side sees it and the other does not.
func attachedToArg(p string) (string, bool) {
	name, arg, has := strings.Cut(p, " ")
	if !has || name != "AttachedTo" {
		return "", false
	}
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", false
	}
	// The dotted two-token form "<class>.<qual>": the qualifier rides the
	// object the candidate is attached to (wordAttachedTo's matcher case
	// evaluates it there), so it is validated here once for both the matcher
	// and the recognition path.
	if class, qual, ok := strings.Cut(arg, "."); ok {
		if qual != "YouCtrl" {
			return "", false
		}
		switch class {
		case "Card", "Permanent", "Spell":
		default:
			if !predicateTypeWords[class] {
				return "", false
			}
		}
		return class + "." + qual, true
	}
	if strings.ContainsAny(arg, "+,") {
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

// enchantedByArg splits the space-bearing two-token predicate
// "EnchantedBy <Type>.<qual>" into its argument halves and validates both.
// <Type> is a literal object class or corpus type word the base grammar
// answers from the attached object in hand (every carrier names Aura), and
// <qual> is one of the possession/otherness map predicates the qualifier is
// evaluated against THE ATTACHED OBJECT -- Other (not the resolving source:
// Daybreak Coronet's "another Aura attached to it", and Face of Divinity's
// static excluding Face itself) and YouCtrl (controlled by the spec's you:
// the Killian / Eriette / Archon / Kaima / Dawn Evangel family). A bare
// "EnchantedBy" token never reaches this parser -- the predicates map's
// attachedBy ("the permanent the resolving source is attached to") is
// consulted first on both the matcher and the recognition path and keeps
// its own meaning. Any other shape -- a resolution-time referent
// (EnchantedBy Aura.Targeted), a nested predicate (EnchantedBy
// Aura.Permanent.YouCtrl), a qualifier outside the allowlist, an
// unrecognised type word, or an absent argument -- stays unrecognised:
// the token fails closed and UnknownPredicates keeps reporting it.
func enchantedByArg(p string) (string, bool) {
	name, arg, has := strings.Cut(p, " ")
	if !has || name != "EnchantedBy" {
		return "", false
	}
	arg = strings.TrimSpace(arg)
	if arg == "" || strings.ContainsAny(arg, "+,!") {
		return "", false
	}
	typ, qual, hasDot := strings.Cut(arg, ".")
	if !hasDot || typ == "" || qual == "" || strings.Contains(qual, ".") {
		return "", false
	}
	switch typ {
	case "Card", "Permanent", "Spell":
	default:
		if !predicateTypeWords[typ] {
			return "", false
		}
	}
	switch qual {
	case "Other", "YouCtrl":
	default:
		return "", false
	}
	return typ + "." + qual, true
}

// hasAttachmentMatching reports whether any battlefield permanent attached
// to id (some permanent's AttachedTo names id) satisfies the base typ and
// the qualifier fn evaluated against THAT ATTACHED OBJECT. The scan is the
// hasAttachmentOfKind walk (deterministic AliveFrom/zone slices, never a
// map), so it is replay-safe as a filter predicate; an attachment list is
// not stored on the bearer, so the scan is the only source.
func hasAttachmentMatching(g *state.Game, id state.ObjID, sc SpecContext, typ string, fn predFn) bool {
	for _, p := range g.AliveFrom(0) {
		for _, sid := range g.Zone(state.ZBattlefield, p) {
			a := g.Obj(sid)
			if a == nil || a.AttachedTo != id {
				continue
			}
			if !matchesBase(g, typ, a, sc) {
				continue
			}
			if fn(g, a, sc.You, sc.Source) {
				return true
			}
		}
	}
	return false
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
	wordMonoColor
	// The game/source-aware families. Each needs more than the object alone:
	// the game (for the active player and the commander list), the source
	// (for combat pairing), or the object's own zone/counters. They are
	// classified here so the matcher and UnknownPredicates cannot disagree
	// about whether a word is recognised, exactly as the type-word family is.
	wordInZone
	wordActivePlayerCtrl
	wordTopLibrary
	wordHasCounters
	wordHistoric
	wordIsCommander
	wordBlockingSource
	wordBlockedBySource
	// Forge's faceDown: a face-down battlefield permanent (a manifested or
	// cloaked card). The game/state-aware family -- needs the object's own
	// zone, classified here so matcher and UnknownPredicates agree.
	wordFaceDown
	// Forge's IsRingbearer (CR 701.54e): the object is its controller's
	// Ring-bearer. Game/state-aware -- needs the object's zone and the
	// players' designations -- classified here so matcher and
	// UnknownPredicates agree.
	wordRingBearer
	// The resolution-only one-token TargetedPlayerCtrl grammar. Its target
	// binding comes from SpecContext rather than a new state tracker.
	wordTargetedPlayerCtrl
	// The two-token space form "AttachedTo <X>": <X> is a literal type or
	// object class answerable from the object in hand (the base grammar).
	wordAttachedTo
	// The two-token space form "sharesCardTypeWith <X>": <X> is a
	// resolution-time referent (RememberedCard, TriggeredCard, ...) the
	// SpecContext resolves.
	wordSharesCardType
	// The creature-subtype twin "sharesCreatureTypeWith <X>": same referent
	// switch, the intersection is over creature subtypes (Heirloom Blade).
	wordSharesCreatureType
	// The two-token space form "EnchantedBy <Type>.<qual>": the candidate
	// bears an attached permanent of the named type whose qualifier holds
	// against that attached object (Daybreak Coronet's "creature with
	// another Aura attached to it", the Aura.YouCtrl family). The bare
	// "EnchantedBy" token keeps its map-predicate meaning (attachedBy) and
	// never reaches this classifier.
	wordEnchantedBy
	// Forge's zone-entry history predicates: "ThisTurnEntered" (the object
	// entered a zone this turn, any zone) and "ThisTurnEnteredFrom_<Zone>"
	// (it entered from <Zone>). Both read the per-object entry provenance
	// events.Move records, which the Count$ThisTurnEntered_* heads share.
	wordThisTurnEntered
	wordThisTurnEnteredFrom
	// The "<Colour>Source" family (Ojer Axonil's Card.RedSource+YouCtrl):
	// the object is a source carrying that colour -- CR 700.7's "a red
	// source" is a source with red in its colour characteristics, which for
	// the object in hand is exactly ColorsOf containing the colour. The
	// Colorless member is a source with no colours at all.
	wordColourSource
	wordColourSourceless
	// The name-predicate family (Forge CardProperty): named<Name> and
	// notnamed<Name> compare the candidate's name characteristics with the
	// argument text (key carries it, `;`/`_` normalised); sameName compares
	// it with the source card, or with the name referent encoded by its own
	// Remembered./Targeted./Triggered. base shape.
	wordNamed
	wordNotnamed
	wordSameName
	// wasCast is Forge's Card.wasCast: the object is a SPELL currently on
	// the stack -- announced, not yet resolved. The AffectedZone$ Stack
	// convoke/cascade grants key on it (Chief Engineer). An ability object
	// (Card == nil) was never cast.
	wordWasCast
	// The and/or Kicker's index form: "kicked 1" / "kicked 2" (the whole
	// two-token form survives the spec splitter) reads the specific part's
	// CastFlags bit. The bare "kicked" word stays in the predicates map.
	wordKickedIndex
)

// wordPredicate classifies a bare predicate word. key is the WUBRG letter for
// wordColor, the corpus type word for wordType, and empty for the others. An
// unrecognised word is wordUnknown: both sides must fail closed on it, never
// turn it into an always-true predicate.
func wordPredicate(p string) (wordKind, string) {
	if l, is := colorLetter[p]; is {
		return wordColor, l
	}
	if c, ok := strings.CutSuffix(p, "Source"); ok {
		if l, is := colorLetter[c]; is {
			return wordColourSource, l
		}
		if c == "Colorless" {
			return wordColourSourceless, ""
		}
	}
	// notnamed before named: both prefixes are literal token prefixes and
	// "notnamed..." does not start with "named", but checking in this order
	// documents that neither is a prefix of the other's grammar. An empty
	// argument (a bare `named`) stays recognised and never matches -- Forge
	// sharesNameWith("") is false.
	if name, ok := strings.CutPrefix(p, "notnamed"); ok {
		if name == "" {
			// A bare `notnamed` would negate to always-true (the negation of
			// "no name at all"), which the fail-closed contract forbids; it
			// stays unknown instead.
			return wordUnknown, ""
		}
		return wordNotnamed, nameArg(name)
	}
	if name, ok := strings.CutPrefix(p, "named"); ok {
		return wordNamed, nameArg(name)
	}
	if p == "sameName" {
		return wordSameName, ""
	}
	// Forge's inZone<Zone> property (CardProperty inZone<Zone>): the object
	// sits in the named zone. The old specific inZoneStack spelling folds
	// into the generic form (both mean Zone == ZStack); an unresolvable zone
	// name falls through to wordUnknown and fails closed.
	if z, ok := strings.CutPrefix(p, "inZone"); ok && z != "" {
		if _, is := parseZone(z); is {
			return wordInZone, z
		}
	}
	// Forge's inRealZone<X> property: the object's REAL (current) zone is
	// <X> -- the same live read inZone<X> gives, spelled to distinguish from
	// an LKI-based zone test (Not of This World's TargetValidTargeting$
	// Permanent.YouCtrl+inRealZoneBattlefield). An unresolvable zone name
	// falls through to wordUnknown and fails closed.
	if z, ok := strings.CutPrefix(p, "inRealZone"); ok && z != "" {
		if _, is := parseZone(z); is {
			return wordInZone, z
		}
	}
	// The and/or Kicker's index form "kicked <n>" (Forge's Card.kicked with
	// the part index -- Wastescape Battlemage's "Card.Self+kicked 1"): the
	// bare "kicked" word is in the predicates map (any CastFlags kicker
	// bit); the index form reads the specific part's bit.
	if rest, ok := strings.CutPrefix(p, "kicked "); ok {
		switch strings.TrimSpace(rest) {
		case "1", "2":
			return wordKickedIndex, strings.TrimSpace(rest)
		}
	}
	switch p {
	case "Colorless":
		return wordColorless, ""
	case "MultiColor":
		return wordMultiColor, ""
	case "MonoColor":
		return wordMonoColor, ""
	case "wasCast":
		return wordWasCast, ""
	case "ActivePlayerCtrl":
		return wordActivePlayerCtrl, ""
	case "TopLibrary":
		return wordTopLibrary, ""
	case "faceDown":
		return wordFaceDown, ""
	case "IsRingbearer":
		return wordRingBearer, ""
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
	if targetReferent(p) {
		return wordTargetedPlayerCtrl, ""
	}
	if p == "ThisTurnEntered" {
		return wordThisTurnEntered, ""
	}
	if z, is := strings.CutPrefix(p, "ThisTurnEnteredFrom_"); is && zoneWordKnown(z) {
		return wordThisTurnEnteredFrom, z
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
	if name, arg, ok := sharesTypeArg(p); ok {
		if name == "sharesCreatureTypeWith" {
			return wordSharesCreatureType, arg
		}
		return wordSharesCardType, arg
	}
	// The two-token space form "EnchantedBy <Type>.<qual>" (the whole token
	// survives the spec splitter -- a space is not a delimiter). Only the
	// shapes enchantedByArg validates become wordEnchantedBy; everything
	// else falls through to wordUnknown and fails closed.
	if arg, ok := enchantedByArg(p); ok {
		return wordEnchantedBy, arg
	}
	if predicateTypeWords[p] {
		return wordType, p
	}
	return wordUnknown, ""
}

// zoneWords maps the zone names the corpus's ThisTurnEnteredFrom_<Zone>
// predicate (and Forge's ZoneType.smartValueOf) spells to state zones.
var zoneWords = map[string]state.Zone{
	"Battlefield": state.ZBattlefield,
	"Graveyard":   state.ZGraveyard,
	"Hand":        state.ZHand,
	"Library":     state.ZLibrary,
	"Exile":       state.ZExile,
	"Stack":       state.ZStack,
	"Command":     state.ZCommand,
}

// zoneWordKnown reports whether a word names a zone (zoneWords membership),
// the recognition half of the ThisTurnEnteredFrom_<Zone> classifier -- an
// unknown zone word stays wordUnknown and fails closed.
func zoneWordKnown(z string) bool {
	_, ok := zoneWords[z]
	return ok
}

// wordMatches reports whether an object satisfies a positively-evaluated
// classifier from wordPredicate. Colorless is "no colour at all" and
// MultiColor "more than one colour"; MonoColor is its twin, "exactly one
// colour" (Tarnation Vista's EachColorAmong_Valid
// Permanent.YouCtrl+MonoColor -- a colourless permanent is not monocolored),
// all read off ColorsOf rather than the face directly -- so a Devoid card (CR 702.114, which ColorsOf already
// implements) is Colorless, which is the whole point of Devoid. The
// game/source-aware families read the live game, the object's own zone or
// counters, and the effect's source (for combat pairing and commander
// membership).
func wordMatches(kind wordKind, key string, g *state.Game, o *state.Object, sc SpecContext) bool {
	source := sc.Source
	switch kind {
	case wordSharesCardType:
		return sharesCardTypeWith(g, o, sc, key)
	case wordSharesCreatureType:
		return sharesCreatureTypeWith(g, o, sc, key)
	case wordColor:
		return strings.Contains(ColorsOf(o), key)
	case wordType:
		return hasTypeCtx(o, key, sc)
	case wordColorless:
		return ColorsOf(o) == ""
	case wordColourSource:
		return strings.Contains(ColorsOf(o), key)
	case wordColourSourceless:
		return ColorsOf(o) == ""
	case wordKickedIndex:
		// The and/or Kicker's part bits (state/object.go): the CastInfo
		// provenance the kicked1/kicked2/kickedboth cast modes ride. A part
		// never paid never matches, and the bare FlagKicked bit alone (a
		// single-cost Kicker) never matches an index form.
		switch key {
		case "1":
			return o.CastFlags&state.FlagKicked1 != 0
		case "2":
			return o.CastFlags&state.FlagKicked2 != 0
		}
		return false
	case wordMultiColor:
		return len(ColorsOf(o)) > 1
	case wordMonoColor:
		return len(ColorsOf(o)) == 1
	case wordWasCast:
		// Forge's wasCast: a spell (Card != nil) currently on the stack. An
		// ability object was activated, never cast. The AsStack override
		// (rules.derivedWith) admits the spell a cast is announcing, which is
		// still in hand at CR 601.2b but IS the spell being cast.
		return (o.Zone == state.ZStack || sc.AsStack) && o.Card != nil
	case wordInZone:
		// Forge's inZone<Zone>: the object is in that zone (measured at the
		// corpus pin: inZoneBattlefield 272 raw occurrences, inZoneStack 30,
		// inZoneGraveyard 20, inZoneHand 9, inZoneLibrary 4, inZoneExile 4 --
		// InZones$ is a separate parameter key, not a predicate word).
		z, ok := parseZone(key)
		return ok && o.Zone == z
	case wordActivePlayerCtrl:
		// Forge's ActivePlayerCtrl: the object is controlled by the active
		// player -- the seat whose turn it is, g.Active.
		return o.Controller == g.Active
	case wordFaceDown:
		// Forge's faceDown: the object is a face-down battlefield permanent
		// (CR 708.5 -- a manifested or cloaked card). The same live state read
		// the rules-side scans gate on (faceDownPrintedHides); a face-down
		// EXILE (Hideaway) is not a permanent and never matches.
		return o.FaceDown && o.Zone == state.ZBattlefield
	case wordRingBearer:
		// Forge's IsRingbearer (CR 701.54e): the object is its controller's
		// Ring-bearer -- true exactly while it is on the battlefield under
		// that player's control and carries the seat's designation. The
		// designation's zone and control halves are enforced by events.Apply
		// (the battlefield-leave and ControlChange clears), so the live check
		// is the id comparison, and an object outside the battlefield (or an
		// LKI of a moved one) never matches.
		return o.Zone == state.ZBattlefield && g.IsRingBearer(o.Controller, o.ID)
	case wordTopLibrary:
		// Forge's TopLibrary: the object is the top card of its library --
		// index 0 of the owner's library slice, the card the next draw takes
		// (effects.drawFor draws lib[0]). A card deeper in the library never
		// matches, and a card that is not in a library at all never matches.
		if o.Zone != state.ZLibrary {
			return false
		}
		ids := g.Zone(state.ZLibrary, o.Owner)
		return len(ids) > 0 && ids[0] == o.ID
	case wordHasCounters:
		// Forge's HasCounters: the object has at least one counter of any
		// kind on it.
		return len(o.Counters) > 0
	case wordHistoric:
		// Forge's Historic: artifact, legendary, or Saga (the reminder text
		// on the Historic keyword).
		return hasTypeCtx(o, "Artifact", sc) || hasTypeCtx(o, "Legendary", sc) || hasTypeCtx(o, "Saga", sc)
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
	case wordTargetedPlayerCtrl:
		matched, ok := matchTargetedPlayerCtrl(g, o, sc)
		return ok && matched
	case wordThisTurnEntered:
		// Forge's ThisTurnEntered: the object entered a zone this turn (any
		// zone). The flag is the same per-object provenance
		// events.Move records that the Count$ThisTurnEntered_* heads read.
		return o.EnteredThisTurn
	case wordThisTurnEnteredFrom:
		// Forge's ThisTurnEnteredFrom_<Zone>: the object entered from <Zone>
		// this turn. An unknown zone word never reaches here (the classifier
		// fails closed), so the map lookup cannot miss.
		return o.EnteredThisTurn && o.EnteredFrom == zoneWords[key]
	case wordNamed:
		// Forge CardProperty "named<X>": card.sharesNameWith the argument.
		return sharesName(o, key)
	case wordNotnamed:
		// Forge implements no notnamed predicate and the corpus carries
		// none (measured); this engine gives the token the negation
		// semantics its shape implies rather than the always-true trap an
		// unrecognised-but-plausible token could be mistaken for.
		return !sharesName(o, key)
	case wordSameName:
		// Forge CardProperty "sameName": card.sharesNameWith(source). The
		// referent is SpecContext.Source as MatchesObjectCtx rewrote it: the
		// resolving ability's source card by default (Evil Twin's
		// ValidTgts$ Creature.sameName), or the Remembered./Targeted./
		// Triggered. context object the alternative's base prefix names
		// (Eradicate's Remembered.sameName, Bifurcate's Targeted.*,
		// Bloodbond March's Triggered.sameName). Both sides use their full
		// name characteristics: a split card off the stack contributes both
		// halves' names (CR 709.4) and a moved DFC only its front face.
		if sc.Source == 0 {
			return false
		}
		return sharesNameWithObject(o, g.Obj(sc.Source))
	case wordAttachedTo:
		// Forge's AttachedTo <X>: this object (an Aura or Equipment) is
		// attached to something, and the permanent it is attached to (its
		// own AttachedTo id) satisfies the base <X>. An unattached object
		// (AttachedTo == 0), or one whose attachment is gone, matches
		// nothing. This is the two-token counterpart of attachedBy, which
		// reads the SOURCE's AttachedTo to find what the source attaches
		// to; here we read the candidate object's own AttachedTo. The
		// dotted two-token "<class>.<qual>" form narrows the attached
		// object by its qualifier (YouCtrl: attached to a permanent the
		// spec's you controls -- Umbra Mystic); the key was validated by
		// attachedToArg, so the re-split here cannot miss.
		if o.AttachedTo == 0 {
			return false
		}
		a := g.Obj(o.AttachedTo)
		if a == nil {
			return false
		}
		if class, qual, ok := strings.Cut(key, "."); ok {
			fn, is := predicates[qual]
			if !is {
				return false
			}
			return matchesBase(g, class, a, sc) && fn(g, a, sc.You, sc.Source)
		}
		return matchesBase(g, key, a, sc)
	case wordEnchantedBy:
		// Forge's two-token "EnchantedBy <Type>.<qual>": the candidate bears
		// an attached permanent of the named type whose qualifier holds
		// against that attached object. The qualifier bodies are the map's
		// own (Other: the attached Aura is not the resolving source -- for a
		// cast the source is not yet attached, so any current Aura
		// qualifies; for a static whose source IS the attached Aura, like
		// Face of Divinity, Face itself is excluded; YouCtrl: the attached
		// Aura is controlled by the spec's you). The key was validated by
		// enchantedByArg, so the re-split here cannot miss.
		typ, qual, ok := strings.Cut(key, ".")
		if !ok {
			return false
		}
		fn, is := predicates[qual]
		if !is {
			return false
		}
		return hasAttachmentMatching(g, o.ID, sc, typ, fn)
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
	if p == "IsRemembered" || p == "token$DifferentCardNames" || strings.HasPrefix(p, "greatestPower") {
		return true
	}
	if positiveRecognisedWord(p) {
		return true
	}
	return false
}

// positiveRecognisedWord is the wordPredicate-driven half of
// positiveRecognised: a recognised classifier word (map predicate, numeric
// predicate, generic non<X> negation, or wordPredicate word).
func positiveRecognisedWord(p string) bool {
	if p == "ChosenCard" || p == "nonChosenCard" || p == "RememberedPlayerCtrl" {
		return true
	}
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

// nameArg normalises a named<Name>/notnamed<Name> argument the way Forge's
// CardProperty does: a card name containing a comma is written with ';' (the
// spec's own ',' is the OR delimiter -- `namedCalim; Djinn Emperor` names
// "Calim, Djinn Emperor"), and '_' stands for a space (`namedAether_Burst`).
func nameArg(p string) string {
	return strings.NewReplacer(";", ",", "_", " ").Replace(p)
}

// filterAlternatives splits the OR grammar without tearing a raw comma out of
// a named<Name>/notnamed<Name> argument. Forge normally spells a name comma
// as ';', but real scripts also carry e.g. Card.namedKorlash, Heir to
// Blackblade in Grandeur costs. A comma remains part of that name unless its
// right side begins a syntactic filter alternative (Card.namedX,Creature...;
// both the dotted and bare-base forms are recognised). Keeping the splitter
// shared means matching, quality classification, and the unknown-predicate
// census all parse the same filter.
func filterAlternatives(spec string) iter.Seq[string] {
	return func(yield func(string) bool) {
		start := 0
		for i := 0; i < len(spec); i++ {
			if spec[i] != ',' || rawNameComma(spec[start:i], spec[i+1:]) {
				continue
			}
			if !yield(spec[start:i]) {
				return
			}
			start = i + 1
		}
		yield(spec[start:])
	}
}

// FilterAlternatives exposes filterAlternatives to rules (the one package
// above effects): rules-side spec rewriting (the cast-provenance qualifier
// split, task castprov1) must split alternatives EXACTLY as the filter does,
// so the two cannot disagree about where a comma is a boundary.
func FilterAlternatives(spec string) iter.Seq[string] { return filterAlternatives(spec) }

// StripPredicateToken removes the EXACT predicate token from ONE filter
// alternative's "+" chain, returning the stripped alternative and whether
// the token was present. The token argument is the exact predicate text to
// remove — "pred" for the positive spelling or "!pred" for the negated one
// (the caller owns the polarity: the cast-provenance split evaluates the two
// spellings as opposite requirements). The token may ride the base's first
// predicate ("Card.wasCastFromYourHandByYou") or a later chain link
// ("Creature.!token+YouCtrl+!wasCastFromYourHandByYou"); both shapes strip
// to the remainder. The base itself (before the first angle-bracket-0 dot)
// is never touched, and an ARGUMENTED spelling of the token
// ("CastSaSource$CardManaCost", "CastSaSource/Plus.2") is a different token
// and is left in place. An alternative that is nothing but the token has no
// base and strips to "" -- the filter then fails closed on it (no corpus
// carrier writes that shape).
func StripPredicateToken(alt, token string) (string, bool) {
	base, preds := splitAltBasePreds(alt)
	if preds == "" {
		return alt, false
	}
	parts := strings.Split(preds, "+")
	out := parts[:0]
	had := false
	for _, p := range parts {
		if p == token {
			had = true
			continue
		}
		out = append(out, p)
	}
	if !had {
		return alt, false
	}
	if len(out) == 0 {
		return base, true
	}
	return base + "." + strings.Join(out, "+"), true
}

// splitAltBasePreds splits one filter alternative at the first
// angle-bracket-depth-0 dot: the base, then the "+" predicate chain
// (possibly empty). A dot inside a named<X.Y>-style argument is not the
// boundary.
func splitAltBasePreds(alt string) (string, string) {
	depth := 0
	for i := 0; i < len(alt); i++ {
		switch alt[i] {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case '.':
			if depth == 0 {
				return alt[:i], alt[i+1:]
			}
		}
	}
	return alt, ""
}

// stripBareCastSaSource removes the exact bare !CastSaSource predicate from
// every comma alternative of a Count$ThisTurnCast_ spec, reporting whether it
// was present anywhere. The bare qualifier is Forge's "other than the spell
// being cast" device (Hotheaded Giant's "unless you've cast another red
// spell this turn", Dream Thief's "another blue spell", Storm Entity's
// "each other spell cast this turn" -- the resolving spell's own PutOnStack
// is unavoidably in the window when an ETB gate reads the count); rules'
// SpellsCastThisTurnMatchingExcluding supplies the exclusion. The ARGUMENTED
// forms (!CastSaSource$CardManaCost, !CastSaSource/Plus.2 -- call_forth_the_
// tempest, thunder_salvo) are different tokens and stay in place, failing
// closed downstream as they always did.
func stripBareCastSaSource(spec string) (string, bool) {
	if !strings.Contains(spec, "CastSaSource") {
		return spec, false
	}
	var b strings.Builder
	first := true
	has := false
	for alt := range filterAlternatives(spec) {
		s1, hadNeg := StripPredicateToken(alt, "!CastSaSource")
		s2, hadPos := StripPredicateToken(s1, "CastSaSource")
		if hadNeg || hadPos {
			has = true
		}
		if !first {
			b.WriteByte(',')
		}
		b.WriteString(s2)
		first = false
	}
	return b.String(), has
}

// stripCastSaSourceAggregate removes the ARGUMENTED !CastSaSource$<Property>
// token (call_forth_the_tempest's `Card.YouCtrl+!CastSaSource$CardManaCost`:
// "damage equal to the total mana value of other spells you've cast this
// turn") from every comma alternative of a Count$ThisTurnCast_ spec,
// returning the stripped spec and the property to AGGREGATE over the
// matching casts instead of counting them one each (the aggregation
// precedent is the zone-count heads' `$<Property>` suffix read). ok is false
// when no alternative carries the token.
func stripCastSaSourceAggregate(spec string) (rest, prop string, ok bool) {
	if !strings.Contains(spec, "!CastSaSource$") {
		return "", "", false
	}
	var b strings.Builder
	first, found := true, false
	for alt := range filterAlternatives(spec) {
		s := alt
		if _, preds := splitAltBasePreds(alt); preds != "" {
			parts := strings.Split(preds, "+")
			out := parts[:0]
			had := false
			for _, p := range parts {
				if strings.HasPrefix(p, "!CastSaSource$") {
					had = true
					if !found {
						prop = strings.TrimPrefix(p, "!CastSaSource$")
					}
					continue
				}
				out = append(out, p)
			}
			if had {
				found = true
				base, _ := splitAltBasePreds(alt)
				if len(out) == 0 {
					s = base
				} else {
					s = base + "." + strings.Join(out, "+")
				}
			}
		}
		if !first {
			b.WriteByte(',')
		}
		b.WriteString(s)
		first = false
	}
	if !found || prop == "" {
		return "", "", false
	}
	return b.String(), prop, true
}

// eachAlternatives recognises Forge's multi-type search grammar:
// "EACH <typeA>[.preds] & <typeB>[.preds] ..." -- one pick of EACH listed
// type (Krosan Verge's "EACH Forest & Plains", Conflux's five Card.<Colour>
// clauses). The prefix is exactly Forge's spelling (the trimmed spec starts
// with "EACH "); the remainder splits on '&' into sub-specs, each an
// ORDINARY filter spec -- dots and '+' predicates intact. No type word, no
// predicate token in this grammar is '&' or contains it, so a flat split is
// the top-level split. An EACH spec matches a candidate when ANY listed
// sub-spec matches it; the per-type one-pick structure lives with the
// hidden-library search (effects/zone.go), which reads the sub-specs in
// order to build its option Groups.
func eachAlternatives(spec string) ([]string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(spec), "EACH ")
	if !ok || strings.TrimSpace(rest) == "" {
		return nil, false
	}
	parts := strings.Split(rest, "&")
	subs := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			// A malformed EACH spec ("A & & B") is not split: it keeps the
			// old whole-string behaviour rather than half-matching.
			return nil, false
		}
		subs = append(subs, p)
	}
	return subs, true
}

// rawNameComma reports whether the comma after left belongs to the last
// predicate of the current alternative. Once '+' has started another
// predicate, a name argument is complete and cannot own a following comma.
func rawNameComma(left, right string) bool {
	_, predicates, has := strings.Cut(left, ".")
	if !has {
		return false
	}
	last := predicates[strings.LastIndexByte(predicates, '+')+1:]
	if _, ok := strings.CutPrefix(last, "named"); !ok {
		if _, ok := strings.CutPrefix(last, "notnamed"); !ok {
			return false
		}
	}
	return !startsFilterAlternative(strings.TrimSpace(right))
}

// startsFilterAlternative recognises the base at the start of an alternative,
// independently of the candidate object. The filter grammar permits the
// universal bases and every known type word (with the ordinary non<X> base
// negation); a name continuation such as "Heir to Blackblade" is none of
// those. A card name literally ending in ", Creature" remains intrinsically
// ambiguous with the documented OR grammar and must use Forge's ';' spelling.
func startsFilterAlternative(s string) bool {
	base := s
	if i := strings.IndexAny(base, ".+,"); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimSpace(base)
	if base == "CARDNAME" || base == "Any" || base == "Card" || base == "Permanent" || base == "Spell" {
		return true
	}
	base = strings.TrimPrefix(base, "non")
	return predicateTypeWords[base]
}

// nameCharacteristics returns o's names as a name-comparison sees them --
// Forge Card.sharesNameWith. A SPLIT card away from the stack (and, as this
// engine scopes it, the battlefield, where a selected face stands for the
// permanent) has both halves' names combined (CR 709.4). A transforming DFC has
// only its front-face characteristics in those zones (CR 712.8a), even when
// its retained FaceIdx (events.Move does not reset it) still identifies the
// face it had while transformed. On the battlefield and stack every layout
// uses its selected face. Empty names are omitted; an ability object (Card
// nil) has no name.
func nameCharacteristics(o *state.Object) []string {
	if o == nil || o.Card == nil {
		return nil
	}
	offPlay := o.Zone != state.ZStack && o.Zone != state.ZBattlefield
	if offPlay && o.Card.AlternateMode == "Split" {
		var names []string
		for _, f := range o.Card.Faces {
			if f != nil && f.Name != "" {
				names = append(names, f.Name)
			}
		}
		return names
	}
	f := o.Face()
	if offPlay && len(o.Card.Faces) != 0 {
		f = o.Card.Faces[0]
	}
	if f == nil || f.Name == "" {
		return nil
	}
	return []string{f.Name}
}

// sharesName reports whether o's name characteristics include name -- Forge
// Card.sharesNameWith(String). An empty name never matches.
func sharesName(o *state.Object, name string) bool {
	if name == "" {
		return false
	}
	for _, n := range nameCharacteristics(o) {
		if n == name {
			return true
		}
	}
	return false
}

// sharesNameWithObject reports whether o and src have at least one name in
// common -- Forge Card.sharesNameWith(Card), which compares the full name
// sets of BOTH cards. A split source in a library or graveyard therefore
// shares a name with a card named for either of its halves (CR 709.4).
func sharesNameWithObject(o, src *state.Object) bool {
	for _, n := range nameCharacteristics(src) {
		if sharesName(o, n) {
			return true
		}
	}
	return false
}

// matchPositive evaluates a recognised positive-evaluation predicate token p
// to its boolean. ok is false for an unknown token OR an unbound trigger
// referent. The latter remains a recognised grammar shape for the census, but
// cannot be negated into a match when its resolution context is absent.
func matchPositive(g *state.Game, p string, o *state.Object, sc SpecContext) (result, ok bool) {
	if p == "token$DifferentCardNames" {
		// Forge's token$DifferentCardNames set-level qualifier (Sandsteppe
		// War Riders, Gimbal Gremlin Prodigy, Audience with Trostani, Neriv
		// Crackling Vanguard -- "the number of differently named <X> tokens
		// you control"). The distinctness is a COUNT-site read
		// (evalCountBody's Count$Valid walk strips the qualifier and counts
		// distinct face names over the matches); per object the recognised
		// meaning is "is a token", so the matcher and UnknownPredicates
		// agree the qualifier is known and a non-count read of it admits
		// every matching token without the distinctness narrowing.
		return o.IsToken, true
	}
	if p == "ChosenCard" || p == "nonChosenCard" {
		if !sc.ChosenValid {
			return false, true
		}
		chosen := false
		for _, t := range sc.Chosen {
			if !t.IsPlayer && t.Obj == o.ID {
				chosen = true
				break
			}
		}
		if p == "nonChosenCard" {
			chosen = !chosen
		}
		return chosen, true
	}
	if p == "RememberedPlayerCtrl" {
		// Forge's RememberedPlayerCtrl: controlled by a player this
		// resolution remembers (Price of Progress's "each player ... they
		// control" inside RepeatEach). Resolution-only; with no remembered
		// player there is no binding, so it fails closed even beneath '!'.
		return matchControlReferent(g, o, sc, "ControlledBy", "RememberedPlayer")
	}
	if p == "blockingTriggeredAttacker" {
		// Forge's Creature.blockingTriggeredAttacker (She-Hulk,
		// Wallbreaker's blocker count): the candidate is a battlefield
		// creature currently blocking the become-blocked trigger's blocked
		// attacker -- the ctx TriggerCard the per-attacker queue entry
		// carried. Resolution-only: with no TriggerCard binding (no trigger
		// ctx, or a trigger whose batch matched none) it fails closed, even
		// beneath '!'. BlockedBy lives on the ATTACKER (events.Apply's
		// DeclareBlockers case appends the blocker to the attacked
		// permanent), so the read is the triggered attacker's own list -- the
		// same read isBlocking makes, scoped to one attacker instead of any.
		if sc.TriggerCard == 0 {
			return false, true
		}
		if o.Zone != state.ZBattlefield {
			return false, true
		}
		a := g.Obj(sc.TriggerCard)
		if a == nil || a.Zone != state.ZBattlefield {
			return false, true
		}
		for _, b := range a.BlockedBy {
			if b == o.ID {
				return true, true
			}
		}
		return false, true
	}
	if p == "IsRemembered" {
		// Forge's IsRemembered (CardProperty "IsRemembered" ->
		// source.isRemembered(card)): the candidate is in the remembered list
		// of the resolving ability's source. Two bindings approximate the one
		// Forge list and are UNIONED, both fail-closed to no-match when empty:
		// the resolution's Remembered set (Ctx.Remembered -- what
		// RememberChanged$/RememberChosen$/RememberDiscarded$ and the trigger
		// capture added this walk, the "each card exiled this way" follow-up
		// shape), and the source object's event-backed Remembered (what an
		// earlier resolution remembered durably, Forge's persistent host list).
		for _, t := range sc.Remembered {
			if !t.IsPlayer && t.Obj == o.ID {
				return true, true
			}
		}
		if src := g.Obj(sc.Source); src != nil {
			for _, t := range src.Remembered {
				if !t.IsPlayer && t.Obj == o.ID {
					return true, true
				}
			}
		}
		return false, true
	}
	if rest, has := strings.CutPrefix(p, "greatestPower"); has {
		// Forge's greatestPower[ControlledBy <players>] (CardProperty): the
		// candidate is a battlefield creature controlled by the named players
		// (all battlefield creatures when no ControlledBy suffix is present)
		// whose net power no other creature in that set exceeds -- TIES MATCH,
		// every creature at the maximum is "the greatest". The candidate must
		// itself be in the set (Forge's non-LKI contains check), so a creature
		// not controlled by the named players never matches even if its power
		// is the greatest on the battlefield. Net power is read the same way
		// numericPred's power predicates read it (face power plus +1/+1
		// counters); a continuous-effect power pump is not visible from here
		// -- recorded as a known limitation in AGENTS.md.
		var players []state.PlayerID
		if ref, is := strings.CutPrefix(rest, "ControlledBy"); is {
			var ok bool
			players, ok = controlReferentPlayers(g, sc, "ControlledBy", strings.TrimSpace(ref))
			if !ok {
				// An unbound referent (no resolution, no remembered player)
				// matches nothing rather than degrading to the uncontrolled
				// whole-battlefield reading.
				return false, true
			}
		}
		if o.Zone != state.ZBattlefield || !hasType(o, "Creature") {
			return false, true
		}
		inSet := len(players) == 0
		for _, p := range players {
			if o.Controller == p {
				inSet = true
			}
		}
		if !inSet {
			return false, true
		}
		mine := objectPower(o)
		for i := range g.Objs {
			other := &g.Objs[i]
			if other.Zone != state.ZBattlefield || !hasType(other, "Creature") || other.ID == o.ID {
				continue
			}
			if len(players) > 0 {
				controlled := false
				for _, p := range players {
					if other.Controller == p {
						controlled = true
					}
				}
				if !controlled {
					continue
				}
			}
			if objectPower(other) > mine {
				return false, true
			}
		}
		return true, true
	}
	if op, ref, recognised := controlReferent(p); recognised {
		return matchControlReferent(g, o, sc, op, ref)
	}
	// Type predicates must use the derived layer-4 type list when rules
	// supplies one. Keep this before the generic predicate map: its legacy
	// functions deliberately remain useful to callers without a SpecContext,
	// but must not bypass the context-aware matcher here.
	if result, ok := typePredicate(p, g, o, sc); ok {
		return result, true
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
		return !wordMatches(nkind, nkey, g, o, sc), true
	}
	if kind, key := wordPredicate(p); kind != wordUnknown {
		return wordMatches(kind, key, g, o, sc), true
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

// hasType reads a printed type plus Changeling's type-defining ability. The
// rules package supplies SpecContext.Types when a layer-derived type list is
// available; this fallback remains deliberately useful to effects, which sits
// below rules and cannot import the layer engine.
func hasType(o *state.Object, t string) bool {
	f := o.Face()
	if f == nil {
		return false
	}
	// CR 702.114e: a bestowed card attached to a creature is an Aura, not a
	// creature, in every filter read (Count$Valid, target offer, cost
	// candidates, statics' Affected$). Derived live state
	// (state.Object.BestowedAttached); the layer walk sees the same switch
	// through rules/layers.go's bestowedTypeSwitch, and hasTypeCtx inherits
	// this gate through the hasType call below.
	if o.BestowedAttached() {
		if strings.EqualFold(t, "Aura") {
			return true
		}
		if strings.EqualFold(t, "Creature") {
			return false
		}
	}
	for _, x := range f.Types {
		if strings.EqualFold(x, t) {
			return true
		}
	}
	// Intrinsic type-defining abilities, answered in every zone (CR 613.4a):
	// Changeling's keyword and the characteristic-defining
	// AddAllCreatureTypes$ True static (Mistform Ultimus). Both go through
	// the positive subtype vocabulary, so a non-creature word (Arcane,
	// Alara, Ajani) can never leak, and neither materialises subtypes into
	// the derived type list.
	return (f.HasKeyword("Changeling") || f.AllCreatureTypesCDA()) && changelingType(t)
}

// typePredicate handles the legacy predicate-map entries whose meaning is a
// type test. Keeping them in one context-aware path ensures layer-4 derived
// types and Changeling apply consistently to both positive and negated forms.
func typePredicate(p string, g *state.Game, o *state.Object, sc SpecContext) (bool, bool) {
	switch p {
	case "Legendary", "Basic", "Snow":
		return hasTypeCtx(o, p, sc), true
	case "nonLand":
		return !hasTypeCtx(o, "Land", sc), true
	case "nonCreature":
		return !hasTypeCtx(o, "Creature", sc), true
	case "nonBasic":
		return !hasTypeCtx(o, "Basic", sc), true
	case "ChosenType":
		s := g.Obj(sc.Source)
		return s != nil && s.ChosenType != "" && hasTypeCtx(o, s.ChosenType, sc), true
	case "IsNotChosenType":
		s := g.Obj(sc.Source)
		return s != nil && s.ChosenType != "" && !hasTypeCtx(o, s.ChosenType, sc), true
	}
	return false, false
}

func hasTypeCtx(o *state.Object, t string, sc SpecContext) bool {
	// ExtraTypes is the layer walk's accumulating type list for the ONE
	// object being matched: a plain value slice, deliberately not a callable
	// resolver. Any call made through a SpecContext field makes escape
	// analysis leak the whole context to the heap on every hot-path
	// construction (the statics/action hotspot pins measure exactly that),
	// while a slice field is read-only and allocation-free.
	for _, x := range sc.ExtraTypes {
		if strings.EqualFold(x, t) {
			return true
		}
	}
	// hasType keeps intrinsic CDAs such as Changeling available without
	// materialising hundreds of creature subtypes into the derived slice.
	return hasType(o, t)
}

// changelingType reports whether t is an actual creature subtype. This uses
// a positive authoritative vocabulary rather than treating every type word
// outside an exclusion list as a creature type: Arcane, Alara, and Ajani are
// respectively spell, plane, and planeswalker subtypes, not types Changeling
// grants.
func changelingType(t string) bool { return CreatureTypeWords(t) }

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
// objectPower is the one net-power read the filter grammar shares (face
// power plus +1/+1 counters -- numericPred's power predicates and the
// greatestPower classifier both use it). A continuous-effect power pump is
// not visible from the filter path; the limitation is recorded in AGENTS.md.
func objectPower(o *state.Object) int {
	f := o.Face()
	if f == nil {
		return 0
	}
	return f.Power() + int(o.Counter("P1P1"))
}

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
			have = objectPower(o)
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

var cmcBraceNormalizer = strings.NewReplacer("{", " ", "}", " ")

// parseCMC counts a mana cost's converted value without importing rules.
func parseCMC(cost string) int32 {
	cost = cmcBraceNormalizer.Replace(cost)
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

// sameNameContextBase recognises only the three base-prefix forms which
// carry sameName's referent. Keeping this rewrite name-specific is important:
// Remembered.*, Targeted.*, and Triggered.* have many unrelated predicates
// whose grammar and behaviour this task must not expand.
func sameNameContextBase(base, rest string) bool {
	if !strings.HasPrefix(base, "Remembered") &&
		!strings.HasPrefix(base, "Targeted") &&
		!strings.HasPrefix(base, "Triggered") {
		return false
	}
	return hasPredicate(rest, "sameName")
}

func hasPredicate(rest, want string) bool {
	for p := range strings.SplitSeq(rest, "+") {
		if p == want {
			return true
		}
	}
	return false
}

// sameNameContextReferent resolves the object whose name a sameName context
// base names. An absent binding fails closed rather than falling back to the
// ability source.
func sameNameContextReferent(g *state.Game, base string, sc SpecContext) (state.ObjID, bool) {
	switch {
	case strings.HasPrefix(base, "Remembered"):
		for _, t := range sc.Remembered {
			if !t.IsPlayer && t.Obj != 0 && g.Obj(t.Obj) != nil {
				return t.Obj, true
			}
		}
	case strings.HasPrefix(base, "Targeted"):
		for _, t := range sc.ResolutionTargets {
			if !t.IsPlayer && t.Obj != 0 && g.Obj(t.Obj) != nil {
				return t.Obj, true
			}
		}
	case strings.HasPrefix(base, "Triggered"):
		if sc.TriggerCard != 0 && g.Obj(sc.TriggerCard) != nil {
			return sc.TriggerCard, true
		}
	}
	return 0, false
}

// isPermanentCard is Forge's card.isPermanent() reading used only by
// Targeted.Permanent+sameName: a battlefield object is permanent, and away
// from the battlefield a card's printed type decides it.
func isPermanentCard(o *state.Object) bool {
	if o.Zone == state.ZBattlefield {
		return true
	}
	f := o.Face()
	return f != nil && f.IsPermanent()
}

// matchesBase handles the base type, including a "non" prefix.
func matchesBase(g *state.Game, base string, o *state.Object, sc SpecContext) bool {
	if neg := strings.TrimPrefix(base, "non"); neg != base {
		return !matchesBase(g, neg, o, sc)
	}
	switch base {
	case "Any":
		return hasTypeCtx(o, "Creature", sc) || hasTypeCtx(o, "Planeswalker", sc) || hasTypeCtx(o, "Battle", sc)
	case "Card":
		return true
	case "Permanent":
		return o.Zone == state.ZBattlefield
	case "PermanentCard":
		// This internal base spelling is selected by rules' target census
		// (targetSpecForZone) and Dig windows (permanentCardSpec) for Forge's
		// `Permanent` base evaluated AWAY from the battlefield, and by rules'
		// SpellCast trigger matcher (spellCastPermanentSpec) for the permanent
		// SPELL a "cast a permanent spell" trigger evaluates on the stack. A
		// permanent CARD is anything whose printed face is a permanent type
		// (CR 109.2) wherever the object sits; the bare `Permanent` case
		// above keeps the on-the-battlefield reading every other filter
		// depends on.
		return o.Face() != nil && o.Face().IsPermanent()
	case "Spell":
		return o.Zone == state.ZStack
	case "SpellAbility":
		// Forge's SpellAbility base (ValidSource$ SpellAbility.OppCtrl on the
		// "becomes the target of a spell or ability" family -- Thunderbreak
		// Regent and 51 more files): any spell or ability object on the stack.
		// The shared filter draws the Spell/SpellAbility line by zone alone;
		// the card-spell-only distinction TargetType$ Spell draws
		// (rules/stack.go's stack kind tokens) is that machinery's own, not
		// this one's.
		return o.Zone == state.ZStack
	}
	return hasTypeCtx(o, base, sc)
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
	// PredicatePrograms is an optional immutable compiled-text sidecar. A nil
	// value keeps the textual matcher authoritative for synthetic fixtures and
	// dynamic source strings.
	PredicatePrograms *PredicatePrograms
	// ResolutionTargets are the state.Object.Targets of the spell or ability
	// currently resolving. They are deliberately absent while a target offer is
	// built: Targeted* is self-referential and cannot determine legality before
	// its own targets have been chosen. Resolving distinguishes a real empty
	// target list from no resolving object at all.
	ResolutionTargets []state.Target
	// AsStack is a DERIVED-CHARACTERISTICS override, not a resolution fact:
	// rules.derivedWith sets it while evaluating an AffectedZone$ Stack grant
	// for the spell a cast is announcing (CR 601.2b runs while the announced
	// spell is still in hand). It makes the wasCast predicate treat the
	// announced spell as the cast spell it is; nothing else reads it, and it
	// is absent from every resolution- and target-time evaluation.
	AsStack bool
	// Remembered is the resolving spell or ability's Remembered set (a
	// RepeatEach iteration binds its subject here). Like ResolutionTargets it
	// is meaningful only while Resolving. It is also the Remembered.* base
	// prefix's context referent (contextReferent): Eradicate's
	// `ChangeType$ Remembered.sameName` shares names with the captured card.
	Remembered []state.Target
	// Chosen is the current resolution's selected cards/players. It is used
	// by Forge's ChosenCard/nonChosenCard predicates, not persisted game state.
	Chosen      []state.Target
	ChosenValid bool
	Resolving   bool
	// ManaValue overrides the object's mana value for cmc predicates, with
	// HasManaValue set. It carries the CR 202.3e chosen-X effect: a caller
	// that has the chosen {X} passes the resulting mana value here so a
	// cmc restriction re-checked late (CR 601.2e) sees the spell as it is,
	// not as it was offered. Zero value with HasManaValue false is the
	// ordinary path (the printed cost, X as 0).
	ManaValue    int32
	HasManaValue bool
	// ExtraTypes optionally supplies layer-4-derived types for the ONE object
	// the spec is being matched against -- the layer walk (rules/layers.go's
	// matchesWithTypes) binds the types its effect applications have
	// accumulated so far. Ordinary filter callers leave it nil and fall back
	// to the printed type line (plus Changeling) above. A value slice,
	// deliberately not a callable resolver: a call made through a
	// SpecContext field makes escape analysis leak the whole context (its
	// Resolve closure included) to the heap on every hot-path construction.
	ExtraTypes []string
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
	if ps := sc.PredicatePrograms; ps != nil {
		switch ps.Evaluate(spec, g, o, sc) {
		case PredicateYes:
			return true
		case PredicateNo:
			return false
		}
	}
	return matchesObjectText(g, spec, o, sc)
}

// matchesObjectText is the original textual filter evaluator. It remains the
// oracle for unbound and partially compiled predicate programs.
func matchesObjectText(g *state.Game, spec string, o *state.Object, sc SpecContext) bool {
	if o == nil {
		return false
	}
	// CR 707.10h: a copy of a SPELL that has left the stack (countered,
	// fizzled, or otherwise gone) matches nothing -- it is a transient
	// reference, not a real object anymore. That is what IsCopy+off-stack
	// was meant to catch, but a blanket "any zone but the stack" also
	// rejected a permanent copy legitimately living on the battlefield
	// (Clone, Rite of Replication, a Myriad/Encore token copy, ...), which
	// must match ordinary filters -- including its own and every bystander's
	// ChangesZone triggers -- exactly like any other permanent. Only reject
	// a copy that is neither on the stack (still a spell) nor on the
	// battlefield (still a permanent).
	if o.IsCopy && o.Zone != state.ZStack && o.Zone != state.ZBattlefield {
		return false
	}
	resolve := sc.Resolve
	if resolve == nil {
		resolve = noResolve
	}
	// Forge's EACH multi-type search grammar: the spec is a '&' list of
	// ordinary sub-specs, and the union matches. Sub-specs are evaluated
	// through this same oracle, so their own predicates and bases keep the
	// ordinary semantics (a bare "EACH Forest & Plains" previously reached
	// the type walk as ONE base and matched nothing).
	if subs, ok := eachAlternatives(spec); ok {
		for _, sub := range subs {
			if matchesObjectText(g, sub, o, sc) {
				return true
			}
		}
		return false
	}
	for alt := range filterAlternatives(spec) {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		base, rest, _ := strings.Cut(alt, ".")
		asc := sc
		contextualSameName := sameNameContextBase(base, rest)
		// Forge's sameName forms can name their referent in the base:
		// Remembered.sameName, Targeted.Permanent+sameName, and
		// Triggered.sameName. Rewrite only those name-predicate alternatives;
		// a global rewrite would activate unrelated Remembered/Targeted/
		// Triggered filters outside this task's scope.
		if contextualSameName {
			ref, bound := sameNameContextReferent(g, base, asc)
			if !bound {
				continue
			}
			asc.Source = ref
			base = "Card"
		}
		if base == "CARDNAME" {
			// CR 201.5: a self-reference means this object, not another
			// object with the same name. Without a source, fail closed.
			if sc.Source == 0 || o.ID != sc.Source {
				continue
			}
		} else if !matchesBase(g, base, o, sc) {
			continue
		}
		all := true
		for p := range strings.SplitSeq(rest, "+") {
			if p == "" {
				continue
			}
			// Permanent is an auxiliary part of Forge's
			// Targeted.Permanent+sameName base spelling, not a globally
			// implemented predicate. Limit its type-based reading to that
			// contextual sameName form so Card.Permanent remains fail-closed.
			if contextualSameName && p == "Permanent" {
				if !isPermanentCard(o) {
					all = false
					break
				}
				continue
			}
			// matchPredicate evaluates every recognised shape -- the predicates
			// map, a numeric predicate, a generic non<X> negation, a
			// wordPredicate classifier word, and a leading-'!' negation of any
			// of those -- to a boolean. An unrecognised token (ok == false) is
			// unknown, so it fails closed: never an always-true fallback, which
			// would silently widen the filter instead of showing up as a
			// missing action.
			res, ok := matchPredicate(g, p, o, asc)
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

// matchesZoneSpecCtx matches a filter over a known zone. Forge's Permanent
// base names a permanent card when a count already scoped the candidates to a
// non-battlefield zone; it must not re-check the object's current zone and
// reject every graveyard, hand, library, or exile card. All other bases and
// predicates retain MatchesObjectCtx's ordinary semantics.
func matchesZoneSpecCtx(g *state.Game, spec string, id state.ObjID, sc SpecContext, zone state.Zone) bool {
	o := g.Obj(id)
	if o == nil {
		return false
	}
	if zone == state.ZBattlefield {
		return MatchesObjectCtx(g, spec, o, sc)
	}
	// filterAlternatives, not a raw comma split: a Count$Valid<Zone>
	// Card.named<Name> argument may carry its printed comma.
	for alt := range filterAlternatives(spec) {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		base, rest, _ := strings.Cut(alt, ".")
		if base == "CARDNAME" {
			if sc.Source == 0 || o.ID != sc.Source {
				continue
			}
		} else if !matchesBaseInZone(g, base, o, sc, zone) {
			continue
		}
		all := true
		for p := range strings.SplitSeq(rest, "+") {
			if p == "" {
				continue
			}
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

func matchesBaseInZone(g *state.Game, base string, o *state.Object, sc SpecContext, zone state.Zone) bool {
	if neg := strings.TrimPrefix(base, "non"); neg != base {
		return !matchesBaseInZone(g, neg, o, sc, zone)
	}
	if base != "Permanent" || zone == state.ZBattlefield {
		return matchesBase(g, base, o, sc)
	}
	return hasTypeCtx(o, "Artifact", sc) || hasTypeCtx(o, "Creature", sc) || hasTypeCtx(o, "Enchantment", sc) ||
		hasTypeCtx(o, "Land", sc) || hasTypeCtx(o, "Planeswalker", sc) || hasTypeCtx(o, "Battle", sc)
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
// It recognizes the state-local Active and life comparison qualifiers used by
// life triggers/replacements; every other qualifier still fails closed.
func MatchesPlayerSpec(g *state.Game, spec string, p, you state.PlayerID) bool {
	return MatchesPlayerSpecFrom(g, spec, p, you, 0)
}

// MatchesPlayerSpecFrom also resolves Player.Chosen and Player.IsRemembered
// against the source object's event-backed choice state. Keeping the source
// explicit lets ordinary player filters retain their existing API while
// trigger matching can supply its owning permanent.
func MatchesPlayerSpecFrom(g *state.Game, spec string, p, you state.PlayerID, source state.ObjID) bool {
	for _, alt := range strings.Split(spec, ",") {
		base, qualifier, qualified := strings.Cut(strings.TrimSpace(alt), ".")
		if (base == "Player" || base == "Any") && qualified && (qualifier == "Chosen" || qualifier == "IsRemembered") {
			o := g.Obj(source)
			if o == nil {
				continue
			}
			set := o.Chosen
			if qualifier == "IsRemembered" {
				set = o.Remembered
			}
			for _, t := range set {
				if t.IsPlayer && t.Player == p {
					return true
				}
			}
			continue
		}
		matchesBase := false
		switch base {
		case "Player", "Any":
			if kind, is := strings.CutPrefix(qualifier, "withMost"); is {
				// Forge's Player.withMost<kind> property (PlayerProperty), now
				// evaluated in the shared player filter so a control grant's
				// NewController$ and a trigger's Attacked$/restriction spec
				// resolve the same seat. Unknown kinds fail closed (no seat
				// matches) exactly like every other unlisted qualifier.
				if playerHasMost(g, p, kind) {
					return true
				}
				continue
			}
			if rem, is := strings.CutPrefix(qualifier, "controlsCreature."); is {
				// Forge's Player.controlsCreature.<objspec> / controlsPermanent.
				// <objspec> property (PlayerControlsCreatures/Permanents): the
				// seat qualifies when its battlefield holds an object matching
				// <objspec> as an object filter, with an optional trailing
				// _GE<n>-style count comparison. See playerControlsMatches.
				if playerControlsMatches(g, p, you, source, "Creature", rem) {
					return true
				}
				continue
			}
			if rem, is := strings.CutPrefix(qualifier, "controlsPermanent."); is {
				if playerControlsMatches(g, p, you, source, "Permanent", rem) {
					return true
				}
				continue
			}
			matchesBase = true
		case "You":
			matchesBase = p == you
		case "Opponent", "Other":
			matchesBase = p != you
		}
		if !matchesBase {
			continue
		}
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
		case "Active":
			if p == g.Active {
				return true
			}
		case "isMonarch":
			// CR 716.2's monarch designation, on the Player/Any base only:
			// the state-local qualifier a control static's GainControl$
			// value (Fealty to the Realm's "The monarch controls enchanted
			// creature") and any other player spec resolve through. A
			// qualified You/Opponent/Other base (You.isMonarch) still fails
			// closed, like every fx20 qualifier not listed here.
			if (base == "Player" || base == "Any") && g.IsMonarch(p) {
				return true
			}
		default:
			if int(p) < len(g.Players) {
				op, n, ok := splitPlayerCompare(qualifier)
				if ok && playerCompare(g.Players[p].Life, op, n) {
					return true
				}
			}
		}
	}
	return false
}

// splitCountCompare strips a trailing "_"-separated count comparison token
// ("GE1", "LT3", ...) from an object-spec remainder. It returns the remainder
// with the token removed, the comparison operator, the threshold, and whether
// a count token was present at all. A trailing token that is not a count
// comparison (e.g. the named-arg convention's "namedAether_Burst") stays part
// of the object spec, and a remainder with no "_" at all is returned whole.
func splitCountCompare(rem string) (string, string, int32, bool) {
	i := strings.LastIndex(rem, "_")
	if i < 0 {
		return rem, "", 0, false
	}
	tok := rem[i+1:]
	if len(tok) < len("GE0") {
		return rem, "", 0, false
	}
	op, digits := tok[:2], tok[2:]
	n, err := strconv.ParseInt(digits, 10, 32)
	if err != nil {
		return rem, "", 0, false
	}
	switch op {
	case "GE", "GT", "EQ", "LE", "LT":
		return rem[:i], op, int32(n), true
	}
	return rem, "", 0, false
}

// playerControlsMatches evaluates Forge's Player.controlsCreature.<spec> /
// controlsPermanent.<spec> qualifiers (PlayerProperty's
// PlayerControlsCreatures/PlayerControlsPermanents family): the seat
// qualifies when the required number of its battlefield objects match
// <spec> as an object filter. The count comparison rides a trailing
// "_GE<n>"-style token and defaults to an existential _GE1; a spec with no
// count token matches when at least one object does. The object filter is
// evaluated with the same SpecContext binding MatchesPlayerSpecFrom carries
// (the perspective seat and the source permanent), so the named<Name>,
// MultiColor, IsRemembered and EnchantedBy object predicates all resolve
// unchanged. A spec that matches nothing -- including one carrying an
// unmodelled predicate, which fails closed inside the object matcher --
// never matches for that seat.
func playerControlsMatches(g *state.Game, p state.PlayerID, you state.PlayerID, source state.ObjID, objBase, rem string) bool {
	spec, op, want, counted := splitCountCompare(rem)
	spec = objBase + "." + spec
	sc := SpecContext{You: you, Source: source}
	n := int32(0)
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if MatchesObjectCtx(g, spec, g.Obj(id), sc) {
			n++
		}
	}
	if !counted {
		return n > 0
	}
	return playerCompare(n, op, want)
}

// playerHasMost is the shared evaluator for Forge's Player.withMost<kind>
// property (PlayerProperty.java). Kinds the corpus spells: Life (ties match
// -- every seat holding the maximum life), CardsInHand (Forge's
// strictly-greater scan keeps the FIRST holder in player order on a tie),
// PermanentInPlay (most permanents; ties match every holder) and
// Type<X>[Only] (most battlefield permanents of type X; "Only" requires a
// UNIQUE holder, and when the top count is shared nobody matches -- Forge
// returns false for every player). Dead seats take part in the scan exactly
// like Forge's game.getPlayers(); the control-grant caller walks only the
// living seats before consulting this, so a dead seat can win a filter match
// but never gain control.
func playerHasMost(g *state.Game, p state.PlayerID, kind string) bool {
	if int(p) >= len(g.Players) {
		return false
	}
	only := false
	if x, is := strings.CutSuffix(kind, "Only"); is {
		only = true
		kind = x
	}
	switch kind {
	case "Life":
		best := g.Players[0].Life
		for i := range g.Players {
			if g.Players[i].Life > best {
				best = g.Players[i].Life
			}
		}
		return g.Players[p].Life == best
	case "CardsInHand":
		// Forge's getPlayerWithMostCardsInHand starts with no candidate and
		// only binds when a player has a positive hand; all-empty hands name
		// nobody. Ties retain the first player in seat order.
		best, holder := 0, -1
		for i := range g.Players {
			if n := len(g.Zone(state.ZHand, state.PlayerID(i))); n > best {
				best, holder = n, i
			}
		}
		return holder >= 0 && int(p) == holder
	}
	count := func(pi state.PlayerID) int {
		n := 0
		for _, id := range g.Zone(state.ZBattlefield, pi) {
			o := g.Obj(id)
			if o == nil {
				continue
			}
			if t, is := strings.CutPrefix(kind, "Type"); is {
				if hasType(o, t) {
					n++
				}
				continue
			}
			if kind == "PermanentInPlay" {
				n++
			}
		}
		return n
	}
	best, holders := -1, 0
	for i := range g.Players {
		n := count(state.PlayerID(i))
		if n > best {
			best, holders = n, 1
		} else if n == best {
			holders++
		}
	}
	// Forge requires a unique leader for PermanentInPlay as well as the
	// explicit Type...Only spelling. A shared top count names nobody.
	if (only || kind == "PermanentInPlay") && holders != 1 {
		return false
	}
	return count(p) == best
}

// splitPlayerCompare accepts Forge's lifeGE1/lifeLT7 player qualifiers.
func splitPlayerCompare(s string) (string, int32, bool) {
	if !strings.HasPrefix(s, "life") || len(s) < len("lifeGE0") {
		return "", 0, false
	}
	op := s[4:6]
	n, err := strconv.ParseInt(s[6:], 10, 32)
	if err != nil {
		return "", 0, false
	}
	switch op {
	case "GE", "GT", "EQ", "LE", "LT":
		return op, int32(n), true
	}
	return "", 0, false
}

func playerCompare(have int32, op string, want int32) bool {
	switch op {
	case "GE":
		return have >= want
	case "GT":
		return have > want
	case "EQ":
		return have == want
	case "LE":
		return have <= want
	case "LT":
		return have < want
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
	// An EACH spec states a quality when ANY listed sub-spec does -- every
	// real carrier lists a named type, so an EACH library search keeps
	// CR 701.23b's fail-to-find allowance. A quantity-only EACH (none in the
	// corpus) would keep the mandatory-find reading of its sub-specs.
	if subs, ok := eachAlternatives(spec); ok {
		for _, sub := range subs {
			if SearchStatesQuality(sub) {
				return true
			}
		}
		return false
	}
	for alt := range filterAlternatives(spec) {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		base, rest, _ := strings.Cut(alt, ".")
		if base != "Card" && base != "Any" {
			return true
		}
		for p := range strings.SplitSeq(rest, "+") {
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
	// An EACH spec is split first: the sub-specs' unknowns are the union, so
	// the census is truthful for the multi-type grammar (the dotted dotted
	// form previously leaked the '&' join and every later clause as garbage
	// predicate tokens; the bare form's unknown base was never checked at
	// all, because a base-position token is not a predicate).
	if subs, ok := eachAlternatives(spec); ok {
		for _, sub := range subs {
			out = append(out, UnknownPredicates(sub)...)
		}
		sort.Strings(out)
		return out
	}
	for alt := range filterAlternatives(spec) {
		_, rest, _ := strings.Cut(strings.TrimSpace(alt), ".")
		for p := range strings.SplitSeq(rest, "+") {
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
