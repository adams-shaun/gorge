package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// chooseMana is the one-off pick among a source's distinct mana abilities.
// The existing chooseFor values occupy 0 through 5 (cast/target/miracle/etc.),
// 6 through 9 are this file's own mana windows, 110-14 the mainline
// opening/suspend/station flows, and the unless window sits above chooseUnlock.
const (
	chooseMana chooseFor = iota + 6
	chooseManaColor
	chooseManaDiscard
	chooseManaExile
)

const (
	chooseManaUnless chooseFor = iota + 17
	chooseUnlessCost
)

// manaActivation is the one outstanding choice among a permanent's distinct
// mana abilities. It is plain data so Clone can preserve a choice at an
// intent boundary. cast says the answer resumes CR 601.2g payment rather
// than the ordinary priority round.
type manaActivation struct {
	player     state.PlayerID
	source     state.ObjID
	abilities  []*cards.SA
	cast       bool
	cumulative bool
}

// manaColorActivation holds an already-paid mana ability while its controller
// chooses the colour that Produced$ Any (or Combo Any) will add. triggers is
// the CR 605.3b triggered-mana batch still to resolve after the answer. When
// trigger is non-nil the choice belongs to that triggered mana ability instead
// (ability is then the Mana sub-ability in its chain, and player the player
// receiving the mana).
type manaColorActivation struct {
	player     state.PlayerID
	source     state.ObjID
	ability    *cards.SA
	cast       bool
	cumulative bool
	triggers   []pendingTrigger
	trigger    *pendingTrigger
}

// manaDiscardActivation holds a synchronous mana ability while its discard
// cost is chosen. It is separate from pendingCast so activating mana during a
// spell's CR 601.2g payment window never overwrites the outer cast flow.
type manaDiscardActivation struct {
	player     state.PlayerID
	source     state.ObjID
	ability    *cards.SA
	cost       Cost
	sacs       []state.ObjID
	discards   []state.ObjID
	exiles     []state.ObjID
	part       int
	exilePart  int
	cast       bool
	cumulative bool
}

// manaUnlessActivation parks an off-stack mana ability while its payer
// answers its UnlessCost$. The ordinary effects resume path needs a stack
// object, so this flow records the same decision and uses the same payment
// helper without pretending a mana ability is on the stack.
type manaUnlessActivation struct {
	player     state.PlayerID
	source     state.ObjID
	ability    *cards.SA
	cast       bool
	cumulative bool
	triggers   []pendingTrigger
	sacs       []state.ObjID
	payers     []state.PlayerID
	next       int
}

// isManaAbilityAPI reports the two supported activated mana ability APIs.
func isManaAbilityAPI(api string) bool { return api == "Mana" || api == "ManaReflected" }

// availableManaAbilities returns exactly the individual mana abilities that
// p may activate from id now. Keeping the CantBeActivated gate here makes the
// priority action, payment window, and the eventual chosen activation share
// one member-by-member eligibility set.
// manaReflectedPresentHolds evaluates an activated ManaReflected ability's
// IsPresent$/PresentCompare$ activation gate. Most shapes use the shared
// deterministic battlefield count. hasAbility Activated.otherAbility is a
// property of the subject's face, so it is handled structurally here: a
// native mana ability excludes itself, while a static-granted SVar (Tazri)
// requires one printed activated ability on that creature.
func (e *Engine) manaReflectedPresentHolds(p state.PlayerID, source state.ObjID, ma *cards.SA) bool {
	spec, ok := ma.Params["IsPresent"]
	if !ok || strings.TrimSpace(spec) == "" {
		return true
	}
	if strings.Contains(spec, "hasAbility Activated.otherAbility") {
		o := e.G.Obj(source)
		if o == nil || o.Face() == nil || !effects.MatchesSpecFrom(e.G,
			strings.TrimSpace(strings.Split(spec, "+hasAbility Activated.otherAbility")[0]), source, p, source) {
			return false
		}
		n := 0
		for _, ab := range o.Face().Abilities {
			if ab.Kind == "AB" && ab != ma {
				n++
			}
		}
		if cmp := strings.TrimSpace(ma.Params["PresentCompare"]); cmp != "" {
			return comparePresent(n, cmp)
		}
		return n > 0
	}
	n := e.countPresent(spec, source, p)
	if cmp := strings.TrimSpace(ma.Params["PresentCompare"]); cmp != "" {
		return comparePresent(n, cmp)
	}
	return n > 0
}

func (e *Engine) availableManaAbilities(p state.PlayerID, id state.ObjID) []*cards.SA {
	return e.availableManaAbilitiesUsing(nil, p, id)
}

