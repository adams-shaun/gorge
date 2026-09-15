package rules

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// sorcerySpeed reports whether p may take a sorcery-speed action right now.
func (e *Engine) sorcerySpeed(p state.PlayerID) bool {
	return e.G.Active == p && e.G.Step.IsMain() && len(e.G.Stack) == 0
}

// mayPlayLandIds returns the ids of lands in player p's zones that an active
// may-play-from-zone grant (a S:Mode$ Continuous static carrying MayPlay$ True
// and an AffectedZone$, e.g. Conduit of Worlds' "You may play lands from your
// graveyard") lets p play this turn, in deterministic order. The result is
// empty unless p is at sorcery speed with a land drop remaining -- the same
// once-per-turn gate the hand walk applies, so a graveyard land and a hand
// land share one land drop per turn. A land already in p's hand never appears
// here (the ordinary hand walk offers it), and neither does a land in a hidden
// zone. Each candidate must match the granting effect's Affects filter (the
// Affected$ spec) through the same spec matcher the layer system uses, so a
// Land.YouOwn grant never offers an opponent's land or a non-land permanent.
//
// Order comes from e.active() (a sorted slice), then each effect's parsed
// AffectedZone order, then the zone slice order -- never a map -- so the
// resulting option list is reproducible run to run. Zones are deduplicated
// per (zone, id) so two grants naming the same zone never offer the same land
// twice. Only zones a land can meaningfully be played from (graveyard, exile)
// are walked, since hand is covered by the normal walk and library is hidden.
func (e *Engine) mayPlayLandIds(p state.PlayerID) []state.ObjID {
	if !e.sorcerySpeed(p) || e.G.Players[p].LandsPlayed >= 1 {
		return nil
	}
	type offered struct {
		zone state.Zone
		id   state.ObjID
	}
	var out []state.ObjID
	var seen []offered
	for _, ce := range e.active() {
		if !ce.MayPlay || ce.Controller != p {
			continue
		}
		zones, all, ok := effects.ParseZones(ce.AffectedZone)
		if !ok && !all {
			continue
		}
		consider := func(z state.Zone) {
			if z != state.ZGraveyard && z != state.ZExile {
				return
			}
			for _, id := range e.G.Zone(z, p) {
				o := e.G.Obj(id)
				if o == nil || o.Face() == nil || !o.Face().IsLand() {
					continue
				}
				if !effects.MatchesSpecFrom(e.G, ce.Affects, id, ce.Controller, ce.Source) {
					continue
				}
				dup := false
				for _, s := range seen {
					if s.zone == z && s.id == id {
						dup = true
						break
					}
				}
				if dup {
					continue
				}
				seen = append(seen, offered{z, id})
				out = append(out, id)
			}
		}
		if all {
			consider(state.ZGraveyard)
			consider(state.ZExile)
		} else {
			for _, z := range zones {
				consider(z)
			}
		}
	}
	return out
}

// abilityZoneOK reports whether ability ab may be activated while the
// source cardinal is in zone z (CR 602.1b): the printed ActivationZone$
// when present, the battlefield by default. Battlefield, Hand and Graveyard
// are enumerated by the legal-action walks; other values (Command, Exile,
// Stack) are not and therefore never offer an option.
func abilityZoneOK(ab *cards.SA, z state.Zone) bool {
	az, ok := ab.Params["ActivationZone"]
	if !ok {
		return z == state.ZBattlefield
	}
	switch strings.TrimSpace(az) {
	case "Battlefield":
		return z == state.ZBattlefield
	case "Graveyard":
		return z == state.ZGraveyard
	case "Hand":
		return z == state.ZHand
	}
	return false
}

// isLoyaltyAbility reports whether ab is a planeswalker loyalty ability
// (CR 606): it carries the Planeswalker$ parameter (case-insensitive -- three
// corpus lines spell it "true"), or its parsed Cost$ contains an
// AddCounter/SubCounter part of the LOYALTY kind. The OR is load-bearing: the
// dynamic [-X] costs (SubCounter<X/LOYALTY>, 20 raw lines) do not parse into a
// SubCounter part (ParseCost keeps the unrecognised-token fallback for them),
// so only the parameter identifies those; conversely the param covers every
// fixed [+N]/[-N] shape, 966 of the 970 raw ability lines carrying it.
func isLoyaltyAbility(ab *cards.SA) bool {
	if v, ok := ab.Params["Planeswalker"]; ok && strings.EqualFold(strings.TrimSpace(v), "True") {
		return true
	}
	c := ParseCost(ab.Params["Cost"])
	for _, part := range c.AddCounter {
		if strings.EqualFold(part.Spec, "LOYALTY") {
			return true
		}
	}
	for _, part := range c.SubCounter {
		if strings.EqualFold(part.Spec, "LOYALTY") {
			return true
		}
	}
	return false
}

