package pay

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/effects/params"
	"github.com/adams-shaun/gorge/engine/events"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// CastWindowOtherPartsAbsent reports whether c carries none of the cost
// components the paid-cost layer does not price. It is deliberately broader
// than manaFreeCost (which only needs a bare tap): every part whose payment
// needs a choice, an event or a resource this probe does not model is
// refused, as is any token the parser did not understand.
func CastWindowOtherPartsAbsent(c costvocab.Cost) bool {
	return castWindowPartsAbsent(c, false)
}

// CastWindowHybridPartsAbsent is CastWindowOtherPartsAbsent with the hybrid
// pip lifted: a hybrid pip is pip-shaped mana (one of its pair's colours is
// the payer's announcement, CR 107.4b), not an "other part" whose payment
// needs an unmodelled resource. The paid-cost layer that admits the shape
// resolves the pip itself. Every other pip form stays refused (fail closed).
func CastWindowHybridPartsAbsent(c costvocab.Cost) bool {
	return castWindowPartsAbsent(c, true)
}

func castWindowPartsAbsent(c costvocab.Cost, hybridOK bool) bool {
	if !hybridOK && len(c.Hybrid) != 0 {
		return false
	}
	return len(c.Discard) == 0 && len(c.SubCounter) == 0 && len(c.AddCounter) == 0 &&
		len(c.Exile) == 0 && len(c.ExileFromTop) == 0 && len(c.Reveal) == 0 && len(c.RevealOrChoose) == 0 && len(c.RevealChosen) == 0 &&
		len(c.Behold) == 0 && len(c.TapPermanent) == 0 && len(c.Blight) == 0 &&
		len(c.Exert) == 0 && !c.Forage && !c.LifeHalfUp && len(c.Draw) == 0 &&
		len(c.Energy) == 0 && len(c.LifeX) == 0 && len(c.DamageYou) == 0 &&
		len(c.GainLife) == 0 &&
		len(c.Return) == 0 && len(c.PutToLib) == 0 && len(c.MoveToGrave) == 0 &&
		len(c.Mill) == 0 && len(c.Evidence) == 0 && len(c.RollDice) == 0 &&
		len(c.Unknown) == 0 && len(c.Phyrexian) == 0 &&
		len(c.Twobrid) == 0 && len(c.HybridPhyrexian) == 0 && c.Snow == 0 && c.X == 0
}

// AlternativeExec is the ability the executor activates for a: ma itself
// for fixed production, else ma with its Produced$ rewritten to the
// selected colour.
func AlternativeExec(a Alt) *cards.SA {
	if a.ExecProduced == "" {
		return a.Ma
	}
	return WithProduced(a.Ma, a.Ma, a.ExecProduced)
}

// PaymentPlanTapOnlyCost is the V1 source contract.  A payment witness can
// record and replay the source, ability and mana result, but it intentionally
// carries no representation for an additional activation cost.  Require an
// actual tap and reject every parsed or unknown companion cost, including
// Mill<N>, even where the ordinary manual mana window can pay it without a
// further choice.
func PaymentPlanTapOnlyCost(c costvocab.Cost) bool {
	return c.Tap && c.XMin == 0 && ManaFreeCost(c) && CastWindowOtherPartsAbsent(c)
}

// PaymentPlanShapeTierOf is paymentPlanAbilityShapeTier's source-independent
// part over ma's Params and its parsed cost: the verdict, or rider true when
// ma carries a SubAbility$ whose chain (read off the source's face) decides
// it.
func PaymentPlanShapeTierOf(ma *cards.SA, cost costvocab.Cost) (tier Tier, c Consequence, detail string, rider bool) {
	deferred := func(detail string) (Tier, Consequence, string, bool) {
		return TierDeferred, Consequence{}, detail, false
	}
	// A reflected-mana ability is planned only in its one opponent-reflecting
	// shape (PaymentPlanReflectedShape); it then takes the same key, cost
	// and SubAbility$ checks as an AB$ Mana ability.
	reflected := PaymentPlanReflectedShape(ma)
	if ma == nil || (ma.API != "Mana" && !reflected) {
		return deferred("source:special_production")
	}
	// The key loop carries the key NAMES only (never a Params value) into
	// the prefix checks below; slices.Sorted(maps.Keys) keeps it a plain
	// string-slice walk rather than a range the param census would have to
	// classify.
	//
	// The verdict is the one the sorted walk reaches first: the smallest key
	// failing any of the three checks decides it. Taking that minimum over
	// the unsorted keys is order-independent (so deterministic) and spares
	// the sorted copy.
	failing, found := "", false
	for key := range maps.Keys(ma.Params) {
		if (!found || key < failing) && paymentPlanShapeKeyFails(key) && !(reflected && paymentPlanReflectedParamSet.Has(key)) {
			failing, found = key, true
		}
	}
	if found {
		key := failing
		if strings.HasPrefix(key, "Condition") {
			return deferred("source:conditional")
		}
		if ContainsTargetFold(key) || key == "ValidTgts" || key == "ValidTarget" {
			return deferred("source:target")
		}
		return deferred("source:param:" + key)
	}
	if PlanHasSpecialProductionParam(ma) {
		return deferred("source:special_production")
	}
	// A TriggersWhenSpent$ rider's verdict reads the source's SVar body, so
	// the source-aware arm (paymentPlanAbilityShapeTier) decides it.
	if effects.ManaOf(ma).TriggersWhenSpent != "" {
		return TierDeferred, Consequence{}, "", true
	}
	if cost.XMin != 0 || cost.X != 0 || cost.Generic != 0 || cost.Colored.Total() != 0 || len(cost.Discard)+len(cost.SubCounter)+len(cost.Exile)+len(cost.ExileFromTop)+len(cost.TapPermanent)+len(cost.Energy)+len(cost.LifeX) != 0 {
		return deferred("source:last_resort")
	}
	if strings.TrimSpace(ma.ParamStr(cards.PKSubAbility)) != "" {
		return TierDeferred, Consequence{}, "", true
	}
	if cost.Sac != nil || cost.Life != 0 || cost.Return != nil {
		// Every other cost part must be absent: the witness discloses only
		// the tap, the self-sacrifice, the life and the self-return.
		if !PlanLastResortCostOK(cost) {
			return deferred("source:last_resort")
		}
		if len(cost.Sac) > 0 && len(cost.Sac) == 1 && PlanSelfCost(cost.Sac[0], 0) {
			c.Sacrifice = true
		} else if len(cost.Sac) > 0 {
			return deferred("source:last_resort")
		}
		if cost.Life > 0 {
			c.Life = uint32(cost.Life)
		}
		if len(cost.Return) > 0 && len(cost.Return) == 1 && PlanSelfCost(cost.Return[0], 0) {
			c.ReturnToHand = true
		} else if len(cost.Return) > 0 {
			return deferred("source:last_resort")
		}
		return TierLastResort, c, "source:last_resort", false
	}
	if !PaymentPlanTapOnlyCost(cost) {
		return deferred("source:last_resort")
	}
	return TierNormal, Consequence{}, "", false
}

