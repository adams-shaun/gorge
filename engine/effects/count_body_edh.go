package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// EmergedSacToughnessSVar is the named runtime SVar the emerge cast's
// payment stores on the spell (engine/rules' payCast, gated on the face
// carrying the Count$Emerged reader): the toughness of the creature
// sacrificed to cast it for its Emerge cost (CR 702.118a). The count
// readers (Count$Emerged's branch and the Emerged$ prefix body) read it,
// and events.Move's leave-the-battlefield reset clears the runtime store
// with the cast-time window (CR 400.7), so a re-entering permanent reads
// "not emerged" and a copy -- put on the stack, never cast -- never
// carries one.
const EmergedSacToughnessSVar = "EmergedSacToughness"

// emergedSacToughness reads one source's stored emerge capture: (value,
// present). Absent means the spell was not cast for its Emerge cost, or
// the sacrifice part of it was not paid (an over-charged emerge).
func emergedSacToughness(g *state.Game, src state.ObjID) (int32, bool) {
	o := g.Obj(src)
	if o == nil || o.RuntimeSVars == nil {
		return 0, false
	}
	v, ok := o.RuntimeSVars[EmergedSacToughnessSVar]
	return v, ok
}

// evalEmergedOK answers an Emerged$<Property> body: the named property of
// the creature sacrificed to cast the resolving spell for its Emerge cost.
// The one property the corpus prints is CardToughness (Adipose Offspring's
// `T:Emerged$CardToughness`, the <yes> operand of Count$Emerged.T.1);
// it reads the stored capture, never a live object -- the sacrifice
// settled before the count runs (the Sacrificed$ LKI rule, CR 608.2g).
// Any other property is not a shape this build models: (0, false). A
// source never emerge-cast reads a legitimate zero.
func evalEmergedOK(h Host, c *Ctx, prop string) (int32, bool) {
	if objectPropertyCodes.Code(prop) != objectPropertyCardToughness {
		return 0, false
	}
	g := h.Game()
	if v, ok := emergedSacToughness(g, c.Source); ok {
		return v, true
	}
	return 0, true
}

// wubrgColorBits is the WUBRG colour order the shared-colour arithmetic
// walks in fixed position, so a sum never depends on iteration order.
var wubrgColorBits = []ColorMask{
	1 << 0, // W
	1 << 1, // U
	1 << 2, // B
	1 << 3, // R
	1 << 4, // G
}

// chosenColoursMask is the resolving source's chosen colours as a colour
// mask: the union of the single-letter ChosenColor answer (the one-colour
// ChooseColor family -- Utopia Sprawl, Call to Arms) and the appended
// letters of the two-colour ChosenColors answer (Seal of the Guildpact).
// Forge keeps both answers on one card list, so every chosen-colour reader
// takes this union.
func chosenColoursMask(g *state.Game, src state.ObjID) ColorMask {
	o := g.Obj(src)
	if o == nil {
		return 0
	}
	var m ColorMask
	m |= colorBit(colourLetter(o.ChosenColor))
	for i := 0; i < len(o.ChosenColors); i++ {
		m |= colorBit(colourLetter(o.ChosenColors[i : i+1]))
	}
	return m
}

// definedCardsForChosenColours resolves the defined-card ref a
// Count$HasNumChosenColors body names: OriginalHost -- the object a cost
// static is pricing (Ctx.AffectedObj; the source itself when unbound, the
// TypesSharedWith convention) -- and TriggeredCard, the triggering cast's
// object (Tablet of the Guilds' trigger). An unreadable ref fails the
// head's verdict.
func definedCardsForChosenColours(c *Ctx, ref string) ([]state.ObjID, bool) {
	switch definedSpecCodes.Code(strings.TrimSpace(ref)) {
	case definedSpecOriginalHost:
		id := c.AffectedObj
		if id == 0 {
			id = c.Source
		}
		if id == 0 {
			return nil, false
		}
		return []state.ObjID{id}, true
	case definedSpecTriggeredCard:
		if c.TriggerCard == 0 {
			return nil, false
		}
		return []state.ObjID{c.TriggerCard}, true
	}
	return nil, false
}