// loyaltyActivationsThisTurn counts how many loyalty abilities of the
// permanent id have been activated in its current battlefield stint this turn.
// It folds the whole replayable log forward because both facts an AbilityPush
// needs are historical: the source's active face at that event, and whether a
// MoveZone ACTUALLY crossed the battlefield boundary. Event.From is advisory
// (events.Move deliberately uses the object's recorded zone instead), so it
// must not decide a stint boundary.
//
// Every genesis object starts outside the battlefield and on face zero. The
// fold keeps just those two facts for id. MoveZone's To is authoritative after
// Apply succeeds, so a same-zone re-append leaves onBattlefield unchanged even
// when a malformed From claims otherwise. FlipFace is applied after any push
// on its old face, exactly as events.Apply does. TurnChange resets the count
// but deliberately retains the folded zone and face for the next turn.
func (e *Engine) loyaltyActivationsThisTurn(id state.ObjID) int {
	o := e.G.Obj(id)
	if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
		return 0
	}

	used := 0
	onBattlefield := false
	faceIdx := 0
	for _, ev := range e.L.Events {
		switch ev.Kind {
		case events.TurnChange:
			// CR 606.3's window is a turn, not a player's own turn.
			// Mirror events.Apply's validity gate: an invalid player leaves
			// Game.Turn unchanged, so its rejected event cannot open a new
			// loyalty-activation window in this historical fold either.
			if int(ev.Player) < len(e.G.Players) {
				used = 0
			}

		case events.MoveZone:
			if ev.Obj != id || !ev.To.Valid() {
				continue
			}
			nextOnBattlefield := ev.To == state.ZBattlefield
			if onBattlefield != nextOnBattlefield {
				// CR 400.7: every real departure or entry starts a new
				// permanent stint. This is derived from the folded zone,
				// never the caller-controlled Event.From.
				used = 0
			}
			onBattlefield = nextOnBattlefield

		case events.AbilityPush:
			if ev.Obj != id || !onBattlefield || int(ev.Player) >= len(e.G.Players) ||
				ev.Amount < 0 || faceIdx >= len(o.Card.Faces) {
				continue
			}
			// Amount indexes the active face's ability list. Check the exact
			// face and bounds that events.Apply used at push time.
			f := o.Card.Faces[faceIdx]
			if f != nil && int(ev.Amount) < len(f.Abilities) && isLoyaltyAbility(f.Abilities[int(ev.Amount)]) {
				used++
			}

		case events.FlipFace:
			if ev.Obj == id && ev.Amount >= 0 && int(ev.Amount) < len(o.Card.Faces) {
				faceIdx = int(ev.Amount)
			}
		}
	}
	return used
}

// loyaltyAbilityLimit resolves how many loyalty abilities the permanent id
// may have activated this turn: 1 (CR 606.3) raised by every live
// S:Mode$ NumLoyaltyAct static whose ValidCard$ matches the permanent --
// Oath of Teferi's "twice each turn rather than only once" (Twice$ True,
// ValidCard$ Planeswalker.YouCtrl) and Urza, Lord Protector's self-scoped
// same (ValidCard$ Card.Self). Any matching Twice$ True raises the base limit
// to 2 (max, so two stacked Twice statics do not compound); every matching
// Additional$ N then adds N -- Forge's two parameters are accumulated
// separately so their result cannot depend on battlefield scan order. An
// additional-activation grant (The Chain Veil's "as though none of its loyalty
// abilities had been activated") therefore stacks on top of a twice grant
// rather than being absorbed by it. Statics that match neither parameter leave
// the limit alone. The ValidCard$ match resolves against the static's own source
// and controller (e.specCtx), exactly as castRestricted and abilityRestricted
// resolve theirs.
func (e *Engine) loyaltyAbilityLimit(id state.ObjID) int {
	twice := false
	additional := 0
	for _, sv := range e.activeStatics("NumLoyaltyAct") {
		if !effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], id, e.specCtx(sv.Source, sv.Controller)) {
			continue
		}
		if sv.Params["Twice"] == "True" {
			twice = true
		}
		if raw, ok := sv.Params["Additional"]; ok {
			additional += int(parseAmount(raw, 0))
		}
	}
	base := 1
	if twice {
		base = 2
	}
	return base + additional
}

// activationLimitReached reports whether this object has already activated the
// indexed ability as many times as its ActivationLimit permits this turn.
// AbilityPush records both pieces of identity (Obj and Amount); scanning
// backward to the latest TurnChange keeps the count derived entirely from the
// replayable event log. The limit itself is resolved by resolveActivationLimit:
// a literal integer is used directly, and a computed expression (an SVar name
// or an inline Count$...) is evaluated through the effects count path, so a
// limit such as Withering Wisps' "number of snow Swamps you control" is
// enforced rather than silently ignored. A limit that resolves to zero or to
// fewer activations than have already been used withholds the offer. An
// expression that genuinely cannot be resolved stays unenforced (today's
// behaviour): resolveActivationLimit reports ok=false.
func (e *Engine) activationLimitReached(id state.ObjID, p state.PlayerID, ability int, raw string) bool {
	limit, ok := e.resolveActivationLimit(id, p, raw)
	if !ok || limit < 0 {
		return false
	}
	used := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.AbilityPush && ev.Obj == id && ev.Amount == int32(ability) {
			used++
		}
	}
	return used >= limit
}