// paymentPlanShapeKeyFails reports whether key alone defers the ability in
// paymentPlanAbilityShapeTier's key walk (a Condition key, a target key or
// an unreviewed parameter).
func paymentPlanShapeKeyFails(key string) bool {
	return strings.HasPrefix(key, "Condition") || ContainsTargetFold(key) || key == "ValidTgts" || key == "ValidTarget" ||
		!paymentPlanKnownManaParam(key)
}

func paymentPlanKnownManaParam(key string) bool {
	if strings.HasPrefix(key, "AddsKeywords") {
		return true
	}
	return paymentPlanKnownManaParamSet.Has(key)
}

func PaymentPlanDamageRider(e Engine, id state.ObjID, mana *cards.SA) (uint32, bool) {
	o := e.Game().Obj(id)
	if o == nil || o.Face() == nil {
		return 0, false
	}
	return paymentPlanDamageBody(cards.ResolveSVar(o.Face().SVars, strings.TrimSpace(mana.ParamStr(cards.PKSubAbility))))
}

// paymentPlanSubFree reports whether rider carries no SubAbility$ chain.
func paymentPlanSubFree(rider *cards.SA) bool {
	return strings.TrimSpace(rider.ParamStr(cards.PKSubAbility)) == ""
}

// paymentPlanBodyNumDmg is rider's literal NumDmg$ amount, positive.
func paymentPlanBodyNumDmg(rider *cards.SA) (uint32, bool) {
	n, err := strconv.ParseUint(strings.TrimSpace(params.DealDamageOf(rider).NumDmg.Text), 10, 32)
	return uint32(n), err == nil && n > 0
}

// paymentPlanDamageBody reports N when rider is exactly `DealDamage |
// Defined$ You | NumDmg$ <literal N>` with no further parameter or sub.
func paymentPlanDamageBody(rider *cards.SA) (uint32, bool) {
	if rider == nil || rider.API != "DealDamage" || !params.DefinedRefOf(rider).Is(params.RefYou) || !paymentPlanSubFree(rider) {
		return 0, false
	}
	n, ok := paymentPlanBodyNumDmg(rider)
	if !ok {
		return 0, false
	}
	for _, k := range slices.Sorted(maps.Keys(rider.Params)) {
		if k != "API" && k != "Defined" && k != "NumDmg" && k != "SpellDescription" && k != "StackDescription" {
			return 0, false
		}
	}
	return n, true
}

func PaymentPlanParadiseRider(e Engine, id state.ObjID, mana *cards.SA) bool {
	o := e.Game().Obj(id)
	if o == nil || o.Face() == nil {
		return false
	}
	rider := cards.ResolveSVar(o.Face().SVars, strings.TrimSpace(mana.ParamStr(cards.PKSubAbility)))
	if rider == nil || rider.API != "Pump" || params.DefinedRefOf(rider).Raw != "Self" {
		return false
	}
	for _, key := range slices.Sorted(maps.Keys(rider.Params)) {
		if key != "API" && key != "Defined" && key != "KW" && key != "Duration" && key != "SpellDescription" && key != "StackDescription" {
			return false
		}
	}
	text := strings.ToLower(strings.Join([]string{rider.ParamStr(cards.PKKW), rider.ParamStr(cards.PKSpellDescription), rider.ParamStr(cards.PKStackDescription)}, " "))
	return strings.Contains(text, "hidden") && strings.Contains(text, "return")
}

// PaymentPlanChoiceShape reports whether a Produced$ value is one of the
// finite choice shapes V1 can plan: the literal Any, a bare Chosen/
// ChosenColor, or a Combo list. It is deliberately about the SHAPE only;
// whether the choice resolves to any colour (an unrecorded Chosen, a Combo
// naming no plain colour, an empty commander identity) is
// paymentPlanChoiceColours' fail-closed answer, not this predicate's.
func PaymentPlanChoiceShape(raw string) bool {
	if paymentPlanChoiceShapeSet.Has(raw) {
		return true
	}
	return strings.HasPrefix(raw, "Combo ")
}

// PaymentSourceZoneSeqScan is the reference answer: it deliberately scans
// backwards so a later incarnation cannot be authorized by a witness made for
// an earlier visit to the battlefield.
func PaymentSourceZoneSeqScan(e Engine, id state.ObjID) uint64 {
	o := e.Game().Obj(id)
	if o == nil {
		return decision.GenesisZoneSeq
	}
	for i := len(e.Log().Events) - 1; i >= 0; i-- {
		ev := e.Log().Events[i]
		if ev.Obj != id {
			continue
		}
		if ev.Kind == events.MoveZone && ev.To == o.Zone {
			return ev.Seq
		}
		if ev.Kind == events.TokenCreate && o.Zone == state.ZBattlefield {
			return ev.Seq
		}
	}
	return decision.GenesisZoneSeq
}

var paymentPlanKnownManaParamSet = state.NewNameSet(
	"API",
	"Cost",
	"Produced",
	"Amount",
	"SubAbility",
	"SpellDescription",
	"StackDescription",
	"AILogic",
	"PrecostDesc",
	"Activation",
	"Activator",
	"ActivationPhases",
	"PlayerTurn",
	"OpponentTurn",
	"ActivationFirstCombat",
	"ActivationAfterBlockers",
	"IsPresent",
	"PresentCompare",
	"CheckSVar",
	"SVarCompare",
	"ActivationLimit",
	"GameActivationLimit",
	"InstantSpeed",
	"RestrictValid",
	"TriggersWhenSpent",
	"AddsCounters",
	"AddsKeywords",
	"AddsKeywordsAll",
	"AddsNoCounter",
	"PersistentMana",
	"UnlessCost",
	"Defined",
)

