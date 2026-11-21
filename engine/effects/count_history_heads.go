package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// The zone, last-known-information and player-state count heads
// (Count$NumInAllHands, Count$LastStateGraveyard, Count$Presence_<Type>, ...).
// Every one reads state the event fold already records; none consults the wall
// clock. Count$Random draws from the engine's seeded generator (HostRNG.Rand)
// and nothing else, so a replay at one seed reads one number.

// castGraveyardHead is the Count$ head spelling whose carriers read the
// graveyards as they stood when the spell was cast. The WithFallback spelling
// contains the plain one.
const castGraveyardHead = "Count$LastStateGraveyard"

// presenceHead is the Count$Presence_<Type>.<yes>.<no> spelling: whether the
// spell's caster revealed a <Type> card as an optional cost or controlled a
// <Type> permanent as the spell was cast. Its carriers need the as-cast
// battlefield, so reading it freezes one.
const presenceHead = "Count$Presence_"

// SVarsReadCastGraveyard reports whether any SVar body counts the as-cast
// graveyards (Count$LastStateGraveyard or ...WithFallback), i.e. whether
// casting the spell owning these SVars must freeze them beside the battlefield.
// The map is only scanned for membership, so its iteration order reaches
// nothing.
func SVarsReadCastGraveyard(svars map[string]string) bool {
	for _, body := range svars {
		if strings.Contains(body, castGraveyardHead) {
			return true
		}
	}
	return false
}

// svarsReadPresence reports whether any SVar body reads Count$Presence_.
func svarsReadPresence(svars map[string]string) bool {
	for _, body := range svars {
		if strings.Contains(body, presenceHead) {
			return true
		}
	}
	return false
}

// evalHistoryPrefixHead answers the heads whose spelling carries its argument
// inside the head token (no space): Count$Random.<min>.<max> and
// Count$SearchedLibrary.<PlayerSpec>. handled is false for any other head.
func evalHistoryPrefixHead(h Host, c *Ctx, g *state.Game, head string) (n int32, ok, handled bool) {
	if rest, found := strings.CutPrefix(head, "Random."); found {
		lo, hi, okRange := parseRandomRange(rest)
		if !okRange {
			return 0, false, true
		}
		// Inclusive on both ends; the draw is the engine's seeded stream.
		return lo + int32(h.Rand(int(hi-lo)+1)), true, true
	}
	if spec, found := strings.CutPrefix(head, "SearchedLibrary."); found {
		// Archive Trap's "an opponent searched their library this turn": how
		// many players matching the spec (relative to the resolving
		// controller) completed a library search this turn.
		var count int32
		for _, p := range g.AliveFrom(0) {
			if g.Players[p].SearchedLibraryThisTurn && MatchesPlayerSpec(g, strings.TrimSpace(spec), p, c.Controller) {
				count++
			}
		}
		return count, true, true
	}
	return 0, false, false
}

// parseRandomRange reads "<min>.<max>" (min <= max, both small non-negative
// literals, as every corpus carrier is).
func parseRandomRange(s string) (lo, hi int32, ok bool) {
	a, b, found := strings.Cut(s, ".")
	if !found {
		return 0, 0, false
	}
	l, errL := strconv.Atoi(strings.TrimSpace(a))
	u, errU := strconv.Atoi(strings.TrimSpace(b))
	if errL != nil || errU != nil || l < 0 || u < l {
		return 0, 0, false
	}
	return int32(l), int32(u), true
}

// chosenColorsShared is Count$HasNumChosenColors.<Ref>: how many of the
// source's chosen colours the target card is. The target is the card the
// trigger fired on for TriggeredCard (Tablet of the Guilds) and the object the
// ability prices or applies to for OriginalHost -- the spell a cost reduction
// is read against (Seal of the Guildpact) -- falling back to the source itself.
func chosenColorsShared(h Host, c *Ctx, g *state.Game, target state.ObjID) int32 {
	src := g.Obj(c.Source)
	o := g.Obj(target)
	if src == nil || o == nil {
		return 0
	}
	chosen := src.ChosenColor
	letters, known := colorLetters(chosen)
	if !known {
		// Not colour words: a bare WUBRG letter string.
		letters = letters[:0]
		for i := 0; i < len(chosen); i++ {
			letters = append(letters, chosen[i:i+1])
		}
	}
	have := h.ObjectColors(o)
	var n int32
	for _, l := range letters {
		if l != "" && strings.Contains(have, l) {
			n++
		}
	}
	return n
}