// searchedLibraryPlayers resolves the player ref a Count$SearchedLibrary
// body names. The bare spelling is Forge's getDefinedPlayers default (the
// resolving controller); "You" the controller, "Opponent" every surviving
// opponent (Archive Trap's spelling); anything else goes through the
// shared defined-targets grammar so the count can never name a player the
// body's own Defined$ would not. An unresolvable ref fails the head's
// verdict.
func searchedLibraryPlayers(h Host, c *Ctx, ref string) ([]state.PlayerID, bool) {
	if len(strings.TrimSpace(ref)) == 0 {
		return []state.PlayerID{c.Controller}, true
	}
	ts, ok := knownDefinedTargets(h, c, ref)
	if !ok {
		return nil, false
	}
	return playerIDsFromTargets(h, c, ref, ts), true
}

// secondMostProminentColour is Forge's SortColorsFromList: a five-slot
// WUBRG tally over the given objects (a multi-coloured object counts once
// per colour it has), sorted ascending, read at the second-largest slot.
// Every object's colours are read through ColorMaskOf, the same printed
// read the colour predicates take.
func secondMostProminentColour(g *state.Game, ids []state.ObjID) int32 {
	var tally [5]int32
	for _, id := range ids {
		o := g.Obj(id)
		if o == nil {
			continue
		}
		mask := ColorMaskOf(o)
		for i, bit := range wubrgColorBits {
			if mask&bit != 0 {
				tally[i]++
			}
		}
	}
	// A fixed 5-slot insertion sort over the tally: deterministic, no map
	// iteration, and small enough that the branch count is trivial.
	for i := 1; i < len(tally); i++ {
		for j := i; j > 0 && tally[j] < tally[j-1]; j-- {
			tally[j], tally[j-1] = tally[j-1], tally[j]
		}
	}
	return tally[len(tally)-2]
}

// triggeredLKIInZoneWords is the zone vocabulary the inZone<Zone>
// qualifier spells, in a fixed order so the admit is deterministic.
var triggeredLKIInZoneWords = []struct {
	name string
	zone state.Zone
}{
	{"Battlefield", state.ZBattlefield},
	{"Graveyard", state.ZGraveyard},
	{"Hand", state.ZHand},
	{"Library", state.ZLibrary},
	{"Exile", state.ZExile},
	{"Stack", state.ZStack},
	{"Command", state.ZCommand},
}

// admitTriggeredLKIInZone admits or strips one `inZone<Zone>` qualifier of
// a TriggeredCardLKI$Valid spec against the triggering zone change's ORIGIN
// zone (Ctx.TriggerCardFrom), returning the reduced spec: the positive
// spelling requires the origin to match, the negated one requires it to
// miss, per comma alternative (the admitProvenanceAlternativesFilter
// rejoin rule). ok is false when no alternative survives -- the spec
// matches nothing. A spec without the qualifier is returned unchanged, so
// every unrelated body is byte-identical. This is the one place an inZone
// qualifier is judged off the live zone: CR 603.10 makes an entrant's own
// reads live (followTriggerObject drops the LKI snapshot), so Fblthp, the
// Lost's "If it entered from your library" can only be answered from the
// origin the triggering event recorded.
func admitTriggeredLKIInZone(spec string, origin state.Zone) (string, bool) {
	for _, zw := range triggeredLKIInZoneWords {
		tok := "inZone" + zw.name
		if !strings.Contains(spec, tok) {
			continue
		}
		holds := origin == zw.zone
		var b strings.Builder
		first, alive := true, false
		for alt := range FilterAlternatives(spec) {
			s1, hadPos := StripPredicateToken(alt, tok)
			s2, hadNeg := StripPredicateToken(s1, "!"+tok)
			if (hadPos && !holds) || (hadNeg && holds) {
				continue
			}
			if !first {
				b.WriteByte(',')
			}
			b.WriteString(s2)
			first = false
			alive = true
		}
		if !alive {
			return "", false
		}
		return b.String(), true
	}
	return spec, true
}