var paymentPlanChoiceShapeSet = state.NewNameSet(
	"Chosen",
	"ChosenColor",
	"ComboChosen",
)

// PaymentPlanConvRestricts classifies a payer's effective conversion set
// (paymentConv, the payment path's own parse of every ManaConvert static
// reaching the payer) by whether it can make a planned payment INVALID.
//
//   - wild / wildC ("spend mana as though it were mana of any color/type":
//     Mycosynth Lattice, Chromatic Orrery, the AnyType->AnyColor family) and
//     to ("White->Red") only ever ADD pips a unit of mana may pay. A witness
//     the ordinary solver proved payable without them stays payable with
//     them, so they are not a global plan-blocker.
//   - onlyC ("you may spend other mana only as though it were colorless
//     mana": Celestial Dawn's nonWhite<-C) REMOVES pips a unit may pay, so a
//     plan priced without it can be unpayable: global.
//
// Every manaConv field is classified here; a field added to manaConv later
// must be classified too. A ManaConversion$ token the parser cannot read is
// inert at payment as well (manaColourFrom/applyManaConversionTo), so it can
// invalidate nothing the solver priced.
func PaymentPlanConvRestricts(c *Conv) bool {
	return slices.Contains(c.OnlyC[:], true)
}

// PaymentPlanObjName is an object's current face name, for diagnostics.
func PaymentPlanObjName(e Engine, id state.ObjID) string {
	if o := e.Game().Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return "object " + strconv.Itoa(int(id))
}

// PaymentPlanNoUntapShape is spec §3.2's no_untap row: exactly the source's
// own "doesn't untap during your untap step" -- ValidCard$ Card.Self, Layer$
// CantHappen, no ReplaceWith$ body, an optional ValidStepTurnToController$
// You and presentation/zone keys only. Anything else is not a fully
// determined consequence.
func PaymentPlanNoUntapShape(r cards.Repl) bool {
	if r.With != nil || strings.TrimSpace(r.ParamStr(cards.PKValidCard)) != "Card.Self" ||
		!strings.EqualFold(strings.TrimSpace(r.ParamStr(cards.PKLayer)), "CantHappen") {
		return false
	}
	if v, ok := r.Param(cards.PKValidStepTurnToController); ok && strings.TrimSpace(v) != "You" {
		return false
	}
	if v, ok := r.Param(cards.PKActiveZones); ok && strings.TrimSpace(v) != "Battlefield" {
		return false
	}
	for _, k := range slices.Sorted(maps.Keys(r.Params)) {
		if !paymentPlanNoUntapShapeKeys.Has(k) {
			return false
		}
	}
	return true
}

// PaymentPlanSelfDamageTrigger is spec §3.2's damage:N trigger row: the
// source's own `Mode$ Taps | ValidCard$ Card.Self` whose effect is exactly
// `DealDamage | Defined$ You | NumDmg$ <literal>` and nothing else (City of
// Brass). f owns the trigger's Execute$ table.
func PaymentPlanSelfDamageTrigger(f *cards.Face, t cards.Trigger) (uint32, bool) {
	if t.Mode != "Taps" || strings.TrimSpace(t.ParamStr(cards.PKValidCard)) != "Card.Self" {
		return 0, false
	}
	if !paymentPlanTriggerZoneOK(t) {
		return 0, false
	}
	for _, k := range slices.Sorted(maps.Keys(t.Params)) {
		switch paymentPlanSelfDamageTriggerCodes.Code(string(k)) {
		case paymentPlanSelfDamageTriggerKnown:
		default:
			return 0, false
		}
	}
	return paymentPlanDamageBody(paymentPlanTriggerBody(f, t))
}

// paymentPlanTriggerZoneOK is the TriggerZones$ gate the tap watchers share:
// absent means Battlefield; any other zone is not this planner's shape.
func paymentPlanTriggerZoneOK(t cards.Trigger) bool {
	if v, ok := t.Param(cards.PKTriggerZones); ok && strings.TrimSpace(v) != "Battlefield" {
		return false
	}
	return true
}

// paymentPlanTriggerBody is the trigger's Execute$ body, resolved against
// f's SVar table when the parsed line carries only the SVar name.
func paymentPlanTriggerBody(f *cards.Face, t cards.Trigger) *cards.SA {
	if t.Effect != nil {
		return t.Effect
	}
	if f != nil {
		return cards.ResolveSVar(f.SVars, strings.TrimSpace(t.ParamStr(cards.PKExecute)))
	}
	return nil
}

// PaymentPlanTapDamageTrigger is spec §3.2's foreign damage trigger row: a
// `Mode$ Taps` trigger on any ValidCard$ whose effect is exactly
// `DB$ DealDamage | Defined$ <player> | NumDmg$ <literal>` plus at most the
// DamageSource provenance key (Hawkeye's Bow's "whenever equipped creature
// becomes tapped, it deals 1 damage to each opponent"). The trigger resolves
// after the payment that caused it (CR 603.2) and changes no payment
// resource, so a payment planning a tap that fires it need only disclose the
// literal amount when the damage can reach the payer itself
// (PaymentPlanTapDamageHits): otherwise the tap stays an ordinary unit and
// the trigger fires on its own at the tap, on both engines. A trigger whose
// shape the planner cannot vet (FirstTime$, a card referent, a sub-chain, a
// non-literal amount) is not known and its source stays deferred. f owns the
// trigger's Execute$ table.
func PaymentPlanTapDamageTrigger(f *cards.Face, t cards.Trigger) (uint32, params.Ref, bool) {
	if t.Mode != "Taps" || !paymentPlanTriggerZoneOK(t) {
		return 0, params.Ref{}, false
	}
	for _, k := range slices.Sorted(maps.Keys(t.Params)) {
		switch paymentPlanTapDamageTriggerCodes.Code(string(k)) {
		case paymentPlanTapDamageTriggerKnown:
		default:
			return 0, params.Ref{}, false
		}
	}
	body := paymentPlanTriggerBody(f, t)
	n, ok := paymentPlanPlayerDamageBody(body)
	if !ok {
		return 0, params.Ref{}, false
	}
	return n, params.DefinedRefOf(body), true
}