// evalHistoryHead answers the space-argument heads of this file.
func evalHistoryHead(h Host, c *Ctx, g *state.Game, code evalCov3HeadCode, arg string, depth int) (int32, bool) {
	switch code {
	case evalCov3HeadNumInAllHands:
		// Multani, Maro-Sorcerer's "total number of cards in all players'
		// hands".
		var n int32
		for p := range g.Players {
			n += int32(len(g.Zone(state.ZHand, state.PlayerID(p))))
		}
		return n, true
	case evalCov3HeadMaxDistinctOnStack:
		// Geometric Weird's "greatest number of spells and abilities from
		// different sources that were on the stack at once this turn": the
		// event-folded per-turn high-water mark.
		return g.MaxDistinctOnStackThisTurn, true
	case evalCov3HeadFinishedUpkeepsThisTurn:
		// Reset's "after their upkeep step": the upkeeps BEGUN this turn
		// (the event-folded g.UpkeepsThisTurn), less the one the walk is
		// currently in.
		n := g.UpkeepsThisTurn
		if g.Step == state.StepUpkeep {
			n--
		}
		return max(n, 0), true
	case evalCov3HeadExactManaCostGraveyard:
		return distinctGraveyardManaCosts(h, g, c, arg), true
	case evalCov3HeadImprintedCardManaCost:
		// Psychic Battle's imprinted top card: the mana value of the source's
		// imprinted cards, summed (a lone imprint is its own mana value).
		var n int32
		if o := g.Obj(c.Source); o != nil {
			for _, id := range o.Imprinted {
				n += objectProperty(g, id, "CardManaCost")
			}
		}
		return n, true
	case evalCov3HeadAbilityYouCtrl:
		return distinctKeywordsAmongCreatures(h, c, g, arg), true
	case evalCov3HeadTotalValue:
		return totalKeywordValue(h, c, arg), true
	case evalCov3HeadSecondMostProminentColor:
		return secondMostProminentColorCount(h, c, g, arg), true
	case evalCov3HeadHasNumChosenColorsTriggeredCard:
		return chosenColorsShared(h, c, g, c.TriggerCard), true
	case evalCov3HeadHasNumChosenColorsOriginalHost:
		target := c.AffectedObj
		if target == 0 {
			target = c.Source
		}
		return chosenColorsShared(h, c, g, target), true
	case evalCov3HeadDevoured:
		return devouredCount(g, c, arg), true
	case evalCov3HeadLastStateGraveyard, evalCov3HeadLastStateGraveyardWithFallback:
		return evalLastStateGraveyard(h, c, g, arg, depth)
	}
	return 0, false
}

// distinctGraveyardManaCosts is Count$ExactManaCostGraveyard <spec> (Embodiment
// of Agonies): the number of different printed mana costs among the matching
// graveyard cards (CR 202.1: {2}{B} and {1}{B}{B} differ). A card with no mana
// cost has none to contribute.
func distinctGraveyardManaCosts(h Host, g *state.Game, c *Ctx, spec string) int32 {
	if strings.TrimSpace(spec) == "" {
		spec = "Card"
	}
	sc := c.SpecContext(c.Controller)
	bindSpecLayerTables(h, &sc.Layers, spec, false)
	seen := map[string]bool{}
	for p := range g.Players {
		for _, id := range g.Zone(state.ZGraveyard, state.PlayerID(p)) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			cost := strings.Join(strings.Fields(o.Face().ManaCost), " ")
			if cost == "" || cost == "no cost" {
				continue
			}
			if matchesZoneSpecCtx(g, spec, id, sc, state.ZGraveyard) {
				seen[cost] = true
			}
		}
	}
	return int32(len(seen))
}

// distinctKeywordsAmongCreatures is Count$AbilityYouCtrl <list> (Odric,
// Blood-Cursed): how many of the comma-separated keyword abilities appear on
// at least one creature the resolving controller controls, each counted once.
func distinctKeywordsAmongCreatures(h Host, c *Ctx, g *state.Game, list string) int32 {
	if c.Controller < 0 || int(c.Controller) >= len(g.Players) {
		return 0
	}
	creatures := g.Zone(state.ZBattlefield, c.Controller)
	var n int32
	for kw := range strings.SplitSeq(list, ",") {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		for _, id := range creatures {
			if h.IsCreature(id) && h.HasKeyword(id, kw) {
				n++
				break
			}
		}
	}
	return n
}

// totalKeywordValue is Count$TotalValue <Keyword> (Takeno, Samurai General's
// "for each point of bushido it has"): the sum of the numeric parameters of the
// named keyword on the source, over its derived keyword list so a granted
// instance counts and each instance adds its own number.
func totalKeywordValue(h Host, c *Ctx, kw string) int32 {
	kw = strings.TrimSpace(kw)
	if kw == "" || c.Source == 0 {
		return 0
	}
	ch := h.Chars(c.Source)
	if ch == nil {
		return 0
	}
	var n int32
	for _, k := range ch.Keywords {
		head, param, _ := strings.Cut(k, ":")
		if !strings.EqualFold(strings.TrimSpace(head), kw) {
			continue
		}
		if v, err := strconv.Atoi(strings.TrimSpace(param)); err == nil {
			n += int32(v)
		}
	}
	return n
}

