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

// abilityZoneOK reports whether ability ab may be activated while the
// source cardinal is in zone z (CR 602.1b): the printed ActivationZone$
// when present, the battlefield by default. Values other than Battlefield /
// Graveyard (Hand, Command, Exile, Stack) are not enumerated by this
// build's zone walk, so they simply never offer an option.
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
	}
	return false
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
		instantSpeed := f.IsInstant() || e.HasKeyword(id, "Flash")
		if !instantSpeed && !sorcery {
			continue
		}
		if !e.castTargetsAvailable(p, id, f.SpellAbility()) {
			continue
		}
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
		if e.castable(p, id, withSpellAbilityExtras(f, base)) {
			add("cast", "Cast "+f.Name, id)
		}
		for i, alt := range e.alternativeCosts(p, id) {
			// Ruling (Task 9 fix round 1, Important 1): this used to gate on
			// mana-only alt.CanPay, but ParseCost now produces Sac/SubCounter/
			// Tap parts that the cast flow enforces -- an AlternativeCost whose
			// Cost$ carries Sac<N/...> was offered without checking that N
			// matching permanents exist, and beginCast then asked a sacrifice
			// decision with zero options that no answer could escape. castable
			// is the same gate every other "cast" option uses.
			if e.castable(p, id, e.offerCostFor(p, id, alt, false)) {
				// AltCostIndex is i+1, not i: the zero value must mean "the
				// card's own cost" so every other Option literal in the tree
				// (play_land, activate, pass, and the base "cast" option
				// added just above via the shared add closure) needs no
				// change to keep meaning that.
				out = append(out, decision.Option{Index: len(out), Kind: "cast",
					Label: altCostLabel(f.Name, i), Obj: id, AltCostIndex: i + 1})
			}
		}
		if kc, ok := kickerCost(f); ok && e.castable(p, id, base.Plus(kc)) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (kicked)", Obj: id, Mode: "kicked"})
		}
		if sc, ok := surgeCost(f); ok && e.spellsCastThisTurn(p) > 0 && e.castable(p, id, e.offerCostFor(p, id, sc, false)) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (surged)", Obj: id, Mode: "surged"})
		}
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
		instantSpeed := f.IsInstant() || e.HasKeyword(id, "Flash")
		if !instantSpeed && !sorcery {
			continue
		}
		if !e.castTargetsAvailable(p, id, f.SpellAbility()) {
			continue
		}
		cost := e.offerCostFor(p, id, e.rawBaseCost(p, id), false)
		if e.castable(p, id, cost) {
			add("cast", "Cast "+f.Name, id)
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
		instantSpeed := f.IsInstant() || e.HasKeyword(id, "Flash")
		if !instantSpeed && !sorcery {
			continue
		}
		if !e.castTargetsAvailable(p, id, f.SpellAbility()) {
			continue
		}
		if fc := e.flashbackCost(id); e.castable(p, id, e.offerCostFor(p, id, fc, false)) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (flashback)", Obj: id, Mode: "flashback"})
		}
	}

	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil || o.Tapped {
			continue
		}
		// A tap-for-mana option exists while at least one individual mana
		// ability is unrestricted. activateMana uses this same per-member set:
		// a singleton resolves directly; distinct abilities sharing this tap
		// cost prompt the controller to select exactly one.
		if len(e.availableManaAbilities(p, id)) > 0 {
			add("activate", "Tap "+f.Name+" for mana", id)
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
	// (mana payable, every Sac/SubCounter part satisfiable) before the option
	// is ever offered. Equip (Task 14's attachments) is carried by this same
	// loop -- its K:Equip expansion (cards/keywords.go) is an AB$ Attach with
	// SorcerySpeed$ True, so the gates above cover it with no carve-out, and
	// rules/activate.go resolves it exactly like any other ability. This was
	// not always true: Task 14 round 1 shipped a second, Equip-only loop and
	// deleted it again on the main merge (one offer path, one activation
	// path), so do not resurrect one.
	for _, z := range []state.Zone{state.ZBattlefield, state.ZGraveyard} {
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
				if !e.castable(p, id, e.offerCostFor(p, id, cost, true)) {
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
		// Task 12: a land with an "as this enters" choice (an
		// ETBReplacement whose ReplaceWith$ is NameCard/ChooseType/
		// ChooseNumber, e.g. Cavern of Souls) goes through the same
		// one-stage cast flow a spell does -- collect the choice, ask it via
		// chooseETB, record it with a Choose event, then commitCast moves the
		// land and logs the play. A land with none keeps the original direct
		// path (no pendingCast, no flow), so ordinary lands are untouched.
		// Both paths share the same continuation machinery: etbAnswer/
		// continueCast/commitCast below, never a parallel one.
		pc := &pendingCast{player: in.Player, card: opt.Obj, from: state.ZHand, mode: "land", ability: -1}
		e.cast = pc
		e.collectETBChoices(in.Player)
		if len(pc.etbs) == 0 {
			e.cast = nil
			e.emit(events.Event{Kind: events.MoveZone, Obj: opt.Obj,
				From: state.ZHand, To: state.ZBattlefield})
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

// grantKeywords splits a KW$ parameter's "&"-joined keyword list into
// head-stripped words (the same separator effects' splitKeywords uses,
// re-expressed here because this package cannot import effects).
func grantKeywords(kw string) []string {
	kw = strings.TrimSpace(kw)
	if kw == "" {
		return nil
	}
	parts := strings.Split(kw, "&")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
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