// A non-nil source belongs only to the caller's current legalActions pass.
// The ordinary wrapper deliberately supplies nil so payment windows and
// activation rechecks discover fresh static membership.
func (e *Engine) availableManaAbilitiesUsing(statics *actionStaticSource, p state.PlayerID, id state.ObjID) []*cards.SA {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return nil
	}
	f := o.Face()
	// CR 708.8: a face-down permanent's printed mana abilities do not exist
	// while it is face down. Its ONE exception is CR 305.6: a permanent
	// whose set type includes a basic land subtype has that land's intrinsic
	// mana ability, so Yedora's face-down Forest land taps for {G}. The set
	// type comes from a ChangeZone FaceDownSetType$ (Object.FaceDownTypeWords);
	// a plain CR 708.5 face-down 2/2 has no basic-land set type and
	// contributes nothing.
	faceDown := e.faceDownPrintedHides(o)
	var manaAbilities []*cards.SA
	switch {
	case faceDown:
		for _, w := range o.FaceDownTypeWords() {
			if ab, ok := cards.IntrinsicManaAbility(w); ok {
				manaAbilities = append(manaAbilities, ab)
			}
		}
	case len(o.MergedCards) == 0:
		// The overwhelmingly common case: a permanent that is not a mutated
		// pile has exactly one face, so its own ManaAbilities slice IS the
		// walk -- no second collection, which is what keeps this walk at its
		// pre-mutate allocation cost (internal/searchprobe's Capture budget
		// runs legalActions over every object on every pass).
		manaAbilities = f.ManaAbilities()
	default:
		// CR 702.140d: a mutated pile's under-card mana abilities are live
		// too. Only the pile pays for the flattening.
		for i := 0; i < o.PileFaceCount(); i++ {
			pf, ok := o.PileFaceAt(i)
			if !ok {
				continue
			}
			manaAbilities = append(manaAbilities, pf.Face.ManaAbilities()...)
		}
	}
	recipientCtx := &effects.Ctx{Source: id, Controller: p, SVars: f.SVars}
	abilityRestricted := func(ma *cards.SA) bool {
		if statics == nil {
			return e.abilityRestricted(p, id, ma)
		}
		return e.abilityRestrictedUsing(statics.get().cantActivate, p, id, ma)
	}
	var out []*cards.SA
	for _, ma := range manaAbilities {
		// CR 605.1b: an activated ability is a mana ability only when it is
		// NOT a loyalty ability. A planeswalker's mana-producing loyalty
		// ability (Koth's [+1], Ugin, Eye of the Storms' [0]: Add {C}{C}{C},
		// 12+ corpus cards) must never enter this path: the mana path taps
		// nothing, poses no CR 606.3 gate, and a zero-loyalty cost is free --
		// the measured ulalek-eldrazi seed-1019 livelock re-tapped Ugin's
		// [0] once per intent, +3 colourless per activation, forever. The
		// ability offer (rules/legal.go) owns these abilities with the full
		// CR 606.3 gates (sorcery timing, once per permanent per turn).
		if e.isLoyaltyAbility(ma) {
			continue
		}
		// Activation$ (Mox Opal's "Activate only if you control three or more
		// artifacts"): the same keyword-condition gate the printed-ability
		// offer loop in rules/legal.go applies, so the priority action, the
		// payment window and the chosen activation share one member set.
		if abilityZoneOK(ma, o.Zone) && e.activationConditionOK(p, ma) && e.manaActivationGateHolds(p, id, ma) &&
			!abilityRestricted(ma) && e.manaAbilityPayable(p, id, ma) {
			// ActivationLimit$ / GameActivationLimit$ (Vivi Ornitier's "only once
			// each turn", Stalking Leonin's "Activate only once"): the non-mana
			// ability offer loops in legal.go gate on these parameters, but this
			// walk is a mana ability's ONLY eligibility gate -- offer, payment
			// window and chosen activation all go through it -- so a mana ability
			// carrying either limit stayed repeatable without bound, and a
			// controller whose bot policy prefers activating mana over passing
			// looped on it forever (a zero-production source whose use never
			// advances any cast). Both limits are checked through the one shared
			// gate, with the printed identity (flat pile index, no SVar).
			if _, limited := ma.Params["ActivationLimit"]; limited || ma.Params["GameActivationLimit"] != "" {
				idx, merged, found := pileAbilityRefOf(o, ma)
				if found && e.activationLimitBlocked(p, id, ma, idx, "", merged) {
					continue
				}
			}
			out = append(out, ma)
		}
	}
	if faceDown {
		// Only the CR 305.6 intrinsics above exist on a face-down permanent:
		// its printed ManaReflected abilities and any Continuous AddAbility$
		// grant are hidden with the rest of its printed face (CR 708.8).
		return out
	}
	// CR 605.2a: a mana ability functions only while its source object is in
	// the zone its ActivationZone$ names -- the battlefield when printed
	// none is. The plain AB$ Mana branch above gates on abilityZoneOK; this
	// closure must agree, or a reflected land in hand/graveyard is offered
	// (and activatable) wherever an opponent's land exists -- Exotic Orchard
	// reporting an "Activate ... for mana" action for the card IN HAND.
	considerReflected := func(ma *cards.SA, ctx *effects.Ctx) {
		if ma.Kind != "AB" || ma.API != "ManaReflected" || !abilityZoneOK(ma, o.Zone) || abilityRestricted(ma) || !e.manaAbilityPayable(p, id, ma) || !e.manaReflectedPresentHolds(p, id, ma) {
			return
		}
		if len(effects.ManaReflectedCandidates(e, ctx, ma)) > 0 {
			out = append(out, ma)
		}
	}
	// A ManaReflected ability may sit on the top face or any under-card; each
	// resolves its own face's table.
	for i := 0; i < o.PileFaceCount(); i++ {
		pf, ok := o.PileFaceAt(i)
		if !ok {
			continue
		}
		// The per-face Ctx is minted only when the face actually prints a
		// ManaReflected ability (measured: no repo-deck card does), so an
		// ordinary permanent's offer pass allocates nothing here.
		var faceCtx *effects.Ctx
		for _, ma := range pf.Face.Abilities {
			if ma.API != "ManaReflected" {
				continue
			}
			if faceCtx == nil {
				faceCtx = &effects.Ctx{Source: id, Controller: p, SVars: pf.Face.SVars}
			}
			considerReflected(ma, faceCtx)
		}
	}
	// A Continuous static may grant an activated ability through AddAbility$.
	// Resolve its named SVar from the static's source but activate it from id:
	// Tazri's ManaReflected reads the recipient creature's colours and its own
	// "another activated ability" condition, not Tazri's. This direct scan is
	// the mana path's membership AND ORDER source for printed Continuous
	// statics: it follows collectActionStatics' seat/zone/static walk (the
	// snapshot a legalActions pass shares), which
	// TestActionStaticMembershipPreservesOrderAndActiveFace pins. The
	// grantedAbilities loop below adds only grants this scan did not already
	// produce -- an Animate's Abilities$ member such as Wrenn and One's
	// "{T}: Add {G}" -- so a printed AddAbility$ is never offered twice (the
	// duplicate-offer trap: staticEffects now emits it into AddAbilities too).
	var continuous []staticView
	if statics == nil {
		continuous = e.activeStatics("Continuous")
	} else {
		continuous = statics.get().continuous
	}
	printed := make(map[string]bool)
	for _, sv := range continuous {
		name := strings.TrimSpace(sv.Params["AddAbility"])
		if name == "" || !effects.MatchesSpecCtx(e.G, sv.Params["Affected"], id, e.specCtx(sv.Source, sv.Controller)) {
			continue
		}
		source := e.G.Obj(sv.Source)
		if source == nil || source.Face() == nil {
			continue
		}
		ma := cards.ResolveSVar(source.Face().SVars, name)
		if ma == nil || ma.Kind != "AB" {
			continue
		}
		printed[ma.Line] = true
		if ma.API == "ManaReflected" {
			considerReflected(ma, recipientCtx)
			continue
		}
		if ma.API == "Mana" && !e.isLoyaltyAbility(ma) && abilityZoneOK(ma, o.Zone) && !abilityRestricted(ma) && e.manaAbilityPayable(p, id, ma) &&
			e.manaActivationGateHolds(p, id, ma) {
			out = append(out, ma)
		}
	}
	// Granted mana abilities (CR 613.1f, rules/legal.go's grantedAbilities):
	// an AddAbilities grant's AB$ Mana/ManaReflected member -- a Saga
	// chapter's "gains '{T}: Add {C}'.", an Animate's Abilities$ member (Wrenn
	// and One) -- is a real mana ability with the same eligibility gates, so
	// the priority offer, the CR 601.2g payment window and the activation all
	// see exactly one member set. A member the printed scan above already
	// produced is skipped (it anchors the same activation); a granted
	// ManaReflected member goes through the same candidate/present gates as a
	// printed one.
	for _, ga := range e.grantedAbilities(p, id) {
		if printed[ga.sa.Line] {
			continue
		}
		if ga.sa.API == "ManaReflected" {
			considerReflected(ga.sa, recipientCtx)
			continue
		}
		if ga.sa.API != "Mana" || e.isLoyaltyAbility(ga.sa) {
			continue
		}
		if !abilityZoneOK(ga.sa, o.Zone) || abilityRestricted(ga.sa) || !e.manaAbilityPayable(p, id, ga.sa) ||
			!e.manaActivationGateHolds(p, id, ga.sa) {
			continue
		}
		out = append(out, ga.sa)
	}
	return out
}

// manaActivationGateHolds evaluates a plain AB$ Mana ability's IsPresent$/
// PresentCompare$ existence gate (the same shape manaReflectedPresentHolds is
// for a reflected ability):
//
//   - IsPresent$ <spec> with PresentCompare$ <op><n>: the count of objects
//     matching <spec> (Shrine of the Forsaken Gods' "Activate only if you
//     control seven or more lands"). PresentCompare$ absent means GE1.
//
// An Activation$ <mechanic> rides rules/legal.go's shared
// activationConditionOK instead (main's vocabulary: Hellbent, Threshold,
// Metalcraft, Delirium), so the two gates compose rather than duplicate.
//
// A gate this build cannot price fails closed: the ability is withheld from
// the offer, the payment window and the activation alike, never widened.
func (e *Engine) manaActivationGateHolds(p state.PlayerID, id state.ObjID, ma *cards.SA) bool {
	// ActivationPhases$ and its rider qualifiers (PlayerTurn$,
	// OpponentTurn$, ActivationFirstCombat$, ActivationAfterBlockers$) are
	// the same offer-time window a non-mana ability is gated by. This walk
	// is a mana ability's ONLY eligibility gate, so reading the window here
	// is what keeps one AB$ Mana carrier (a charge-counter source whose
	// "any player may activate ... only during their turn before the end
	// step" line was previously offered outside its window) bound to it.
	if !e.activationPhasesOK(p, ma) {
		return false
	}
	if spec, ok := ma.Params["IsPresent"]; ok && strings.TrimSpace(spec) != "" {
		n := e.countPresent(strings.TrimSpace(spec), id, p)
		if cmp := strings.TrimSpace(ma.Params["PresentCompare"]); cmp != "" {
			if !comparePresent(n, cmp) {
				return false
			}
		} else if n <= 0 {
			return false
		}
	}
	return true
}

// activateMana activates one of source's currently available mana abilities
// at PRIORITY -- the priority window's activate option (rules/legal.go),
// where the activating player holds priority. A singleton retains the old
// no-extra-decision path. Several abilities are distinct activated abilities
// sharing one tap cost, so their controller must choose one before the
// source is tapped.

func (e *Engine) activateMana(p state.PlayerID, source state.ObjID, cast bool) {
	e.activateManaFor(p, source, cast, false, true)
}

// activateManaPayment is the payment-window form: the payer is paying a cost
// (the CR 601.2g cast window, a ward payment) rather than acting on
// priority, so "Activate only as an instant" mana abilities are withheld.
func (e *Engine) activateManaPayment(p state.PlayerID, source state.ObjID, cast bool) {
	e.activateManaFor(p, source, cast, false, false)
}