// secondMostProminentColorCount is Count$SecondMostProminentColor <spec> (Call
// to Arms): over the battlefield objects matching the spec, the second-highest
// per-colour tally of the five colours, so a colour tied for most reads as
// the most (the "isn't tied for most common" half of the card).
func secondMostProminentColorCount(h Host, c *Ctx, g *state.Game, spec string) int32 {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return 0
	}
	sc := c.SpecContext(c.Controller)
	bindSpecLayerTables(h, &sc.Layers, spec, false)
	var tally [5]int32
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			o := g.Obj(id)
			if o == nil || !matchesZoneSpecCtx(g, spec, id, sc, state.ZBattlefield) {
				continue
			}
			have := h.ObjectColors(o)
			for i := 0; i < len("WUBRG"); i++ {
				if strings.IndexByte(have, "WUBRG"[i]) >= 0 {
					tally[i]++
				}
			}
		}
	}
	var first, second int32
	for _, v := range tally {
		switch {
		case v > first:
			first, second = v, first
		case v > second:
			second = v
		}
	}
	return second
}

// evalLastStateGraveyard is Count$LastStateGraveyard[WithFallback] <spec>: the
// ValidGraveyard scan over the graveyards as the source spell was cast, from
// the frozen snapshot the cast flow takes (rules' castBattlefieldEvent adds
// the graveyard cards to it for a face that reads this head), else -- a
// source with no snapshot, an uncast source -- the live graveyards. A frozen
// empty graveyard is an authoritative zero. The scan is evalCountBodyZone
// itself, so the filter grammar cannot drift from Count$ValidGraveyard. An empty
// filter is not a body this build can count.
func evalLastStateGraveyard(h Host, c *Ctx, g *state.Game, arg string, depth int) (int32, bool) {
	if strings.TrimSpace(arg) == "" {
		return 0, false
	}
	if src := g.Obj(c.Source); src != nil && src.CastBattlefield != nil &&
		(src.Zone == state.ZStack || src.Zone == state.ZBattlefield) {
		snap := src.CastBattlefield
		n, ok, _ := evalCountBodyZone(frozenHost{Host: h, snap: snap}, c, snap.Game, "ValidGraveyard", arg, depth)
		return n, ok
	}
	n, ok, _ := evalCountBodyZone(h, c, g, "ValidGraveyard", arg, depth)
	return n, ok
}

// presenceHolds is Count$Presence_<Type>: the caster revealed a <Type> card as
// the spell's optional additional cost (the paid optional cost, CR 601.2b), or
// controlled a <Type> permanent as the spell was cast (the frozen battlefield;
// the live one for a source with no snapshot).
func presenceHolds(h Host, c *Ctx, g *state.Game, typ string) bool {
	src := g.Obj(c.Source)
	if src == nil || src.IsCopy {
		return false
	}
	if src.OptionalCostPaid || src.CastFlags&state.FlagOptionalCostPaid != 0 {
		return true
	}
	spec := strings.TrimSpace(typ) + ".YouCtrl"
	if src.CastBattlefield != nil && (src.Zone == state.ZStack || src.Zone == state.ZBattlefield) {
		snap := src.CastBattlefield
		n, _, _ := evalCountBodyZone(frozenHost{Host: h, snap: snap}, c, snap.Game, "Valid", spec, 0)
		return n > 0
	}
	n, _, _ := evalCountBodyZone(h, c, g, "Valid", spec, 0)
	return n > 0
}

// historyBranchHolds answers the yes/no branch predicates of this file that
// cov3BranchHolds's own switch does not carry: Count$Night.<y>.<n>,
// Count$MaxSpeed, Count$Escaped and Count$wasCastFromYourLibrary.
func historyBranchHolds(c *Ctx, g *state.Game, pred string) (holds, ok bool) {
	you := c.Controller
	valid := you >= 0 && int(you) < len(g.Players)
	src := g.Obj(c.Source)
	switch cov3BranchHoldsCodes.Code(pred) {
	case cov3BranchHoldsNight:
		// CR 731: it is night.
		return g.DayNight == state.DayNightNight, true
	case cov3BranchHoldsEscaped:
		// CR 702.138: the source was cast with its Escape cost (the flag
		// survives onto the permanent for the ETB counters).
		return src != nil && !src.IsCopy && src.CastFlags&state.FlagEscaped != 0, true
	}
	if playerSpecQualifierCodes.Code(pred) == playerSpecQualifierMaxSpeed {
		// CR 702.179e: the resolving controller has max speed (the player
		// spec's own MaxSpeed word).
		return valid && g.Players[you].Speed >= maxSpeed, true
	}
	if pred == "wasCastFromYourLibrary" {
		// The source's latest cast came from a library (Fblthp, the Lost).
		return src != nil && !src.IsCopy && src.HasLatestCast && src.LatestCastFrom == state.ZLibrary, true
	}
	return false, false
}