// PaymentPlanTapDamageHits reports whether a vetted tap-damage trigger's
// referent can name the player paying for the tap: the payer's life is the
// only payment resource the damage could consume. Defined$ You names the
// trigger's controller; Opponent/EachOpponent name the controller's
// opponent(s), so the payer is hit only when it is not the controller
// (conservative in multiplayer, where "opponent" is every opponent);
// TriggeredCardController names the tapped permanent's controller. Anything
// else is conservatively assumed to reach the payer.
func PaymentPlanTapDamageHits(ref params.Ref, carrierCtrl, activator, tappedCtrl state.PlayerID) bool {
	switch {
	case ref.Kind == params.RefYou:
		return activator == carrierCtrl
	case ref.Text == "Opponent" || ref.Text == "EachOpponent":
		return activator != carrierCtrl
	case ref.Text == "TriggeredCardController":
		return activator == tappedCtrl
	default:
		return true
	}
}

// PaymentPlanTapDestroySelfTrigger is the destroy-self trigger row: a
// `Mode$ Taps` trigger on any ValidCard$ whose effect is exactly
// `DB$ Destroy | Defined$ TriggeredCardLKICopy` (Brink of Disaster's "when
// enchanted permanent becomes tapped, destroy it"). The trigger resolves
// after the payment that caused it (CR 405.6c/603.2) and its only effect is
// to destroy the permanent whose tap already produced its mana (CR 605.4a),
// so it touches no payment resource and the tapped source stays an ordinary
// unit. A trigger whose shape the planner cannot vet (another selector, a
// compound one, a sub-chain, extra body keys) is not known and its source
// stays deferred. f owns the trigger's Execute$ table.
func PaymentPlanTapDestroySelfTrigger(f *cards.Face, t cards.Trigger) bool {
	if t.ModeKind() != cards.TriggerTaps || !paymentPlanTriggerZoneOK(t) {
		return false
	}
	for _, k := range slices.Sorted(maps.Keys(t.Params)) {
		switch paymentPlanTapDamageTriggerCodes.Code(string(k)) {
		case paymentPlanTapDamageTriggerKnown:
		default:
			return false
		}
	}
	body := paymentPlanTriggerBody(f, t)
	if body == nil || body.APIKind() != cards.APIDestroy || !paymentPlanSubFree(body) {
		return false
	}
	ref := params.DefinedRefOf(body)
	if ref.Flags&params.RefCompound != 0 || !slices.Contains(paymentPlanDestroySelfRefs, ref.Text) {
		return false
	}
	for _, k := range slices.Sorted(maps.Keys(body.Params)) {
		if !paymentPlanDestroySelfBodyKey(k) {
			return false
		}
	}
	return true
}

// paymentPlanDestroySelfRefs are the Defined$ selectors that name the tapped
// permanent itself: the LKI copy and the plain card. Anything else (another
// referent, a compound selector) is not the destroy-self shape.
var paymentPlanDestroySelfRefs = []string{"TriggeredCardLKICopy", "TriggeredCard"}

// paymentPlanAPIKey is the dispatch key some constructed abilities keep in
// Params beside SA.API. It has no ParamKey constant because the parser pulls
// the DB$/SP$ head into SA.API.
const paymentPlanAPIKey = "API"

// paymentPlanDestroySelfBodyKey reports whether key is part of the closed
// destroy-self body: the dispatch key and the two descriptive captions. The
// vocabulary constants keep the check in the ParamKey spelling rather than a
// literal comparison chain.
func paymentPlanDestroySelfBodyKey(key string) bool {
	switch key {
	case paymentPlanAPIKey, cards.PKDefined.String(), cards.PKSpellDescription.String(), cards.PKStackDescription.String():
		return true
	}
	return false
}

// paymentPlanPoisonBodyKey reports whether key is part of the closed poison
// body: the dispatch key, the referent, the counter amount (unread: the
// trigger fires after the payment whatever it counts) and the two descriptive
// captions.
func paymentPlanPoisonBodyKey(key string) bool {
	switch key {
	case paymentPlanAPIKey, cards.PKDefined.String(), cards.PKNum.String(), cards.PKSpellDescription.String(), cards.PKStackDescription.String():
		return true
	}
	return false
}

// PaymentPlanTapPoisonTrigger is the poison trigger row: a `Mode$ Taps`
// trigger on any ValidCard$ whose effect is exactly `DB$ Poison |
// Defined$ <player>` (Relic Putrescence's "whenever enchanted artifact
// becomes tapped, its controller gets a poison counter"). Like the
// destroy-self row the trigger resolves after the payment that caused it (CR
// 405.6c/603.2) and its only effect is a poison counter on a player the
// payment cannot move, so it touches no payment resource and the tapped
// source stays an ordinary unit; the trigger fires on its own at the tap, on
// both engines. The body's Num$ is not read: the amount the effect applies
// changes nothing about the payment that triggered it. A trigger whose shape
// the planner cannot vet (another referent, a compound one, a sub-chain, an
// extra body key) is not known and its source stays deferred. f owns the
// trigger's Execute$ table.
func PaymentPlanTapPoisonTrigger(f *cards.Face, t cards.Trigger) bool {
	if t.ModeKind() != cards.TriggerTaps || !paymentPlanTriggerZoneOK(t) {
		return false
	}
	for _, k := range slices.Sorted(maps.Keys(t.Params)) {
		switch paymentPlanTapDamageTriggerCodes.Code(string(k)) {
		case paymentPlanTapDamageTriggerKnown:
		default:
			return false
		}
	}
	body := paymentPlanTriggerBody(f, t)
	if body == nil || body.API != "Poison" || !paymentPlanSubFree(body) {
		return false
	}
	ref := params.DefinedRefOf(body)
	if ref.Flags&params.RefCompound != 0 {
		return false
	}
	switch {
	case ref.Kind == params.RefYou:
	case ref.Text == "Opponent" || ref.Text == "EachOpponent" || ref.Text == "TriggeredCardController":
	default:
		return false
	}
	for _, k := range slices.Sorted(maps.Keys(body.Params)) {
		if !paymentPlanPoisonBodyKey(k) {
			return false
		}
	}
	return true
}