// activatePaymentMana opens the mana-ability-only window used while a
// cumulative-upkeep or triggered-Untap cost is being paid.
func (e *Engine) activatePaymentMana(p state.PlayerID, source state.ObjID) {
	e.activateManaFor(p, source, false, true, false)
}

// instantSpeedOnly reports whether a mana ability's InstantSpeed$ True
// timing restriction is present ("Activate only as an instant", Lion's Eye
// Diamond): the ability is activatable exactly when its controller holds
// priority. The engine activates mana abilities in exactly two contexts: a
// priority window (the "activate for mana" action) and a payment window
// (paying for a spell, a ward or a cumulative-upkeep cost). CR 605.4 lets a
// player activate mana abilities while paying a cost only as far as the
// ability's own rules permit, and the card's text bars everything but a
// priority moment -- so the ability is activatable at priority and never
// inside a payment window.
func (e *Engine) instantSpeedOnly(ma *cards.SA) bool {
	return strings.EqualFold(strings.TrimSpace(ma.Params["InstantSpeed"]), "True")
}

// availableManaAbilitiesForWindow is the member set for one window: the
// ordinary priority set, or the payment-window set with the InstantSpeed$
// timing-restricted abilities withheld.
func (e *Engine) availableManaAbilitiesForWindow(p state.PlayerID, id state.ObjID, atPriority bool) []*cards.SA {
	all := e.availableManaAbilities(p, id)
	if atPriority {
		return all
	}
	out := make([]*cards.SA, 0, len(all))
	for _, ma := range all {
		if e.instantSpeedOnly(ma) {
			continue
		}
		out = append(out, ma)
	}
	return out
}

func (e *Engine) activateManaFor(p state.PlayerID, source state.ObjID, cast, cumulative, atPriority bool) {
	abilities := e.availableManaAbilitiesForWindow(p, source, atPriority)
	if len(abilities) == 0 {
		return
	}
	if len(abilities) == 1 {
		e.resolveManaAbility(p, source, abilities[0], cast, cumulative)
		if cumulative && e.choosing == chooseNone {
			e.paymentWindowAsk()
		}
		return
	}
	o := e.G.Obj(source)
	d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Choose a mana ability of " + o.Face().Name, Source: source}
	chosen := e.chosenProducedColour(source)
	for i, ma := range abilities {
		// An explicit multi-colour "Produced$ Combo <colours>" ability is
		// flattened into one option per colour (task fb-20260917T232800Z):
		// the player asked for per-colour pip bubbles, not the prose "Add B
		// or R" plus a second stage-2 colour ask. Each flattened option
		// carries the ability's own index, so the answer resolves that
		// ability with its Produced$ rewritten to the chosen colour and the
		// cost is paid once with no follow-up. Only plain AB$ Mana abilities
		// flatten -- a ManaReflected ability's colours come from what other
		// sources produce, never from its own Produced$ token. The colour
		// order is the ability's own token order, the same order askManaColor
		// offers today, so the two cannot disagree. Any / Combo Any / Chosen
		// keep the single option + stage-2 ask (the choice there is not
		// enumerable at option-build time).
		if cols, ok := manaAbilityComboColours(ma, chosen); ok {
			for _, col := range cols {
				d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "mana", Obj: source,
					Ability: i, Label: "Add " + col})
			}
			continue
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "mana", Obj: source,
			Ability: i, Label: manaAbilityLabel(ma, chosen)})
	}
	e.manaActivation = &manaActivation{player: p, source: source, abilities: abilities, cast: cast, cumulative: cumulative}
	e.choosing = chooseMana
	e.ask(d)
}

// manaAbilityLabel renders a mana ability's Produced$ value as the label the
// stage-1 "choose a mana ability" wheel shows (task fb-e079def5), with a
// recorded "Chosen" token already substituted (the combo-Chosen family), so
// the wheel shows "Add R", never the raw "Combo R Chosen" jargon. The raw
// script token ("Combo B R", "Any") is engine jargon a player cannot read,
// and it defeats the client's mana-pip styling for the whole wheel (the web
// tints an option list only when EVERY label is a single "Add <C>"), so a
// Talisman-of-Indulgence-shaped source rendered as plain grey text. The
// sibling ask sites (askManaColor, askTriggeredManaColor, the replacement
// colour ask in replacement.go) already emit resolved single colours -- this
// was the one label site that leaked the raw Produced string. Combo
// <colours> reads "Add B or R" (three or more: comma-separated, the last
// joined with "or"); Any/Combo Any read "Add any color" (the oracle's own
// wording, CR 107.4); Produced$ Chosen reads "Add chosen color"; anything
// else -- a plain single colour or C (the shape the wheel tints), a doubled
// "RR", a Special expression -- keeps the bare "Add <value>" shape.
// manaAbilityComboColours reports whether the ability's own Produced$ is an
// explicit MULTI-colour combo ("Combo B R") and returns the colour list in
// the ability's own token order -- the same order askManaColor offers, so the
// flattened stage-1 wheel and the stage-2 ask cannot disagree. A trailing
// "Chosen" token is first substituted from the recorded as-enters colour
// (the Thriving-lands/gate family, substituteChosenProduced). Both call
// sites guard on ma.API == "Mana" before calling (a ManaReflected ability's
// colours come from what other sources produce, never its own Produced$
// token), so this helper's Produced$ read is Mana-attributed
// (apiSpecificRulesSA) rather than joining the generic rules union.
func manaAbilityComboColours(ma *cards.SA, chosen string) ([]string, bool) {
	if ma == nil || ma.API != "Mana" {
		return nil, false
	}
	produced := substituteChosenProduced(strings.TrimSpace(ma.Params["Produced"]), chosen)
	cols, ok := effects.ComboColours(produced)
	if !ok || len(cols) <= 1 {
		return nil, false
	}
	return cols, true
}

func manaAbilityLabel(ma *cards.SA, chosen string) string {
	produced := substituteChosenProduced(strings.TrimSpace(ma.Params["Produced"]), chosen)
	switch produced {
	case "Any", "Combo Any":
		return "Add any color"
	case "Chosen":
		return "Add chosen color"
	}
	if cols, ok := effects.ComboColours(produced); ok {
		if len(cols) == 1 {
			return "Add " + cols[0]
		}
		last := len(cols) - 1
		return "Add " + strings.Join(cols[:last], ", ") + " or " + cols[last]
	}
	return "Add " + produced
}

// manaAbilityPayable is the mana-ability equivalent of the cast cost gate.
// A source with a sacrifice cost is not offered unless this synchronous path
// can pay it without a chooser. Discard costs have their own continuation:
// ordinary discard asks, while random and discard-your-hand do not.
func (e *Engine) manaAbilityPayable(p state.PlayerID, source state.ObjID, ma *cards.SA) bool {
	return e.manaAbilityPayablePool(p, source, ma, nil)
}

// manaAbilityPayablePool is manaAbilityPayable with the mana part priced
// against an optional explicit pool: hyp nil keeps the ordinary real-pool
// gate (the restriction-adjusted manaAvailableFor the offer walk uses), hyp
// non-nil prices the activation against the potential-action walk's growing
// hypothetical bound (rules/potential.go PotentialMana), which is what lets
// a source's paid activation be reached after the seat floats mana from a
// cheaper source first. Every non-mana read -- tap state, sacrifice,
// discard and exile candidates, the announced-part refusals -- is real in
// both modes: hypothetical mana never satisfies a sacrifice.
func (e *Engine) manaAbilityPayablePool(p state.PlayerID, source state.ObjID, ma *cards.SA, hyp *state.Mana) bool {
	o := e.G.Obj(source)
	if o == nil || o.Face() == nil {
		return false
	}
	cost := e.parseCost(ma.Params["Cost"])
	av := e.manaAvailableFor(p, source, true)
	pool := av.pool
	typed := av.typed
	if hyp != nil {
		pool = *hyp
		// A hypothetical bound is a pure mana bound that may include
		// restricted units, so its typed partition is the raw tally (the
		// typed counts never affect payability anyway).
		typed = e.G.Players[p].TypedMana
	}
	if cost.X != 0 || len(cost.Reveal) > 0 || len(cost.Behold) > 0 || len(cost.TapPermanent) > 0 ||
		len(cost.Blight) > 0 || cost.Forage || (cost.Tap && o.Tapped) || !e.costPayablePool(p, source, true, cost, pool, typed) {
		return false
	}
	// The mana-activation path has no X ask and no mid-payment suspension, so
	// an announced PayLife<X> or SubCounter<X/Kind> component could never be
	// settled here: the ability is not offered rather than paid for free.
	if len(cost.LifeX) > 0 {
		return false
	}
	for _, part := range cost.SubCounter {
		if part.Announced {
			return false
		}
		if o.Counter(part.Spec) < part.N {
			return false
		}
	}
	if _, ok := e.manaSacrifices(p, source, cost); !ok {
		return false
	}
	if _, ok := e.manaDiscards(p, source, cost); !ok {
		return false
	}
	_, ok := e.manaExiles(p, source, cost)
	return ok
}