// sumPlayerScalarProperty answers the PlayerCount<group>$<Prop> reads that sum
// a per-seat figure (playerScalarProperty's vocabulary: hand size, graveyard
// and library size, cards drawn, life, life gained this turn) over the group --
// Enemy of Enlightenment's opponents' hands, Needlebite Trap's life gained. ok
// is false for any other property.
func sumPlayerScalarProperty(h Host, g *state.Game, players []state.PlayerID, prop string) (int32, bool) {
	prop = strings.TrimSpace(prop)
	if _, known := playerScalarProperty(h, g, 0, prop); !known {
		return 0, false
	}
	var n int32
	for _, p := range players {
		v, _ := playerScalarProperty(h, g, p, prop)
		n += v
	}
	return n, true
}

// devouredCount is Count$Devoured <spec> (Marrow Chomper, Skullmulcher, Tar
// Fiend, Voracious Dragon): the creatures the source devoured as it entered
// (CR 702.82), those matching the spec. The devour replacement sacrifices them
// as it enters and then forgets them, so the record is this turn's
// battlefield-departure ledger (g.Entered): the unbroken run of sacrifices by
// the source's controller that the replacement logs straight after the
// source's own battlefield entry (the entry is folded first, then the
// replacement's sacrifices). A source that did not enter this turn, or entered
// without sacrificing, devoured nothing. Any other zone entry ends the run, so
// only a sacrifice made with no card cast or moved in between (the ETB trigger
// resolves before that in ordinary play) is read as devoured.
func devouredCount(g *state.Game, c *Ctx, spec string) int32 {
	src := g.Obj(c.Source)
	if src == nil {
		return 0
	}
	entry := -1
	for i := len(g.Entered) - 1; i >= 0; i-- {
		if g.Entered[i].Obj == c.Source && g.Entered[i].To == state.ZBattlefield {
			entry = i
			break
		}
	}
	if entry < 0 {
		return 0
	}
	if strings.TrimSpace(spec) == "" {
		spec = "Card"
	}
	sc := c.SpecContext(c.Controller)
	var n int32
	for _, en := range g.Entered[entry+1:] {
		if !en.Sacrificed || en.Controller != src.Controller {
			break
		}
		if matchesEnteredEntry(g, spec, en, sc) {
			n++
		}
	}
	return n
}

// nonActivePlayers is the PlayerCountNonActive$ group: every living player but
// the active one, in seat order.
func nonActivePlayers(g *state.Game) []state.PlayerID {
	var out []state.PlayerID
	for _, p := range g.AliveFrom(0) {
		if p != g.Active {
			out = append(out, p)
		}
	}
	return out
}

// The per-player StoreSVar accumulator (Plague of Vermin's `DB$ StoreSVar |
// SVar$ EachPlayer | Type$ AdditiveForEach`, read back by
// PlayerCountRemembered$SVarAmount). It rides the existing events.StoreSVar
// fold on the resolving source: one runtime SVar per seat under
// svarAmountKey, so no new Kind, Event field or Object field is needed, the
// write is a keyed map insert (no map order reaches an event), and CR 400.7
// clears it with the rest of the source's runtime SVars when it leaves the
// stack.

// svarAmountKey is the runtime-SVar name seat p's AdditiveForEach total is
// stored under. '#' is not a legal SVar-name character, so it cannot collide
// with a printed or Forge-written name.
func svarAmountKey(p state.PlayerID) string {
	return "SVarAmount#" + strconv.Itoa(int(p))
}

// svarAmountOf is seat p's accumulated AdditiveForEach total on the resolving
// source; 0 when nothing was stored for the seat.
func svarAmountOf(h Host, c *Ctx, p state.PlayerID) int32 {
	if c == nil || c.Source == 0 {
		return 0
	}
	o := h.Game().Obj(c.Source)
	if o == nil {
		return 0
	}
	return o.RuntimeSVars[svarAmountKey(p)]
}

// svarAmountSum is the AdditiveForEach totals of the given seats added up.
func svarAmountSum(h Host, c *Ctx, players []state.PlayerID) int32 {
	var n int32
	for _, p := range players {
		n += svarAmountOf(h, c, p)
	}
	return n
}

// rememberedLoopPlayer is the player a RepeatEach over players currently
// binds: the last player entry of Ctx.Remembered (RepeatEach appends the loop
// subject, so it is the newest).
func rememberedLoopPlayer(c *Ctx) (state.PlayerID, bool) {
	for i := len(c.Remembered) - 1; i >= 0; i-- {
		if t := c.Remembered[i]; t.IsPlayer {
			return t.Player, true
		}
	}
	return 0, false
}