// paymentPlanPlayerDamageBody is paymentPlanDamageBody over a player
// referent: the same literal-amount DealDamage body whose Defined$ names a
// player the payment cannot move (the payer, an opponent, the tapped
// permanent's controller) rather than a card a later plan step could remove.
func paymentPlanPlayerDamageBody(rider *cards.SA) (uint32, bool) {
	if rider == nil || rider.API != "DealDamage" || !paymentPlanSubFree(rider) {
		return 0, false
	}
	switch ref := params.DefinedRefOf(rider); {
	case ref.Kind == params.RefYou:
	case ref.Text == "Opponent" || ref.Text == "EachOpponent" || ref.Text == "TriggeredCardController":
	default:
		return 0, false
	}
	n, ok := paymentPlanBodyNumDmg(rider)
	if !ok {
		return 0, false
	}
	for _, k := range slices.Sorted(maps.Keys(rider.Params)) {
		switch k {
		case "API", "Defined", "NumDmg", "DamageSource", "SpellDescription", "StackDescription":
		default:
			return 0, false
		}
	}
	return n, true
}

// PlanTapMana is the deterministic extra mana one static TapsForMana trigger
// adds when its carrier's matcher accepts a planned tap (Forsaken Monument,
// Badgermole Cub, Crypt Ghast, Wild Growth). ToTapped records Defined$
// TriggeredCardController (the mana goes to the tapped permanent's controller
// rather than the trigger controller).
type PlanTapMana struct {
	Mana     state.Mana
	ToTapped bool
}

// PaymentPlanFoldableTapMana classifies a `Mode$ TapsForMana | Static$ True`
// trigger of face f as a fixed additional-mana body the planner can fold into
// a source's production: closed-world over the trigger's parameters and a
// single `DB$ Mana` body naming one literal symbol and a literal amount, with
// no SubAbility$, cost or other parameter. Anything else is not foldable and
// the caller keeps deferring.
func PaymentPlanFoldableTapMana(f *cards.Face, t cards.Trigger) (PlanTapMana, bool) {
	body := paymentPlanStaticTapManaBody(f, t, "Mana")
	if body == nil {
		return PlanTapMana{}, false
	}
	for _, k := range slices.Sorted(maps.Keys(body.Params)) {
		switch k {
		case "API", "Produced", "Amount", "Defined", "SpellDescription", "StackDescription":
		default:
			return PlanTapMana{}, false
		}
	}
	var out PlanTapMana
	switch ref := params.DefinedRefOf(body); {
	case ref.Kind == params.RefAbsent || ref.Kind == params.RefYou:
	case ref.Text == "TriggeredCardController":
		out.ToTapped = true
	default:
		return PlanTapMana{}, false
	}
	mp := params.ManaOf(body)
	raw := mp.Produced
	if len(raw) != 1 || !strings.Contains("WUBRGC", raw) {
		return PlanTapMana{}, false
	}
	n := 1
	if mp.Amount.Present {
		if !mp.AmountIsLit || mp.AmountLit <= 0 || mp.AmountLit > 20 {
			return PlanTapMana{}, false
		}
		n = mp.AmountLit
	}
	counts, anyColour := cards.ProducedCounts(raw)
	if anyColour {
		return PlanTapMana{}, false
	}
	for i, c := range counts {
		out.Mana[i] = c * int32(n)
	}
	return out, true
}

// paymentPlanStaticTapManaBody is the body of a `Mode$ TapsForMana |
// Static$ True` battlefield trigger of face f whose parameters are all in the
// closed world paymentPlanTapManaTriggerCodes names and whose body is one api
// ability with no SubAbility$, or nil.
func paymentPlanStaticTapManaBody(f *cards.Face, t cards.Trigger, api string) *cards.SA {
	if t.Mode != "TapsForMana" || !strings.EqualFold(strings.TrimSpace(t.ParamStr(cards.PKStatic)), "True") {
		return nil
	}
	if v, ok := t.Param(cards.PKTriggerZones); ok && strings.TrimSpace(v) != "Battlefield" {
		return nil
	}
	for _, k := range slices.Sorted(maps.Keys(t.Params)) {
		if paymentPlanTapManaTriggerCodes.Code(string(k)) == 0 {
			return nil
		}
	}
	body := t.Effect
	if body == nil && f != nil {
		body = cards.ResolveSVar(f.SVars, strings.TrimSpace(t.ParamStr(cards.PKExecute)))
	}
	if body == nil || body.API != api || body.Sub != nil || strings.TrimSpace(body.ParamStr(cards.PKSubAbility)) != "" {
		return nil
	}
	return body
}

// PaymentPlanFoldableManaAdded classifies a `Mode$ ManaAdded | Static$ True`
// trigger of face f whose body adds one mana of the trigger source's own
// chosen colour (Caged Sun's `DB$ Mana | Produced$ Chosen | Amount$ 1`, the
// CR 605.1b triggered mana ability), closed-world as
// PaymentPlanFoldableTapMana. The mana is object state
// (state.Object.ChosenColor), so the classifier only vets the SHAPE and the
// caller reads the carrier's chosen colour, crediting one unit per ManaAdd
// batch the production emits in that colour -- the batch cadence effMana
// and the real resolution agree on. Player$ You is required: the bonus goes
// to the carrier's controller, which is who the fold credits. A non-foldable
// ManaAdded line defers its carrier instead, the direction every unvetted
// observer takes. The zone question is not re-vetted here: the planner only
// classifies lines the real matcher has already matched, and the matchers
// own the zone gates.
func PaymentPlanFoldableManaAdded(f *cards.Face, t cards.Trigger) bool {
	body := paymentPlanStaticManaAddedBody(f, t, "Mana")
	if body == nil {
		return false
	}
	if !cards.ManaBodyKeysAdmitted(body) {
		return false
	}
	switch ref := params.DefinedRefOf(body); ref.Kind {
	case params.RefAbsent, params.RefYou:
	default:
		return false
	}
	mp := params.ManaOf(body)
	if !cards.ProducedIsChosen(mp.Produced) {
		return false
	}
	if mp.Amount.Present && (!mp.AmountIsLit || mp.AmountLit != 1) {
		return false
	}
	return true
}