// manaSacrifices finds the forced sacrifice for each cost part. An exact
// candidate count means no player choice is being hidden: each candidate is
// paid in deterministic battlefield order. More candidates than required are
// intentionally not offered by manaAbilityPayable until a KChoose continuation
// can collect that cost choice.
func (e *Engine) manaSacrifices(p state.PlayerID, source state.ObjID, cost Cost) ([]state.ObjID, bool) {
	var sacs []state.ObjID
	reserved := map[state.ObjID]bool{}
	for _, part := range cost.Sac {
		var candidates []state.ObjID
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if e.SacrificeBlocked(id, true) {
				continue
			}
			if !reserved[id] && effects.MatchesSpecFrom(e.G, part.Spec, id, p, source) {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) != int(part.N) {
			return nil, false
		}
		for _, id := range candidates {
			reserved[id] = true
			sacs = append(sacs, id)
		}
	}
	return sacs, true
}

// manaDiscards performs the pure offer-side feasibility walk for a mana
// ability's discard cost. It reserves deterministic candidates but consumes
// no RNG; the payment continuation makes the actual choice.
func (e *Engine) manaExiles(p state.PlayerID, source state.ObjID, cost Cost) ([]state.ObjID, bool) {
	var exiles []state.ObjID
	reserved := map[state.ObjID]bool{}
	for _, part := range cost.Exile {
		zone := part.Zone
		if zone == 0 {
			zone = state.ZHand
		}
		var candidates []state.ObjID
		for _, id := range e.G.Zone(zone, p) {
			if !reserved[id] && effects.MatchesSpecFrom(e.G, part.Spec, id, p, source) {
				candidates = append(candidates, id)
			}
		}
		if part.N <= 0 || int(part.N) > len(candidates) {
			return nil, false
		}
		for i := 0; i < int(part.N); i++ {
			reserved[candidates[i]] = true
			exiles = append(exiles, candidates[i])
		}
	}
	return exiles, true
}

func (e *Engine) manaDiscards(p state.PlayerID, source state.ObjID, cost Cost) ([]state.ObjID, bool) {
	var discards []state.ObjID
	reserved := map[state.ObjID]bool{}
	for _, part := range cost.Discard {
		candidates := e.discardCandidates(p, source, part, false, reserved)
		if strings.EqualFold(part.Spec, "Hand") {
			for _, id := range candidates {
				reserved[id] = true
				discards = append(discards, id)
			}
			continue
		}
		n := int(part.N)
		if n <= 0 || n > len(candidates) {
			return nil, false
		}
		for i := 0; i < n; i++ {
			id := candidates[0]
			reserved[id] = true
			discards = append(discards, id)
			candidates = candidates[1:]
		}
	}
	return discards, true
}

// continueManaDiscard walks a mana ability's discard parts without putting
// the ability on the stack. Ordinary parts ask their controller; Random and
// Hand select internally using the same rules as discardAsk.
func (e *Engine) continueManaDiscard() {
	md := e.manaDiscardActivation
	if md == nil {
		return
	}
	for md.part < len(md.cost.Discard) {
		part := md.cost.Discard[md.part]
		reserved := make(map[state.ObjID]bool, len(md.discards))
		for _, id := range md.discards {
			reserved[id] = true
		}
		candidates := e.discardCandidates(md.player, md.source, part, false, reserved)
		if strings.EqualFold(part.Spec, "Hand") {
			md.discards = append(md.discards, candidates...)
			md.part++
			continue
		}
		n := int(part.N)
		if n <= 0 || n > len(candidates) {
			e.manaDiscardActivation = nil
			e.choosing = chooseNone
			return
		}
		if strings.EqualFold(part.Spec, "Random") {
			for i := 0; i < n; i++ {
				pick := e.Rand(len(candidates))
				md.discards = append(md.discards, candidates[pick])
				candidates = append(candidates[:pick], candidates[pick+1:]...)
			}
			md.part++
			continue
		}
		d := &decision.Decision{Player: md.player, Kind: decision.KChoose, Min: n, Max: n,
			Prompt: "Discard a card to pay the mana ability cost", Source: md.source}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "mana_discard",
				Obj: id, Label: e.G.Obj(id).Face().Name})
		}
		e.choosing = chooseManaDiscard
		e.ask(d)
		return
	}
	for md.exilePart < len(md.cost.Exile) {
		part := md.cost.Exile[md.exilePart]
		zone := part.Zone
		if zone == 0 {
			zone = state.ZHand
		}
		reserved := make(map[state.ObjID]bool, len(md.exiles))
		for _, id := range md.exiles {
			reserved[id] = true
		}
		var candidates []state.ObjID
		for _, id := range e.G.Zone(zone, md.player) {
			if !reserved[id] && effects.MatchesSpecFrom(e.G, part.Spec, id, md.player, md.source) {
				candidates = append(candidates, id)
			}
		}
		n := int(part.N)
		if n <= 0 || n > len(candidates) {
			e.manaDiscardActivation = nil
			e.choosing = chooseNone
			return
		}
		if n == 1 && len(candidates) == 1 && candidates[0] == md.source &&
			strings.EqualFold(part.Spec, "CARDNAME") {
			md.exiles = append(md.exiles, md.source)
			md.exilePart++
			continue
		}
		d := &decision.Decision{Player: md.player, Kind: decision.KChoose, Min: n, Max: n,
			Prompt: "Exile a card to pay the mana ability cost", Source: md.source}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "mana_exile",
				Obj: id, Label: e.G.Obj(id).Face().Name})
		}
		e.choosing = chooseManaExile
		e.ask(d)
		return
	}
	e.commitManaDiscard()
}

func (e *Engine) commitManaDiscard() {
	md := e.manaDiscardActivation
	if md == nil || !e.payManaConvFor(md.player, md.source, true, md.cost, e.paymentConv(md.player, md.source, true)) {
		e.manaDiscardActivation = nil
		e.choosing = chooseNone
		return
	}
	for _, id := range md.discards {
		e.emit(events.DiscardCost(id))
	}
	for _, id := range md.exiles {
		if o := e.G.Obj(id); o != nil {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone,
				To: state.ZExile, Text: "exiled as a mana ability cost"})
		}
	}
	var manaTriggers []pendingTrigger
	if md.cost.Tap {
		manaTriggers = e.emitManaTap(md.player, md.source, md.ability)
	}
	for _, part := range md.cost.SubCounter {
		e.emit(events.Event{Kind: events.CounterChange, Obj: md.source, Counter: part.Spec, Amount: -part.N})
	}
	for _, id := range md.sacs {
		e.emit(events.Sacrifice(id))
	}
	e.manaDiscardActivation = nil
	e.choosing = chooseNone
	e.resolveManaEffect(md.player, md.source, md.ability, md.cast, md.cumulative, manaTriggers, md.sacs)
	if md.cumulative && e.choosing == chooseNone {
		e.paymentWindowAsk()
	}
}

// answerManaDiscard records one ordinary discard part and continues payment.
// It reports whether this mana activation belongs to an outer cast window.
func (e *Engine) answerManaDiscard(chosen []decision.Option) bool {
	md := e.manaDiscardActivation
	if md == nil {
		return false
	}
	for _, opt := range chosen {
		md.discards = append(md.discards, opt.Obj)
	}
	md.part++
	cast := md.cast
	e.continueManaDiscard()
	return cast
}

func (e *Engine) answerManaExile(chosen []decision.Option) bool {
	md := e.manaDiscardActivation
	if md == nil {
		return false
	}
	for _, opt := range chosen {
		md.exiles = append(md.exiles, opt.Obj)
	}
	md.exilePart++
	cast := md.cast
	e.continueManaDiscard()
	return cast
}

