package effects

import (
	"math/bits"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// colourLetter parses one Forge colour spelling -- a WUBRG letter or a full
// colour word, case-insensitive -- to its WUBRG letter, or 0 when the token
// names no colour. The callers that fail closed on 0 keep an unreadable
// colour unresolvable rather than counting a fake zero.
func colourLetter(s string) byte {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) == 1 {
		switch s[0] {
		case 'W', 'U', 'B', 'R', 'G':
			return s[0]
		}
		return 0
	}
	if v, ok := colourLetterTab.Get(s); ok {
		return v
	}
	return 0
}

// devotionCount is CR 700.5's devotion to one or two colours: the number of
// mana symbols whose colours intersect mask among the mana costs of the
// permanents the player controls. Every printed spelling the corpus carries
// counts once -- plain pips ("2 B B"), two-colour hybrid ("GW", CR 107.4e: a
// symbol that is both colours), monocolour hybrid ("2B"/"2/B"), Phyrexian
// ("BP", CR 107.4f: it is a symbol of its colour) and the compleated
// tricolour ("GWP"/"PRG") -- through cards.ManaCostChroma's per-symbol read,
// so a symbol of both queried colours is never counted twice. A face-down
// battlefield permanent has no mana cost (CR 708.5's vanilla set) and
// contributes nothing; an out-of-range controller counts nothing.
func devotionCount(g *state.Game, controller state.PlayerID, mask uint8) int32 {
	if int(controller) < 0 || int(controller) >= len(g.Players) {
		return 0
	}
	var n int32
	for _, id := range g.Zone(state.ZBattlefield, controller) {
		o := g.Obj(id)
		if o == nil || (o.FaceDown && o.Zone == state.ZBattlefield) {
			continue
		}
		f := o.Face()
		if f == nil {
			continue
		}
		n += cards.ManaCostChroma(f.ManaCost, mask)
	}
	return n
}

// countZone maps a Count$ head to the zone it scopes over. ValidAll is NOT
// here: it scopes over every zone at once (countAllZones plus one stack
// pass, handled directly in the zone-count branch), and a single-zone
// mapping cannot express that.
// multiCountZones parses a multi-zone count head, Valid<Zone>,<Zone>[,...]
// (Count$ValidGraveyard,Battlefield Cave.YouCtrl), into its zones: nil for a
// single-zone head (countZone's) or any list naming an unknown or repeated
// zone word.
func multiCountZones(head string) []state.Zone {
	rest, ok := strings.CutPrefix(head, "Valid")
	if !ok || !strings.Contains(rest, ",") {
		return nil
	}
	var out []state.Zone
	for _, w := range strings.Split(rest, ",") {
		z, known := zoneWords[w]
		if !known {
			return nil
		}
		for _, have := range out {
			if have == z {
				return nil
			}
		}
		out = append(out, z)
	}
	return out
}

func countZone(head string) (state.Zone, bool) {
	switch countZoneCodes.Code(string(head)) {
	case countZoneValid:
		return state.ZBattlefield, true
	case countZoneValidHand:
		return state.ZHand, true
	case countZoneValidGraveyard:
		return state.ZGraveyard, true
	case countZoneValidLibrary:
		return state.ZLibrary, true
	case countZoneValidExile:
		return state.ZExile, true
	case countZoneValidStack:
		return state.ZStack, true
	case countZoneValidCommand:
		return state.ZCommand, true
	}
	return 0, false
}