// resolveActivationLimit interprets an ActivationLimit$ value. A literal
// integer is used directly. A non-literal value is resolved through the
// Count$/SVar evaluator the rest of the tree uses (effects.EvalCount), bound
// to the source object and its SVar table, so a computed limit is enforced
// rather than silently ignored. ok reports whether the limit was resolvable at
// all: false keeps the pre-fix behaviour of leaving the limit unenforced, which
// is also what a value that is neither a literal nor an SVar reference (such
// as a description-suffixed literal from a keyword template) gets.
func (e *Engine) resolveActivationLimit(id state.ObjID, p state.PlayerID, raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if n, err := strconv.Atoi(raw); err == nil {
		return n, true
	}
	o := e.G.Obj(id)
	if o == nil {
		return 0, false
	}
	f := o.Face()
	if f == nil {
		return 0, false
	}
	ctx := &effects.Ctx{Source: id, Controller: p, SVars: f.SVars}
	if strings.HasPrefix(raw, "Count$") {
		return int(effects.EvalCount(e, ctx, raw)), true
	}
	if body, ok := f.SVars[raw]; ok {
		return int(effects.EvalCount(e, ctx, body)), true
	}
	return 0, false
}

// targetsAvailable reports whether the narrow target requirement that can
// be proved before offering a spell or ability is satisfiable. A missing
// TargetMin$/TargetMax$ pair is Forge's unconditional one-target shape.
// Dynamic bounds and modal or announced choices stay offerable until the
// post-announcement askTarget backstop can evaluate them with those choices
// made. excludeSelf is the CR 115.5 self-targeting object: the offered card
// for a spell cast from a zone that could contain it, 0 for an activated
// ability (whose Source permanent IS a legal target of its own ability).
func (e *Engine) targetsAvailable(p state.PlayerID, id, excludeSelf state.ObjID, sa *cards.SA) bool {
	if sa == nil || strings.TrimSpace(sa.Params["ValidTgts"]) == "" || sa.API == "Charm" ||
		sa.Params["Choices"] != "" || sa.Params["Announce"] != "" {
		return true
	}
	if _, ok := sa.Params["TargetMin"]; ok {
		return true
	}
	if _, ok := sa.Params["TargetMax"]; ok {
		return true
	}
	return len(e.legalTargetCandidates(p, id, excludeSelf, sa)) > 0
}

// castTargetsAvailable is the cast-offer guard: the spell card may not target
// itself (CR 115.5), so excludeSelf is the card id.
func (e *Engine) castTargetsAvailable(p state.PlayerID, id state.ObjID, sa *cards.SA) bool {
	return e.targetsAvailable(p, id, id, sa)
}

// abilityTargetsAvailable is the activated-ability offer guard. It is what
// stops an ability with no legal target from being re-offered in a loop after
// the transaction aborts it (CR 602.2b / 601.2c: such an ability cannot be
// activated at all). Unlike a cast, an activated ability CAN target its own
// Source permanent (Mother of Runes targeting itself), so no self-exclusion
// applies.
func (e *Engine) abilityTargetsAvailable(p state.PlayerID, id state.ObjID, ab *cards.SA) bool {
	return e.targetsAvailable(p, id, 0, ab)
}

