package kgen

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// tapExtraStatic lowers a `Mode$ TapsForMana | Static$ True` trigger to the
// StatTapExtra static when gorge's planner folds it into a tap's production
// (pay.PaymentPlanFoldableTapMana: one literal symbol and amount, no other
// parameter), or a reflected body over lands (Lavaleaper, Mana Flare:
// pay.PaymentPlanReflectedTapMana, one mana of the type the tap produced).
// A body whose colour is not a literal gets its own model: an any-colour body
// asks a colour choice at tap time (tapExtraAnyColour), a chosen-colour body is
// a source gorge's planner defers (tapExtraChosen), and a count-scaled literal
// (Elvish Guidance: Produced$ G | Amount$ X, X Count$Valid Elf) is a dynamic
// extra (tapExtraDynamic).
func (lc *lowerCtx) tapExtraStatic(t cards.Trigger) error {
	if handled, err := lc.tapExtraChosen(t); handled {
		return err
	}
	if handled, err := lc.tapExtraAnyColour(t); handled {
		return err
	}
	if handled, err := lc.tapExtraDynamic(t); handled {
		return err
	}
	cond, gated, gerr := lc.tapExtraGate(t)
	if gerr != nil {
		return gerr
	}
	if gated {
		return lc.tapExtraGated(t, cond)
	}
	tm, ok := pay.PaymentPlanFoldableTapMana(lc.face, t)
	if !ok {
		return lc.tapExtraReflected(t)
	}
	tp := newParams(t.Params)
	tp.consume("Mode", "Execute", "Static", "TriggerZones", "TriggerDescription")
	st := lStatic{Kind: "TapExtra"}
	// A body adding mana to the tapped permanent's controller (Defined$
	// TriggeredCardController) reaches every tap of that permanent, not just
	// the trigger controller's: the tapped permanent's controller is the only
	// player who can tap it, so the extra is the tapper's (TapAnyPlayer). A
	// body adding to the trigger controller is its own taps only.
	switch v := strings.TrimSpace(tp.str("Activator")); {
	case v == "You" && !tm.ToTapped:
	case v == "" && tm.ToTapped:
		st.Flags = append(st.Flags, "TapAnyPlayer")
	default:
		return refuse("TapsForMana Activator$ %q with Defined$ to the tapped permanent's controller %v", v, tm.ToTapped)
	}
	tp.consume("Activator")
	var err error
	if v := tp.str("ValidCard"); v != "" {
		if st.F, err = lc.filter(v); err != nil {
			return err
		}
		tp.consume("ValidCard")
	}
	if st.F == nil {
		return refuse("TapsForMana without ValidCard$")
	}
	switch v := strings.TrimSpace(tp.str("Produced")); v {
	case "":
	case "C":
		st.Flags = append(st.Flags, "TapOnC")
		tp.consume("Produced")
	default:
		return refuse("TapsForMana Produced$ %q", v)
	}
	n := 0
	for i, c := range tm.Mana {
		if c > 0 {
			n++
			st.TapCol, st.TapAmt = uint8(i), uint8(c)
		}
	}
	if n != 1 {
		return refuse("TapsForMana adding several colours")
	}
	lc.out.Statics = append(lc.out.Statics, st)
	return nil
}

