package rules

import (
	"regexp"
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
// twice. The zones walked are the graveyard and exile, plus the top card of
// the library (the kw-mayplay fallback below): the hand is covered by the
// normal walk, and a library card BELOW the top is hidden and cannot be
// meaningfully named.
func (e *Engine) mayPlayLandIds(p state.PlayerID) []state.ObjID {
	if !e.sorcerySpeed(p) || e.G.Players[p].LandsPlayed >= int32(1+e.adjustLandPlays(p)) {
		return nil
	}
	type offered struct {
		zone state.Zone
		id   state.ObjID
	}
	var out []state.ObjID
	var seen []offered
	limited := e.mayPlaysThisTurn(p)
	for _, ce := range e.active() {
		if !ce.MayPlay || ce.Controller != p {
			continue
		}
		if ce.MayPlayPlayerTurn && e.G.Active != p {
			continue
		}
		if ce.MayPlayLimit > 0 && int32(limited) >= ce.MayPlayLimit {
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
			// Exile and graveyard are public zones keyed by the card's OWNER,
			// and a grant's cards can sit in another seat's slice (Opposition
			// Agent exiles a card from an OPPONENT's searching library, then
			// lets its controller play it), so every seat's slice is walked in
			// deterministic seat order -- the same shape mayPlaySpellIds'
			// walk already is. The Affects match decides ownership claims;
			// walking the slices only enumerates candidates.
			for _, q := range e.G.AliveFrom(0) {
				for _, id := range e.G.Zone(z, q) {
					o := e.G.Obj(id)
					if o == nil || o.Face() == nil || !o.Face().IsLand() {
						continue
					}
					sc := effects.SpecContext{You: ce.Controller, Source: ce.Source,
						Remembered: rememberedTargets(ce.Remembered), Resolving: true}
					if !effects.MatchesSpecCtx(e.G, ce.Affects, id, sc) {
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
	// kw-mayplay: the card's OWN static (or a battlefield static naming it)
	// can also grant the play -- the same predicate beginCast's "mayplay"
	// cost case consults, so offer and charge agree. It covers the zones the
	// ce walk above cannot reach: a self-grant on a card sitting in a
	// graveyard or exile (staticEffects never reads a non-battlefield
	// source) and a Library-zone grant (Ka-Zar of the Savage Land), offered
	// for the TOP CARD ONLY so the hidden library never leaks a deeper
	// identity. The membership check keeps a card the ce walk already
	// offered from being offered twice.
	contains := func(id state.ObjID) bool {
		for _, got := range out {
			if got == id {
				return true
			}
		}
		return false
	}
	// landGranted is the land walk's grant gate: the may-play permission AND
	// no RaiseCost$ surcharge. A land play is FREE -- there is no cost site
	// that could charge a surcharge -- so a granting static carrying ANY
	// RaiseCost$ (priced or not) is withheld whole rather than granted
	// uncharged, the widening direction this file refuses. Measured: no
	// corpus RaiseCost$ carrier is a land, so this is a guard against the
	// next one, not a live behaviour change.
	landGranted := func(id state.ObjID) bool {
		if _, ok := e.mayPlayGrant(p, id); !ok {
			return false
		}
		if _, hasRaise, _ := e.mayPlayRaiseCost(p, id); hasRaise {
			return false
		}
		return true
	}
	for _, z := range []state.Zone{state.ZGraveyard, state.ZExile} {
		for _, id := range e.G.Zone(z, p) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil || !o.Face().IsLand() || o.Controller != p {
				continue
			}
			if landGranted(id) && !contains(id) {
				out = append(out, id)
			}
		}
	}
	if lib := e.G.Zone(state.ZLibrary, p); len(lib) > 0 {
		if o := e.G.Obj(lib[0]); o != nil && o.Face() != nil && o.Face().IsLand() && o.Controller == p {
			if landGranted(lib[0]) && !contains(lib[0]) {
				out = append(out, lib[0])
			}
		}
	}
	return out
}

// mayPlaysThisTurn counts the card plays this turn that went through a
// may-play-from-zone grant, in deterministic log order since the last
// TurnChange: CastInfo events carrying the mayplay flag (attributed by the
// card's OWNER -- CastInfo predates a Player field on the kind and setting
// one now would re-shape every replayed log's encoding, so the owner, which
// never changes, stands in; a control-steal corner may mis-attribute and
// then the cap can only undercount, never wedge), plus land plays whose
// MoveZone came from a granted (never hand) zone. Only a MayPlayLimit$ cap
// consults it; an unlimited grant ignores the count.
func (e *Engine) mayPlaysThisTurn(p state.PlayerID) int {
	n := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		switch ev.Kind {
		case events.CastInfo:
			if !strings.Contains(ev.Counter, "mayplay") {
				continue
			}
			if o := e.G.Obj(ev.Obj); o != nil && o.Owner == p {
				n++
			}
		case events.MoveZone:
			if ev.From != state.ZGraveyard && ev.From != state.ZExile {
				continue
			}
			if ev.To != state.ZBattlefield {
				continue
			}
			if o := e.G.Obj(ev.Obj); o != nil && o.Owner == p && o.Face() != nil && o.Face().IsLand() {
				n++
			}
		}
	}
	return n
}

// mayPlaySpellIds returns the ids of NON-LAND cards in player p's zones that
// an active may-play-from-zone grant lets p CAST this turn, in deterministic
// order (e.active(), then each effect's parsed AffectedZone order, then the
// zone slice order). A card is offered once even when several grants cover
// it (per (zone,id) dedupe). Only zones a spell can meaningfully be cast
// from (graveyard, exile) are walked; hand is the ordinary walk and library
// is hidden. Each candidate must match the granting effect's Affects filter
// through MatchesSpecCtx with the grant's Remembered set loaded as the
// SpecContext's Remembered, so the dominant Affected$ Card.IsRemembered
// grant (227 corpus files) selects exactly the cards its delivering Effect
// captured -- the same direct-list reading restrictionApplies uses for
// Effect-delivered CantTarget/CantRegenerate. A MayPlayLimit$ grant whose
// cap is already reached does not offer through itself; another grant
// covering the same card still may.
func (e *Engine) mayPlaySpellIds(p state.PlayerID) []state.ObjID {
	type offered struct {
		zone state.Zone
		id   state.ObjID
	}
	var out []state.ObjID
	var seen []offered
	limited := e.mayPlaysThisTurn(p)
	consider := func(z state.Zone, id state.ObjID) bool {
		for _, s := range seen {
			if s.zone == z && s.id == id {
				return false
			}
		}
		seen = append(seen, offered{z, id})
		out = append(out, id)
		return true
	}
	// A card's OWN S: static can grant its cast from a public zone it sits
	// in -- Misthollow Griffin / Eternal Scourge's exile self-grant,
	// Gravecrawler's "as long as you control a Zombie" graveyard cast -- or
	// from the library's top card (Korlessa, Scale Singer). staticEffects
	// never reads a non-battlefield source, so this scan covers exactly the
	// self-grant shapes; the same mayPlayGrant predicate beginCast's
	// "mayplay" cost case consults evaluates the card's statics (its gates
	// -- Condition$ PlayerTurn, IsPresent$, the unread-gate family -- fail
	// closed inside) AND every battlefield static naming the card, so the
	// ce walk below and this scan agree on every grant either discovers.
	// A Library-zone grant is offered for the TOP CARD ONLY, so the hidden
	// library never leaks a deeper card identity into the option list. The
	// dedupe keeps a card the ce walk already offered from being offered
	// twice.
	for _, z := range []state.Zone{state.ZGraveyard, state.ZExile} {
		for _, q := range e.G.AliveFrom(0) {
			for _, id := range e.G.Zone(z, q) {
				o := e.G.Obj(id)
				if o == nil || o.Face() == nil || o.Face().IsLand() || o.Controller != p {
					continue
				}
				if _, ok := e.mayPlayGrant(p, id); ok {
					consider(z, id)
				}
			}
		}
	}
	if lib := e.G.Zone(state.ZLibrary, p); len(lib) > 0 {
		if o := e.G.Obj(lib[0]); o != nil && o.Face() != nil && !o.Face().IsLand() && o.Controller == p {
			if _, ok := e.mayPlayGrant(p, lib[0]); ok {
				consider(state.ZLibrary, lib[0])
			}
		}
	}
	for _, ce := range e.active() {
		if !ce.MayPlay || ce.Controller != p {
			continue
		}
		if ce.MayPlayPlayerTurn && e.G.Active != p {
			continue
		}
		if ce.MayPlayLimit > 0 && int32(limited) >= ce.MayPlayLimit {
			continue
		}
		zones, all, ok := effects.ParseZones(ce.AffectedZone)
		if !ok && !all {
			continue
		}
		// Exile and graveyard are public zones keyed by the card's OWNER, and
		// a grant's cards can sit in another seat's slice (Intellect
		// Devourer exiles an OPPONENT's hand card, then lets its controller
		// play it), so every seat's slice is walked in deterministic seat
		// order -- never a map. The Affects match decides ownership claims;
		// walking the slices only enumerates candidates.
		for _, q := range e.G.AliveFrom(0) {
			for _, z := range func() []state.Zone {
				if all {
					return []state.Zone{state.ZGraveyard, state.ZExile}
				}
				return zones
			}() {
				if z != state.ZGraveyard && z != state.ZExile {
					continue
				}
				for _, id := range e.G.Zone(z, q) {
					o := e.G.Obj(id)
					if o == nil || o.Face() == nil || o.Face().IsLand() {
						continue
					}
					sc := effects.SpecContext{You: ce.Controller, Source: ce.Source,
						Remembered: rememberedTargets(ce.Remembered), Resolving: true}
					if !effects.MatchesSpecCtx(e.G, ce.Affects, id, sc) {
						continue
					}
					consider(z, id)
				}
			}
		}
	}
	return out
}

// coreCardTypes is CardType.CoreType (forge-card's CardType.java) -- the
// distinct-type census Delirium's gate counts. Kindred is included: modern
// type lines spell it as a word and the engine's printed Types carry it.
var coreCardTypes = []string{"Artifact", "Battle", "Creature", "Enchantment",
	"Instant", "Kindred", "Land", "Planeswalker", "Sorcery"}

// activationConditionOK evaluates an ability's Activation$ activation
// condition -- the "Hellbent —", "Threshold —", "Metalcraft —",
// "Delirium —" cost-prompt family (Sea Gate Wreckage's draw, Mox Opal's
// mana). Forge's SpellAbilityCondition.areMet keyword half, on the
// ACTIVATOR (the controller asking to activate), evaluated at OFFER time
// like the CheckSVar$ gate below: an ability whose condition fails is not
// offered, so a paid no-op activation is never reachable:
//
//   - Hellbent: the activator's hand is empty (Player.hasHellbent);
//   - Threshold: the activator's graveyard holds 7+ cards;
//   - Metalcraft: the activator controls 3+ artifacts;
//   - Delirium: the activator's graveyard holds 4+ distinct core card types
//     (AbilityUtils.countCardTypesFromList's non-permanent form).
//
// Solved (the Case permanents' solved flag) and Blessing (the city's
// blessing) name state this build does not track, so their gate FAILS
// CLOSED -- the conservative direction for an "only if" condition whose
// meeting cannot be verified. No repo-deck card carries either (measured at
// the current corpus pin: 3 raw lines each, none in the decks).
func (e *Engine) activationConditionOK(p state.PlayerID, ab *cards.SA) bool {
	raw, ok := ab.Params["Activation"]
	if !ok || strings.TrimSpace(raw) == "" {
		return true
	}
	switch strings.TrimSpace(raw) {
	case "Hellbent":
		return len(e.G.Zone(state.ZHand, p)) == 0
	case "Threshold":
		return len(e.G.Zone(state.ZGraveyard, p)) >= 7
	case "Metalcraft":
		n := 0
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if o := e.G.Obj(id); o != nil && slices.Contains(e.Derived(id).Types, "Artifact") {
				n++
			}
		}
		return n >= 3
	case "Delirium":
		seen := map[string]bool{}
		for _, id := range e.G.Zone(state.ZGraveyard, p) {
			if o := e.G.Obj(id); o != nil {
				for _, ty := range o.Face().Types {
					seen[ty] = seen[ty] || slices.Contains(coreCardTypes, ty)
				}
			}
		}
		n := 0
		for _, ty := range coreCardTypes {
			if seen[ty] {
				n++
			}
		}
		return n >= 4
	}
	return false
}

// activationGameTypesOK evaluates an activated ability's ActivationGameTypes$
// format list at offer time (War Room's "Activate only in a game of
// Commander, Brawl, Tiny Leaders, or Oathbreaker", the corpus's only
// carrier). The value is a comma list of the game formats the ability exists
// in; gorge models exactly two formats -- FormatCommander and
// FormatConstructed -- and only the "Commander" token maps to one. Brawl,
// TinyLeaders and Oathbreaker are not modelled and match nothing, so the
// list never admits a Constructed game: a format-gated ability is withheld
// before it could ever be announced (CR 602.1/601.2c: an illegal activation
// is not offered). Deterministic pure read -- no map range, tokens trimmed.
func activationGameTypesOK(f Format, raw string) bool {
	for _, tok := range strings.Split(raw, ",") {
		switch strings.TrimSpace(tok) {
		case "Commander":
			if f == FormatCommander {
				return true
			}
		}
	}
	return false
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

// abilityPresentHolds evaluates an activated ability's IsPresent$ /
// PresentCompare$ activation gate at OFFER time -- Mistveil Plains' "Activate
// only if you control two or more white permanents" (IsPresent$
// Permanent.White+YouCtrl, PresentCompare$ GE2). The same deterministic
// battlefield count the statics' presentGate (rules/statics.go) and the
// reflected-mana offer's manaReflectedPresentHolds (rules/mana_activation.go)
// evaluate; PresentCompare$ defaults to "at least one" when it is absent, the
// shared reading everywhere else the gate appears. The gate is offer-time
// only, exactly like SorcerySpeed$/PlayerTurn$/CheckSVar$: no state can move
// between the offer and the answer inside one priority window, and the
// resolution does not re-gate.
//
// Because the gate applies to every non-mana activation (the offer loop
// skips isManaAbilityAPI abilities -- a plain AB$ Mana ability's own
// IsPresent$ gate is the mana path's business), its reads are excluded from
// the census's generic rules-side SA union for Mana/ManaReflected: see
// genericSAExcludes in paramcensus_test.go.
func (e *Engine) abilityPresentHolds(p state.PlayerID, id state.ObjID, ab *cards.SA) bool {
	if !e.classBandGateHolds(ab.Params, id) {
		return false
	}
	spec := strings.TrimSpace(ab.Params["IsPresent"])
	if spec == "" {
		return true
	}
	n := e.countPresent(spec, id, p)
	if cmp := strings.TrimSpace(ab.Params["PresentCompare"]); cmp != "" {
		return comparePresent(n, cmp)
	}
	return n > 0
}

// sVarGateOK evaluates the ability's CheckSVar$/SVarCompare$ intervening-if
// at OFFER time: Bloodsoaked Champion's Raid ("Activate only if you attacked
// this turn", CheckSVar$ RaidTest = Count$AttackersDeclared) and Ojer
// Axonil's transformed Temple of Power ("Activate only if red sources you
// controlled dealt 4 or more noncombat damage this turn" — a count head this
// build does not model, so its gate reads 0 and the transform stays
// unoffered, the documented degrade-to-zero direction). The same evaluator
// conditionMet (ConditionCheckSVar$, effects/conditions.go) and the statics'
// checkSVarHolds delegate to: effects.CheckSVarHolds. Because the gate
// applies to every non-mana activation, its reads are the census's generic
// rules-side SA set, not any one api's.
func (e *Engine) sVarGateOK(p state.PlayerID, id state.ObjID, ab *cards.SA, merged int) bool {
	check, ok := ab.Params["CheckSVar"]
	if !ok {
		return true
	}
	o := e.G.Obj(id)
	if o == nil || o.PileFaceFor(merged) == nil {
		return false
	}
	svars := e.pileSVars(id, merged)
	ctx := &effects.Ctx{Source: id, Controller: p, SVars: svars}
	holds, evaluated := effects.CheckSVarHolds(e, ctx, check, ab.Params["SVarCompare"])
	if !evaluated {
		// The gate's count body is not one the evaluator models: fail OPEN —
		// the ability is still offered. A gate you cannot read must not
		// silently remove a card's activation (Ojer Axonil's transformed
		// Temple of Power counts noncombat damage by source this build does
		// not track; suppressing the transform on that account would brick
		// the card's mechanic on an unreadable gate).
		return true
	}
	return holds
}

// ownReduceCost evaluates an ability's own ReduceCost$ parameter (Otawara,
// Soaring City's Channel: "This ability costs {1} less to activate for each
// legendary creature you control" — ReduceCost$ X over
// SVar:X:Count$Valid Creature.Legendary+YouCtrl): the generic reduction the
// value resolves to. A literal is the value; a name (X) resolves through the
// source face's SVar table via effects.EvalCountOK — the same resolver
// fixLifeXCost uses — and an unresolvable body degrades to zero (a
// reduction this build cannot compute is never silently over-applied; the
// full-cost ability stays legal, just never discounted). The CR 601.2f
// composition: folded into the offer gate's cost AND beginActivation's
// stored cost, so the two can never disagree. Because the read applies to
// every non-mana activation, it joins the census's generic rules-side SA
// set, not one api's.
//
// targets are the chosen targets, carried on the Ctx so a target-dependent
// body (Raft Security Officer's AllTargeted$Valid Creature.powerLE3) can
// resolve. The offer/projection sites pass nil — targets do not exist yet
// at offer time, so a target-dependent reduction reads 0 there (full price,
// fail closed) — and repriceForTargets re-runs the evaluation with the
// answered targets at CR 601.2c, before CR 601.2h pays.
func (e *Engine) ownReduceCost(p state.PlayerID, id state.ObjID, ab *cards.SA, targets []state.Target, merged int) int32 {
	v := strings.TrimSpace(ab.Params["ReduceCost"])
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil {
		if n < 0 {
			return 0
		}
		return int32(n)
	}
	o := e.G.Obj(id)
	if o == nil || o.PileFaceFor(merged) == nil {
		return 0
	}
	svars := e.pileSVars(id, merged)
	body := v
	if b, ok := svars[v]; ok {
		body = b
	}
	ctx := &effects.Ctx{Source: id, Controller: p, SVars: svars, Targets: targets}
	if n, ok := effects.EvalCountOK(e, ctx, body); ok && n > 0 {
		return n
	}
	return 0
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
	return isLoyaltyAbilityCost(ab, ParseCost(ab.Params["Cost"]))
}

// isLoyaltyAbility is the engine-owned form of the loyalty classifier. Its
// card-script cost is configured text, so use the immutable parser sidecar.
func (e *Engine) isLoyaltyAbility(ab *cards.SA) bool {
	return isLoyaltyAbilityCost(ab, e.parseCost(ab.Params["Cost"]))
}

func isLoyaltyAbilityCost(ab *cards.SA, c Cost) bool {
	if v, ok := ab.Params["Planeswalker"]; ok && strings.EqualFold(strings.TrimSpace(v), "True") {
		return true
	}
	// Ultimate$ (Ugin, Eye of the Storms' [-X]: AB$ ChangeZone ... Ultimate$
	// True) marks the planeswalker's ultimate for Forge's deck-tooling and
	// the client's loyalty-UI presentation; the rules meaning -- a loyalty
	// ability, once per permanent per turn (CR 606.3) -- is already covered
	// by the Planeswalker$ marker this gate reads. The recognition keeps the
	// parameter census honest; the presentation half is named in the deck
	// import report's Issues.
	_ = ab.Params["Ultimate"]
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
			if f != nil && int(ev.Amount) < len(f.Abilities) && e.isLoyaltyAbility(f.Abilities[int(ev.Amount)]) {
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

// activationUsedCount counts this object's activations of one ability over
// the replayable event log. The identity has two shapes, exactly as
// boastGateOK reads them: a PRINTED or mana activation mints an AbilityPush /
// ManaActivate whose Amount is the ability's FLAT pile index (CR 702.140d,
// top face first then under-cards); a GRANTED activation (AddAbility$ /
// Animate) mints a DelayedPush or GrantAbilityPush whose Amount is -1 and
// whose Counter names the granting SVar instead. ability < 0 selects the
// granted shape, svar == "" the printed shape -- so one scanner serves both
// ActivationLimit$ (thisTurn) and GameActivationLimit$ (the whole game), and
// a third caller cannot forget one of them.
//
// thisTurn bounds the walk at the latest TurnChange: the per-turn limit's
// window (CR 606.3's "this turn", and Forge's ActivationLimit$, which
// ActivationTable resets each turn). The per-game walk deliberately does NOT
// break there, and deliberately does not reset on a zone change either:
// Forge keys the count on the host CARD (Card.numberGameActivations, read
// through SpellAbility.getActivationsThisGame), which survives a battlefield
// departure, so a permanent that leaves and returns has still spent its
// once-per-game activation.
func (e *Engine) activationUsedCount(id state.ObjID, ability int, svar string, thisTurn bool) int {
	used := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if thisTurn && ev.Kind == events.TurnChange {
			break
		}
		if ev.Obj != id {
			continue
		}
		switch ev.Kind {
		case events.AbilityPush, events.ManaActivate:
			if ability >= 0 && ev.Amount == int32(ability) {
				used++
			}
		case events.DelayedPush, events.GrantAbilityPush:
			if svar != "" && ev.Counter == svar {
				used++
			}
		}
	}
	return used
}

// activationLimitReachedAt reports whether this object has already activated
// the ability at the FLAT pile index ability as many times as its
// ActivationLimit permits this turn. merged selects the face whose SVar table
// a computed limit resolves against (0 = the top face). The limit itself is
// resolved by resolveActivationLimitAt: a literal integer is used directly,
// and a computed expression (an SVar name or an inline Count$...) is
// evaluated through the effects count path, so a limit such as Withering
// Wisps' "number of snow Swamps you control" is enforced rather than silently
// ignored. A limit that resolves to zero or to fewer activations than have
// already been used withholds the offer. An expression that genuinely cannot
// be resolved stays unenforced (today's behaviour): resolveActivationLimitAt
// reports ok=false.
func (e *Engine) activationLimitReachedAt(id state.ObjID, p state.PlayerID, ability int, raw string, merged int) bool {
	limit, ok := e.resolveActivationLimitAt(id, p, raw, merged)
	if !ok || limit < 0 {
		return false
	}
	return e.activationUsedCount(id, ability, "", true) >= limit
}

// activationLimitBlocked is the ONE gate every activation offer site calls
// for the two sibling limits of a non-mana activated ability: ActivationLimit$
// (this turn) and GameActivationLimit$ (the whole game). Both are read here so
// a new offer site cannot honour one and miss the other -- the two loops in
// this file (printed, granted) and the mana walk in mana_activation.go all
// funnel through it. svar is the granted-ability identity ("" for a printed
// ability); merged selects the face a computed limit resolves against. The
// per-GAME count is scanned with the same identity shapes and no turn or
// stint boundary (see activationUsedCount).
//
// A limit that resolves to zero or to fewer activations than already used
// withholds; an unresolvable expression stays unenforced, exactly as the
// per-turn gate already documented.
func (e *Engine) activationLimitBlocked(p state.PlayerID, id state.ObjID, sa *cards.SA, ability int, svar string, merged int) bool {
	if sa == nil {
		return false
	}
	if raw, ok := sa.Params["ActivationLimit"]; ok {
		if limit, ok := e.resolveActivationLimitAt(id, p, raw, merged); ok && limit >= 0 &&
			e.activationUsedCount(id, ability, svar, true) >= limit {
			return true
		}
	}
	if raw, ok := sa.Params["GameActivationLimit"]; ok {
		if limit, ok := e.resolveActivationLimitAt(id, p, raw, merged); ok && limit >= 0 &&
			e.activationUsedCount(id, ability, svar, false) >= limit {
			return true
		}
	}
	return false
}

// boastGateOK implements CR 702.142's Boast activation restriction for an
// ability whose SA carries Boast$ True: the ability may be activated only if
// its source creature attacked this turn, and only once each turn. Both
// halves are read from replay-derivable state: "attacked this turn" is the
// event-folded Object.AttacksThisTurn (events.Apply's DeclareAttackers case,
// reset in TurnChange's per-object loop -- the same fact the Raid gate's
// Count$AttackersDeclared and the FirstAttack$ trigger gate read), and
// "already used this turn" is the activation-event scan the ActivationLimit$
// gate uses, with one extension.
//
// The extension is the granted-ability identity. A PRINTED AB$ mints an
// AbilityPush whose Amount is the ability's face index (events.Apply's
// AbilityPush case); a GRANTED AB$ (Besieged Viking Village's AddAbility$
// ABBoast) goes through beginGrantedActivation, which mints a DelayedPush
// whose Amount is -1 and whose Counter names the granting SVar instead
// (rules/speed.go). A scan that only looked at AbilityPush/Amount --
// activationLimitReached's shape -- could not see a granted Boast at all and
// would re-offer it every window. Matching either identity closes that: the
// printed form matches on index, the granted form on the SVar name.
func (e *Engine) boastGateOK(id state.ObjID, ability int, svar string) bool {
	o := e.G.Obj(id)
	if o == nil || o.AttacksThisTurn == 0 {
		return false
	}
	used := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Obj != id {
			continue
		}
		switch ev.Kind {
		case events.AbilityPush:
			if svar == "" && ev.Amount == int32(ability) {
				used++
			}
		case events.DelayedPush, events.GrantAbilityPush:
			// A granted activation's identity is its SVar name. A self-grant
			// mints through DelayedPush (Counter = the name); a CROSS-object
			// grant -- a printed Continuous AddAbility$ static such as
			// Besieged Viking Village's "All creatures have 'Boast -- {1}: ...'"
			// -- mints through GrantAbilityPush, whose Counter is the same
			// name. Reading only DelayedPush would leave the granted Boast
			// re-offered in every priority window of the turn it was used.
			if svar != "" && ev.Counter == svar {
				used++
			}
		}
	}
	return used == 0
}

// resolveActivationLimitAt interprets an ActivationLimit$ value. A literal
// integer is used directly. A non-literal value is resolved through the
// Count$/SVar evaluator the rest of the tree uses (effects.EvalCount), bound
// to the source object and the SVar table of the face at pile position merged
// (0 = the top face), so a computed limit is enforced rather than silently
// ignored. ok reports whether the limit was resolvable at all: false keeps the
// pre-fix behaviour of leaving the limit unenforced, which is also what a
// value that is neither a literal nor an SVar reference (such as a
// description-suffixed literal from a keyword template) gets.
func (e *Engine) resolveActivationLimitAt(id state.ObjID, p state.PlayerID, raw string, merged int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if n, err := strconv.Atoi(raw); err == nil {
		return n, true
	}
	o := e.G.Obj(id)
	if o == nil || o.PileFaceFor(merged) == nil {
		return 0, false
	}
	svars := e.pileSVars(id, merged)
	ctx := &effects.Ctx{Source: id, Controller: p, SVars: svars}
	if strings.HasPrefix(raw, "Count$") {
		return int(effects.EvalCount(e, ctx, raw)), true
	}
	if body, ok := svars[raw]; ok {
		return int(effects.EvalCount(e, ctx, body)), true
	}
	return 0, false
}

// targetsAvailable reports whether the narrow target requirement that can
// be proved before offering a spell or ability is satisfiable. A missing
// TargetMin$/TargetMax$ pair is Forge's unconditional one-target shape.
// Dynamic bounds and modal or announced choices stay offerable until the
// post-announcement askTarget backstop can evaluate them with those choices
// made. xPending extends that same carve-out to the announced {X}: a cost
// that announces an X makes a ValidTgts$ X-bound dynamic, so a spec whose
// ONLY zero-candidate reason is its unresolvable X bound stays offerable.
// excludeSelf is the CR 115.5 self-targeting object: the offered card
// for a spell cast from a zone that could contain it, 0 for an activated
// ability (whose Source permanent IS a legal target of its own ability).
func (e *Engine) targetsAvailable(p state.PlayerID, id, excludeSelf state.ObjID, sa *cards.SA, xPending bool) bool {
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
	if n := len(e.legalTargetCandidates(p, id, excludeSelf, sa)); n > 0 {
		return true
	}
	// A zero-candidate census is only a withhold when the spec's X bound is
	// static. With an announced X pending the bound is dynamic: offer, and
	// let the post-announcement backstop evaluate it against the chosen
	// value (a wrong value still fizzles the proposal at 601.2c).
	return xPending && specNamesXBound(sa.Params["ValidTgts"])
}

// costAnnouncesX reports whether paying this cost announces a value for {X}
// before targets are chosen: a printed {X} mana symbol or a PayEnergy<X>
// energy part (Forge announces both through the same ability X). A cost that
// announces X makes every ValidTgts$ bound that reads that X (cmcEQX and its
// siblings) a DYNAMIC bound -- the same carve-out TargetMin$/TargetMax$/
// Announce$/Choices$ already have -- so the offer gate does not withhold the
// action on a bound whose value does not exist yet; the post-announcement
// askTarget backstop evaluates it once the X is fixed (CR 601.2b before
// 601.2c).
// costAnnouncesX reports whether the cost announces an X the cast or
// activation chooses (CR 601.2b/107.3i): a printed {X} mana symbol, a
// PayEnergy<X> part, an announced PayLife<X> payment, an announced
// SubCounter<X/Kind> removal, or a tapXType<X/Spec> part whose tap election
// announces it (the dynTapCost head). The tap-election clause does not add
// the announced-Sac clause: this gate's callers treat a true answer as "the
// X-bound targets are dynamic -- offer and evaluate at the ask", and the
// tap election is announced BEFORE the target ask (the tap stage precedes
// targetAsk in continueCast), so the bound is already fixed when targets
// are chosen either way; the clause only stops the offer gate from
// withholding the action on a bound whose value the tap election will
// supply (Aryel's powerLEX).
func costAnnouncesX(c Cost) bool {
	if c.X > 0 {
		return true
	}
	for _, part := range c.Energy {
		if part.Spec == "X" {
			return true
		}
	}
	if len(c.LifeX) > 0 {
		return true
	}
	for _, part := range c.SubCounter {
		if part.Announced {
			return true
		}
	}
	for _, part := range c.TapPermanent {
		if part.Dyn == "X" {
			return true
		}
	}
	return false
}

// specNamesXBound reports whether a ValidTgts$ spec carries a numeric bound
// whose right-hand side is the paid {X}: the <field><CMP>X family numericPred
// resolves through SpecContext.Resolve (powerGEX, cmcEQX, toughnessLTX,
// counters_GTX_<KIND>). Only these shapes are dynamic in X; a literal bound
// (cmcGE3) is static and stays gated at offer time.
var xBoundRe = regexp.MustCompile(`(?i)(power|toughness|cmc)(LE|GE|EQ|LT|GT)X|counters_(?:LE|GE|EQ|LT|GT)X_`)

func specNamesXBound(spec string) bool {
	return xBoundRe.MatchString(spec)
}

// castTargetsAvailable is the cast-offer guard: the spell card may not target
// itself (CR 115.5), so excludeSelf is the card id. A cost that announces an
// X (printed {X} or a SpellAbility Cost$ PayEnergy<X>) relaxes an X-bound
// spec to the post-announcement backstop (costAnnouncesX above).
func (e *Engine) castTargetsAvailable(p state.PlayerID, id state.ObjID, sa *cards.SA) bool {
	xPending := false
	if o := e.G.Obj(id); o != nil && o.Face() != nil {
		xPending = costAnnouncesX(e.parseCost(o.Face().ManaCost))
		if ab := o.Face().SpellAbility(); ab != nil {
			xPending = xPending || costAnnouncesX(e.parseCost(ab.Params["Cost"]))
		}
	}
	return e.targetsAvailable(p, id, id, sa, xPending)
}

// abilityTargetsAvailable is the activated-ability offer guard. It is what
// stops an ability with no legal target from being re-offered in a loop after
// the transaction aborts it (CR 602.2b / 601.2c: such an ability cannot be
// activated at all). Unlike a cast, an activated ability CAN target its own
// Source permanent (Mother of Runes targeting itself), so no self-exclusion
// applies. An ability cost that announces an X (a {X} mana symbol or
// PayEnergy<X>) relaxes an X-bound spec to the post-announcement backstop.
func (e *Engine) abilityTargetsAvailable(p state.PlayerID, id state.ObjID, ab *cards.SA) bool {
	return e.targetsAvailable(p, id, 0, ab, costAnnouncesX(e.parseCost(ab.Params["Cost"])))
}

// grantedAbility is one ability a continuous ability grant (CR 613.1f,
// state.ContinuousEffect.AddAbilities -- a Saga chapter's Animate) gives an
// object right now: the parsed AB and the SVar name on the granting face's
// table that re-resolves it.
type grantedAbility struct {
	sa *cards.SA
	// source is the object the grant came from (state.ContinuousEffect.Source):
	// the static's own permanent, which need not be the affected object the
	// ability is activated from. It is threaded into decision.Option.GrantSource
	// so the activation resolves the SVar body from here while the minted
	// ability's Source stays the recipient.
	source state.ObjID
	svar   string
}

// grantedAbilities collects the activated abilities the battlefield's
// AddAbilities grants give id right now. Each entry is gated on the granting
// effect actually applying to id (its Affects spec, "You" bound to the
// effect's own controller) and re-resolved against the granting source's
// face SVar table -- the name travels, never a parsed copy, so a replay
// re-executing the grant reads the identical body. Only AB$ lines grant;
// a name whose body is missing or is not an AB degrades to no grant (the
// same totality stance every SVar resolution takes). Order: active()'s own
// stable layer/timestamp sort, names in the grant's own order.
func (e *Engine) grantedAbilities(p state.PlayerID, id state.ObjID) []grantedAbility {
	var out []grantedAbility
	for _, ce := range e.active() {
		if len(ce.AddAbilities) == 0 {
			continue
		}
		if !effects.MatchesSpecFrom(e.G, ce.Affects, id, ce.Controller, ce.Source) {
			continue
		}
		src := e.G.Obj(ce.Source)
		if src == nil || src.Face() == nil {
			continue
		}
		for _, nm := range ce.AddAbilities {
			svars := ce.SVars
			if svars == nil {
				svars = src.Face().SVars
			}
			ab := cards.ResolveSVar(svars, nm)
			if ab == nil || ab.Kind != "AB" {
				continue
			}
			out = append(out, grantedAbility{sa: ab, source: ce.Source, svar: nm})
		}
	}
	return out
}

// adjustLandPlays reports how many land drops BEYOND the ordinary one
// (CR 305.2a) player p gets this turn: the SUM over the active
// additional-land-drops grants (Azusa, Oracle of Mul Daya, Exploration)
// whose Affects spec matches p -- the sum, never the max, because each
// grant's printed sentence modifies the one-drop normal independently
// (Azusa plus Exploration is three drops). Each grant's Affects is a
// PLAYER spec evaluated with MatchesPlayerSpecFrom against the granting
// effect's controller, so "Affected$ You" is the SOURCE's controller: a
// stolen Azusa grants its new controller, and a spec with a qualifier the
// matcher does not implement matches nobody (fail closed, no grant).
// "On each of your turns" is not evaluated here -- the offer gates only
// ever offer a play_land to the active player in a main phase, and the
// per-turn counter resets at the TurnChange untap. The walk is a pure read
// over active()'s sorted slice (never a map), so the resulting option list
// stays reproducible run to run.
func (e *Engine) adjustLandPlays(p state.PlayerID) int {
	total := 0
	for _, ce := range e.active() {
		if ce.AdjustLandPlays <= 0 {
			continue
		}
		if !effects.MatchesPlayerSpecFrom(e.G, ce.Affects, p, ce.Controller, ce.Source) {
			continue
		}
		total += int(ce.AdjustLandPlays)
	}
	return total
}

// legalActions enumerates everything p may legally do with priority. The
// result is the complete rules surface a client ever sees.
func (e *Engine) legalActions(p state.PlayerID) []decision.Option {
	return e.legalActionsPriced(p, nil)
}

// aftermathAlternateFace returns the Aftermath alternate face (face 1 --
// ALTERNATE starts face 1 in cards/parse.go) of a two-face Split card whose
// front face is current, or nil when the object is not a well-formed
// aftermath carrier: AlternateMode must be Split (a Room is Split too, but
// no Room half carries K:Aftermath, and the keyword gate is what keeps Rooms
// and Adventures on their own paths), the object must have exactly two
// faces, and the card must still be at its front face -- the aftermath half
// is cast only from a graveyard card whose printed front is showing.
func aftermathAlternateFace(o *state.Object) *cards.Face {
	if o == nil || o.Card == nil || o.Card.AlternateMode != "Split" || len(o.Card.Faces) != 2 || int(o.FaceIdx) != 0 {
		return nil
	}
	af := o.Card.Faces[1]
	if af == nil || !af.HasKeyword("Aftermath") {
		return nil
	}
	return af
}

// legalActionsPriced is legalActions with the mana affordability priced
// against an OVERBOUND hypothetical pool instead of the seat's floating one:
// hyp nil keeps the ordinary floating-pool pricing, hyp non-nil prices every
// cast/activation gate against *hyp -- the pool the seat would hold if it
// first floated every mana its untapped sources could produce (PotentialMana).
// The walk itself is otherwise IDENTICAL: same zones (hand, command zone,
// graveyard flashback, battlefield abilities), same timing, restriction,
// target and non-mana-cost gates, same live RaiseCost/ReduceCost composition
// (offerCostFor) -- so a potential action is by construction the same option
// the engine WOULD offer once the mana floated, never a client-side
// re-derivation that can drift from the engine's own cost rules.
//
// It is a pure read: no event is emitted, no state field is written, and the
// hypothetical pool lives only in local copies, so replay is untouched.
func (e *Engine) legalActionsPriced(p state.PlayerID, hyp *state.Mana) []decision.Option {
	var out []decision.Option
	add := func(kind, label string, obj state.ObjID) {
		out = append(out, decision.Option{Index: len(out), Kind: kind, Label: label, Obj: obj})
	}
	sorcery := e.sorcerySpeed(p)
	// hyp is the pricing mode the walk runs in: nil is the ordinary real-pool
	// offer walk; non-nil is the potential-action walk's hypothetical bound.
	// The two affordability gates below are the ONLY pricing difference:
	// every non-mana part (Sac/Discard/SubCounter/Tap) is checked against the
	// REAL state in both modes -- floating mana never satisfies a sacrifice --
	// and the cost composition (offerCostFor) is pool-independent, so the
	// two walks cannot drift inside the body they share.
	costStatics := costStaticSource{e: e}
	actionStatics := actionStaticSource{e: e}
	castRestricted := func(p state.PlayerID, id state.ObjID) bool {
		return e.castRestrictedUsing(actionStatics.get().cantCast, p, id)
	}
	abilityRestricted := func(p state.PlayerID, id state.ObjID, ab *cards.SA) bool {
		return e.abilityRestrictedUsing(actionStatics.get().cantActivate, p, id, ab)
	}
	offerCastable := func(p state.PlayerID, id state.ObjID, base Cost, scope costScope, ability bool) bool {
		return e.offerCastableUsing(costStatics.get(), p, id, base, scope, ability, hyp)
	}
	// affordable is the composed-cost gate the two direct e.castable sites of
	// the walk use (the may-play and escape walks price an already-composed
	// cost, so they cannot re-run the modifier composition offerCastable
	// owns); hyp==nil is the ordinary castable, hyp!=nil prices the same
	// composed cost against the hypothetical pool.
	affordable := func(q state.PlayerID, id state.ObjID, c Cost, ability bool) bool {
		if hyp == nil {
			return e.castable(q, id, c, ability)
		}
		return e.castablePriced(q, id, c, ability, *hyp)
	}
	offerCostFor := func(p state.PlayerID, id state.ObjID, base Cost, scope costScope) Cost {
		return e.offerCostForUsing(costStatics.get(), p, id, base, scope)
	}

	for _, id := range e.G.Zone(state.ZHand, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil {
			continue
		}
		if f.IsLand() {
			if sorcery && e.G.Players[p].LandsPlayed < int32(1+e.adjustLandPlays(p)) {
				add("play_land", "Play "+f.Name, id)
			}
			continue
		}
		if castRestricted(p, id) {
			continue
		}
		if e.castSuppressed(p, id) {
			continue
		}
		// CR 709.4/709.5: a non-Room split card's alternate half is castable
		// on its own (mode split_alt, consumed by beginCast's FlipFace exactly
		// like room_alt) and, when the card carries K:Fuse, BOTH halves may be
		// cast as one fused spell (mode fuse, paying the combined cost and
		// resolving both halves). Both offers read the half's OWN timing
		// (review MAJOR 2), targets and cost through
		// splitCastTargetsAvailable, so a half with no legal target -- or one
		// the seat cannot pay for -- is withheld independently of the front
		// face. They stand ABOVE the front-face spellTimingOK gate below on
		// purpose: CR 709.4 gives each half its own timing, so a front-Sorcery
		// half must not withhold its instant alternate half during an
		// opponent's turn (incubation_incongruity, discovery_dispersal,
		// said_done, spring_mind). For that reason they also precede the
		// plain cast in the option list for a split card. The card-level
		// castRestricted/castSuppressed gates above stay shared -- casting a
		// half is casting the card.
		if sf := splitAlternateCastFace(o); sf != nil {
			instant := sf.IsInstant() || e.HasKeyword(id, "Flash") || e.castWithFlash(p, id)
			if (instant || sorcery) && e.splitCastTargetsAvailable(p, id, sf) {
				if offerCastable(p, id, withSpellAbilityExtras(sf, e.parseCost(sf.ManaCost)), spellScope(""), false) {
					out = append(out, decision.Option{Index: len(out), Kind: "cast",
						Label: "Cast " + sf.Name, Obj: id, Mode: "split_alt"})
				}
			}
		}
		if ff, fa := fusedSplitFaces(o); ff != nil {
			if e.fusedTimingOK(p, id, ff, fa, sorcery) &&
				e.splitCastTargetsAvailable(p, id, ff) && e.splitCastTargetsAvailable(p, id, fa) {
				if offerCastable(p, id, e.fuseCost(ff, fa), spellScope(""), false) {
					out = append(out, decision.Option{Index: len(out), Kind: "cast",
						Label: "Cast " + ff.Name + " // " + fa.Name + " (fused)", Obj: id, Mode: "fuse"})
				}
			}
		}
		if !e.spellTimingOK(p, id, f, sorcery) {
			// MayFlashCost (Forge's K:MayFlashCost, CR 702.8): when the ordinary
			// timing gate fails, a face printed with the keyword is NOT skipped
			// outright -- it may be cast at instant timing by paying the extra.
			// The mayflash offer below is the ONLY option this branch adds; the
			// plain cast and every other mode on the face stay behind the
			// sorcery-speed gate. When it is already sorcery timing the plain
			// cast is strictly cheaper, so no mayflash option is offered and the
			// face takes the ordinary path (no redundant duplicate offer).
			if e.mayflashTimingOK(p, f) {
				if extra, ok := mayflashExtraCost(f); ok && e.castTargetsAvailable(p, id, f.SpellAbility()) {
					if offerCastable(p, id, withSpellAbilityExtras(f, e.castOfferBase(p, id)).Plus(extra), spellScope("mayflash"), false) {
						out = append(out, decision.Option{Index: len(out), Kind: "cast",
							Label: "Cast " + f.Name + " (may-flash)", Obj: id, Mode: "mayflash"})
					}
				}
			}
			continue
		}
		targetsAvailable := e.castTargetsAvailable(p, id, f.SpellAbility())
		// offerCastable prices the MANA the offer will charge (601.2f
		// modifiers, then the commander tax) over the RAW base beginCast will
		// store; withSpellAbilityExtras adds the spell's own ADDITIONAL
		// non-mana parts on top. Both are needed and they compose in this
		// order: an additional cost is never reduced by a cost modifier, the
		// same reason the commander tax lands after the modifiers rather than
		// before them.
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
		// convokeBase is the offer gate's composed RAW base: the printed mana
		// cost with CR 702.51 Convoke and CR 702.66 Improvise's generic credits.
		// castOfferBase is the shared recipe so the plain and mayflash offers
		// cannot drift (the mayflash branch above uses it too).
		convokeBase := e.castOfferBase(p, id)
		// An either-or additional cost (AlternateAdditionalCost) makes the
		// plain cast's gate existential: the cast is offerable when AT LEAST
		// ONE alternative part is payable (the choice itself is asked by the
		// cast flow, altAddAsk), never when all of them are unpayable. Cards
		// without the keyword keep the ordinary single-cost gate.
		altParts := altAddCostParts(f)
		if targetsAvailable {
			if len(altParts) > 0 {
				for _, part := range altParts {
					if offerCastable(p, id, withSpellAbilityExtras(f, convokeBase).Plus(e.parseCost(part)), spellScope(""), false) {
						add("cast", "Cast "+f.Name, id)
						break
					}
				}
			} else if offerCastable(p, id, withSpellAbilityExtras(f, convokeBase), spellScope(""), false) {
				add("cast", "Cast "+f.Name, id)
			}
		}
		// CR 309.4b: either door of a Room may be cast. Mode room_alt is
		// consumed by beginCast, which records a FlipFace before the ordinary
		// cast transaction; from then on every cost/target/resolution reader
		// sees the selected face. This is structural over every two-door Room,
		// not a card-name exception (Spiked Corridor is the front-trigger case).
		// (The split_alt/fuse offers live ABOVE the front-face timing gate --
		// see their comment there.)
		if rf := roomAlternateCastFace(o); rf != nil {
			instant := rf.IsInstant() || e.HasKeyword(id, "Flash")
			if (instant || sorcery) && e.castTargetsAvailable(p, id, rf.SpellAbility()) {
				if offerCastable(p, id, withSpellAbilityExtras(rf, e.parseCost(rf.ManaCost)), spellScope(""), false) {
					out = append(out, decision.Option{Index: len(out), Kind: "cast",
						Label: "Cast " + rf.Name, Obj: id, Mode: "room_alt"})
				}
			}
		}
		// CR 714.3a: the Adventure spell face of an Adventure card may be cast
		// from hand. Mode adventure_alt is consumed by beginCast, which records
		// a FlipFace to the spell face before the ordinary cast transaction;
		// the resolution then exiles the card into the adventure zone
		// (spellRestZone's FlagAdventure branch). Gated on the ADVENTURE face's
		// own timing, targets and cost -- an Instant Adventure casts at instant
		// speed, a Sorcery Adventure only at sorcery timing -- exactly like the
		// Room offer above, including the withSpellAbilityExtras fold (the
		// spell face's own SP Cost$ additional parts).
		if int(o.FaceIdx) == 0 {
			if af := adventureSpellFace(o); af != nil && e.spellTimingOK(p, id, af, sorcery) &&
				e.castTargetsAvailable(p, id, af.SpellAbility()) {
				if offerCastable(p, id, withSpellAbilityExtras(af, ParseCost(af.ManaCost)), spellScope(""), false) {
					out = append(out, decision.Option{Index: len(out), Kind: "cast",
						Label: "Cast " + af.Name, Obj: id, Mode: "adventure_alt"})
				}
			}
		}
		for i, alt := range e.alternativeCosts(p, id) {
			if !targetsAvailable {
				continue
			}
			// An announce-bearing alternative (the Shoal cycle's Announce$ X):
			// the exile filter's cmcEQX is bound to the value the caster will
			// announce, so the "some announcement is payable" gate is
			// existential over X — at least one exilable card must match at
			// SOME mana value. The gate below replaces offerCastable's
			// unannounced (X=0) exile check with that existential scan and
			// gates the mana-only remainder on the same cost minus its exile
			// parts; beginCast asks the X (xAsk's announce arm), then exAsk
			// re-walks the part with the announced X bound.
			gate := alt.cost
			if alt.announce != "" {
				gate.Exile = nil
				if len(e.altCostXCandidates(p, id, alt)) == 0 {
					continue
				}
			}
			// Ruling (Task 9 fix round 1, Important 1): this used to gate on
			// mana-only alt.CanPay, but ParseCost now produces Sac/SubCounter/
			// Tap parts that the cast flow enforces -- an AlternativeCost whose
			// Cost$ carries Sac<N/...> was offered without checking that N
			// matching permanents exist, and beginCast then asked a sacrifice
			// decision with zero options that no answer could escape. castable
			// is the same gate every other "cast" option uses.
			if offerCastable(p, id, gate, spellScope(""), false) {
				// AltCostIndex is i+1, not i: the zero value must mean "the
				// card's own cost" so every other Option literal in the tree
				// (play_land, activate, pass, and the base "cast" option
				// added just above via the shared add closure) needs no
				// change to keep meaning that.
				out = append(out, decision.Option{Index: len(out), Kind: "cast",
					Label: altCostLabel(f.Name, i), Obj: id, AltCostIndex: i + 1})
			}
		}
		// The and/or Kicker (Forge's colon-separated two-part Kicker:<a>:<b>,
		// Wastescape Battlemage's "Kicker {G} and/or {1}{U}"): each part is an
		// independent optional additional cost (CR 601.2b), so each payable
		// combination is its own cast option -- part 1, part 2, or both. The
		// modes ride FlagKicked1/FlagKicked2 (modeFlags), which the
		// "Card.Self+kicked <n>" trigger and replacement specs read.
		if c1, c2, ok := twoPartKickerCosts(f); ok {
			base := e.rawBaseCost(p, id)
			for _, kp := range [...]struct {
				mode, label string
				cost        Cost
			}{
				{"kicked1", "Cast " + f.Name + " (kicked 1)", c1},
				{"kicked2", "Cast " + f.Name + " (kicked 2)", c2},
				{"kickedboth", "Cast " + f.Name + " (kicked both)", c1.Plus(c2)},
			} {
				if targetsAvailable && offerCastable(p, id, base.Plus(kp.cost), spellScope(kp.mode), false) {
					out = append(out, decision.Option{Index: len(out), Kind: "cast",
						Label: kp.label, Obj: id, Mode: kp.mode})
				}
			}
		} else if kc, ok := kickerCost(f); ok && targetsAvailable && offerCastable(p, id, e.rawBaseCost(p, id).Plus(kc), spellScope("kicked"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (kicked)", Obj: id, Mode: "kicked"})
		}
		if sc, ok := surgeCost(f); ok && targetsAvailable && e.spellsCastThisTurn(p) > 0 && offerCastable(p, id, sc, spellScope("surged"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (surged)", Obj: id, Mode: "surged"})
		}
		// Replicate (CR 702.55a): the replicated variant is its own cast
		// option paying the base cost plus ONE replicate payment -- one
		// payment is what gates the offer; the count ask (replicateAsk)
		// settles how many afterwards and the 601.2g payment window may still
		// produce mana for the composed total, exactly like a kicked cast, so
		// the max count cannot be fixed at offer time. The non-mana parts of
		// the payment fail closed in nonManaCastable (offerCastable's shared
		// tail), so the two tapXType carriers' replicate never offers.
		if rc, ok := replicateCost(f); ok && targetsAvailable &&
			offerCastable(p, id, e.rawBaseCost(p, id).Plus(rc), spellScope("replicated"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (replicated)", Obj: id, Mode: "replicated"})
		}
		// Multikicker (CR 702.43): the multikicked variant is its own cast
		// option paying the base cost plus ONE multikicker payment -- the
		// replicate offer's exact shape (one payment is what gates the offer;
		// the count ask, multikickAsk, settles how many afterwards). No corpus
		// carrier pairs Kicker with Multikicker (measured), so this offer
		// never collides with the kicked family above.
		if mkc, ok := multikickerCost(f); ok && targetsAvailable &&
			offerCastable(p, id, e.rawBaseCost(p, id).Plus(mkc), spellScope("multikicked"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (multikicked)", Obj: id, Mode: "multikicked"})
		}
		// Squad (CR 702.66): the squadded variant pays the base cost plus ONE
		// squad payment -- the replicate/multikicker offer's exact shape (one
		// payment gates the offer; the count ask, squadAsk, settles how many
		// afterwards, and the 601.2g payment window may still produce mana for
		// the composed total). No corpus carrier pairs Squad with
		// Replicate/Multikicker/Kicker (measured over the 15 K:Squad files), so
		// this offer never collides with the count asks above. Non-mana parts
		// fail closed in nonManaCastable (offerCastable's shared tail), so a
		// squad cost ParseCost cannot price never offers.
		if sqc, ok := squadCost(f); ok && targetsAvailable &&
			offerCastable(p, id, e.rawBaseCost(p, id).Plus(sqc), spellScope("squadded"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (squadded)", Obj: id, Mode: "squadded"})
		}
		// Conspire (CR 702.78a): the conspired variant pays NO extra mana --
		// the base cost is unchanged and the cost is the tap of two untapped
		// creatures the caster controls that share a colour with the spell.
		// So unlike the replicate/multikicker offers there is no cost to
		// compose: the offer is gated on the same base cast being offerable
		// (re-checked with the base cost, exactly what the plain offer used)
		// AND on the derived-keyword read (hasCastConspire, the
		// hasCastConvoke shape, so a layer-6 grant reaching the stack matches)
		// AND on at least two eligible creatures existing. Do NOT route the
		// tap through offerCastable with a fabricated cost -- the tap has no
		// Cost$ representation; conspireAsk enforces it at announcement.
		if targetsAvailable && e.hasCastConspire(id) &&
			len(e.conspireCandidates(p, id)) >= 2 &&
			offerCastable(p, id, withSpellAbilityExtras(f, convokeBase), spellScope(""), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (conspired)", Obj: id, Mode: "conspired"})
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
				!offerCastable(p, id, alt, spellScope(ka.mode), false) {
				continue
			}
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (" + ka.mode + ")", Obj: id, Mode: ka.mode})
		}
		// Bestow (CR 702.114a): the bestowed cast is its own "cast" option
		// paying the bestow cost in place of the mana cost, and the spell is
		// an Aura with enchant creature, so the offer gates on the targets of
		// the SYNTHESIZED attach SA -- the face has no SP of its own, so the
		// plain cast's targetsAvailable (from the nil SpellAbility) says
		// nothing about it. bestowCost withholds the exotic bestow costs (an
		// {X}, detectives_phoenix's CollectEvidence<6>, hypnotic_siren's
		// colon-suffixed metadata line), the replicateCost convention.
		if ba, ok := bestowCost(f); ok && e.castTargetsAvailable(p, id, bestowedAttachSA()) &&
			offerCastable(p, id, ba, spellScope("bestowed"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (bestowed)", Obj: id, Mode: "bestowed"})
		}
		// Mutate (CR 702.140a): the mutate cast pays the mutate cost in place
		// of the mana cost and targets a non-Human creature its controller
		// owns. Like bestow, the gate is the SYNTHESIZED target SA's
		// feasibility -- the creature face has no SP for the mutation.
		if mc, ok := mutateCost(f); ok && e.castTargetsAvailable(p, id, mutateTargetSA()) &&
			offerCastable(p, id, mc, spellScope("mutated"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (mutated)", Obj: id, Mode: "mutated"})
		}
		if bc, ok := buybackCost(f); ok && offerCastable(p, id, e.rawBaseCost(p, id).Plus(bc), spellScope("buyback"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast", Label: "Cast " + f.Name + " (buyback)", Obj: id, Mode: "buyback"})
		}
		// Offspring (CR 702.175a): the optional ADDITIONAL cost half, offered
		// beside the plain cast. e.offspringCost reads the DERIVED keyword list
		// with the stack-zone override (rules/offspring.go), so BOTH the
		// printed K:Offspring line and a layer-6 AddKeyword$ Offspring grant
		// reaching the cast (Zinnia, Valley's Voice's "Creature spells you cast
		// have offspring {2}") are priced through the ONE read -- the granted
		// cost is charged, never a printed one, and the offer and beginCast's
		// modeCost stage structurally cannot disagree. The offer gate is the
		// same base+additional composition beginCast will charge, and
		// targetsAvailable keeps a target-bearing creature spell's offer honest
		// (the plain cast's gate, which the offspring cast shares).
		if targetsAvailable && e.hasCastOffspring(id) {
			if oc, ok := e.offspringCost(id); ok &&
				offerCastable(p, id, e.rawBaseCost(p, id).Plus(oc), spellScope("offspring"), false) {
				out = append(out, decision.Option{Index: len(out), Kind: "cast",
					Label: "Cast " + f.Name + " (offspring)", Obj: id, Mode: "offspring"})
			}
		}
		if sc, ok := suspendCost(f); ok {
			offer := sc.cost
			if sc.timeX {
				// XMin<N> is part of the announcement, not a later payment
				// preference: do not offer a Suspend X action that cannot pay
				// even its smallest legal X.
				offer = offer.WithX(sc.minTime)
			}
			if offerCastable(p, id, offer, spellScope("suspend"), false) {
				out = append(out, decision.Option{Index: len(out), Kind: "cast", Label: "Suspend " + f.Name, Obj: id, Mode: "suspend"})
			}
		}
		// Foretell (CR 702.126a): the special action pays {2} and exiles the
		// card from the hand FACE DOWN -- never the keyword's own colon
		// parameter, which prices the LATER cast. "During your turn" is the
		// only timing gate (deliberately NO instant/sorcery-speed check,
		// unlike Suspend); the hand walk's own castRestricted/castSuppressed
		// and spellTimingOK continues bound the offer, the same window the
		// Suspend offer above inherits.
		if _, ok := f.KeywordParam("Foretell"); ok && e.G.Active == p &&
			offerCastable(p, id, Cost{Generic: 2}, spellScope("foretell"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast", Label: "Foretell " + f.Name, Obj: id, Mode: "foretell"})
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

	// A may-play-from-zone grant also makes NON-LAND cards in a granted zone
	// castable this turn (CR 401.5; Atsushi's "you may play those cards",
	// Opposition Agent's MayPlay+IgnoreColor static). This is a THIRD cast
	// source alongside the hand and command-zone walks, never a replacement;
	// the cast pays the card's PRINTED cost (the grant changes where it may
	// come from, not what it costs), with the grant's MayPlayIgnoreColor$
	// rider payable-as-any-colour consulted by castable (the card is still
	// in the granted zone here) and recorded on the pendingCast at beginCast
	// for the window and the payment to keep. A MayPlayLimit$ grant whose cap
	// is reached offers nothing through itself (mayPlaySpellIds). The offer
	// gate folds the card's own SpellAbility additional costs exactly like
	// the hand walk does, so an offered may-play cast and the cost beginCast
	// charges structurally cannot disagree.
	for _, id := range e.mayPlaySpellIds(p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil || f.IsLand() || castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		if !e.spellTimingOK(p, id, f, sorcery) {
			continue
		}
		// The permission's ValidSA$ decides which cast shapes it permits
		// (Brokkos, Apex of Forever's `ValidSA$ Spell.Mutate` permits ONLY
		// the mutate cast from the graveyard, CR 903.3d/702.140a). An empty
		// ValidSA$ is the ordinary permission, so this splits the historical
		// single offer into its plain and mutate halves without changing any
		// unrestricted grant's behaviour.
		plain, mutate := e.mayPlayKinds(p, id)
		if plain && e.castTargetsAvailable(p, id, f.SpellAbility()) {
			base := e.rawBaseCost(p, id)
			if free, ok := e.mayPlayGrant(p, id); ok && free {
				// MayPlayWithoutManaCost$ True (the kw-mayplay predicate): the
				// mana part is free, exactly as beginCast's "mayplay" case will
				// charge it; non-mana additional costs still apply (CR 118.9).
				base = Cost{}
			}
			// CR 118.3a: the granting static's RaiseCost$ surcharge is added on
			// top of the printed cost (Kotis, Sibsig Champion's "by exiling
			// three other cards ... in addition to paying its other costs").
			// Composed before offerCostFor so static cost modifiers apply to the
			// raised cost, the CR 601.2f order; the affordability gate below then
			// prices the whole cost against real state (nonManaCastable). A raise
			// mayPlayStatic could not price never reaches here -- mayPlayGrant
			// withholds the card -- but the defensive continue keeps the two
			// sites agreeing if that ever changes.
			if raise, hasRaise, priced := e.mayPlayRaiseCost(p, id); hasRaise {
				if !priced {
					continue
				}
				base = base.Plus(raise)
			}
			cost := withSpellAbilityExtras(f, offerCostFor(p, id, base, spellScope("mayplay")))
			if affordable(p, id, cost, false) {
				out = append(out, decision.Option{Index: len(out), Kind: "cast",
					Label: "Cast " + f.Name, Obj: id, Mode: "mayplay"})
			}
		}
		// Mutate half: the permission names the mutate cast. The mutate cast
		// pays the mutate cost in place of the mana cost and targets a
		// non-Human creature its controller owns -- the same synthesized
		// target SA and the same cost substitution the hand walk's mutate
		// offer uses (legal.go's hand branch, beginCast's "mutated" case),
		// which prices the mutate cost regardless of the zone the card is
		// cast from. A may-play mutate carries no printed-mana "free"
		// exemption: MayPlayWithoutManaCost$ is a property of the permission,
		// but the mutate cost IS the mana cost this cast pays.
		if mutate {
			if mc, ok := mutateCost(f); ok && e.castTargetsAvailable(p, id, mutateTargetSA()) &&
				offerCastable(p, id, mc, spellScope("mutated"), false) {
				out = append(out, decision.Option{Index: len(out), Kind: "cast",
					Label: "Cast " + f.Name + " (mutated)", Obj: id, Mode: "mutated"})
			}
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
		if castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		if !e.spellTimingOK(p, id, f, sorcery) {
			continue
		}
		targetsAvailable := e.castTargetsAvailable(p, id, f.SpellAbility())
		if targetsAvailable && offerCastable(p, id, e.rawBaseCost(p, id), spellScope(""), false) {
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
				!offerCastable(p, id, alt, spellScope(ka.mode), false) {
				continue
			}
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (" + ka.mode + ")", Obj: id, Mode: ka.mode})
		}
		// Bestow (CR 702.114a), the command-zone half (a bestowed commander,
		// kestia_the_cultivator's shape): the same synthesized-attach-SA gate
		// the hand walk applies.
		if ba, ok := bestowCost(f); ok && e.castTargetsAvailable(p, id, bestowedAttachSA()) &&
			offerCastable(p, id, ba, spellScope("bestowed"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (bestowed)", Obj: id, Mode: "bestowed"})
		}
		// Mutate (CR 702.140a), the command-zone half (a commander printed
		// with mutate may be cast for its mutate cost, CR 903.3d): the same
		// synthesized-target-SA gate the hand walk applies.
		if mc, ok := mutateCost(f); ok && e.castTargetsAvailable(p, id, mutateTargetSA()) &&
			offerCastable(p, id, mc, spellScope("mutated"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (mutated)", Obj: id, Mode: "mutated"})
		}
		// Offspring (CR 702.175a), the command-zone half (a commander printed
		// with offspring, or granted it by a static): the same base+additional
		// composition the hand walk offers.
		if targetsAvailable && e.hasCastOffspring(id) {
			if oc, ok := e.offspringCost(id); ok &&
				offerCastable(p, id, e.rawBaseCost(p, id).Plus(oc), spellScope("offspring"), false) {
				out = append(out, decision.Option{Index: len(out), Kind: "cast",
					Label: "Cast " + f.Name + " (offspring)", Obj: id, Mode: "offspring"})
			}
		}
	}

	// Harmonize is a graveyard alternative. It is offered as its own cast
	// transaction, then spellRestZone exiles it after resolution.
	for _, id := range e.G.Zone(state.ZGraveyard, p) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		f := o.Face()
		if !e.spellTimingOK(p, id, f, sorcery) {
			continue
		}
		if hc, ok := harmonizeCost(f); ok && e.castTargetsAvailable(p, id, f.SpellAbility()) {
			hc, _ = e.harmonizePayment(p, id, hc)
			if offerCastable(p, id, hc, spellScope("harmonize"), false) {
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
		if castRestricted(p, id) {
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
		if fc := e.flashbackCost(id); offerCastable(p, id, fc, spellScope("flashback"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (flashback)", Obj: id, Mode: "flashback"})
		}
	}

	// Aftermath (CR 702.85a): the alternate face of a Split card may be cast
	// from its owner's graveyard for its printed mana cost (plus its own SP
	// Cost$ additional parts -- start_finish's Sac<1/Creature>), then exiled.
	// Gated on the ALTERNATE face's K:Aftermath, which is what excludes Rooms
	// (both halves are Rooms, neither carries Aftermath) and Adventures
	// (AlternateMode Adventure, not Split). Mode aftermath is consumed by
	// beginCast, which records a FlipFace to the alternate face before the
	// ordinary cast transaction -- rawBaseCost, targets and resolution then
	// read the aftermath face. The withSpellAbilityExtras fold prices the
	// face's own SP Cost$ parts exactly like the adventure_alt offer above:
	// without it a Finish-shaped gate would offer an unpayable cast.
	for _, id := range e.G.Zone(state.ZGraveyard, p) {
		o := e.G.Obj(id)
		af := aftermathAlternateFace(o)
		if af == nil || castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		if !e.spellTimingOK(p, id, af, sorcery) {
			continue
		}
		if !e.castTargetsAvailable(p, id, af.SpellAbility()) {
			continue
		}
		if offerCastable(p, id, withSpellAbilityExtras(af, ParseCost(af.ManaCost)), spellScope(""), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + af.Name + " (aftermath)", Obj: id, Mode: "aftermath"})
		}
	}

	// MayPlay static offers are the main-side walk above (mayPlayLandIds /
	// mayPlaySpellIds over the continuous-effect grants plus the self-grant
	// scan); the kw-mayplay branch's separate mayPlayGrantOffers walk is
	// retired here so a granted card is offered exactly once.

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
		if !ok || !warpGraveyardAllowed(f) || castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		instantSpeed := f.IsInstant() || e.HasKeyword(id, "Flash")
		if !instantSpeed && !sorcery {
			continue
		}
		if !e.castTargetsAvailable(p, id, f.SpellAbility()) {
			continue
		}
		if offerCastable(p, id, wc, spellScope("warped"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (warped)", Obj: id, Mode: "warped"})
		}
	}

	// Escape (CR 702.42a): a card in its owner's graveyard carrying the
	// Escape keyword -- printed (Kroxa's K:Escape) or granted by a continuous
	// effect (Underworld Breach's AddKeyword$ Escape, which Derived reads off
	// the layer system) -- may be cast for its escape cost: the card's mana
	// cost plus ExileFromGrave<N/Card.Other> parts, exiling N OTHER cards
	// from the same graveyard. The cast is a normal cast (CR 702.42a gives
	// no post-resolution destination change -- unlike flashback the spell
	// goes where it would otherwise go), so modeFlags marks it FlagEscaped
	// and the ETB machinery reads the flag through Card.Self+escaped.
	for _, id := range e.G.Zone(state.ZGraveyard, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil || castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		if !e.HasKeyword(id, "Escape") {
			continue
		}
		ec, ok := e.escapeCost(id)
		if !ok || !e.spellTimingOK(p, id, f, sorcery) ||
			!e.castTargetsAvailable(p, id, f.SpellAbility()) {
			continue
		}
		if affordable(p, id, offerCostFor(p, id, ec, spellScope("escape")), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (escape)", Obj: id, Mode: "escape"})
		}
	}

	// Retrace (CR 702.81a): a card in its owner's graveyard carrying the
	// Retrace keyword may be cast from there by paying its printed mana cost
	// PLUS an additional cost of discarding a land card. Unlike Escape this
	// is not a cost substitution, so the offer prices the ordinary plain-cast
	// base (castOfferBase credits Convoke/Improvise, withSpellAbilityExtras
	// adds the spell's own additional parts) with retraceExtra folded on top.
	// The discard is a real hand cost, so the offer is withheld unless a land
	// card is actually there to discard -- an option that cannot be paid must
	// never be offered (the offerCastable/withSpellAbilityExtras ruling).
	for _, id := range e.G.Zone(state.ZGraveyard, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil || castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		if !e.HasKeyword(id, "Retrace") {
			continue
		}
		if !e.spellTimingOK(p, id, f, sorcery) ||
			!e.castTargetsAvailable(p, id, f.SpellAbility()) {
			continue
		}
		rx := retraceExtra()
		if !e.discardCostPayable(p, id, rx.Discard, true) {
			continue
		}
		if offerCastable(p, id, withSpellAbilityExtras(f, e.castOfferBase(p, id)).Plus(rx), spellScope("retrace"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (retrace)", Obj: id, Mode: "retrace"})
		}
	}

	// Jump-start (CR 702.84a): "You may cast this card from your graveyard by
	// discarding a card in addition to paying its other costs. Then exile this
	// card." The retrace shape with any card discardable in place of a land,
	// and the flashback destination on resolution. The keyword takes no
	// parameter (all 13 corpus lines are the bare K:Jump-start), so the whole
	// cost is the printed mana cost plus the additional Discard<1/Card>.
	// Gated on the derived keyword (HasKeyword), so a continuous-effect grant
	// would count; the same timing/target/restriction gates as every other
	// graveyard alt-cast, and the discount payable gate -- an option whose
	// discard cannot be paid must never be offered (the offerCastable ruling).
	for _, id := range e.G.Zone(state.ZGraveyard, p) {
		o := e.G.Obj(id)
		f := o.Face()
		if f == nil || castRestricted(p, id) || e.castSuppressed(p, id) {
			continue
		}
		if !e.HasKeyword(id, "Jump-start") {
			continue
		}
		if !e.spellTimingOK(p, id, f, sorcery) ||
			!e.castTargetsAvailable(p, id, f.SpellAbility()) {
			continue
		}
		js := jumpstartExtra()
		if !e.discardCostPayable(p, id, js.Discard, true) {
			continue
		}
		if offerCastable(p, id, withSpellAbilityExtras(f, e.castOfferBase(p, id)).Plus(js), spellScope("jumpstart"), false) {
			out = append(out, decision.Option{Index: len(out), Kind: "cast",
				Label: "Cast " + f.Name + " (jump-start)", Obj: id, Mode: "jumpstart"})
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
		// CR 714.3a: the main face of an Adventure card resting in the
		// adventure zone (exile, at its Adventure spell face) may be cast from
		// there. Mode adventure_recast is consumed by beginCast, which flips
		// the card back to its main face before the ordinary cast transaction
		// -- so everything downstream reads the main face. The provenance (the
		// card got here by RESOLVING an adventure_alt cast) is log-derived:
		// adventureZoneAvailable. The cost is built from the MAIN face
		// explicitly -- rawBaseCost reads o.Face(), which is still the
		// Adventure face in exile, exactly the reason the Room offer parses its
		// own face's cost too. The adventure-zone entry from a non-resolution
		// path (CR 714.3b) is out of scope, and adventureZoneAvailable refuses
		// it.
		if adventureSpellFace(o) != nil && int(o.FaceIdx) == 1 && e.adventureZoneAvailable(id) &&
			!castRestricted(p, id) && !e.castSuppressed(p, id) {
			front := o.Card.Faces[0]
			if e.spellTimingOK(p, id, front, sorcery) && e.castTargetsAvailable(p, id, front.SpellAbility()) &&
				offerCastable(p, id, ParseCost(front.ManaCost), spellScope(""), false) {
				out = append(out, decision.Option{Index: len(out), Kind: "cast",
					Label: "Cast " + front.Name + " (from adventure zone)", Obj: id, Mode: "adventure_recast"})
			}
			continue
		}
		// Foretell cast (CR 702.126a): a card exiled face down by the {2}
		// Foretell ACTION (not by any other effect -- the flag is the action's
		// own provenance marker) may be cast from exile for its FORETELL cost
		// (the K: line's colon parameter) on a later turn. "Later turn" is
		// log-derived (foretellCastAvailable): the flag alone cannot say it,
		// the same reason the warp recast offer below is log-derived. The cast
		// follows the card's own timing (spellTimingOK) and targets. The block
		// sits BEFORE the warp gate's continue: a non-warp card (every foretell
		// carrier) would otherwise never reach it.
		if raw, ok := f.KeywordParam("Foretell"); ok && o.CastFlags&state.FlagForetold != 0 &&
			e.foretellCastAvailable(id) && !castRestricted(p, id) && !e.castSuppressed(p, id) &&
			e.spellTimingOK(p, id, f, sorcery) && e.castTargetsAvailable(p, id, f.SpellAbility()) {
			// The K:Foretell parameter prices the later cast (CR 702.126a);
			// a face with no parameter falls back to the rule's action default
			// {2} -- every corpus carrier carries one (measured 55/55), so the
			// fallback is latent. The RAW parsed cost is offered here; cost
			// modifiers (CR 601.2f) apply later, in manaToPay, exactly like
			// the other alternative-cost recasts.
			fc := Cost{Generic: 2}
			if strings.TrimSpace(raw) != "" {
				fc = ParseCost(raw)
			}
			if offerCastable(p, id, fc, spellScope("foretell_cast"), false) {
				out = append(out, decision.Option{Index: len(out), Kind: "cast",
					Label: "Cast " + f.Name + " (foretold)", Obj: id, Mode: "foretell_cast"})
			}
		}
		_, ok := keywordAltCost(f, "Warp")
		if !ok || !e.warpRecastAvailable(id) || castRestricted(p, id) || e.castSuppressed(p, id) {
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
		if offerCastable(p, id, normal, spellScope("warp_recast"), false) {
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
			mas := e.availableManaAbilitiesUsing(&actionStatics, p, id)
			if len(mas) == 0 {
				continue
			}
			opt := decision.Option{Index: len(out), Kind: "activate", Label: "Activate " + f.Name + " for mana", Obj: id}
			// fb-led1: a mana ability that costs more than a bare tap is the
			// play the window exists for — carry its cost so the client's
			// empty-priority-window floor stops instead of passing it away.
			if marker := manaActivationCostMarker(mas); marker != "" {
				opt.Cost = marker
			}
			out = append(out, opt)
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
			if e.faceDownPrintedHides(o) {
				// CR 708.8: a face-down permanent's printed activated abilities
				// and mana abilities do not exist while it is face down, and
				// turn-face-up (CR 708.6) is not implemented -- nothing on a
				// face-down permanent is offered at all.
				continue
			}
			// CR 702.140d: a mutated permanent has the top card's abilities
			// PLUS all abilities of the cards beneath it. Walk the FLAT pile
			// list (top face first, then each under-card): the flat index is
			// the identity AbilityPush records and the activation-limit
			// census counts, and a top-face ability keeps exactly its old
			// index. abFace is the face that carries the ability -- an
			// under-card's label and SVar table must be its own, never the
			// pile top's.
			for i, pn := 0, o.PileAbilityCount(); i < pn; i++ {
				pa, okAb := o.PileAbilityAt(i)
				if !okAb {
					continue
				}
				ab := pa.SA
				abFace := o.PileFaceFor(pa.Merged)
				if abFace == nil {
					continue
				}
				if ab.Kind != "AB" {
					continue
				}
				// CR 605.1b: a mana ability is never a loyalty ability, so a
				// loyalty-marked AB$ Mana (Koth's [+1], Ugin, Eye of the
				// Storms' [0]: Add {C}{C}{C}) is NOT exempted here: it is a
				// loyalty ability, offered through this loop under the CR 606.3
				// gates below -- sorcery timing, once per permanent per turn --
				// exactly like every other [+N]/[-N]. The mana-ability path
				// (availableManaAbilitiesUsing) excludes it symmetrically; the
				// exclusion there is what closed the ulalek-eldrazi seed-1019
				// livelock (an un-tapping, gate-free, zero-cost repeatable
				// +3 colourless activation re-offered every priority window).
				if isManaAbilityAPI(ab.API) && !e.isLoyaltyAbility(ab) {
					continue
				}
				if !abilityZoneOK(ab, z) {
					continue
				}
				if ab.Params["SorcerySpeed"] == "True" && !sorcery {
					continue
				}
				// PlayerTurn$ True (Wishclaw Talisman's "Activate only during
				// your turn"): the ability is offered only while its
				// controller is the active player. CR 602.1b would otherwise
				// offer it on any player's priority. ActivationPhases$ and the
				// other window riders (OpponentTurn$, ActivationFirstCombat$,
				// ActivationAfterBlockers$) ride the same shared
				// offer-time gate, so one helper covers the cast and ability
				// halves alike.
				if !e.activationPhasesOK(p, ab) {
					continue
				}
				// ActivationGameTypes$ (activationGameTypesOK, above): a comma
				// list of the formats the ability exists in. In a Constructed
				// game every token list fails closed and the ability is
				// withheld -- one gate here covers both the real-pool offer and
				// the hypothetical walk (offerCastable's hyp variants share
				// this loop body).
				if raw, ok := ab.Params["ActivationGameTypes"]; ok && !activationGameTypesOK(e.format, raw) {
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
				if e.isLoyaltyAbility(ab) {
					if !sorcery {
						continue
					}
					if e.loyaltyActivationsThisTurn(id) >= e.loyaltyAbilityLimit(id) {
						continue
					}
				}
				if abilityRestricted(p, id, ab) {
					continue
				}
				// Activation$ (Sea Gate Wreckage's "Activate only if you have
				// no cards in hand"): the keyword activation condition at offer
				// time, the same funnel the CheckSVar$ gate below applies --
				// a gate you can read must not leave a paid no-op reachable.
				if !e.activationConditionOK(p, ab) {
					continue
				}
				// F05-2 (CR 733.2): a card whose activation aborted with no
				// progress twice in this window is held out here too, exactly
				// like the cast options above -- the suppression is per CARD
				// (abortCast keys it on pc.card, which for an activation is the
				// source), and the abort sites cover "cast/activation" alike.
				// Without this check an activation the payer cannot complete
				// (a mis-answered phyrexian pip, a pool that moved) re-offered
				// forever inside one priority window: measured, the commander
				// bench spun 20000 intents on Solphim's {1}{R/P}{R/P} ability
				// (seed 1295, 2026-09-15) because the ability-offer path never
				// read the map the abort wrote.
				if e.castSuppressed(p, id) {
					continue
				}
				if e.activationLimitBlocked(p, id, ab, i, "", pa.Merged) {
					continue
				}
				// kw:Boast (CR 702.142): a Boast ability (Forge's `Boast$ True`
				// parameter on the AB, not a K: keyword line) may be activated
				// only if the source creature attacked this turn, and only once
				// each turn. The once-per-turn half folds into the same
				// activation-event scan the ActivationLimit$ gate uses.
				if strings.EqualFold(strings.TrimSpace(ab.Params["Boast"]), "True") && !e.boastGateOK(id, i, "") {
					continue
				}
				cost := e.parseCost(ab.Params["Cost"])
				// The ability's own ReduceCost$ (Otawara's Channel): the CR
				// 601.2f composition the offer gate and beginActivation's
				// charge share, so an offered cost and the paid one agree.
				if n := e.ownReduceCost(p, id, ab, nil, pa.Merged); n > 0 && cost.Generic >= n {
					cost.Generic -= n
				} else if n > 0 {
					cost.Generic = 0
				}
				if cost.Tap && (o.Tapped || (z == state.ZBattlefield && o.SummonSick && slices.Contains(e.Derived(id).Types, "Creature") && !e.HasKeyword(id, "Haste"))) {
					continue
				}
				if !offerCastable(p, id, cost, abilityScope(ab), true) {
					continue
				}
				if !e.abilityTargetsAvailable(p, id, ab) {
					continue
				}
				// CR 603.2's intervening-if at activation: an ability whose
				// CheckSVar$ fails is not offered (Bloodsoaked Champion's Raid —
				// "Activate only if you attacked this turn"). Offer time, not
				// resolve time: the resolution runs the effect's own
				// Condition* gate (conditionMet) where the SA carries one; an
				// offered-but-gated activation that resolves into nothing would
				// be a paid no-op the offer loop could have withheld.
				if !e.sVarGateOK(p, id, ab, pa.Merged) {
					continue
				}
				// IsPresent$/PresentCompare$ (Mistveil Plains' "Activate only if
				// you control two or more white permanents"): the same offer-time
				// gate funnel as the CheckSVar$ read above.
				if !e.abilityPresentHolds(p, id, ab) {
					continue
				}
				out = append(out, decision.Option{Index: len(out), Kind: "ability",
					Label: abFace.Name + ": " + ab.Params["SpellDescription"], Obj: id, Ability: i,
					Grant: e.abilityGrant(id, ab)})
			}
		}
	}

	// Granted activated abilities (CR 613.1f, rules/legal.go's
	// grantedAbilities): the abilities an AddAbilities continuous grant -- a
	// Saga chapter's Animate, "CARDNAME gains '{T}: Add {C}'." -- gives a
	// permanent. Non-mana ones are offered here with the SVar anchor
	// beginActivation resolves (the same anchor the max-speed "granted"
	// option carries); mana ones flow through availableManaAbilities below so
	// the "Tap for mana" priority action and the payment window share one
	// member set. Gates mirror the printed loop above minus the loyalty gate
	// (a grant is never a loyalty ability). The two activation limits are
	// checked here too, with the SVar-name identity (see the gate's own
	// comment below): Touch of Vitae carries GameActivationLimit$ 1 on an
	// Animate-delivered AddAbility$ body.
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || e.faceDownPrintedHides(o) {
			// A face-down permanent is not offered granted abilities: the
			// offer label reads the printed face name, which CR 708.8 says
			// does not exist while face down.
			continue
		}
		for _, ga := range e.grantedAbilities(p, id) {
			ab := ga.sa
			// isManaAbilityAPI, not a bare "Mana" check: a granted
			// ManaReflected flows through availableManaAbilities too (its
			// IsPresent$ gate lives in manaReflectedPresentHolds, which knows
			// the hasAbility Activated.otherAbility special form).
			if isManaAbilityAPI(ab.API) {
				continue
			}
			if ab.Params["SorcerySpeed"] == "True" && !sorcery {
				continue
			}
			if abilityRestricted(p, id, ab) {
				continue
			}
			cost := e.parseCost(ab.Params["Cost"])
			// The granted twin of the printed loop's own ReduceCost$ fold.
			if n := e.ownReduceCost(p, id, ab, nil, 0); n > 0 && cost.Generic >= n {
				cost.Generic -= n
			} else if n > 0 {
				cost.Generic = 0
			}
			if cost.Tap && (o.Tapped || (o.SummonSick && slices.Contains(e.Derived(id).Types, "Creature") && !e.HasKeyword(id, "Haste"))) {
				continue
			}
			if !offerCastable(p, id, cost, abilityScope(ab), true) {
				continue
			}
			if !e.abilityTargetsAvailable(p, id, ab) {
				continue
			}
			// IsPresent$/PresentCompare$: the same offer-time gate the printed
			// loop applies, so a granted ability and its printed twin share one
			// eligibility set.
			if !e.abilityPresentHolds(p, id, ab) {
				continue
			}
			// kw:Boast (CR 702.142): the granted twin of the printed loop's
			// Boast gate. The identity is the SVar name the grant anchored on,
			// because beginGrantedActivation mints a DelayedPush rather than an
			// AbilityPush (boastGateOK reads both).
			if strings.EqualFold(strings.TrimSpace(ab.Params["Boast"]), "True") && !e.boastGateOK(id, -1, ga.svar) {
				continue
			}
			// The two activation limits, for a GRANTED ability: the same shared
			// gate the printed loop above calls, with the SVar-name identity
			// because the mint is a DelayedPush/GrantAbilityPush. The printed
			// loop's old claim -- "no corpus granted ability carries a limit" --
			// is FALSE: Touch of Vitae carries GameActivationLimit$ 1 on the
			// AddAbility$ body it animates onto a target. (That specific grant
			// does not resolve yet for an unrelated reason -- its SVar lives on
			// the Instant's face, while Animate resolves granted names off the
			// ANIMATED object's table; see the report's Issues.) The gate is kept
			// so a granted ability with a limit is never re-offered once used,
			// and self-animate grants -- where the SVar table IS the recipient's
			// -- are pinned by TestGameActivationLimitGrantedAbilityWithheldAfterOneUse.
			if e.activationLimitBlocked(p, id, ab, -1, ga.svar, 0) {
				continue
			}
			out = append(out, decision.Option{Index: len(out), Kind: "ability",
				Label: o.Face().Name + ": " + ab.Params["SpellDescription"], Obj: id, SVar: ga.svar,
				GrantSource: ga.source})
		}
	}

	// kw:Station (CR 702.150, rules/station.go): each Spacecraft the player
	// controls may be stationed as a sorcery by tapping another creature.
	// The offer is gated on the sorcery window (Station only as a sorcery)
	// and on a legal tap candidate existing, so an unpayable station is
	// never offered; the tap candidate itself is the KChoose askStation
	// poses after the option is chosen.
	if sorcery {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil || !e.HasKeyword(id, "Station") {
				continue
			}
			if len(e.stationCandidates(p, id)) == 0 {
				continue
			}
			add("station", "Station "+o.Face().Name, id)
		}
		// Room unlock (CR 309.5, rules/rooms.go): a room whose second door is
		// still locked may be unlocked as a sorcery by paying that half's own
		// mana cost. The gate is the same castable total the cast options
		// use, so an unpayable unlock is never offered (and the payment on
		// the answer cannot disagree with the offer).
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil || e.faceDownPrintedHides(o) {
				continue
			}
			cost, ok := e.unlockRoomCost(o)
			if !ok {
				continue
			}
			if offerCastable(p, id, cost, costScope{kind: "Ability"}, true) {
				add("unlock", "Unlock "+roomLockedFace(o).Name, id)
			}
		}
	}

	// kw:Start your engines (CR 702.163c, rules/speed.go): a max-speed
	// static grants its AddAbility$ while its controller has speed 4, and
	// the granted ability is offered through the same cost/target gates
	// every other activation uses. NOT sorcery-gated: the grant is an
	// ordinary activated ability (Amonkhet Raceway's {T}: pump) whose timing
	// is its own cost's -- it needs a priority window, not a main phase, so
	// the offer sits outside the sorcery block with the other activation
	// offers.
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		for _, ab := range e.maxSpeedAbilities(p, id) {
			if abilityRestricted(p, id, ab) {
				continue
			}
			cost := e.parseCost(ab.Params["Cost"])
			if cost.Tap && (o.Tapped || (o.SummonSick && slices.Contains(e.Derived(id).Types, "Creature") && !e.HasKeyword(id, "Haste"))) {
				continue
			}
			if !offerCastable(p, id, cost, abilityScope(ab), true) {
				continue
			}
			if !e.abilityTargetsAvailable(p, id, ab) {
				continue
			}
			// IsPresent$/PresentCompare$: the same offer-time gate the printed
			// loop applies, so a max-speed grant and its printed twin share one
			// eligibility set.
			if !e.abilityPresentHolds(p, id, ab) {
				continue
			}
			// kw:Boast (CR 702.142): the max-speed grant is a third offer site
			// for an SVar-anchored ability, so it shares the Boast gate. The
			// identity is the SVar name beginGrantedActivation mints its
			// DelayedPush with (abSVarName), never a face index.
			sv := abSVarName(o.Face(), ab)
			if strings.EqualFold(strings.TrimSpace(ab.Params["Boast"]), "True") && !e.boastGateOK(id, -1, sv) {
				continue
			}
			out = append(out, decision.Option{Index: len(out), Kind: "granted",
				Label: o.Face().Name + ": " + ab.Params["SpellDescription"],
				Obj:   id, SVar: sv})
		}
	}
	// K:Split second (CR 702.62, rules/split_second.go): while a split-second
	// spell is on the stack, players can't cast spells or activate abilities
	// that aren't mana abilities. The filter runs here -- at the ONE choke
	// point every cast source (hand, command zone, may-play, flashback/
	// aftermath/harmonize/warp/escape, exile recasts) and both ability loops
	// flow through -- rather than at each of the ~30 append sites, so the next
	// cast source added to this walk is covered by construction. Playing a
	// land, mana abilities, Station and Room unlock stay legal; the Suspend
	// and Foretell offers ride the "cast" Kind but are special actions, not
	// spell casts, so they stay too.
	if e.splitSecondHolds() {
		out = e.filterSplitSecondActions(out)
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
		// chooseETB, record it with a Choose event, then continueCast's
		// payCast moves the land and logs the play. A land with none keeps
		// the original direct path (no pendingCast, no flow), so ordinary
		// lands are untouched. Both paths share the same continuation
		// machinery: etbAnswer/continueCast/commitCast below, never a
		// parallel one.
		//
		// The source zone is the object's CURRENT zone, not hardcoded to the
		// hand: since the MayPlay grants (rules/mayplay.go) the play_land
		// offer also comes from the graveyard, exile or the top of the
		// library, a hand land and a graveyard land must move From the zone
		// they were actually offered from (CR 118.3a -- the permission names
		// the zone it grants). A land that somehow left its zone between the
		// offer and the answer resolves from whatever zone it is in, and the
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

	case "station":
		// kw:Station (CR 702.150, rules/station.go): the spacecraft is
		// stationed by tapping another creature the KChoose below names. The
		// pass-count reset matches every other non-pass action.
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		e.askStation(in.Player, opt)

	case "unlock":
		// Room unlock (CR 309.5, rules/rooms.go): pay the locked half's mana
		// cost and emit the DoorUnlock event. The offer gated on castable,
		// so the payment here cannot disagree with the offer; a stale option
		// (the room left play or was unlocked between offer and answer -- the
		// same seat's answer, so the board cannot have moved) degrades to a
		// no-op through unlockRoomCost's nil face.
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		o := e.G.Obj(opt.Obj)
		cost, ok := e.unlockRoomCost(o)
		if !ok {
			return
		}
		if !e.payMana(in.Player, cost) {
			return
		}
		e.emit(events.Event{Kind: events.DoorUnlock, Obj: opt.Obj})

	case "granted":
		// kw:Start your engines (CR 702.163c, rules/speed.go): a max-speed
		// static's granted ability, activated through the ordinary cost
		// payment and the delayed-shape ability mint.
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Priority, Amount: 0})
		e.beginGrantedActivation(in.Player, opt)

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