// paymentPlanStaticManaAddedBody is the body of a `Mode$ ManaAdded |
// Static$ True` trigger of face f whose parameters are all in the closed
// world paymentPlanManaAddedLineKeys names, whose Produced$ names the
// trigger source's own chosen colour and whose Player$ You binds the bonus
// to the carrier's controller; its body is one api ability with no
// SubAbility$, or nil.
func paymentPlanStaticManaAddedBody(f *cards.Face, t cards.Trigger, api string) *cards.SA {
	if t.ModeKind() != cards.TriggerManaAdded || !t.StaticTrue() || !t.ProducedChosenColour() || !t.PlayerIsYou() {
		return nil
	}
	if !t.ManaAddedLineKeysAdmitted() {
		return nil
	}
	body := t.Effect
	if body == nil && f != nil {
		// parseParams trims every parameter value at load, so the stored
		// SVar value is resolved directly.
		body = cards.ResolveSVar(f.SVars, t.ParamStr(cards.PKExecute))
	}
	if body == nil || body.API != api || body.Sub != nil || body.ParamStr(cards.PKSubAbility) != "" {
		return nil
	}
	return body
}

// PaymentPlanReflectedTapMana classifies a static TapsForMana trigger of
// face f whose body adds one mana of the type the tap produced (a
// ManaReflected of the produced type: Lavaleaper, Mana Flare; CR 106.12a,
// 605.1b), closed-world as
// PaymentPlanFoldableTapMana. Its mana is a read of the planned production,
// not a literal: PaymentPlanReflectedMana sizes it per production. ToTapped
// records a Defined$ naming the tapping player (TriggeredActivator) or the
// tapped permanent's controller (TriggeredCardController): either is the
// payer of a planned tap.
func PaymentPlanReflectedTapMana(f *cards.Face, t cards.Trigger) (PlanTapMana, bool) {
	body := paymentPlanStaticTapManaBody(f, t, "ManaReflected")
	if body == nil {
		return PlanTapMana{}, false
	}
	for _, k := range slices.Sorted(maps.Keys(body.Params)) {
		switch k {
		case "API", "ColorOrType", "ReflectProperty", "Defined", "SpellDescription", "StackDescription":
		default:
			return PlanTapMana{}, false
		}
	}
	mr := effects.ManaReflectedOf(body)
	if mr.ReflectProperty != "Produced" || !mr.WidenType {
		return PlanTapMana{}, false
	}
	var out PlanTapMana
	switch mr.Defined {
	case "", "You":
	case "TriggeredActivator", "TriggeredCardController":
		out.ToTapped = true
	default:
		return PlanTapMana{}, false
	}
	return out, true
}

// PaymentPlanReflectedMana is the mana a PaymentPlanReflectedTapMana body
// adds to a tap that produced `produced` (a PaymentPlanProductions entry):
// one mana of its type when it names exactly one type. A production of
// several types leaves the body a choice (effManaReflected's ask), which
// is not foldable.
func PaymentPlanReflectedMana(produced string) (state.Mana, bool) {
	var out state.Mana
	counts, anyColour := cards.ProducedCounts(produced)
	if anyColour {
		return out, false
	}
	n := 0
	for i, c := range counts {
		if c > 0 {
			n++
			out[i] = 1
		}
	}
	return out, n == 1
}

type paymentPlanTapManaTriggerCode uint16

var paymentPlanTapManaTriggerCodes = state.NewStrCodes(
	state.StrEntry[paymentPlanTapManaTriggerCode]{Key: "Mode", Val: 1},
	state.StrEntry[paymentPlanTapManaTriggerCode]{Key: "ValidCard", Val: 1},
	state.StrEntry[paymentPlanTapManaTriggerCode]{Key: "Activator", Val: 1},
	state.StrEntry[paymentPlanTapManaTriggerCode]{Key: "Produced", Val: 1},
	state.StrEntry[paymentPlanTapManaTriggerCode]{Key: "Execute", Val: 1},
	state.StrEntry[paymentPlanTapManaTriggerCode]{Key: "Static", Val: 1},
	state.StrEntry[paymentPlanTapManaTriggerCode]{Key: "TriggerZones", Val: 1},
	state.StrEntry[paymentPlanTapManaTriggerCode]{Key: "TriggerDescription", Val: 1},
)

// PaymentPlanAlive mirrors the engine walks' AliveFrom scope: an object in a
// lost player's zones is not walked for triggers or replacements.
func PaymentPlanAlive(e Engine, o *state.Object) bool {
	holder := o.Owner
	if o.Zone == state.ZBattlefield {
		holder = o.Controller
	}
	return int(holder) < len(e.Game().Players) && !e.Game().Players[holder].Lost
}

// FaceGrantsSunburstForPlan reports whether f carries a keyword GRANT naming
// Sunburst: a Keywords$/KW$/AddKeyword$ value in any ability, trigger body,
// replacement body, SVar or static. The face's own printed keywords are not
// grants.
func FaceGrantsSunburstForPlan(f *cards.Face) bool {
	// Cheap gate first: nearly every face never mentions the word.
	if f == nil || !f.Mentions("Sunburst") {
		return false
	}
	grantKey := func(k string) bool {
		return k == "Keywords" || k == "KW" || k == "AddKeyword" || k == "AddKeywords"
	}
	grantValue := func(v string) bool {
		for kw := range strings.SplitSeq(v, "&") {
			if strings.HasPrefix(strings.TrimSpace(kw), "Sunburst") {
				return true
			}
		}
		return false
	}
	grants := func(params map[string]string) bool {
		for _, k := range [...]string{"Keywords", "KW", "AddKeyword", "AddKeywords"} {
			if grantValue(params[k]) {
				return true
			}
		}
		return false
	}
	var walk func(sa *cards.SA, depth int) bool
	walk = func(sa *cards.SA, depth int) bool {
		return sa != nil && depth <= 32 && (grants(sa.Params) || walk(sa.Sub, depth+1))
	}
	for _, a := range f.Abilities {
		if walk(a, 0) {
			return true
		}
	}
	for _, t := range f.Triggers {
		if walk(t.Effect, 0) {
			return true
		}
	}
	for _, r := range f.Repls {
		if walk(r.With, 0) {
			return true
		}
	}
	for _, st := range f.Statics {
		if grants(st.Params) {
			return true
		}
	}
	// SVar bodies are raw `Key$ Value | ...` text: read the grant keys off
	// their segments without parsing whole abilities. The answer is a plain
	// any-of, so the map's iteration order cannot matter.
	for _, raw := range f.SVars {
		for seg := range strings.SplitSeq(raw, "|") {
			if k, v, ok := strings.Cut(strings.TrimSpace(seg), "$"); ok && grantKey(strings.TrimSpace(k)) && grantValue(v) {
				return true
			}
		}
	}
	return false
}