// tapExtraGate parses t's IsPresent$ as the gated tap-extra's shape:
// `Card.Self+counters_GE<N>_<KIND>`, N a literal 1..127 and KIND a counter
// kind, with no PresentCompare$ beyond the default GE1, no PresentDefined$ and
// no PresentZone$ (a battlefield cPresent, kop.Static.Cond2). gated is false
// when IsPresent$ is absent; err refuses every present-but-other shape.
func (lc *lowerCtx) tapExtraGate(t cards.Trigger) (lCond, bool, error) {
	ip, ok := t.Param(cards.PKIsPresent)
	if !ok {
		return lCond{}, false, nil
	}
	if v, ok := t.Param(cards.PKPresentCompare); ok && strings.TrimSpace(v) != "GE1" {
		return lCond{}, true, refuse("TapsForMana IsPresent$ %q with PresentCompare$ %q", ip, v)
	}
	if _, ok := t.Param(cards.PKPresentDefined); ok {
		return lCond{}, true, refuse("TapsForMana IsPresent$ %q with PresentDefined$", ip)
	}
	if _, ok := t.Param(cards.PKPresentZone); ok {
		return lCond{}, true, refuse("TapsForMana IsPresent$ %q with PresentZone$", ip)
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(ip), "Card.Self+counters_GE")
	if !ok {
		return lCond{}, true, refuse("TapsForMana IsPresent$ %q", ip)
	}
	thr, kind, ok := strings.Cut(rest, "_")
	if !ok {
		return lCond{}, true, refuse("TapsForMana IsPresent$ %q", ip)
	}
	n, err := strconv.Atoi(thr)
	if err != nil || n < 1 || n > 127 {
		return lCond{}, true, refuse("TapsForMana IsPresent$ %q (threshold %q)", ip, thr)
	}
	if _, err := counterKind(kind); err != nil {
		return lCond{}, true, refuse("TapsForMana IsPresent$ %q (%v)", ip, err)
	}
	f, err := lc.filter(strings.TrimSpace(ip))
	if err != nil {
		return lCond{}, true, err
	}
	return lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}, true, nil
}