// zoneCountFold is the per-candidate accumulator the zone-count branch's
// scan drives. The visit body is shared between the single-zone scan and
// the ValidAll all-zones scan through a pointer-receiver method, so neither
// loop materialises a candidate slice and the common single-zone head keeps
// its allocation-free iteration (the maps are created only by the CardTypes
// and token$DifferentCardNames heads, which need a heap map anyway; the
// struct itself stays on the caller's stack because visit never leaks its
// receiver). The spec's SpecContext is NOT a field: it is built once by the
// caller and passed to visit as a parameter, so the *Ctx the caller built
// (the hot layer-walk Ctx) never escapes through this type.
type zoneCountFold struct {
	h           Host
	g           *state.Game
	spec        string
	pricedSpell state.ObjID
	prop        string
	extreme     bool
	isLeast     bool
	hasBareHand bool
	// readsPT is computed ONCE per fold from SpecReadsPT(spec): the fold
	// binds a battlefield candidate's layer-derived P/T into its SpecContext
	// only when the spec actually reads one of the four P/T comparison
	// fields. Without the gate every Count$Valid pays a full rules layer walk
	// per candidate, and a P/T CDA that itself counts permanents (Master of
	// Etherium) makes that walk recurse into the same count -- the
	// in-progress frame guard stops the cycle but not the factorial fan-out
	// (4 Masters 2.9 ms, 5 44 ms, 6 577 ms, 7 10.6 s for ONE Derived).
	readsPT        bool
	n, best        int32
	seen           bool
	seenTokenNames map[string]bool
	seenCardTypes  map[string]bool
	// seenCreatureTypes is the CreatureType spelling's distinct set (task
	// diffcount2): creature-subtype words among the matched faces, read
	// only through len.
	seenCreatureTypes map[string]bool
	// seenPlaneswalkerTypes is the PlaneswalkerType spelling's distinct set
	// (cr-g222): planeswalker-subtype words among the matched faces, read
	// only through len.
	seenPlaneswalkerTypes map[string]bool
	colorsSeen            ColorMask
	// diffKind and the seen sets for the Different* distinct-set property
	// family (task diffcount1). Both maps are read only through len, so no
	// map ordering ever reaches an event or a view.
	diffKind       differentPropertyKind
	seenDiffValues map[int32]bool
	seenDiffNames  map[string]bool
}