// emitManaTap preserves "tapped for mana" as transient engine context while
// trigger matching runs. Tap's existing event payload stays unchanged, so a
// mana activation without a matching trigger keeps its historic event chain.
func (e *Engine) emitManaTap(p state.PlayerID, source state.ObjID, sa *cards.SA) []pendingTrigger {
	// A TapsForMana trigger can restrict the mana's Produced$ value. Keep the
	// activating SA alongside the synchronous tap marker so matching sees the
	// same output declaration that the activation will resolve. This context is
	// rebuilt by replay because replay takes the same activation path.
	produced := ""
	if sa != nil {
		produced = strings.TrimSpace(sa.Params["Produced"])
	}
	before := len(e.pendingTriggers)
	e.tappingForMana, e.tappingManaProduced = source, produced
	e.emitTap(source, p, false)
	e.tappingForMana, e.tappingManaProduced = 0, ""

	// CR 605.3b: a triggered mana ability resolves immediately after the mana
	// ability that caused it, without using the stack. Separate only newly
	// matched triggers from this Tap event; older pending triggers and ordinary
	// tap reactions such as Manabarbs remain in the normal APNAP queue.
	matched := e.pendingTriggers[before:]
	kept := e.pendingTriggers[:before]
	var immediate []pendingTrigger
	for _, pt := range matched {
		if e.isTriggeredManaAbility(pt) {
			immediate = append(immediate, pt)
			continue
		}
		kept = append(kept, pt)
	}
	e.pendingTriggers = kept
	return immediate
}

// isTriggeredManaAbility recognises Forge's Static$ True marker for a
// TapsForMana trigger. Forge uses that marker for CR 605.1b triggered mana
// abilities; checking the real trigger by source/index avoids treating every
// TapsForMana reaction (notably Manabarbs) as immediate. A target anywhere in
// the linked effect chain keeps the trigger on the stack, as CR 605.1b
// requires a mana ability not to require a target.
func (e *Engine) isTriggeredManaAbility(pt pendingTrigger) bool {
	t, ok := e.triggerOf(pt)
	if !ok {
		return false
	}
	if t.Mode != "TapsForMana" || !strings.EqualFold(t.Params["Static"], "True") {
		return false
	}
	for sa := pt.SA; sa != nil; sa = sa.Sub {
		if strings.TrimSpace(sa.Params["ValidTgts"]) != "" {
			return false
		}
	}
	return pt.SA != nil
}

// resolveTriggeredManaAbilities executes the CR 605.3b batch directly after
// the activated mana ability has resolved. These abilities never mint stack
// objects; their ordinary effect events are enough for deterministic replay.
//
// A triggered mana ability whose mana is a colour choice -- Produced$ Any or
// Combo Any (Fertile Ground, Regal Behemoth, Market Festival) or a Combo
// colour list -- is not resolved as colourless: the rest of the batch is
// parked and the player receiving the mana chooses, exactly as the activated
// path's askManaColor asks. The answer resolves that ability with the chosen
// colour and continues the batch (answerManaColor). cast is the payment
// window flag the parked activation carries back to the caller.
func (e *Engine) resolveTriggeredManaAbilities(triggers []pendingTrigger, cast bool) {
	for i := range triggers {
		pt := triggers[i]
		if int(pt.Controller) >= len(e.G.Players) || e.G.Players[pt.Controller].Lost {
			continue
		}
		if src := e.G.Obj(pt.Source); src != nil {
			// CR 702.140d: a triggered mana ability's SVar table is the
			// table of the face that carries its SA, not the pile top. The
			// SA here is a Trigger.Effect body minted from a face's Triggers
			// list, which is NEVER in that face's Abilities, so the owning
			// face is recovered by findTriggerForAbilityFace (the trigger
			// scan) -- pileFaceForSA, which searches Abilities, can only ever
			// report ok=false for a trigger body. pileFaceForSA stays as the
			// second try for the rare SA that IS a printed activated ability
			// arriving through this queue; the pile-top fallback is last, the
			// same three-step order rules/stack.go's resolving-object SVar
			// resolution uses.
			if _, f, ok := e.findTriggerForAbilityFace(pt.Source, pt.SA); ok && f != nil {
				effects.SetSVars(&pt.Ctx, f.SVars)
			} else if f, ok := e.pileFaceForSA(pt.Source, pt.SA); ok {
				effects.SetSVars(&pt.Ctx, f.SVars)
			} else if src.Face() != nil {
				effects.SetSVars(&pt.Ctx, src.Face().SVars)
			}
		}
		pt = e.rewriteChosenMana(pt)
		if e.askTriggeredManaColor(pt, triggers[i+1:], cast) {
			return
		}
		effects.Resolve(e, &pt.Ctx, pt.SA)
	}
}

// rewriteChosenMana resolves a triggered Mana sub-ability's Produced$ Chosen
// (Utopia Sprawl: "Whenever enchanted Forest is tapped for mana, its
// controller adds an additional one mana of the chosen color"): the colour
// was already chosen by the triggering source's as-enters ChooseColor choice
// (state.Object.ChosenColor), so it is a read, not a choice -- the chain is
// rewritten via withProduced so effMana sees a plain letter, and any later
// genuinely-choice-valued sub still asks through askTriggeredManaColor. With
// nothing recorded the chain is returned untouched and effMana keeps its
// loud fail-closed (never invent a colour). Measured on the corpus, the
// "Combo <letter> Chosen" family (Thriving Bluff, Citadel Gate) carries its
// Produced$ on AB$ activations only -- 0 triggered carriers -- so this
// rewrite keeps the exact-"Chosen" read and does not need the combo
// substitution (substituteChosenProduced covers it if a trigger ever
// carries one).
func (e *Engine) rewriteChosenMana(pt pendingTrigger) pendingTrigger {
	for sa, d := pt.SA, 0; sa != nil && d < 32; sa, d = sa.Sub, d+1 {
		if sa.API != "Mana" || strings.TrimSpace(sa.Params["Produced"]) != "Chosen" {
			continue
		}
		o := e.G.Obj(pt.Source)
		if o == nil {
			return pt
		}
		col := strings.TrimSpace(o.ChosenColor)
		if len(col) != 1 || !strings.ContainsRune("WUBRG", rune(col[0])) {
			return pt
		}
		pt.SA = withProduced(pt.SA, sa, col)
		return pt
	}
	return pt
}

// askTriggeredManaColor poses the colour choice for the first colour-choice
// Mana sub-ability in pt's chain, parking pt and the rest of its batch, and
// reports whether it asked. The chooser is the first player the Mana
// sub-ability adds mana for (effects.ManaRecipients, which reads Defined$):
// Fertile Ground on an opponent's land asks that land's controller.
func (e *Engine) askTriggeredManaColor(pt pendingTrigger, rest []pendingTrigger, cast bool) bool {
	mana, colours := triggeredManaColourChoice(pt.SA)
	if mana == nil {
		return false
	}
	chooser := pt.Controller
	if ps := effects.ManaRecipients(e, &pt.Ctx, mana); len(ps) > 0 {
		chooser = ps[0]
	}
	d := &decision.Decision{Player: chooser, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: manaColourPrompt(mana), Source: pt.Source}
	for i, color := range colours {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "mana", Obj: pt.Source, Label: "Add " + color})
	}
	parked := pt
	e.manaColorActivation = &manaColorActivation{player: chooser, source: pt.Source, ability: mana,
		cast: cast, triggers: rest, trigger: &parked}
	e.choosing = chooseManaColor
	e.ask(d)
	return true
}

// triggeredManaColourChoice finds the first Mana sub-ability in a triggered
// ability's chain whose Produced$ is a colour choice, with the colours it
// offers. nil when the chain adds only fixed mana (or none).
func triggeredManaColourChoice(sa *cards.SA) (*cards.SA, []string) {
	for d := 0; sa != nil && d < 32; d, sa = d+1, sa.Sub {
		if sa.API != "Mana" {
			continue
		}
		produced := strings.TrimSpace(sa.Params["Produced"])
		if produced == "Any" || produced == "Combo Any" {
			return sa, []string{"W", "U", "B", "R", "G"}
		}
		if colours, ok := effects.ComboColours(produced); ok {
			return sa, colours
		}
	}
	return nil, nil
}

// withProduced copies the chain from head down to target, with target's
// Produced$ rewritten to the chosen colour. Corpus SAs are shared immutable
// data, so the rewrite never touches them.
func withProduced(head, target *cards.SA, produced string) *cards.SA {
	if head == nil {
		return nil
	}
	cp := *head
	if head == target {
		cp.Params = make(map[string]string, len(head.Params))
		for k, v := range head.Params {
			cp.Params[k] = v
		}
		cp.Params["Produced"] = produced
		return &cp
	}
	cp.Sub = withProduced(head.Sub, target, produced)
	return &cp
}