// tapExtraGated lowers a foldable literal tap-extra body under an IsPresent$
// counter gate (Rikala, Homarid King: "whenever you tap an Island for mana, if
// there are three or more tide counters on Rikala, add an additional {U}").
//
// gorge's planner cannot fold a gated trigger: the gate is no
// paymentPlanTapManaTriggerCodes key, so paymentPlanStaticTapManaBody rejects
// the line and paymentPlanTapObservers' real matcher instead classes every
// Island the gate reaches as TierDeferred (deferBy), with the potential bound
// (paymentPlanTapExtraMana, whole=true) summing only folded triggers. The
// kernel mirrors both halves: the static carries the gate as lStatic.Cond
// (kop.Static.Cond2) and a class-only Taps line (kop.Tf5TapsForMana with no body) whose
// Trigger.Cond is the same gate defers the same sources
// (mana_tapsdefer.go tapsWatchClass) while the gate holds; the extra itself is
// never priced into the census or the potential (mana_tapextra.go
// tapExtraReaches skips a Cond2-gated static) and floats at the tap itself
// (addGatedExtra), exactly the trigger's resolution at the tap.
func (lc *lowerCtx) tapExtraGated(t cards.Trigger, cond lCond) error {
	// gorge's classifier over the line without the gate decides the body's
	// shape: only the foldable literal lowers here, anything else keeps the
	// generic refusal (a reflected or chosen body under a gate is no shape
	// this model holds).
	stripped := t
	stripped.Params = maps.Clone(t.Params)
	delete(stripped.Params, "IsPresent")
	tm, ok := pay.PaymentPlanFoldableTapMana(lc.face, stripped)
	if !ok {
		return refuse("trigger mode TapsForMana")
	}
	tp := newParams(t.Params)
	tp.consume("Mode", "Execute", "Static", "TriggerZones", "TriggerDescription", "IsPresent")
	// Activator$ You, the trigger controller's own taps: the deferral and the
	// extra are the controller's (Islands of Rikala's controller). A body
	// naming the tapped permanent's controller stays the generic refusal.
	if v := strings.TrimSpace(tp.str("Activator")); v != "You" || tm.ToTapped {
		return refuse("trigger mode TapsForMana")
	}
	tp.consume("Activator")
	var err error
	vc := strings.TrimSpace(tp.str("ValidCard"))
	if vc == "" {
		return refuse("trigger mode TapsForMana")
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	tp.consume("ValidCard")
	if len(tp.unread()) != 0 {
		return refuse("trigger mode TapsForMana")
	}
	st := lStatic{Kind: "TapExtra", Cond: cond, F: f}
	n := 0
	for i, c := range tm.Mana {
		if c > 0 {
			n++
			st.TapCol, st.TapAmt = uint8(i), uint8(c)
		}
	}
	if n != 1 {
		return refuse("TapsForMana adding several colours")
	}
	lc.out.Statics = append(lc.out.Statics, st)
	// The class-only line: a TrigTaps watcher of the same filter, the
	// controller's own taps (Whose You), Defer-class, gated by the same
	// condition, and no body -- the static is the extra's whole effect and
	// objTrigGen never queues the line (tapsManaOK).
	tr := &lTrigger{
		Event:       "Taps",
		F:           f,
		Whose:       whoYou,
		Zone:        zBattlefield,
		Cond:        cond,
		TapPlan:     uint8(kop.TapPlanDefer),
		TapsForMana: true,
	}
	ab := &lAbility{Kind: akTriggered, Index: len(lc.out.Abilities), Trigger: tr}
	lc.out.Abilities = append(lc.out.Abilities, ab)
	return nil
}

// tapExtraDynamic lowers a count-scaled literal extra (Elvish Guidance:
// "its controller adds an additional {G} for each Elf on the battlefield") to
// a StatTapExtra static flagged StTapDynamic whose Amount is the value
// reference. gorge's planner never folds a dynamic amount, so it defers the
// enchanted source to the manual window and its potential bound reads the
// source without the extra; the static is unpriced and the extra floats at the
// tap (mana_tapextra.go addGatedExtra), beside a class-only Taps line that
// defers the source, exactly the chosen-colour Aura's shape.
// handled is false for any other trigger, so the foldable/reflected
// classifiers run.
func (lc *lowerCtx) tapExtraDynamic(t cards.Trigger) (handled bool, err error) {
	if t.Mode != "TapsForMana" || !strings.EqualFold(strings.TrimSpace(t.ParamStr(cards.PKStatic)), "True") {
		return false, nil
	}
	body := t.Effect
	if body == nil && lc.face != nil {
		body = cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(t.ParamStr(cards.PKExecute)))
	}
	if body == nil || body.API != "Mana" || body.Sub != nil ||
		strings.TrimSpace(body.ParamStr(cards.PKSubAbility)) != "" {
		return false, nil
	}
	mp := effects.ManaOf(body)
	if len(mp.Unread) != 0 || mp.Produced == "" || mp.AmountTrim == "" || mp.AmountIsLit {
		return false, nil
	}
	if len(mp.Produced) != 1 {
		return false, nil
	}
	col := strings.IndexByte("WUBRG", mp.Produced[0])
	if col < 0 {
		return false, nil
	}
	if strings.TrimSpace(body.ParamStr(cards.PKDefined)) != "TriggeredCardController" {
		return false, nil // the trigger controller's mana, not the tapped permanent's
	}
	tp := newParams(t.Params)
	tp.consume("Mode", "Execute", "Static", "TriggerZones", "TriggerDescription")
	if tp.str("Produced") != "" || tp.str("Activator") != "" { // a tap gate, or the trigger controller's taps
		return false, nil
	}
	vc := strings.TrimSpace(tp.str("ValidCard"))
	if vc == "" {
		return false, nil
	}
	tp.consume("ValidCard")
	if len(tp.unread()) != 0 {
		return false, nil // an unmodelled trigger parameter: the generic refusal names it
	}
	v, err := lc.value(mp.AmountTrim)
	if err != nil {
		return true, err
	}
	f, err := lc.filter(vc)
	if err != nil {
		return true, err
	}
	// The extra is the tapped permanent's controller's (TapAnyPlayer) in the
	// body's literal colour, scaled by the amount's value. gorge defers the
	// enchanted source (its planner never folds a dynamic amount), so the line
	// pairs the static with a class-only Taps watcher of the same filter: the
	// kernel's static is skipped by the planner's pricing readers and floats at
	// the tap (mana_tapextra.go addGatedExtra).
	lc.out.Statics = append(lc.out.Statics, lStatic{
		Kind: "TapExtra", Flags: []string{"TapAnyPlayer", "TapDynamic"},
		F: f, TapCol: uint8(col), Amount: v,
	})
	tr := &lTrigger{
		Event:       "Taps",
		F:           f,
		Whose:       whoAny,
		Zone:        zBattlefield,
		TapPlan:     uint8(kop.TapPlanDefer),
		TapsForMana: true,
	}
	lc.out.Abilities = append(lc.out.Abilities, &lAbility{Kind: akTriggered, Index: len(lc.out.Abilities), Trigger: tr})
	return true, nil
}