// visit folds one candidate: the shared per-candidate body of the
// zone-count scan (the bare-hand provenance split, the match against the
// candidate's OWN zone, then the plain-count / extreme / sum /
// distinct-set accumulation).
func (f *zoneCountFold) visit(id state.ObjID, zone state.Zone, specCtx SpecContext) {
	if zone == state.ZBattlefield && id == specCtx.ExcludeFromBattlefieldCount && id != 0 {
		return
	}
	if id == f.pricedSpell && id != 0 {
		return
	}
	matchSpec := f.spec
	if f.hasBareHand {
		s, ok := castFromHandAnyAdmitsFilter(f.h, f.spec, id)
		if !ok {
			return
		}
		matchSpec = s
	}
	// Count$Valid is an effects-side scan, but battlefield numeric filters
	// still read rules' layer-derived characteristics. Bind the candidate's
	// values through a small optional value interface; effects remains below
	// rules and SpecContext carries no callable resolver. Skip the bind
	// entirely unless the spec reads a P/T comparison field (readsPT): a
	// spec that reads none cannot observe the values, and the bind runs a
	// full rules layer walk per candidate -- which a P/T CDA that counts
	// permanents turns into a factorial recursion (see readsPT's field
	// comment).
	if zone == state.ZBattlefield && f.readsPT {
		if provider, ok := f.h.(interface {
			FilterDerivedPT(state.ObjID) (power, toughness, basePower, baseToughness int32, ok bool)
		}); ok {
			if power, toughness, basePower, baseToughness, found := provider.FilterDerivedPT(id); found {
				specCtx.DerivedPower, specCtx.DerivedToughness, specCtx.HasDerivedPT = power, toughness, true
				specCtx.BasePower, specCtx.BaseToughness, specCtx.HasBasePT = basePower, baseToughness, true
			}
		}
	}
	specCtx.DerivedPTs = append(specCtx.DerivedPTs, GreatestPowerDerivedPTs(f.g, matchSpec, f.h)...)
	if !matchesZoneSpecCtx(f.g, matchSpec, id, specCtx, zone) {
		return
	}
	if f.prop == "" {
		if f.seenTokenNames != nil {
			if o := f.g.Obj(id); o != nil && o.Face() != nil {
				f.seenTokenNames[o.Face().Name] = true
			}
			return
		}
		f.n++
		return
	}
	o := f.g.Obj(id)
	if o == nil || o.Face() == nil {
		return
	}
	if f.extreme {
		// The DERIVED, layer-aware characteristic (h.Power/h.Toughness),
		// not the printed face: Forge sizes "greatest power" from the
		// game's actual power, so a lord's bonus or a -1/-1 counter
		// counts. The sibling CardPower/CardToughness sums read the same
		// derived value for a battlefield permanent.
		v := extremePropertyValue(f.h, o, f.prop)
		if !f.seen || (f.isLeast && v < f.best) || (!f.isLeast && v > f.best) {
			f.best, f.seen = v, true
		}
		return
	}
	// CardCounters.<KIND> sums one counter kind over the matched set (Kate
	// Stewart's time counters, Kyler's P1P1); CardCounters.ALL sums every
	// kind. state.Object.Counter is the ONE home for that marker, so this
	// read and the $<Ref>$CardCounters readers (evalRefProperty, the bare
	// source head) cannot disagree.
	if kind, ok := strings.CutPrefix(f.prop, "CardCounters."); ok {
		f.n += o.Counter(kind)
		return
	}
	switch zoneFoldPropCodes.Code(string(f.prop)) {
	case zoneFoldPropCardPower:
		// A permanent's power is its current, layer-derived power (CR
		// 208.3; Forge's getNetPower): Ghalta, Primal Hunger's "the total
		// power of creatures you control" counts an anthem's bonus. Off the
		// battlefield the card's printed value stands (plus any counters).
		if o.Zone == state.ZBattlefield {
			f.n += f.h.Power(o.ID)
			return
		}
		dp, _ := o.CounterPTTotals()
		f.n += int32(o.Face().Power()) + dp
	case zoneFoldPropCardToughness:
		if o.Zone == state.ZBattlefield {
			f.n += f.h.Toughness(o.ID)
			return
		}
		_, dt := o.CounterPTTotals()
		f.n += int32(o.Face().Toughness()) + dt
	case zoneFoldPropCardManaCost:
		f.n += int32(objectManaValue(f.h.Game(), o))
	case zoneFoldPropCardTypes:
		vocab := cardTypeWords
		if f.prop == "CardTypesPermanent" {
			vocab = permanentTypeWords
		}
		for _, typ := range o.Face().Types {
			if vocab[typ] {
				f.seenCardTypes[typ] = true
			}
		}
	case zoneFoldPropCreatureType:
		for _, typ := range o.Face().Types {
			if creatureSubtypeWords[typ] {
				f.seenCreatureTypes[typ] = true
			}
		}
	case zoneFoldPropPlaneswalkerType:
		// CR 205.3j: a planeswalker type is one of the subtypes printed on a
		// planeswalker's type line. Read every subtype word off the matched
		// object itself rather than gating on a separate vocabulary: Forge's
		// CardType.getPlaneswalkerTypes() returns the card's subtypes when it
		// is a planeswalker, and a roster can fall behind a printed type
		// (Svega, the Unconventional) into the same silent zero this property
		// used to read. CR 205.1 orders the type line supertypes, then card
		// types, then subtypes, so the subtypes are the words after the
		// Planeswalker card type. An object without the card type contributes
		// nothing.
		types := o.Face().Types
		walker := -1
		for i, typ := range types {
			if typ == "Planeswalker" {
				walker = i
				break
			}
		}
		if walker < 0 {
			return
		}
		for _, typ := range types[walker+1:] {
			f.seenPlaneswalkerTypes[typ] = true
		}
	case zoneFoldPropColors:
		f.colorsSeen |= ColorMaskOf(o)
	default:
		// The Different* distinct-set properties (task diffcount1): each
		// matching object contributes its value to the seen set. The nil-face
		// guard lives inside differentPropertyValue; visit's early return
		// already guarantees o.Face() != nil for the name read.
		if f.diffKind != diffNone {
			if f.seenDiffNames != nil {
				f.seenDiffNames[o.Face().Name] = true
			} else if v, ok := differentPropertyValue(f.h, o, f.diffKind); ok {
				f.seenDiffValues[v] = true
			}
		}
	}
}