// chosenPlayerOf is the source's recorded ChoosePlayer answer (Call to
// Arms' "choose an opponent"): the player target on the source's Chosen
// list, the fold events.Choose's player arm writes. Zero when none.
func chosenPlayerOf(g *state.Game, src state.ObjID) state.PlayerID {
	o := g.Obj(src)
	if o == nil {
		return 0
	}
	for _, t := range o.Chosen {
		if t.IsPlayer {
			return t.Player
		}
	}
	return 0
}

// etbChromaCount is the bare Count$Chroma head's read (Springjack
// Shepherd's `X:Count$Chroma.White`): the number of <arg> mana
// symbols among the mana costs of the permanents the resolving controller
// controls (Forge's Chroma block default zone, Battlefield). The colour
// token parses through the shared chromaSourceMask grammar; an unreadable
// colour fails the head's verdict.
func etbChromaCount(h Host, g *state.Game, c *Ctx, colour string) (int32, bool) {
	mask, ok := chromaSourceMask("." + strings.TrimSpace(colour))
	if !ok {
		return 0, false
	}
	var total int32
	for _, p := range g.AliveFrom(0) {
		if p != c.Controller {
			continue
		}
		for _, id := range g.Zone(state.ZBattlefield, p) {
			// A phased-out permanent is treated as though it does not
			// exist (CR 702.26b), as devotionCount skips it.
			if o := g.Obj(id); o != nil && !o.PhasedOut {
				total += chromaObjectCount(o, mask)
			}
		}
	}
	return total, true
}

// exactManaCostZone maps the zone suffix an ExactManaCost<Zone> head
// carries to the state zone (Forge's ZoneType.listValueOf on the head's
// substring after "ExactManaCost"). The count-zone head table is
// deliberately NOT reused: its keys are the Valid* count heads, and adding
// bare zone names there would newly claim whole `Count$Graveyard` bodies
// for the zone-count dispatch. An unreadable suffix fails the head's
// verdict.
func exactManaCostZone(name string) (state.Zone, bool) {
	return parseZone(name)
}

// distinctManaCosts is Forge's ExactManaCost read (Embodiment of Agonies'
// `Count$ExactManaCostGraveyard Card.nonLand+YouOwn`): walk one zone
// across EVERY player, keep the objects the spec admits, and count the
// DISTINCT printed mana costs among them (Forge's
// getManaCost().getShortString() set, the empty cost -- a "no cost" face,
// a face-down card -- removed with ManaCost.NO_COST). The spec matches
// through MatchesSpecFrom, the same resolver the zone heads' fold uses.
func distinctManaCosts(h Host, c *Ctx, g *state.Game, zone state.Zone, spec string) int32 {
	seen := map[string]struct{}{}
	src := g.Obj(c.Source)
	var srcID state.ObjID
	if src != nil {
		srcID = src.ID
	}
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(zone, p) {
			o := g.Obj(id)
			if o == nil || o.FaceDownHidden() {
				continue
			}
			if spec != "" && !MatchesSpecFrom(g, spec, id, p, srcID) {
				continue
			}
			cost := ""
			if f := o.Face(); f != nil {
				cost = f.ManaCost
			}
			if cards.IsNoManaCost(cost) {
				// Forge removes ManaCost.NO_COST from the set: a card with
				// no printed cost is not a distinct mana cost.
				continue
			}
			seen[cost] = struct{}{}
		}
	}
	return int32(len(seen))
}

// evalYesNoBranch resolves a "<yes>.<no>" operand tail: the <yes> operand
// when holds, else the <no> operand, both through the ordinary operand
// grammar (an unresolvable operand reads zero).
func evalYesNoBranch(h Host, c *Ctx, holds bool, tail string, depth int) (int32, bool, bool) {
	yesTok, noTok, _ := strings.Cut(tail, ".")
	tok := noTok
	if holds {
		tok = yesTok
	}
	n, ok := resolveCountOperand(h, c, tok, depth)
	if !ok {
		n = 0
	}
	return n, true, true
}