// substituteChosenProduced replaces the literal "Chosen" token in a Mana
// ability's Produced$ value with the recorded as-enters chosen colour
// (state.Object.ChosenColor): the bare "Chosen" (Quirion Elves) and the
// "Combo <letter> Chosen" family (Thriving Bluff, Citadel Gate, the five
// thriving lands and five gates: "Add {R} or one mana of the chosen
// color"). It is a read, not a choice; with nothing valid recorded the
// value is returned unchanged and the caller keeps its loud fail-closed
// handling (never invent a colour). The classifier for the substituted
// value stays effects.ComboColours -- a pure string classifier shared with
// the tests and the mana projection, deliberately not taught about state --
// so the substitution happens here in rules, where ChosenColor is readable,
// and every caller (resolveManaEffect, manaAbilityComboColours,
// manaAbilityLabel) sees one consistent value. Measured on the corpus,
// every Chosen token is the value's LAST token ("Combo Chosen" has no fixed
// letter), so the rewrite only ever touches the tail.
func substituteChosenProduced(produced, chosen string) string {
	if chosen == "" {
		return produced
	}
	trimmed := strings.TrimSpace(produced)
	if trimmed == "Chosen" {
		return chosen
	}
	toks := strings.Fields(trimmed)
	if len(toks) >= 2 && toks[0] == "Combo" && toks[len(toks)-1] == "Chosen" {
		toks[len(toks)-1] = chosen
		// The recorded colour can equal a fixed letter ("Combo R Chosen" with
		// R recorded): dedup so the value stays "Combo R" -- a decision nobody
		// could answer differently resolves directly, and the duplicate
		// "Combo R R" would otherwise pose a two-option ask over one colour.
		out := toks[:1]
		for _, tok := range toks[1:] {
			seen := false
			for _, prev := range out {
				if prev == tok {
					seen = true
					break
				}
			}
			if !seen {
				out = append(out, tok)
			}
		}
		return "Combo " + strings.Join(out[1:], " ")
	}
	return produced
}

// chosenProducedColour reads the recorded as-enters chosen colour for
// source: a single WUBRG letter, else "" (nothing valid recorded). Shared
// by the activation-path Produced$ read sites.
func (e *Engine) chosenProducedColour(source state.ObjID) string {
	if o := e.G.Obj(source); o != nil {
		if col := strings.TrimSpace(o.ChosenColor); len(col) == 1 && strings.ContainsRune("WUBRG", rune(col[0])) {
			return col
		}
	}
	return ""
}

// resolveManaAbility pays this ability's actual activation cost, then resolves
// it outside the stack. In particular, Sac and Discard costs are emitted
// before the mana effect, and no phantom generic mana is charged.
func (e *Engine) resolveManaAbility(p state.PlayerID, source state.ObjID, ma *cards.SA, cast bool, cumulative ...bool) {
	payment := len(cumulative) > 0 && cumulative[0]
	if !e.manaAbilityPayable(p, source, ma) {
		return
	}
	// The activation-limit scan marker: ManaAdd events carry no source
	// attribution, so an ability that carries EITHER limit records its
	// activation here (events.ManaActivate's own comment). Emitted only for a
	// limit-bearing ability so no existing game's log shape changes.
	if _, limited := ma.Params["ActivationLimit"]; limited || ma.Params["GameActivationLimit"] != "" {
		// The flat pile index (top face first, then under-cards) is the SAME
		// identity availableManaAbilitiesUsing's limit gate checks, so an
		// under-card mana ability's census cannot be counted against a
		// top-face ability.
		if idx, _, found := pileAbilityRefOf(e.G.Obj(source), ma); found {
			e.emit(events.Event{Kind: events.ManaActivate, Player: p, Obj: source, Amount: int32(idx)})
		}
	}
	cost := e.parseCost(ma.Params["Cost"])
	sacs, _ := e.manaSacrifices(p, source, cost)
	if len(cost.Discard) > 0 || len(cost.Exile) > 0 {
		e.manaDiscardActivation = &manaDiscardActivation{player: p, source: source,
			ability: ma, cost: cost, sacs: sacs, cast: cast, cumulative: payment}
		e.continueManaDiscard()
		return
	}
	if !e.payManaConvFor(p, source, true, cost, e.paymentConv(p, source, true)) {
		return
	}
	var manaTriggers []pendingTrigger
	if cost.Tap {
		manaTriggers = e.emitManaTap(p, source, ma)
	}
	for _, part := range cost.SubCounter {
		e.emit(events.Event{Kind: events.CounterChange, Obj: source, Counter: part.Spec, Amount: -part.N})
	}
	for _, id := range sacs {
		e.emit(events.Sacrifice(id))
	}
	e.resolveManaEffect(p, source, ma, cast, payment, manaTriggers, sacs)
}

// resolveManaEffect resolves the mana a paid ability produces. sacs carries
// the permanents the ability's Sac<...> cost sacrificed, so a ManaReflected
// Valid$ "Defined.Sacrificed" selector (Squandered Resources) can read them
// through the resolution context's Remembered list.
func (e *Engine) resolveManaEffect(p state.PlayerID, source state.ObjID, ma *cards.SA, cast, cumulative bool, triggers []pendingTrigger, sacs []state.ObjID) {
	if strings.TrimSpace(ma.Params["UnlessCost"]) != "" {
		e.askManaUnless(p, source, ma, cast, cumulative, triggers, sacs)
		return
	}
	produced := strings.TrimSpace(ma.Params["Produced"])
	// A "Chosen" token (Quirion Elves' second activation: "Add one mana of
	// the chosen color"; the Thriving-lands/gate family: "Add {R} or one mana
	// of the chosen color") is a READ, not a choice: the colour was already
	// chosen by the source's as-enters ChooseColor choice
	// (state.Object.ChosenColor). With nothing recorded the local keeps the
	// raw value and the fall-through keeps effMana's loud fail-closed (never
	// invent a colour).
	if col := e.chosenProducedColour(source); col != "" {
		produced = substituteChosenProduced(produced, col)
	}
	if ma.API == "ManaReflected" {
		// CR 702.140d: resolve against the face that CARRIES this ability,
		// not the pile top -- an under-card mana ability's Reflected SVars
		// live on its own face. A granted/non-printed ability keeps the
		// top-face fallback (pileFaceForSA reports ok=false).
		svars := func() map[string]string {
			if o := e.G.Obj(source); o != nil {
				if f, ok := e.pileFaceForSA(source, ma); ok {
					return f.SVars
				}
				if o.Face() != nil {
					return o.Face().SVars
				}
			}
			return nil
		}()
		ctx := &effects.Ctx{Source: source, Controller: p, SVars: svars}
		for _, id := range sacs {
			ctx.Remembered = append(ctx.Remembered, state.Target{Obj: id})
		}
		cols := effects.ManaReflectedCandidates(e, ctx, ma)
		switch len(cols) {
		case 0:
			e.emit(events.Event{Kind: events.Note, Obj: source, Text: "ManaReflected found no mana to reflect"})
		case 1:
			e.resolveManaEffectColor(p, source, ma, cols[0])
			e.resolveTriggeredManaAbilities(triggers, cast)
			if cumulative && e.choosing == chooseNone {
				e.paymentWindowAsk()
			}
		default:
			e.askManaColor(p, source, ma, cast, cumulative, triggers, cols)
		}
		return
	}
	if produced == "Any" || produced == "Combo Any" {
		e.askManaColor(p, source, ma, cast, cumulative, triggers, []string{"W", "U", "B", "R", "G"})
		return
	}
	// A "Combo <colours>" shape is "add one of these", not "add each of
	// these": it asks, but restricted to exactly the colours it names --
	// "Combo R G" offers R and G, never the other three. The classifier
	// (effects.ComboColours) rejects "Combo Any" (kept on the five-colour
	// branch above) and every combo it cannot resolve to a plain colour list,
	// which then falls to resolveManaEffectColor and fails closed in effMana.
	// A SINGLE-colour list never asks -- a decision nobody could answer
	// differently is never posed (the same convention the ManaReflected and
	// ColorIdentity siblings apply): it resolves directly. Measured on the
	// corpus, no raw script carries "Produced$ Combo <one letter>"; the only
	// live carriers are the flattened stage-1 answers, whose ability arrives
	// with its Produced$ already rewritten to the chosen colour (task
	// fb-20260917T232800Z), so the shortcut skips the otherwise-posed
	// one-option stage-2 ask.
	if colours, ok := effects.ComboColours(produced); ok {
		if len(colours) == 1 {
			e.resolveManaEffectColor(p, source, ma, colours[0])
			e.resolveTriggeredManaAbilities(triggers, cast)
			if cumulative && e.choosing == chooseNone {
				e.paymentWindowAsk()
			}
			return
		}
		e.askManaColor(p, source, ma, cast, cumulative, triggers, colours)
		return
	}
	// "ColorIdentity" (Command Tower, Arcane Signet: "Add one mana of any
	// color in your commander's color identity") is a colour choice scoped to
	// the activating player's commander colour identity (CR 903.4). Every
	// corpus occurrence is an AB$ Mana activation (6 files), so the branch
	// lives on the activation path only; the token is matched on the Produced
	// value itself (with or without the "Combo " prefix) rather than on the
	// API, so a future trigger carrying it cannot silently degrade. Colours
	// come in fixed WUBRG order: none (no commander, or a colourless one) keeps
	// today's fail-closed fall-through (CR 903.4's "any color" of an empty
	// identity is nothing, not colourless), one resolves directly (a decision
	// nobody could answer differently must not be posed), two or more ask.
	if isColourIdentityProduced(produced) {
		cols := e.commanderIdentityColours(p)
		switch len(cols) {
		case 1:
			e.resolveManaEffectColor(p, source, ma, cols[0])
			e.resolveTriggeredManaAbilities(triggers, cast)
			if cumulative && e.choosing == chooseNone {
				e.paymentWindowAsk()
			}
			return
		default:
			if len(cols) > 1 {
				e.askManaColor(p, source, ma, cast, cumulative, triggers, cols)
				return
			}
			// 0 colours: fall through to the fail-closed resolve below.
		}
	}
	e.resolveManaEffectColor(p, source, ma, produced)
	e.resolveTriggeredManaAbilities(triggers, cast)
	if cumulative && e.choosing == chooseNone {
		e.paymentWindowAsk()
	}
}