// countAllZones is the ordered per-seat zone list a Count$ValidAll body
// scans -- every per-player card zone in enum order. ZStack is global and is
// appended once by the ValidAll branch itself, never here; ZCeased has no
// membership list and is never scanned. The order matters only for
// determinism -- a count and an extreme fold are order-insensitive -- but a
// fixed order keeps every evaluation byte-identical run to run.
var countAllZones = []state.Zone{
	state.ZLibrary, state.ZHand, state.ZBattlefield,
	state.ZGraveyard, state.ZExile, state.ZCommand,
}

// isExtremeProperty reports whether prop is one of the four extreme-reduction
// property suffixes (`Count$Valid <spec>$GreatestCardPower` and its siblings),
// as opposed to the summed (CardPower/CardManaCost) or distinct-set
// (CardTypes/Colors) properties. This is the ENTIRE extreme grammar: the
// corpus carries no other spelling and no argument-less form. A property the
// corpus writes but this build does not yet read -- the Different* distinct
// family -- is deliberately NOT admitted here and keeps the whole-token
// fail-closed read.
func isExtremeProperty(prop string) bool {
	return isExtremePropertySet.Has(prop)
}

// differentPropertyKind classifies a Different* distinct-set property -- the
// value family the count dedups over. diffNone means prop is not one of them.
type differentPropertyKind int

const (
	// diffNone is the zero value: prop is not a Different* property.
	diffNone differentPropertyKind = iota
	// diffManaCost: DifferentCardManaCost -- distinct printed mana values.
	diffManaCost
	// diffPower: DifferentCardPower -- distinct DERIVED powers (a lord's
	// bonus or a -1/-1 counter changes the value, matching the
	// GreatestCardPower read).
	diffPower
	// diffToughness: DifferentCardToughness -- distinct derived toughnesses.
	diffToughness
	// diffName: DifferentCardNames -- distinct face names.
	diffName
	// diffColorPair: DifferentColorPair -- distinct two-colour pairs among
	// permanents that are EXACTLY two colours (Niv-Mizzet, Guildpact).
	diffColorPair
)

// differentPropertyKindOf classifies the Different* distinct-set property
// family of a Count$Valid<zone> <spec>$<Property> body: the properties that
// count DISTINCT VALUES among the matching cards rather than the cards
// themselves. The three numeric spellings and the name spelling all existed in
// the corpus unread (whole-token fail-closed to zero) before task diffcount1;
// classifying them in ONE place keeps the matcher, the value fold and the
// verdict from drifting apart. A spelling outside this set returns diffNone
// and keeps the pre-existing behaviour.
func differentPropertyKindOf(prop string) differentPropertyKind {
	if v, ok := differentPropertyKindOfTab.Get(prop); ok {
		return v
	}
	return diffNone
}

// differentPropertyValue reads one matching object's contribution to a
// numeric Different* set. ok is false when the object contributes nothing
// (an exactly-two-colour property read against a card that is not exactly two
// colours), so the value is never a meaningless zero that would collide with
// a real zero-mana-value card. A diffName property has no numeric value and
// must be handled through the name map at the call site; it returns ok=false
// here so a caller that routed it wrongly adds nothing rather than a zero
// that would inflate the count.
func differentPropertyValue(h Host, o *state.Object, kind differentPropertyKind) (int32, bool) {
	switch kind {
	case diffManaCost:
		// Face() is nil for a Card==nil or out-of-range FaceIdx object
		// (engine/state/object.go); a remembered/targeted shell contributes
		// nothing rather than panicking the match. objectManaValue applies
		// the CR 709.5 Room rule for a battlefield Room.
		if o.Face() != nil {
			return int32(objectManaValue(h.Game(), o)), true
		}
		return 0, false
	case diffPower:
		// The derived, layer-aware power, matching extremePropertyValue's
		// GreatestCardPower read: a lord's bonus or a counter counts.
		return refPower(h, o, false), true
	case diffToughness:
		return refToughness(h, o, false), true
	case diffColorPair:
		mask := ColorMaskOf(o)
		// Only permanents that are EXACTLY two colours contribute a pair
		// (Niv-Mizzet, Guildpact's "exactly two colors"): a monocoloured or
		// colourless permanent has no pair to contribute.
		if bits.OnesCount8(uint8(mask)) != 2 {
			return 0, false
		}
		return int32(mask), true
	}
	return 0, false
}