// evalWasCastFromYourLibrary answers Count$wasCastFromYourLibrary.<yes>.<no>
// (Fblthp, the Lost's "If it was cast from your library, draw two cards
// instead"): Forge's wasCastFrom<Zone> branch. The provenance is the
// object's latest PutOnStack (LatestCastFrom/LatestCastBy, folded by
// events.Apply and preserved onto the permanent), the same discipline the
// wasCastFrom* sibling branches take: a copy was never cast, a card never
// put on the stack reads false. LatestCastBy is the caster, which is the
// library's owner for every cast the corpus makes from a library.
func evalWasCastFromYourLibrary(h Host, c *Ctx, g *state.Game, tail string, depth int) (int32, bool, bool) {
	holds := false
	if o := g.Obj(c.Source); o != nil && !o.IsCopy {
		holds = o.HasLatestCast && o.LatestCastFrom == state.ZLibrary &&
			o.LatestCastBy == c.Controller
	}
	return evalYesNoBranch(h, c, holds, tail, depth)
}

// evalEmergedBranch answers Count$Emerged.<yes>.<no> (Adipose Offspring):
// <yes> when the spell was cast for its Emerge cost (CR 702.118a), read from
// the capture the emerge payment stored on the spell, present exactly when
// the sacrifice part of the cost was paid; an over-charged emerge and a
// never-cast copy both read <no>.
func evalEmergedBranch(h Host, c *Ctx, g *state.Game, tail string, depth int) (int32, bool, bool) {
	_, emerged := emergedSacToughness(g, c.Source)
	return evalYesNoBranch(h, c, emerged, tail, depth)
}

// evalHasNumChosenColors answers Count$HasNumChosenColors.<ref> (Seal of the
// Guildpact's cost reduction, Tablet of the Guilds' cast trigger): for each
// defined card, the number of the resolving source's chosen colours (the
// ChosenColor/ChosenColors union) that card shares, summed. An unresolvable
// ref fails the head's verdict.
func evalHasNumChosenColors(c *Ctx, g *state.Game, ref string) (int32, bool, bool) {
	def, okd := definedCardsForChosenColours(c, ref)
	if !okd {
		return 0, false, true
	}
	chosen := chosenColoursMask(g, c.Source)
	if chosen == 0 {
		return 0, true, true
	}
	var total int32
	for _, id := range def {
		o := g.Obj(id)
		if o == nil {
			continue
		}
		mask := ColorMaskOf(o)
		for _, bit := range wubrgColorBits {
			if chosen&bit != 0 && mask&bit != 0 {
				total++
			}
		}
	}
	return total, true, true
}

// evalSearchedLibrary answers Count$SearchedLibrary.<Player> (Archive Trap's
// alternative-cost gate): the number of completed searches of a player's OWN
// library this turn, folded from the SearchedLibrary marker onto
// Player.SearchedLibraryThisTurn. The bare spelling reads the resolving
// controller.
func evalSearchedLibrary(h Host, c *Ctx, g *state.Game, ref string) (int32, bool, bool) {
	players, okp := searchedLibraryPlayers(h, c, ref)
	if !okp {
		return 0, false, true
	}
	var total int32
	for _, p := range players {
		total += g.Players[p].SearchedLibraryThisTurn
	}
	return total, true, true
}

// evalCountBodyDottedEDH claims the dotted EDH heads: Count$Chroma.<Colour>
// (Springjack Shepherd, see etbChromaCount), wasCastFromYourLibrary
// (Fblthp), Emerged (Adipose Offspring), HasNumChosenColors (Seal of the
// Guildpact, Tablet of the Guilds) and SearchedLibrary (Archive Trap). The
// third result is false for every other head, which falls through to
// evalCountBodyDotted.
func evalCountBodyDottedEDH(h Host, c *Ctx, g *state.Game, head string, depth int) (int32, bool, bool) {
	if tail, ok := strings.CutPrefix(head, "Chroma."); ok {
		n, okc := etbChromaCount(h, g, c, tail)
		return n, okc, true
	}
	if tail, ok := strings.CutPrefix(head, "wasCastFromYourLibrary."); ok {
		return evalWasCastFromYourLibrary(h, c, g, tail, depth)
	}
	dot := strings.IndexByte(head, '.')
	if dot <= 0 {
		return 0, false, false
	}
	tail := head[dot+1:]
	switch evalCountBodyCostCodes.Code(head[:dot]) {
	case evalCountBodyCostEmerged:
		return evalEmergedBranch(h, c, g, tail, depth)
	case evalCountBodyCostHasNumChosenColors:
		return evalHasNumChosenColors(c, g, tail)
	case evalCountBodyCostSearchedLibrary:
		return evalSearchedLibrary(h, c, g, tail)
	}
	return 0, false, false
}