// legalActions enumerates everything p may legally do with priority. The
// result is the complete rules surface a client ever sees.
func (e *Engine) legalActions(p state.PlayerID) []decision.Option {
	var out []decision.Option
	add := func(kind, label string, obj state.ObjID) {
		out = append(out, decision.Option{Index: len(out), Kind: kind, Label: label, Obj: obj})
	}
	sorcery := e.sorcerySpeed(p)

	for _, id := range e.G.Zone(state.ZHand, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil {
			continue
		}
		if f.IsLand() {
			if sorcery && e.G.Players[p].LandsPlayed < 1 {
				add("play_land", "Play "+f.Name, id)
			}
			continue
		}
		if e.castRestricted(p, id) {
			continue
		}
		if e.castSuppressed(p, id) {
			continue
		}
		if !e.spellTimingOK(p, id, f, sorcery) {
			continue
		}
		targetsAvailable := e.castTargetsAvailable(p, id, f.SpellAbility())
		// offerCostFor prices the MANA the offer will charge (601.2f
		// modifiers, then the commander tax); withSpellAbilityExtras adds the
		// spell's own ADDITIONAL non-mana parts on top. Both are needed and
		// they compose in this order: an additional cost is never reduced by
		// a cost modifier, the same reason the commander tax lands after the
		// modifiers rather than before them.
		//
		// The gate must see the SAME cost beginCast will charge, additional
		// non-mana parts included, or an unpayable cast gets offered and then
		// aborts having consumed nothing -- which, since the abort leaves the
		// board that produced the offer untouched, is an unbounded livelock
		// rather than a wasted click. That is exactly what Village Rites did
		// to a live 4-player game (see withSpellAbilityExtras in cast.go).
		// Only the plain cast folds the extras, matching beginCast's own
		// condition: the kicked/surged/flashback/miracle offers below set
		// Mode, and beginCast skips the fold for those.
		base := e.offerCostFor(p, id, e.rawBaseCost(p, id), false)
		convokeBase, _ := e.convokeCost(p, id, base)
		// An either-or additional cost (AlternateAdditionalCost) makes the
		// plain cast's gate existential: the cast is offerable when AT LEAST
		// ONE alternative part is payable (the choice itself is asked by the
		// cast flow, altAddAsk), never when all of them are unpayable. Cards
		// without the keyword keep the ordinary single-cost gate.
		altParts := altAddCostParts(f)
		if targetsAvailable {
			if len(altParts) > 0 {
				for _, part := range altParts {
					if e.castable(p, id, withSpellAbilityExtras(f, convokeBase).Plus(ParseCost(part)), false) {
						add("cast", "Cast "+f.Name, id)
						break
					}
				}
			} else if e.castable(p, id, withSpellAbilityExtras(f, convokeBase), false) {
				add("cast", "Cast "+f.Name, id)
			}
		}
		for i, alt := range e.alternativeCosts(p, id) {
			if !targetsAvailable {
				continue
			}
			// Ruling (Task 9 fix round 1, Important 1): this used to gate on
			// mana-only alt.CanPay, but ParseCost now produces Sac/SubCounter/
			// Tap parts that the cast flow enforces -- an AlternativeCost whose
			// Cost$ carries Sac<N/...> was offered without checking that N
			// matching permanents exist, and beginCast then asked a sacrifice
			// decision with zero options that no answer could escape. castable
			// is the same gate every other "cast" option uses.
			if e.castable(p, id, e.offerCostFor(p, id, alt, false), false) {
				// AltCostIndex is i+1, not i: the zero value must mean "the
				// card's own cost" so every other Option literal in the tree
				// (play_land, activate, pass, and the base "cast" option
				// added just above via the shared add closure) needs no
				// change to keep meaning that.
				out = append(out, decision.Option{Index: len(out), Kind: "cast",
					Label: altCostLabel(f.Name, i), Obj: id, AltCostIndex: i + 1})
			}
		}
		if kc, ok := kickerCost(f); ok && targetsAvailable && e.castable(p, id, base.Plus(kc), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (kicked)", Obj: id, Mode: "kicked"})
		}
		if sc, ok := surgeCost(f); ok && targetsAvailable && e.spellsCastThisTurn(p) > 0 && e.castable(p, id, e.offerCostFor(p, id, sc, false), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (surged)", Obj: id, Mode: "surged"})
		}
		// The alternative-cost keyword family (altcosts), from the hand: evoke
		// (CR 702), dash, overload and warp each become their own "cast" mode
		// option paying the printed keyword cost in place of the mana cost.
		// Madness does NOT offer from the hand here (CR 702.35a: the madness
		// cast window opens only on the discard, through the pending-trigger
		// machinery, exactly like Miracle); warp additionally offers from the
		// graveyard and -- after an end-step exile -- from exile, in the walks
		// below.
		for _, ka := range [...]struct{ mode, head string }{
			{"evoked", "Evoke"}, {"dashed", "Dash"}, {"overloaded", "Overload"}, {"warped", "Warp"},
		} {
			alt, ok := keywordAltCost(f, ka.head)
			if !ok || (ka.mode != "overloaded" && !targetsAvailable) ||
				!e.castable(p, id, e.offerCostFor(p, id, alt, false), false) {
				continue
			}
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (" + ka.mode + ")", Obj: id, Mode: ka.mode})
		}
		if bc, ok := buybackCost(f); ok && e.castable(p, id, e.offerCostFor(p, id, base.Plus(bc), false), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast", Label: "Cast " + f.Name + " (buyback)", Obj: id, Mode: "buyback"})
		}
		if sc, ok := suspendCost(f); ok {
			offer := sc.cost
			if sc.timeX {
				// XMin<N> is part of the announcement, not a later payment
				// preference: do not offer a Suspend X action that cannot pay
				// even its smallest legal X.
				offer = offer.WithX(sc.minTime)
			}
			if e.castable(p, id, e.offerCostFor(p, id, offer, false), false) {
				out = append(out, decision.Option{Index: len(out), Kind: "cast", Label: "Suspend " + f.Name, Obj: id, Mode: "suspend"})
			}
		}
	}

	// A may-play-from-zone grant (Conduit of Worlds, Crucible of Worlds, ...)
	// makes lands in a granted zone playable this turn. This is the SECOND
	// play_land source alongside the hand walk above, never a replacement for
	// it; the once-per-turn land-drop gate is the same sorcerySpeed &&
	// LandsPlayed < 1 condition the hand walk applies, so a graveyard land
	// and a hand land share one land drop. The zones walked, the Affects
	// filter each grant applies, and the deterministic order all come from
	// mayPlayLandIds.
	for _, id := range e.mayPlayLandIds(p) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		add("play_land", "Play "+o.Face().Name, id)
	}

	// Command zone (CR 903.8, Commander format): a player may cast a
	// commander they own from the command zone. This is a SECOND cast source
	// alongside the hand walk above, never a replacement for it. A
	// commander's timing is its own card's -- a creature commander is
	// sorcery-speed, an instant/flash one instant-speed -- gated by the same
	// sorcery check every other cast uses. The offer is gated on castable
	// with the SAME taxed cost beginCast will charge (commanderTaxFor over
	// the same board), so an offered command-zone cast and the cost it pays
	// structurally cannot disagree. Only a Commander game offers anything
	// here; the explicit format gate is what keeps these rules out of every
	// other game (and TestCommanderTaxGatedOffOutsideCommanderFormat exercises
	// it with a commander actually present in the command zone of a
	// Constructed game), not the
	// incident that a Constructed command zone is normally empty.
	for _, id := range e.G.Zone(state.ZCommand, p) {
		if e.format != FormatCommander {
			continue
		}
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil {
			continue
		}
		if e.castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		if !e.spellTimingOK(p, id, f, sorcery) {
			continue
		}
		targetsAvailable := e.castTargetsAvailable(p, id, f.SpellAbility())
		cost := e.offerCostFor(p, id, e.rawBaseCost(p, id), false)
		if targetsAvailable && e.castable(p, id, cost, false) {
			add("cast", "Cast "+f.Name, id)
		}
		// Alternative costs replace the printed mana cost but not additional
		// costs such as commander tax (CR 118.9d, 903.8). Dash and the other
		// cast alternatives therefore remain available from the command zone;
		// offerCostFor applies the same tax beginCast later charges.
		for _, ka := range [...]struct{ mode, head string }{
			{"evoked", "Evoke"}, {"dashed", "Dash"}, {"overloaded", "Overload"},
		} {
			alt, ok := keywordAltCost(f, ka.head)
			if !ok || (ka.mode != "overloaded" && !targetsAvailable) ||
				!e.castable(p, id, e.offerCostFor(p, id, alt, false), false) {
				continue
			}
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (" + ka.mode + ")", Obj: id, Mode: ka.mode})
		}
	}

	// Harmonize is a graveyard alternative. It is offered as its own cast
	// transaction, then spellRestZone exiles it after resolution.
	for _, id := range e.G.Zone(state.ZGraveyard, p) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || e.castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		f := o.Face()
		if !e.spellTimingOK(p, id, f, sorcery) {
			continue
		}
		if hc, ok := harmonizeCost(f); ok && e.castTargetsAvailable(p, id, f.SpellAbility()) {
			hc, _ = e.harmonizePayment(p, id, hc)
			if e.castable(p, id, e.offerCostFor(p, id, hc, false), false) {
				out = append(out, decision.Option{Index: len(out), Kind: "cast", Label: "Cast " + f.Name + " (harmonize)", Obj: id, Mode: "harmonize"})
			}
		}
	}

	// Flashback: a graveyard walk, same instant-speed timing as hand cards,
	// gated on the derived keyword (so a continuous-effect grant, e.g.
	// Snapcaster Mage, counts) rather than the printed one.
	for _, id := range e.G.Zone(state.ZGraveyard, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil || !e.HasKeyword(id, "Flashback") {
			continue
		}
		if e.castRestricted(p, id) {
			continue
		}
		if e.castSuppressed(p, id) {
			continue
		}
		if !e.spellTimingOK(p, id, f, sorcery) {
			continue
		}
		if !e.castTargetsAvailable(p, id, f.SpellAbility()) {
			continue
		}
		if fc := e.flashbackCost(id); e.castable(p, id, e.offerCostFor(p, id, fc, false), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (flashback)", Obj: id, Mode: "flashback"})
		}
	}

	// Warp from the graveyard requires a separate MayPlay Spell.Warp static;
	// Warp itself grants only the hand alternative. Timeline Culler is the
	// corpus shape carrying that explicit graveyard permission.
	for _, id := range e.G.Zone(state.ZGraveyard, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil {
			continue
		}
		wc, ok := keywordAltCost(f, "Warp")
		if !ok || !warpGraveyardAllowed(f) || e.castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		instantSpeed := f.IsInstant() || e.HasKeyword(id, "Flash")
		if !instantSpeed && !sorcery {
			continue
		}
		if !e.castTargetsAvailable(p, id, f.SpellAbility()) {
			continue
		}
		if e.castable(p, id, e.offerCostFor(p, id, wc, false), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (warped)", Obj: id, Mode: "warped"})
		}
	}

	// Warp recast from exile (CR 702: "exile this creature at the beginning
	// of the next end step, then you may cast it from exile on a later
	// turn"). The exile-zone walk offers the cast only to a warp card that
	// the log shows was warp-cast and end-step-exiled, on a turn strictly
	// after that exile -- the flag alone cannot say it (CastFlags reset when
	// the permanent left the battlefield), but the log can. This later cast
	// pays the normal mana cost and is not itself flagged warped.
	for _, id := range e.G.Zone(state.ZExile, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil || o.IsToken {
			continue
		}
		_, ok := keywordAltCost(f, "Warp")
		if !ok || !e.warpRecastAvailable(id) || e.castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		instantSpeed := f.IsInstant() || e.HasKeyword(id, "Flash")
		if !instantSpeed && !sorcery {
			continue
		}
		if !e.castTargetsAvailable(p, id, f.SpellAbility()) {
			continue
		}
		normal := e.rawBaseCost(p, id)
		if e.castable(p, id, e.offerCostFor(p, id, normal, false), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (from warp exile)", Obj: id, Mode: "warp_recast"})
		}
	}

	// Mana abilities may explicitly function from the battlefield, hand or
	// graveyard (Spirit Guides and Jack-o'-Lantern). availableManaAbilities
	// applies each ability's ActivationZone and full cost gate.
	for _, z := range []state.Zone{state.ZBattlefield, state.ZHand, state.ZGraveyard} {
		for _, id := range e.G.Zone(z, p) {
			o := e.G.Obj(id)
			f := o.Face()
			if f == nil {
				continue
			}
			if len(e.availableManaAbilities(p, id)) > 0 {
				add("activate", "Activate "+f.Name+" for mana", id)
			}
		}
	}

	// Activated abilities (Task 10): every non-mana AB$ ability on a
	// permanent p controls, and every one on a card in p's graveyard whose
	// ActivationZone$ names the graveyard, offered as an "ability" option the
	// way a cast is (rules/activate.go resolves one). Each is gated on the
	// same rules the cast options are: its own zone matches ActivationZone$,
	// SorcerySpeed$ True needs a full sorcery window, no CantBeActivated
	// restriction scopes down to it, a {T} cost needs an untapped source that
	// is neither tapped nor (for a creature, CR 302.6) summoning-sick without
	// Haste, and -- the totality gate -- the whole cost must be castable
	// (mana payable, every Sac/Discard/SubCounter part satisfiable) before the option
	// is ever offered. Equip (Task 14's attachments) is carried by this same
	// loop -- its K:Equip expansion (cards/keywords.go) is an AB$ Attach with
	// SorcerySpeed$ True, so the gates above cover it with no carve-out, and
	// rules/activate.go resolves it exactly like any other ability. This was
	// not always true: Task 14 round 1 shipped a second, Equip-only loop and
	// deleted it again on the main merge (one offer path, one activation
	// path), so do not resurrect one.
	for _, z := range []state.Zone{state.ZBattlefield, state.ZGraveyard, state.ZHand} {
		for _, id := range e.G.Zone(z, p) {
			o := e.G.Obj(id)
			f := o.Face()
			if f == nil {
				continue
			}
			for i, ab := range f.Abilities {
				if ab.Kind != "AB" || ab.API == "Mana" {
					continue
				}
				if !abilityZoneOK(ab, z) {
					continue
				}
				if ab.Params["SorcerySpeed"] == "True" && !sorcery {
					continue
				}
				// CR 606.3: a planeswalker's loyalty ability may be activated
				// only at the time a sorcery could be played -- during the
				// controller's own main phase with an empty stack -- and a
				// player may not activate a loyalty ability of a PERMANENT if
				// any loyalty ability OF THAT PERMANENT has already been
				// activated this turn. The gate is per permanent, not per
				// ability index: after [+2] the [0] draw-three is just as
				// withheld as a second [+2]. sorcerySpeed already folds the
				// own-turn and main-phase halves; the once-per-turn half is
				// the loyaltyActivationsThisTurn scan below (the event-log
				// scan the ActivationLimit$ gate uses, keyed to the object
				// and bounded by its current battlefield stint, CR 400.7),
				// because no corpus loyalty ability carries ActivationLimit$
				// and the gate must exist anyway (before this gate the
				// [+2]/[0] abilities were offered, payable and repeatable
				// without bound -- the live Jace draw-three exploit).
				if isLoyaltyAbility(ab) {
					if !sorcery {
						continue
					}
					if e.loyaltyActivationsThisTurn(id) >= e.loyaltyAbilityLimit(id) {
						continue
					}
				}
				if e.abilityRestricted(p, id, ab) {
					continue
				}
				if raw, ok := ab.Params["ActivationLimit"]; ok && e.activationLimitReached(id, p, i, raw) {
					continue
				}
				cost := ParseCost(ab.Params["Cost"])
				if cost.Tap && (o.Tapped || (z == state.ZBattlefield && o.SummonSick && slices.Contains(e.Derived(id).Types, "Creature") && !e.HasKeyword(id, "Haste"))) {
					continue
				}
				if !e.castable(p, id, e.offerCostFor(p, id, cost, true), true) {
					continue
				}
				if !e.abilityTargetsAvailable(p, id, ab) {
					continue
				}
				out = append(out, decision.Option{Index: len(out), Kind: "ability",
					Label: f.Name + ": " + ab.Params["SpellDescription"], Obj: id, Ability: i,
					Grant: e.abilityGrant(id, ab)})
			}
		}
	}

	// Pass is second-to-last. A client that wants to do nothing must choose
	// it explicitly: from M2d-3 the FINAL option is "concede" (R-M3, always
	// last), and a client defaulting to the final option would concede on
	// every single priority decision.
	add("pass", "Pass priority", 0)
	// M2d-3 (R-M3): concession, last after pass. Choosing it emits the
	// existing PlayerLost event with Text "conceded" (CR 104.3a) -- see
	// handlePriority. Offered on every priority decision, i.e. to every
	// living seat: grantPriority never hands a Lost seat priority, so no
	// extra guard is needed here.
	add("concede", "Concede", 0)
	return out
}