// isLeastProperty reports whether an extreme property takes the MINIMUM over
// the matches (Least*) rather than the maximum (Greatest*).
func isLeastProperty(prop string) bool {
	return prop == "LeastCardPower"
}

// extremePropertyValue reads one object's contribution to an extreme
// property: the DERIVED, layer-aware power/toughness (h.Power/h.Toughness)
// for the power/toughness extremes, so a lord's bonus or a -1/-1 counter is
// seen the way Forge sizes "greatest power", and the printed mana value for
// GreatestCardManaCost (the same read the summed CardManaCost case uses).
// An unrecognised extreme reads 0 -- but isExtremeProperty admitted it, so a
// missing case here is a compile-time-visible oversight, not a silent one.
func extremePropertyValue(h Host, o *state.Object, prop string) int32 {
	switch extremePropertyValueCodes.Code(string(prop)) {
	case extremePropertyValueExtremeCardPower:
		return h.Power(o.ID)
	case extremePropertyValueGreatestCardToughness:
		return h.Toughness(o.ID)
	case extremePropertyValueGreatestCardManaCost:
		return int32(objectManaValue(h.Game(), o))
	}
	return 0
}

// controlsAllUrzaLands reports whether the player controls at least one
// permanent of each Urza land subtype on the battlefield -- the "is the Urza
// lands assembly complete?" predicate Count$UrzaLands encodes. Subtypes are
// matched on the Types line, where Power-Plant is hyphenated: the card Name
// "Urza's Power Plant" is a different string and matching it would be the
// exact defect the UrzaLands head exists to avoid. Iterating the dense
// object arena in order (never a map) keeps the count deterministic; it scans
// every battlefield permanent regardless of who owns it, so the controller
// test is purely o.Controller.
func controlsAllUrzaLands(g *state.Game, controller state.PlayerID) bool {
	var mine, tower, plant bool
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Controller != controller || o.Face() == nil {
			continue
		}
		switch {
		case hasSubtype(o, "Urza's Mine"):
			mine = true
		case hasSubtype(o, "Urza's Tower"):
			tower = true
		case hasSubtype(o, "Urza's Power-Plant"):
			plant = true
		}
		if mine && tower && plant {
			return true
		}
	}
	return false
}