// refValidSpec is the spec evalRefProperty matches `<ref>$Valid <spec>` against for one
// referenced object. Fblthp, the Lost's `TriggeredCardLKI$Valid
// Card.inZoneLibrary`: an entrant's own reads are LIVE after the entry (CR
// 603.10 -- followTriggerObject drops the LKI snapshot), so an inZone<Zone>
// qualifier on the triggering zone change's ref is judged against the ORIGIN
// the triggering event recorded (Ctx.TriggerCardFrom), not the live zone. The
// qualifier is admitted or stripped from the spec before the ordinary match;
// a spec with no surviving alternative matches nothing.
func refValidSpec(c *Ctx, ref, spec string, obj state.ObjID) (string, bool) {
	if refTargetsCodes.Code(ref) == refTargetsTriggeredCardLKI && obj == c.TriggerCard && c.TriggerCardFromSet {
		return admitTriggeredLKIInZone(spec, c.TriggerCardFrom)
	}
	return spec, true
}

// evalCountBodyEDH claims the EDH-ticket count heads, dotted and plain,
// before evalCountBodyDotted and evalCountBodySimple see the body (their
// functions are at their size ceilings): see evalCountBodyDottedEDH for the
// dotted ones; the plain ones are FinishedUpkeepsThisTurn, NumInAllHands,
// ExactManaCost<Zone>, ImprintedCardManaCost, SecondMostProminentColor and
// the bare SearchedLibrary. The third result is false for every other head.
func evalCountBodyEDH(h Host, c *Ctx, g *state.Game, head, arg string, depth int) (int32, bool, bool) {
	if v, ok, matched := evalCountBodyDottedEDH(h, c, g, head, depth); matched {
		return v, ok, true
	}
	switch evalCountBodyCostCodes.Code(head) {
	case evalCountBodyCostFinishedUpkeepsThisTurn:
		// Forge's Count$FinishedUpkeepsThisTurn (Reset's cast gate "cast
		// this spell only during an opponent's turn after their upkeep
		// step"): getNumUpkeep() minus one while the walk is currently IN an
		// upkeep step. state.Game's UpkeepsThisTurn is the event-folded count
		// of upkeep-step entries this turn (the EndStepsThisTurn shape), so
		// the read is the same in-step adjustment
		// FinishedEndOfTurnsThisTurn's arm above takes. A turn that has not
		// reached an upkeep step reads zero.
		if arg != "" {
			return 0, false, true
		}
		n := g.UpkeepsThisTurn
		if g.Step == state.StepUpkeep {
			n--
		}
		if n < 0 {
			n = 0
		}
		return n, true, true
	case evalCountBodyCostNumInAllHands:
		// Forge's Count$NumInAllHands (Werewolf of Ancient Hunger's CDA
		// "power and toughness are each equal to the total number of cards
		// in all players' hands"): the total size of every living player's
		// hand. A bare head; an argument is a shape this build does not
		// model.
		if arg != "" {
			return 0, false, true
		}
		var total int32
		for _, p := range g.AliveFrom(0) {
			total += int32(len(g.Zone(state.ZHand, p)))
		}
		return total, true, true
	case evalCountBodyCostExactManaCostGraveyard:
		// Forge's Count$ExactManaCost<Zone> family (Embodiment of Agonies'
		// the head `ExactManaCostGraveyard` over `Card.nonLand+YouOwn`: "enters
		// with a +1/+1 counter on it for each different mana cost among
		// nonland cards in your graveyard"): the number of DISTINCT printed
		// mana costs among the cards the spec admits, in the named zone
		// across every player (Forge's game.getCardsIn(ZoneType...)), the
		// costless card removed from the set (Forge's ManaCost.NO_COST).
		// The head suffix after ExactManaCost names the zone head; an
		// unreadable zone fails the head's verdict.
		zoneName := strings.TrimPrefix(head, "ExactManaCost")
		zone, okz := exactManaCostZone(zoneName)
		if !okz {
			return 0, false, true
		}
		if strings.TrimSpace(arg) == "" {
			return 0, false, true
		}
		return distinctManaCosts(h, c, g, zone, arg), true, true
	case evalCountBodyCostImprintedCardManaCost:
		// Forge's Count$ImprintedCardManaCost (Psychic Battle's
		// `Count$ImprintedCardManaCost` behind its
		// highest-mana-value compare): the mana value of the FIRST card the
		// resolving source imprinted (state.Object.Imprinted, the fold
		// events.Imprint writes). Psychic Battle imprints every card tied
		// for the highest, so the first is the shared maximum -- never a
		// sum. A bare head; no imprint (or no source) reads zero.
		if arg != "" {
			return 0, false, true
		}
		if o := g.Obj(c.Source); o != nil && len(o.Imprinted) > 0 {
			if io := g.Obj(o.Imprinted[0]); io != nil {
				return int32(objectManaValue(g, io)), true, true
			}
		}
		return 0, true, true
	case evalCountBodyCostSecondMostProminentColor:
		// Forge's Count$SecondMostProminentColor (Call to Arms'
		// `Count$SecondMostProminentColor Permanent.!token+ChosenCtrl`):
		// the second-most common colour among the battlefield permanents the
		// spec admits (Forge's SortColorsFromList: a five-slot WUBRG tally,
		// sorted ascending, read at the second-largest slot -- a
		// multi-coloured permanent counts once per colour it has). The
		// ChosenCtrl qualifier scopes the count to the permanents the
		// source's recorded ChoosePlayer answer controls (Call to Arms'
		// "choose an opponent") and is stripped before the ordinary spec
		// match; a source that never chose a player counts an EMPTY list
		// (Forge's ChosenCtrl matches nothing), never the whole battlefield.
		// The remaining spec matches through MatchesSpecFrom, the zone
		// heads' resolver.
		if strings.TrimSpace(arg) == "" {
			return 0, false, true
		}
		spec := arg
		controller := c.Controller
		if i := strings.Index(spec, "+ChosenCtrl"); i >= 0 {
			chosen := chosenPlayerOf(g, c.Source)
			if chosen == 0 {
				// Forge's ChosenCtrl property matches nothing for a source
				// that never chose a player, so the list is empty and the
				// prominence read is a legitimate zero -- not a whole-
				// battlefield count and not an unresolvable head.
				return 0, true, true
			}
			controller = chosen
			spec = spec[:i] + spec[i+len("+ChosenCtrl"):]
			spec = strings.Trim(spec, "+")
		}
		if spec == "" {
			return 0, false, true
		}
		var ids []state.ObjID
		srcID := c.Source
		for _, id := range g.Zone(state.ZBattlefield, controller) {
			if MatchesSpecFrom(g, spec, id, controller, srcID) {
				ids = append(ids, id)
			}
		}
		return secondMostProminentColour(g, ids), true, true
	case evalCountBodyCostSearchedLibrary:
		// The bare Count$SearchedLibrary spelling: Forge's own-library
		// search tally for the resolving controller (the dotted
		// <Player> spelling is evalCountBodyDotted's arm). A bare head; an
		// argument belongs to that sibling.
		if arg != "" {
			return 0, false, true
		}
		if c.Controller < 0 || int(c.Controller) >= len(g.Players) {
			return 0, true, true
		}
		return g.Players[c.Controller].SearchedLibraryThisTurn, true, true
	}
	return 0, false, false
}