// PaymentPlanSpellTargets reports whether casting f's ordinary spell
// announces a target (CR 601.2c): an Aura spell (CR 303.4a), a mutating
// creature spell (CR 702.140a), or a spell ability whose chain -- its
// SubAbility$ links and a Charm's Choices$ modes -- declares a target. It
// fails closed (true) on a nil face.
func PaymentPlanSpellTargets(f *cards.Face) bool {
	if f == nil {
		return true
	}
	if slices.Contains(f.Types, "Aura") || f.HasKeywordRef(kwRefEnchant) {
		return true
	}
	if _, ok := f.KeywordParam("Enchant"); ok {
		return true
	}
	if _, ok := f.KeywordParam("Mutate"); ok {
		return true
	}
	targets := func(sa *cards.SA) bool {
		return params.TargetsOf(sa).Has(params.TgtDeclares)
	}
	var walk func(sa *cards.SA, depth int) bool
	walk = func(sa *cards.SA, depth int) bool {
		if sa == nil || depth > 32 {
			return false
		}
		if targets(sa) || walk(sa.Sub, depth+1) {
			return true
		}
		if name := strings.TrimSpace(sa.ParamStr(cards.PKSubAbility)); name != "" && sa.Sub == nil {
			if walk(cards.ResolveSVar(f.SVars, name), depth+1) {
				return true
			}
		}
		for mode := range strings.SplitSeq(sa.ParamStr(cards.PKChoices), ",") {
			if mode = strings.TrimSpace(mode); mode != "" && walk(cards.ResolveSVar(f.SVars, mode), depth+1) {
				return true
			}
		}
		return false
	}
	return walk(f.SpellAbility(), 0)
}

var paymentPlanNoUntapShapeKeys = state.NewNameSet("Event", "ValidCard", "Layer", "ValidStepTurnToController", "ActiveZones", "Description")

type paymentPlanSelfDamageTriggerCode uint16

const (
	paymentPlanSelfDamageTriggerKnown paymentPlanSelfDamageTriggerCode = iota + 1
)

var paymentPlanSelfDamageTriggerCodes = state.NewStrCodes(
	state.StrEntry[paymentPlanSelfDamageTriggerCode]{Key: "Mode", Val: paymentPlanSelfDamageTriggerKnown},
	state.StrEntry[paymentPlanSelfDamageTriggerCode]{Key: "ValidCard", Val: paymentPlanSelfDamageTriggerKnown},
	state.StrEntry[paymentPlanSelfDamageTriggerCode]{Key: "Execute", Val: paymentPlanSelfDamageTriggerKnown},
	state.StrEntry[paymentPlanSelfDamageTriggerCode]{Key: "TriggerZones", Val: paymentPlanSelfDamageTriggerKnown},
	state.StrEntry[paymentPlanSelfDamageTriggerCode]{Key: "TriggerDescription", Val: paymentPlanSelfDamageTriggerKnown},
)

type paymentPlanTapDamageTriggerCode uint16

const (
	paymentPlanTapDamageTriggerKnown paymentPlanTapDamageTriggerCode = iota + 1
)

// paymentPlanTapDamageTriggerCodes is the closed key set of the Taps-watcher
// trigger rows the planner vets (the tap-damage row and, sharing it, the
// destroy-self row: both watchers carry Mode$, ValidCard$, Execute$,
// TriggerZones$ and TriggerDescription$ and nothing else).
var paymentPlanTapDamageTriggerCodes = state.NewStrCodes(
	state.StrEntry[paymentPlanTapDamageTriggerCode]{Key: "Mode", Val: paymentPlanTapDamageTriggerKnown},
	state.StrEntry[paymentPlanTapDamageTriggerCode]{Key: "ValidCard", Val: paymentPlanTapDamageTriggerKnown},
	state.StrEntry[paymentPlanTapDamageTriggerCode]{Key: "Execute", Val: paymentPlanTapDamageTriggerKnown},
	state.StrEntry[paymentPlanTapDamageTriggerCode]{Key: "TriggerZones", Val: paymentPlanTapDamageTriggerKnown},
	state.StrEntry[paymentPlanTapDamageTriggerCode]{Key: "TriggerDescription", Val: paymentPlanTapDamageTriggerKnown},
)