func (e *Engine) handlePriority(d *decision.Decision, in decision.Intent) {
	opt := d.Chosen(in)[0]
	switch opt.Kind {
	case "pass":
		passes := e.G.Passes + 1
		if passes >= int32(e.G.AliveCount()) {
			if len(e.G.Stack) > 0 {
				e.resolveTop()
				// CR 117.5: nobody receives priority in the middle of a
				// resolution. A resolution that suspends on a mid-resolution
				// ask (a modal spell's KModes, an as-enters choose, an
				// unless-pay, a discard) is parked on that question: no player
				// has priority while the question is outstanding, so the log
				// must not record that priority returned to the active player
				// here. The one and only grant for that resolution happens when
				// it actually completes: the answering Submit re-enters
				// grantPriority (through resumeTriggerDrain / the step loop)
				// once e.resume is cleared, so an unsuspended resolution emits
				// the priority-returns-to-active marker below while a suspended
				// one defers it to its completion. Exactly one grant either
				// way; a resolution that suspends more than once (a nested ask)
				// still completes once and grants once.
				if e.Suspended() {
					return
				}
				// The pass count resets: priority returns to the active
				// player after a resolution, same as at the start of any
				// other step.
				e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
				return
			}
			// advanceStep's own emit carries the reset pass count; the count
			// this round reached is never itself a value anything observes.
			e.advanceStep()
			return
		}
		e.emit(events.Event{Kind: events.Priority, Player: e.G.NextAlive(e.G.Priority), Amount: passes})

	case "play_land":
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		// Task 12: a land with an "as this enters" choice goes through the
		// same one-stage cast flow a spell does -- collect the choice, ask it
		// via chooseETB, record it with a Choose event, then continueCast's
		// payCast moves the land and logs the play. A land with none keeps
		// the original direct path (no pendingCast, no flow), so ordinary
		// lands are untouched. Both paths share the same continuation
		// machinery, never a parallel one.
		//
		// The source zone is the object's CURRENT zone, not hardcoded to the
		// hand: a may-play grant lets a land be played from the graveyard (or
		// exile), so a hand land and a graveyard land must move From the zone
		// they were actually offered from. A land that somehow left its zone
		// between the offer and the answer (only a hand-built intent makes
		// that possible) resolves from whatever zone it is in, and the
		// MoveZone/LandPlayed below still records a legal play.
		from := state.ZHand
		if o := e.G.Obj(opt.Obj); o != nil {
			from = o.Zone
		}
		pc := &pendingCast{player: in.Player, card: opt.Obj, from: from, mode: "land", ability: -1}
		e.cast = pc
		e.collectETBChoices(in.Player)
		if len(pc.etbs) == 0 {
			e.cast = nil
			e.emit(events.Event{Kind: events.MoveZone, Obj: opt.Obj,
				From: from, To: state.ZBattlefield})
			e.emit(events.Event{Kind: events.LandPlayed, Player: in.Player})
			return
		}
		e.continueCast()

	case "activate":
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		e.activateMana(in.Player, opt.Obj, false)

	case "ability":
		// Task 10: an activated ability (non-mana AB$) was chosen. Reset the
		// pass count the same way every other non-pass action does, then drive
		// the same cost flow a cast drives (rules/activate.go's
		// beginActivation -> pendingCast -> continueCast -> commitCast).
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		e.beginActivation(in.Player, opt)

	case "concede":
		// M2d-3 (R-M3): choosing the concede option emits the existing
		// PlayerLost event with Text "conceded" (CR 104.3a) -- one event,
		// the same one a 0-life elimination emits. PlayerLost's Apply marks
		// the seat Lost; this checkStateBased then sweeps its permanents
		// (CR 800.4a) and ends the game with the last remaining seat the
		// winner (checkGameOver, CR 104.2a) -- a concession is just another
		// way to be Lost. With three or more seats still alive, Submit's own
		// tail Advance continues the match with the Lost seat skipped
		// everywhere (grantPriority, NextAlive, beginTurn).
		e.emit(events.Event{Kind: events.PlayerLost, Player: in.Player, Text: "conceded"})
		e.checkStateBased()

	case "cast":
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		e.beginCast(in.Player, opt)
	}
}