// tapExtraChosen lowers a `DB$ Mana | Produced$ Chosen` body whose mana goes
// to the tapped permanent's controller (Utopia Sprawl, Shimmerwilds Growth).
// The extra's colour is the source's as-enters choice, so it poses no tap-time
// ask; but gorge's planner never folds a Produced$ Chosen body
// (pay.PaymentPlanFoldableTapMana), so it defers the enchanted source to the
// manual window and its potential bound reads the source without the extra.
// The kernel models exactly that as the gated tap-extra does: a StatTapExtra
// static flagged StTapChosen that the planner's pricing readers skip and that
// floats the chosen colour at the tap, beside a class-only Taps line that
// defers the source.
// handled is false for any other trigger, so the foldable/reflected
// classifiers run.
func (lc *lowerCtx) tapExtraChosen(t cards.Trigger) (handled bool, err error) {
	if t.Mode != "TapsForMana" || !strings.EqualFold(strings.TrimSpace(t.ParamStr(cards.PKStatic)), "True") {
		return false, nil
	}
	body := t.Effect
	if body == nil && lc.face != nil {
		body = cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(t.ParamStr(cards.PKExecute)))
	}
	if body == nil || body.API != "Mana" || body.Sub != nil ||
		strings.TrimSpace(body.ParamStr(cards.PKSubAbility)) != "" {
		return false, nil
	}
	mp := effects.ManaOf(body)
	if mp.Produced != "Chosen" || len(mp.Unread) != 0 {
		return false, nil
	}
	if v := mp.AmountTrim; v != "" && (!mp.AmountIsLit || mp.AmountLit < 1 || mp.AmountLit > 20) {
		return false, nil
	}
	if strings.TrimSpace(body.ParamStr(cards.PKDefined)) != "TriggeredCardController" {
		return false, nil // the trigger controller's mana, not the tapped permanent's
	}
	tp := newParams(t.Params)
	tp.consume("Mode", "Execute", "Static", "TriggerZones", "TriggerDescription")
	if tp.str("Produced") != "" || tp.str("Activator") != "" { // a tap gate, or the trigger controller's taps
		return false, nil
	}
	vc := strings.TrimSpace(tp.str("ValidCard"))
	if vc == "" {
		return false, nil
	}
	tp.consume("ValidCard")
	if len(tp.unread()) != 0 {
		return false, nil // an unmodelled trigger parameter: the generic refusal names it
	}
	f, err := lc.filter(vc)
	if err != nil {
		return true, err
	}
	amt := 1
	if mp.AmountTrim != "" {
		amt = mp.AmountLit
	}
	// The extra is the tapped permanent's controller's (TapAnyPlayer) in the
	// source's chosen colour (TapChosen). gorge defers the enchanted source
	// (its planner never folds the body), so the line pairs the static with a
	// class-only Taps watcher of the same filter, as the gated tap-extra does
	// (tapExtraGated): the kernel's static is skipped by the planner's pricing
	// readers and floats at the tap (mana_tapextra.go addGatedExtra).
	lc.out.Statics = append(lc.out.Statics, lStatic{Kind: "TapExtra", Flags: []string{"TapAnyPlayer", "TapChosen"}, F: f, TapAmt: uint8(amt)})
	tr := &lTrigger{
		Event:       "Taps",
		F:           f,
		Whose:       whoAny,
		Zone:        zBattlefield,
		TapPlan:     uint8(kop.TapPlanDefer),
		TapsForMana: true,
	}
	lc.out.Abilities = append(lc.out.Abilities, &lAbility{Kind: akTriggered, Index: len(lc.out.Abilities), Trigger: tr})
	return true, nil
}