// hasSubtype reports whether an object's type line contains the given
// (possibly multi-word) subtype as consecutive tokens. Forge writes a
// subtype such as "Urza's Mine" inside the space-separated Types line, so
// the engine's fields-split representation breaks it into ["Land","Urza's",
// "Mine"]; a single-token EqualFold is therefore wrong for these and must be
// a consecutive-token match. An empty target never matches.
func hasSubtype(o *state.Object, sub string) bool {
	f := o.Face()
	if f == nil {
		return false
	}
	words := strings.Fields(sub)
	if len(words) == 0 {
		return false
	}
	for i := 0; i+len(words) <= len(f.Types); i++ {
		match := true
		for j, w := range words {
			if !strings.EqualFold(f.Types[i+j], w) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

var colourLetterTab = state.NewStrTable[byte](
	state.StrEntry[byte]{Key: "WHITE", Val: 'W'},
	state.StrEntry[byte]{Key: "BLUE", Val: 'U'},
	state.StrEntry[byte]{Key: "BLACK", Val: 'B'},
	state.StrEntry[byte]{Key: "RED", Val: 'R'},
	state.StrEntry[byte]{Key: "GREEN", Val: 'G'},
)

var isExtremePropertySet = state.NewNameSet(
	"GreatestCardPower",
	"GreatestCardToughness",
	"GreatestCardManaCost",
	"LeastCardPower",
)

var differentPropertyKindOfTab = state.NewStrTable[differentPropertyKind](
	state.StrEntry[differentPropertyKind]{Key: "DifferentCardManaCost", Val: diffManaCost},
	state.StrEntry[differentPropertyKind]{Key: "DifferentCardPower", Val: diffPower},
	state.StrEntry[differentPropertyKind]{Key: "DifferentCardToughness", Val: diffToughness},
	state.StrEntry[differentPropertyKind]{Key: "DifferentCardNames", Val: diffName},
	state.StrEntry[differentPropertyKind]{Key: "DifferentColorPair", Val: diffColorPair},
)

type countZoneCode uint16

const (
	countZoneValid countZoneCode = iota + 1
	countZoneValidHand
	countZoneValidGraveyard
	countZoneValidLibrary
	countZoneValidExile
	countZoneValidStack
	countZoneValidCommand
)

var countZoneCodes = state.NewStrCodes(
	state.StrEntry[countZoneCode]{Key: "Valid", Val: countZoneValid},
	state.StrEntry[countZoneCode]{Key: "ValidHand", Val: countZoneValidHand},
	state.StrEntry[countZoneCode]{Key: "ValidGraveyard", Val: countZoneValidGraveyard},
	state.StrEntry[countZoneCode]{Key: "ValidLibrary", Val: countZoneValidLibrary},
	state.StrEntry[countZoneCode]{Key: "ValidExile", Val: countZoneValidExile},
	state.StrEntry[countZoneCode]{Key: "ValidStack", Val: countZoneValidStack},
	state.StrEntry[countZoneCode]{Key: "ValidCommand", Val: countZoneValidCommand},
)

type zoneFoldPropCode uint16

const (
	zoneFoldPropCardPower zoneFoldPropCode = iota + 1
	zoneFoldPropCardToughness
	zoneFoldPropCardManaCost
	zoneFoldPropCardTypes
	zoneFoldPropCreatureType
	zoneFoldPropPlaneswalkerType
	zoneFoldPropColors
)

var zoneFoldPropCodes = state.NewStrCodes(
	state.StrEntry[zoneFoldPropCode]{Key: "CardPower", Val: zoneFoldPropCardPower},
	state.StrEntry[zoneFoldPropCode]{Key: "CardToughness", Val: zoneFoldPropCardToughness},
	state.StrEntry[zoneFoldPropCode]{Key: "CardManaCost", Val: zoneFoldPropCardManaCost},
	state.StrEntry[zoneFoldPropCode]{Key: "CardTypes", Val: zoneFoldPropCardTypes},
	state.StrEntry[zoneFoldPropCode]{Key: "CardTypesPermanent", Val: zoneFoldPropCardTypes},
	state.StrEntry[zoneFoldPropCode]{Key: "CreatureType", Val: zoneFoldPropCreatureType},
	state.StrEntry[zoneFoldPropCode]{Key: "PlaneswalkerType", Val: zoneFoldPropPlaneswalkerType},
	state.StrEntry[zoneFoldPropCode]{Key: "Colors", Val: zoneFoldPropColors},
)

type extremePropertyValueCode uint16

const (
	extremePropertyValueExtremeCardPower extremePropertyValueCode = iota + 1
	extremePropertyValueGreatestCardToughness
	extremePropertyValueGreatestCardManaCost
)

var extremePropertyValueCodes = state.NewStrCodes(
	state.StrEntry[extremePropertyValueCode]{Key: "GreatestCardPower", Val: extremePropertyValueExtremeCardPower},
	state.StrEntry[extremePropertyValueCode]{Key: "LeastCardPower", Val: extremePropertyValueExtremeCardPower},
	state.StrEntry[extremePropertyValueCode]{Key: "GreatestCardToughness", Val: extremePropertyValueGreatestCardToughness},
	state.StrEntry[extremePropertyValueCode]{Key: "GreatestCardManaCost", Val: extremePropertyValueGreatestCardManaCost},
)