// abilityGrant builds the server-side decision.Grant no-op flag for an
// activated ability ab on the permanent id (the option's Obj), only when the
// ability's whole effect is a pure, idempotent keyword grant. It returns nil
// for every other ability, so an additive or non-grant activation is never
// mistaken for a repeatable no-op. For a pure keyword grant it sets the two
// independent redundant halves the bot policy's no-op rule (A1) reads:
// Already (the granting permanent currently has every granted keyword, so
// the grant is already in effect) and Duplicate (an identical grant from the
// same source is already on the stack unresolved).
func (e *Engine) abilityGrant(id state.ObjID, ab *cards.SA) *decision.Grant {
	kw := pureGrantKeywords(ab)
	if len(kw) == 0 {
		return nil
	}
	g := &decision.Grant{Keywords: kw}
	all := true
	for _, k := range kw {
		if !e.HasKeyword(id, k) {
			all = false
			break
		}
	}
	g.Already = all
	g.Duplicate = e.grantPending(id, kw)
	return g
}

// pureGrantKeywords returns the keywords an SA grants when its whole effect is
// a pure, idempotent keyword grant AIMED AT ITS OWN SOURCE: a Pump adding only
// keywords (a non-empty KW$), with no additive stat change (NumAtt/NumDef), no
// SubAbility$ chain, and no colon-parametrised keyword. nil means the
// activation is not such a grant, so it stacks and must never be treated as a
// no-op. This is the engine-side definition of "idempotent keyword grant" that
// decision.Grant is built from.
//
// Every clause here excludes a corpus population the caller cannot reason
// about, measured over .cards/cardsfolder with /usr/bin/grep. Of the 588 raw
// A:AB$ Pump/PumpAll lines carrying a KW$ and no NumAtt/NumDef:
//
//   - 252 are TARGETED (ValidTgts$). The recipient is the target, not the
//     source, and the activation option carries no target at all -- targets
//     arrive at a later KTarget decision -- so neither "the source already has
//     this keyword" nor "an identical grant from this source is pending" says
//     anything about the creature that would actually receive it. Worse, the
//     pending check would refuse to aim a second copy at a DIFFERENT creature.
//   - 66 are PumpAll, granting to a filtered set rather than to Obj.
//   - 66 carry a SubAbility$ whose sub-effect is additive (DBUntap,
//     DBPutCounter, DBDealDamage), so suppressing the activation would forfeit
//     an untap, a counter or damage -- a worse bug than the one being fixed.
//   - 29 carry a colon-parametrised KW$, 20 of them Landwalk:<type>.
//     cards.KeywordHead strips at the colon, so Landwalk:Forest and
//     Landwalk:Island collapse to the same head on both sides of HasKeyword:
//     a creature with islandwalk would decline gaining forestwalk.
//
// Narrowing to the self-aimed Pump was measured to move no acceptance head and
// no botbench golden, so the wider form bought nothing it could be trusted on.
func pureGrantKeywords(ab *cards.SA) []string {
	if ab == nil || ab.API != "Pump" {
		return nil
	}
	if ab.Sub != nil {
		return nil
	}
	switch ab.Params["Defined"] {
	case "Self", "Parent":
	case "":
		if _, targeted := ab.Params["ValidTgts"]; targeted {
			return nil
		}
	default:
		return nil
	}
	if _, att := ab.Params["NumAtt"]; att {
		return nil
	}
	if _, def := ab.Params["NumDef"]; def {
		return nil
	}
	if strings.Contains(ab.Params["KW"], ":") {
		return nil
	}
	return grantKeywords(ab.Params["KW"])
}