// tapExtraAnyColour lowers a static TapsForMana extra whose DB$ Mana body
// adds mana of any colour, one colour for the whole amount (Produced$ Any:
// Fertile Ground) or a per-unit combination (Produced$ Combo Any: Market
// Festival). gorge resolves the trigger with a real colour ask at the tap
// (effects.askManaChoice), so its planner never folds the body and defers the
// tapped source to the manual window. The kernel mirrors the chosen-colour
// extra: a StatTapExtra static flagged StTapAnyCol that the planner's pricing
// readers skip and that floats the ask's answer at the tap (mana_tapextra.go
// addGatedExtra), beside a class-only Taps line that defers the source. A is
// the ask's shape (0 one colour for B mana, 1 B units each any colour) and B
// the amount. handled is false for any other trigger, so the foldable,
// gated and reflected classifiers run.
func (lc *lowerCtx) tapExtraAnyColour(t cards.Trigger) (handled bool, err error) {
	if t.Mode != "TapsForMana" || !strings.EqualFold(strings.TrimSpace(t.ParamStr(cards.PKStatic)), "True") {
		return false, nil
	}
	if cp := strings.TrimSpace(t.ParamStr(cards.PKCheckDefinedPlayer)); cp != "" {
		return true, refuse("TapsForMana trigger condition CheckDefinedPlayer$ %q: outside the closed world", cp)
	}
	body := t.Effect
	if body == nil && lc.face != nil {
		body = cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(t.ParamStr(cards.PKExecute)))
	}
	if body == nil || body.API != "Mana" || body.Sub != nil ||
		strings.TrimSpace(body.ParamStr(cards.PKSubAbility)) != "" {
		return false, nil
	}
	mp := effects.ManaOf(body)
	if !mp.CountsAny || mp.NeedsChosen || len(mp.Unread) != 0 {
		return false, nil
	}
	combo := strings.EqualFold(strings.TrimSpace(mp.Produced), "Combo Any")
	if !combo && !strings.EqualFold(strings.TrimSpace(mp.Produced), "Any") {
		return false, nil
	}
	amt := 1
	if mp.AmountTrim != "" {
		if !mp.AmountIsLit || mp.AmountLit < 1 || mp.AmountLit > 20 {
			return false, nil
		}
		amt = mp.AmountLit
	}
	if combo && amt > 3 {
		return true, refuse("TapsForMana extra of any colour in a Combo of %d: the ask's per-unit answers exceed the tap-extra model", amt)
	}
	if strings.TrimSpace(body.ParamStr(cards.PKDefined)) != "TriggeredCardController" {
		return false, nil // the trigger controller's mana, not the tapped permanent's
	}
	tp := newParams(t.Params)
	tp.consume("Mode", "Execute", "Static", "TriggerZones", "TriggerDescription")
	if tp.str("Produced") != "" || tp.str("Activator") != "" { // a tap gate, or the trigger controller's taps
		return false, nil
	}
	vc := strings.TrimSpace(tp.str("ValidCard"))
	if vc == "" {
		return false, nil
	}
	tp.consume("ValidCard")
	if len(tp.unread()) != 0 {
		return false, nil // an unmodelled trigger parameter: the generic refusal names it
	}
	f, err := lc.filter(vc)
	if err != nil {
		return true, err
	}
	mode := 0
	if combo {
		mode = 1
	}
	lc.out.Statics = append(lc.out.Statics, lStatic{
		Kind: "TapExtra", Flags: []string{"TapAnyPlayer", "TapAnyCol"},
		F: f, TapCol: uint8(mode), TapAmt: uint8(amt),
	})
	tr := &lTrigger{
		Event:       "Taps",
		F:           f,
		Whose:       whoAny,
		Zone:        zBattlefield,
		TapPlan:     uint8(kop.TapPlanDefer),
		TapsForMana: true,
	}
	lc.out.Abilities = append(lc.out.Abilities, &lAbility{Kind: akTriggered, Index: len(lc.out.Abilities), Trigger: tr})
	return true, nil
}

// tapReflectLandFilters are the ValidCard$ filters of a reflected TapsForMana
// body that the kernel prices: lands, any permanent (Eloren Wilds), and
// (Kinnan, Bonder Prodigy) any nonland permanent. A nonland filter reaches the
// same one-plain-tap sources a land filter does -- a mana rock's tap makes its
// types exactly as a land's does (Roxanne's Artifact.token precedent) -- so
// the extra reads the tapped permanent's produced types exactly as gorge folds
// it per production (pay.PaymentPlanProductions, PaymentPlanReflectedMana):
// one mana of the produced type for a one-type source, one more of the pick
// for a choice source, a stall wherever the source is not one plain tap.
var tapReflectLandFilters = cards.NewNameSet("Land.Basic", "Land", "Mountain,Forest,Plains", "Permanent", "Permanent.nonLand")