// UnlessManaReachable reports whether pool plus one production alternative
// per window unit (or none of a unit) can satisfy cost's mana/life component.
// It is the exact affordability question the offer gate and the window's
// safety ordering both ask: a unit with several free abilities offers its
// alternatives as a CHOICE (the payer taps the permanent for one of them),
// never as their sum, so a dual land contributes {U} or {R} -- and the
// search proves a {U} tax payable from it. An alt whose activation carries a
// literal mana fee pays the fee from the floating mana BEFORE its production
// is added (CR 605.3b), so an earlier same-window activation can fund it --
// Apprentice Wizard's "{U}, {T}: Add {C}{C}{C}" beside an Island's {U}. A
// fee the floating mana cannot cover leaves the alt out of that branch. The
// search is bounded by manaPipCount plus the fees the units pay (a minimal
// cover never needs more sources than the mana it supplies) and by a hard
// node budget; beyond that it fails closed, which is the conservative
// direction (never offer a Pay the window cannot complete).
func UnlessManaReachable(e Engine, p state.PlayerID, cost costvocab.Cost, pool, snow state.Mana, typed [8]state.Mana, life int32, conv *Conv, units []WindowUnit) bool {
	// CR 119.7/119.8: under a CantChangeLife static no positive life may be
	// paid, so neither the charge's own fixed life nor a PayLife source's
	// activation is reachable -- exactly the solver's answer for a payer
	// with 0 life (a fixed life it lacks plus the "leave the payer alive"
	// alt guard), taken as ONE read instead of threading a new parameter
	// through every caller.
	if e.Eval().LifePaymentForbidden(p, SAOther) {
		life = 0
	}
	payable := func(pool state.Mana, lifeNow int32) bool {
		_, ok := ResolveManaWith(cost, pool, snow, typed, lifeNow,
			e.Eval().PayLifeInsteadOfB(p), PipRider{}, conv)

		return ok
	}
	if payable(pool, life) {
		return true
	}
	budget := cost.ManaPipCount()
	for i := range units {
		budget += int(unitMaxFee(units[i]))
	}
	// The walk takes units in index order, so a fee-carrying unit listed
	// before the source that funds its fee would never see that mana (a Mire's
	// {1} beside a later Bridge). The payer taps in any order (askUnlessMana),
	// so price the fee-free units first: a stable sort keeps the census order
	// within each class and the answer deterministic.
	units = slices.Clone(units)
	slices.SortStableFunc(units, func(a, b WindowUnit) int { return int(unitMaxFee(a)) - int(unitMaxFee(b)) })
	if budget <= 0 {
		return false
	}
	nodes := 0
	var rec func(start, remaining int, acc state.Mana, lifeLeft int32) bool
	rec = func(start, remaining int, acc state.Mana, lifeLeft int32) bool {
		if payable(ManaAdd(pool, acc), lifeLeft) {
			return true
		}
		if remaining <= 0 {
			return false
		}
		nodes++
		if nodes > 1<<18 {
			return false
		}
		for i := start; i < len(units); i++ {
			for _, a := range units[i].Alts {
				// A PayLife activation spends real life: it must be
				// present before the tap and is gone for the priced cost
				// afterwards. Every shared-window alt carries life 0, so
				// this is inert for the attack and unless windows. A
				// zero-debit alt is unaffected by the payer's life total,
				// even when the payer has been carried below 0.
				if (a.Life > 0 && a.Life > lifeLeft) || (a.LifeKeep && a.Life >= lifeLeft) {
					continue
				}
				// A literal mana fee is charged from the floating mana
				// (the base pool plus every earlier activation) before
				// this activation resolves (CR 605.3b), so the alt only
				// joins a branch whose accumulated mana can fund it. The
				// base pool may itself fund the fee, which is why the
				// debit lands on the pool-relative delta: the walk prices
				// every later payable() against pool plus the delta.
				next := acc
				if fee := a.Fee(); fee > 0 {
					floating := ManaAdd(pool, acc)
					funded, ok := feeFromMana(a, floating, conv)
					if !ok {
						continue
					}
					tryFee := func(afterFee state.Mana) bool {
						return rec(i+1, remaining-1, ManaAdd(ManaSub(afterFee, pool), a.Mana()), lifeLeft-a.Life)
					}
					found := tryFee(funded)
					if !found && a.CostGeneric > 0 && manaCovers(floating, a.CostColored) {
						// The fee's colours are the payer's choice (CR 601.2h
						// through CR 602.2b):
						// ResolveMana's deduction spends the first available
						// colour on the generic part, which may be a colour the
						// unless cost still needs (Overflowing Basin's {1} beside
						// a {W}{U}{B}{R}{G} tax: spending the Plains' {W} leaves
						// no white, while the second Mountain pays it just as
						// well). Try every other split of the unrestricted pool
						// before dropping the activation, exactly as the cast
						// window's reach walk does.
						found = forEachFeeSplit(ManaSub(floating, a.CostColored), a.CostGeneric, func(afterFee state.Mana) bool {
							return afterFee != funded && tryFee(afterFee)
						})
					}
					if found {
						return true
					}
					continue
				}
				if rec(i+1, remaining-1, ManaAdd(next, a.Mana()), lifeLeft-a.Life) {
					return true
				}
			}
		}
		return false
	}
	return rec(0, budget, state.Mana{}, life)
}

// unitMaxFee is the largest literal mana fee among a window unit's alts.
func unitMaxFee(u WindowUnit) int32 {
	fee := int32(0)
	for _, a := range u.Alts {
		fee = max(fee, a.Fee())
	}
	return fee
}

// feeFromMana pays one activation's literal mana fee (WindowAlt.CostGeneric
// beside WindowAlt.CostColored) out of the floating mana under this
// payment's conversion set, and returns the mana left. The fee is charged
// from the pool before the ability resolves (CR 605.3b), so mana an earlier
// same-window activation produced can fund it; snow and typed batches are
// deliberately not consulted, a conservative miss (the live window can fund
// a fee from them, the census just does not promise one it does).
func feeFromMana(a WindowAlt, pool state.Mana, conv *Conv) (state.Mana, bool) {
	paid, ok := ResolveMana(costvocab.Cost{Generic: a.CostGeneric, Colored: a.CostColored},
		pool, state.Mana{}, [8]state.Mana{}, 0, conv)
	if !ok {
		return state.Mana{}, false
	}
	// Payment.Pool is the pool the fee LEFT (solve.go's Payment carries the
	// remaining floating mana, spent = before - after); returning it directly
	// prices every later branch against the mana the fee did not consume.
	return paid.Pool, true
}

// manaCovers reports whether pool holds every unit of need, colour by colour.
func manaCovers(pool, need state.Mana) bool {
	for i := range need {
		if pool[i] < need[i] {
			return false
		}
	}
	return true
}

// forEachFeeSplit calls try with pool less each way of spending n generic
// mana from it, in colour order, until try accepts one. The generic part of a
// mana-ability activation fee is the payer's to assign (CR 601.2h), so a
// reach walk must not let ResolveMana's fixed deduction order decide which
// colour is spent: a colour the priced cost still needs can be the only one
// the fee may take (CR 605.3a lets the payer activate the fee's source while
// a rule asks for the mana payment).
func forEachFeeSplit(pool state.Mana, n int32, try func(state.Mana) bool) bool {
	var walk func(from int, left int32, cur state.Mana) bool
	walk = func(from int, left int32, cur state.Mana) bool {
		if left == 0 {
			return try(cur)
		}
		for c := from; c < len(cur); c++ {
			if cur[c] > 0 {
				cur[c]--
				if walk(c, left-1, cur) {
					return true
				}
				cur[c]++
			}
		}
		return false
	}
	return walk(0, n, pool)
}