// grantKeywords splits a KW$ parameter's keyword list into head-stripped
// words through cards.SplitKeywordList -- the shared Forge parser -- because
// this package cannot re-express the grammar without drifting from it. The
// Forge list separator is ampersand; a comma remains part of a parameter.
func grantKeywords(kw string) []string {
	var out []string
	for _, p := range cards.SplitKeywordList(kw) {
		out = append(out, cards.KeywordHead(p))
	}
	return out
}

// grantPending reports whether an identical keyword grant (the same granted
// keyword set) from the same source permanent id is already on the stack
// unresolved. It walks the shared stack order -- never a map -- and matches
// an ability object whose Source is id and whose own whole effect is the
// same pure keyword grant, so a stack spell, a trigger, or an additive
// activation from id never counts as a duplicate of an idempotent grant.
func (e *Engine) grantPending(id state.ObjID, kw []string) bool {
	for _, oid := range e.G.Zone(state.ZStack, 0) {
		o := e.G.Obj(oid)
		if o == nil || o.Source != id {
			continue
		}
		if grantSetsEqual(pureGrantKeywords(o.Ability), kw) {
			return true
		}
	}
	return false
}

// grantSetsEqual reports whether two granted keyword sets are the same,
// order-independently and case-insensitively.
func grantSetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		found := false
		for _, y := range b {
			if strings.EqualFold(x, y) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