// tapExtraReflected lowers a reflected TapsForMana body (gorge
// pay.PaymentPlanReflectedTapMana) to a StTapReflect StatTapExtra: one mana
// of the type the tap produced. A land or any-permanent filter lowers
// (tapReflectLandFilters):
// gorge folds the extra per production (pay.PaymentPlanReflectedMana over each
// single-colour alternative of a choice source), so the kernel prices it on
// the pick; a source making several types in one tap stays the kernel's
// StallTapExtra. ValidCard$ Artifact.token (Roxanne, Starfall Savant) reaches a
// source that makes any colour (Meteorite): the planner reads it as one tap of
// one more mana of the colour picked (dirac mana_tapextra.go); a token whose
// extra it cannot price that way (a Treasure) stalls there, as every other
// shape. Any other filter refuses. The extra goes to the tapping
// player: either the controller's own taps (Activator$ You, Defined$ You) or
// every player's (no Activator$, Defined$ TriggeredActivator or
// TriggeredCardController, the payer of a planned tap), StTapAnyPlayer.
func (lc *lowerCtx) tapExtraReflected(t cards.Trigger) error {
	tm, ok := pay.PaymentPlanReflectedTapMana(lc.face, t)
	if !ok {
		return refuse("trigger mode TapsForMana")
	}
	tp := newParams(t.Params)
	tp.consume("Mode", "Execute", "Static", "TriggerZones", "TriggerDescription")
	st := lStatic{Kind: "TapExtra", TapAmt: 1, Flags: []string{"TapReflect"}}
	if strings.TrimSpace(t.ParamStr(cards.PKTriggerZones)) == "Command" {
		// A face that is not a battlefield permanent: its printed TapsForMana
		// static lives in the command zone, and the kernel's eachTapExtra walks
		// it there. The classifier above admits Command only on a Vanguard face
		// (Sisay) or a plane face (Eloren Wilds; pay
		// paymentPlanStaticTapManaBody). A Vanguard is marked only on this
		// lowered shape (every other Vanguard face keeps its old inert
		// subtype, out of scope for this static); a plane keeps its own type
		// and lives in its owner's planar deck, which eachTapExtra visits as
		// the seat's face-up plane (CR 901).
		if lc.face == nil || !lc.face.IsPlane() {
			lc.out.Types |= tVanguard
			lc.out.Subtypes = slices.DeleteFunc(lc.out.Subtypes, func(s string) bool { return s == "Vanguard" })
		}
		st.Zone = zCommand
	}
	switch v := strings.TrimSpace(tp.str("Activator")); {
	case v == "You" && !tm.ToTapped:
	case v == "" && tm.ToTapped:
		st.Flags = append(st.Flags, "TapAnyPlayer")
	default:
		return refuse("TapsForMana reflected mana with Activator$ %q", v)
	}
	tp.consume("Activator")
	if v := strings.TrimSpace(tp.str("ValidCard")); !tapReflectLandFilters.Has(v) && v != "Artifact.token" {
		return refuse("TapsForMana reflected mana over ValidCard$ %q: not a land filter", v)
	}
	var err error
	if st.F, err = lc.filter(tp.str("ValidCard")); err != nil {
		return err
	}
	tp.consume("ValidCard")
	if v := strings.TrimSpace(tp.str("Produced")); v != "" {
		return refuse("TapsForMana reflected mana with Produced$ %q", v)
	}
	lc.out.Statics = append(lc.out.Statics, st)
	return nil
}

// effectTapExtraRefuse refuses the Mode$ TapsForMana | Static$ True trigger of
// an until-end-of-turn Effect (Bubbling Muck: "until end of turn, whenever a
// player taps a Swamp for mana, that player adds an additional {B}"). Unlike a
// battlefield trigger, gorge registers it as a delayed trigger that
// pay.PaymentPlanFoldableTapMana never sees: its planner defers every source
// the registration matches to the manual window (paymentPlanDelayedTapObserver)
// and its potential bound reads the tap without the extra, while the tap
// itself adds it. A kernel StatTapExtra in an end-of-turn emblem prices the
// extra into the potential and the plan, so lockstep games offered casts gorge
// did not (measured: 9 of the first 50 seeds of dirac/cardset/pools/fixtures/
// mbcmuck diverged at the options); mirroring the deferral is the Taps-watcher
// prefix-script surface (mana_tapsdefer.go), a separate feature.
func effectTapExtraRefuse() error {
	return refuse("Effect TapsForMana trigger until end of turn: gorge defers the tapped source to the manual window and its potential bound omits the extra, which the kernel's tap-extra static would price")
}