// askManaUnless handles the one off-stack instance of the shared UnlessCost$
// contract. Activated mana abilities cannot use effects.Host.Ask's stack
// resume point, but their payer still receives an ordinary KModes decision
// and rules charges exactly the same parsed cost on a "pay" answer.
func (e *Engine) askManaUnless(p state.PlayerID, source state.ObjID, ma *cards.SA, cast, cumulative bool, triggers []pendingTrigger, sacs []state.ObjID) {
	ctx := &effects.Ctx{Source: source, Controller: p}
	var payers []state.PlayerID
	for _, t := range effects.UnlessPayers(e, ctx, ma) {
		if t.IsPlayer {
			payers = append(payers, t.Player)
		}
	}
	if len(payers) == 0 {
		payers = []state.PlayerID{p}
	}
	e.manaUnlessActivation = &manaUnlessActivation{player: p, source: source, ability: ma,
		cast: cast, cumulative: cumulative, triggers: triggers, sacs: sacs, payers: payers}
	e.askManaUnlessDecision()
}

func (e *Engine) askManaUnlessDecision() {
	m := e.manaUnlessActivation
	if m == nil || m.next >= len(m.payers) {
		return
	}
	raw := strings.TrimSpace(m.ability.Params["UnlessCost"])
	cost := capitaliseFirst(costPhrase(ParseCost(raw)))
	if cost == "" {
		// A cost costPhrase cannot render (a malformed or entirely
		// unmodelled token) keeps the raw text rather than emitting an
		// empty prompt -- the fallback is still better than "pay ?".
		cost = "Pay " + raw
	}
	d := &decision.Decision{Player: m.payers[m.next], Kind: decision.KModes, Min: 1, Max: 1,
		Source: m.source, ResumeKind: "mana_unless", Prompt: cost + ", or decline",
		Options: []decision.Option{
			{Index: 0, Kind: "mode", Obj: m.source, Player: m.payers[m.next], Label: cost},
			{Index: 1, Kind: "mode", Obj: m.source, Player: m.payers[m.next], Label: "Don't pay"},
		}}
	e.choosing = chooseManaUnless
	e.ask(d)
}

// answerManaUnless applies one payer's answer. Declines visit later named
// payers in turn; paying ends the sequence. A malformed cost is deliberately
// a decline, matching resumeResolution's ordinary unless-pay path.
func (e *Engine) answerManaUnless(chosen []decision.Option) bool {
	m := e.manaUnlessActivation
	if m == nil || m.next >= len(m.payers) {
		return false
	}
	payer := m.payers[m.next]
	paid := false
	if len(chosen) == 1 && chosen[0].Index == 0 {
		if cost, ok := ParseUnlessCost(m.ability.Params["UnlessCost"]); ok {
			if len(cost.Sac) > 0 || len(cost.Discard) > 0 || len(cost.Reveal) > 0 {
				// Activated mana stays off stack, but a sacrifice/discard/reveal
				// in its unless cost is still a real payer choice. The payment
				// continuation returns through finishManaUnlessPayment.
				e.beginUnlessPayment(payer, cost, &effects.Ctx{Source: m.source, Controller: m.player}, m.source, nil)
				return m.cast
			}
			paid = e.payUnlessCost(payer, cost, &effects.Ctx{Source: m.source, Controller: m.player}, m.source)
		}
	}
	e.finishManaUnlessPayment(paid)
	return m.cast
}

// finishManaUnlessPayment completes one payer's answer after either a direct
// payment or an asynchronous Sac/Discard choice.
func (e *Engine) finishManaUnlessPayment(paid bool) {
	m := e.manaUnlessActivation
	if m == nil {
		return
	}
	switched := strings.EqualFold(strings.TrimSpace(m.ability.Params["UnlessSwitched"]), "True")
	if paid || m.next+1 == len(m.payers) {
		e.manaUnlessActivation = nil
		e.choosing = chooseNone
		if paid == switched {
			// Do not send this already-answered gate through effects.Resolve a
			// second time. Sub-abilities retain their own UnlessCost$ gates.
			cp := *m.ability
			cp.Params = make(map[string]string, len(m.ability.Params))
			for k, v := range m.ability.Params {
				cp.Params[k] = v
			}
			delete(cp.Params, "UnlessCost")
			delete(cp.Params, "UnlessPayer")
			delete(cp.Params, "UnlessSwitched")
			e.resolveManaEffect(m.player, m.source, &cp, m.cast, m.cumulative, m.triggers, m.sacs)
		} else {
			e.resolveTriggeredManaAbilities(m.triggers, m.cast)
		}
		return
	}
	m.next++
	e.askManaUnlessDecision()
}

// askManaColor poses the colour choice for a Produced value that names a
// fixed set (the five colours for Any/Combo Any, or the named colours of a
// "Combo <colours>" shape) and pauses until it is answered.
func (e *Engine) askManaColor(p state.PlayerID, source state.ObjID, ma *cards.SA, cast, cumulative bool, triggers []pendingTrigger, colours []string) {
	d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: manaColourPrompt(ma), Source: source}
	for i, color := range colours {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "mana", Obj: source, Label: "Add " + color})
	}
	e.manaColorActivation = &manaColorActivation{player: p, source: source, ability: ma, cast: cast, cumulative: cumulative, triggers: triggers}
	e.choosing = chooseManaColor
	e.ask(d)
}

// manaColourPrompt names the amount of mana the ability adds when the script
// carries an EXPLICIT, positive literal Amount$, so a player choosing the
// colour of "Add three mana of any one color" (Lion's Eye Diamond) sees the
// whole deal instead of a bare "Choose a colour of mana" that reads like the
// card only offered one colour of mana. Every other shape keeps the generic
// prompt: an absent Amount$ (the prompt must not invent "1" for the pool that
// effMana will actually resolve — the brief's boundary is explicit-literal
// only), a non-literal amount (X, Y, an SVar or inline Count$ expression) the
// ask site cannot price, and a non-positive literal — a wrong number in the
// prompt is worse than no number. The "any one color" wording is kept
// verbatim from the oracle shape (Produced$ Any / Combo Any, CR 107.4) so the
// player sees that the ONE choice covers all of the mana; a restricted
// "Combo <colours>" shape is not "any" colour, so its prompt only names the
// amount; a ColorIdentity shape names the commander-identity restriction so
// the player knows WHY the offer is narrower than five colours (the oracle
// wording, "any color in your commander's color identity", kept verbatim).
func manaColourPrompt(ma *cards.SA) string {
	generic := "Choose a colour of mana"
	shape := strings.TrimSpace(ma.Params["Produced"])
	identity := isColourIdentityProduced(shape)
	if identity {
		generic = "Choose a colour in your commander's color identity"
	}
	raw, ok := ma.Params["Amount"]
	if !ok {
		// No Amount$ param: stay generic — the prompt must not invent an
		// amount the script never stated.
		return generic
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		// A non-literal amount (X, Y, an SVar or inline Count$ expression)
		// or a non-positive literal: the ask site cannot price it, and a
		// wrong number in the prompt is worse than no number.
		return generic
	}
	switch {
	case identity:
		return fmt.Sprintf("Add %d mana of any color in your commander's color identity — choose the colour", n)
	case shape == "Any" || shape == "Combo Any":
		return fmt.Sprintf("Add %d mana of any one color — choose the colour", n)
	default:
		return fmt.Sprintf("Add %d mana — choose the colour", n)
	}
}

func (e *Engine) resolveManaEffectColor(p state.PlayerID, source state.ObjID, ma *cards.SA, produced string) {
	o := e.G.Obj(source)
	if o == nil || o.Face() == nil {
		return
	}
	copy := *ma
	copy.Params = make(map[string]string, len(ma.Params)+1)
	for k, v := range ma.Params {
		copy.Params[k] = v
	}
	copy.Params["Produced"] = produced
	// ProduceMana replacements need the ability's source and whether its
	// paid cost included T. Preserve both only for effMana's synchronous emit:
	// producer attribution is replacement matching context, not a durable
	// property of the resulting ManaAdd (putting it in Event.Obj moved every
	// replay chain head). A sacrifice-only KCI activation therefore identifies
	// its source but is not tap-produced.
	savedTap, savedProducer := e.manaFromTap, e.manaProducer
	e.manaFromTap = e.parseCost(ma.Params["Cost"]).Tap
	e.manaProducer = source
	e.resolveAbility(source, p, nil, &copy, o.Face().SVars)
	e.manaFromTap, e.manaProducer = savedTap, savedProducer
}

// answerManaColor completes a Produced$ Any choice after the activation cost
// has already been paid. It returns whether the suspended activation belonged
// to a cast-time mana window.
func (e *Engine) answerManaColor(chosen []decision.Option) bool {
	ma := e.manaColorActivation
	e.manaColorActivation = nil
	e.choosing = chooseNone
	if ma == nil || len(chosen) != 1 {
		return false
	}
	color := strings.TrimPrefix(chosen[0].Label, "Add ")
	if len(color) != 1 || !strings.Contains("WUBRGC", color) {
		return ma.cast
	}
	if ma.trigger != nil {
		pt := *ma.trigger
		pt.SA = withProduced(pt.SA, ma.ability, color)
		// A later colour-choice Mana sub-ability in the same chain asks in
		// turn; otherwise the ability resolves and the batch continues.
		if e.askTriggeredManaColor(pt, ma.triggers, ma.cast) {
			return ma.cast
		}
		effects.Resolve(e, &pt.Ctx, pt.SA)
		e.resolveTriggeredManaAbilities(ma.triggers, ma.cast)
		return ma.cast
	}
	e.resolveManaEffectColor(ma.player, ma.source, ma.ability, color)
	e.resolveTriggeredManaAbilities(ma.triggers, ma.cast)
	if ma.cumulative && e.choosing == chooseNone {
		e.paymentWindowAsk()
	}
	return ma.cast
}

// answerManaActivation completes a multi-ability choice and returns whether
// it came from the cast-time mana window. A flattened combo choice (the
// stage-1 wheel offered one option per colour of an explicit "Produced$ Combo
// <colours>" ability, task fb-20260917T232800Z) resolves the ability with its
// Produced$ rewritten to the chosen colour: the cost is paid exactly once by
// resolveManaAbility and the stage-2 colour ask never opens, because
// resolveManaEffect sees a single fixed colour. The rewrite guards on the
// ability's own Produced$ being a MULTI-colour combo and the label being one
// "Add <C>" pip, so a plain single-colour ability that happens to share the
// label keeps its unrewritten resolution.
func (e *Engine) answerManaActivation(chosen []decision.Option) bool {
	ma := e.manaActivation
	e.manaActivation = nil
	e.choosing = chooseNone
	if ma == nil || len(chosen) != 1 {
		return false
	}
	idx := chosen[0].Ability
	if idx >= 0 && idx < len(ma.abilities) {
		ab := ma.abilities[idx]
		if _, ok := manaAbilityComboColours(ab, e.chosenProducedColour(ma.source)); ok {
			color := strings.TrimPrefix(chosen[0].Label, "Add ")
			if len(color) == 1 && strings.Contains("WUBRGC", color) {
				// abilities entries are chain heads (printed faces list
				// top-level abilities; granted and static-granted ones
				// come from ResolveSVar bodies), so head == target copies
				// the whole Sub chain with Produced$ rewritten.
				e.resolveManaAbility(ma.player, ma.source, withProduced(ab, ab, color), ma.cast, ma.cumulative)
				return ma.cast
			}
		}
		e.resolveManaAbility(ma.player, ma.source, ab, ma.cast, ma.cumulative)
	}
	return ma.cast
}

// isColourIdentityProduced reports whether a Produced$ value is the
// commander-identity choice shape — the literal "ColorIdentity" or the combo
// form "Combo ColorIdentity" (Command Tower, Arcane Signet, Commander's
// Sphere, Hidden Hideout, Opal Palace, Path of Ancestry). Shared by the
// activation-path branch in resolveManaEffect and manaColourPrompt's
// restricted wording so the two cannot disagree.
func isColourIdentityProduced(produced string) bool {
	return produced == "ColorIdentity" || produced == "Combo ColorIdentity"
}

// commanderIdentityColours expands seat p's commander colour identity to the
// colours it names, in fixed WUBRG order (the order cards.Face's colour bits
// are declared in, and the order deck/deck.go's identity checks read — no map
// range, so the offer order is deterministic). The identity is the bitwise
// union of every commander's full-card identity (cards.Card.ColourIdentity
// unions over faces), read off the live command-zone objects
// (state.Player.Commanders, populated at genesis and stable across zone
// moves). A seat with no commanders — or commanders whose identity is empty
// (colourless, CR 903.4) — yields a nil slice: "any color" of an empty
// identity is no colour at all, so the caller keeps its fail-closed
// behaviour.
func (e *Engine) commanderIdentityColours(p state.PlayerID) []string {
	if int(p) >= len(e.G.Players) {
		return nil
	}
	var m uint8
	for _, cid := range e.G.Players[p].Commanders {
		o := e.G.Obj(cid)
		if o == nil || o.Card == nil {
			continue
		}
		m |= o.Card.ColourIdentity()
		// CR 903.4b: a commander whose printed CDA says "choose a color before
		// the game begins" derives its identity from the recorded choice. Gate
		// on the CDA static, never the bare ChosenColor field: a commander with
		// an ordinary "as this enters" colour choice must not leak its
		// battlefield choice into its identity, and before the pregame answer
		// (or for a non-commander) ChosenColor is empty anyway.
		if o.Card.Faces[0] != nil && o.Card.Faces[0].CommanderColourChoiceCDA() {
			if cols, ok := resolveChosenColors("ChosenColor", o); ok {
				for _, l := range cols {
					if len(l) == 0 {
						continue
					}
					if i := strings.IndexByte("WUBRG", l[0]); i >= 0 {
						m |= 1 << uint(i)
					}
				}
			}
		}
	}
	var cols []string
	for i, sym := range []string{"W", "U", "B", "R", "G"} {
		if m&(1<<uint(i)) != 0 {
			cols = append(cols, sym)
		}
	}
	return cols
}

// CommanderIdentityColourCount is the effects.Host read over
// commanderIdentityColours: how many colours seat p's commander colour
// identity names. This is Count$ColorsColorIdentity's backing (War Room's
// fixed "Pay life equal to the number of colors in your commanders' color
// identity"); it reads the same genesis bookkeeping the replay rebuilds in
// Config order, so a replay derives the identical count.
func (e *Engine) CommanderIdentityColourCount(p state.PlayerID) int {
	return len(e.commanderIdentityColours(p))
}
