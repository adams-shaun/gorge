package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// tapExtraStatic lowers a `Mode$ TapsForMana | Static$ True` trigger to the
// StatTapExtra static when gorge's planner folds it into a tap's production
// (pay.PaymentPlanFoldableTapMana: one literal symbol and amount, no other
// parameter), or a reflected body over lands (Lavaleaper, Mana Flare:
// pay.PaymentPlanReflectedTapMana, one mana of the type the tap produced).
// A body whose colour is not a literal keeps refusing with its own reason: an
// any-colour body asks a colour choice at tap time (tapExtraAnyColour), and a
// chosen-colour body is a source gorge's planner defers (tapExtraChosen).
func (lc *lowerCtx) tapExtraStatic(t cards.Trigger) error {
	if handled, err := lc.tapExtraChosen(t); handled {
		return err
	}
	tm, ok := pay.PaymentPlanFoldableTapMana(lc.face, t)
	if !ok {
		if lc.tapExtraAnyColour(t) {
			return refuse("TapsForMana extra of any colour: the trigger asks a colour choice at tap time, which the tap-extra model cannot pose")
		}
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

// tapExtraChosen refuses a `DB$ Mana | Produced$ Chosen` body whose mana goes
// to the tapped permanent's controller (Utopia Sprawl, Shimmerwilds Growth).
// The extra's colour is the source's as-enters choice, so it poses no tap-time
// ask; but gorge's planner never folds a Produced$ Chosen body
// (pay.PaymentPlanFoldableTapMana), so it defers the enchanted source to the
// manual window and its potential bound reads the source without the extra.
// The kernel's potential surface cannot mirror that deferral (a lockstep game
// offered casts gorge did not), so the card keeps refusing with this reason.
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
	if tp.str("ValidCard") == "" {
		return false, nil
	}
	tp.consume("ValidCard")
	if len(tp.unread()) != 0 {
		return false, nil // an unmodelled trigger parameter: the generic refusal names it
	}
	return true, refuse("TapsForMana extra of the source's chosen colour: gorge defers a Produced$ Chosen body and the kernel's potential bound cannot mirror it")
}

// tapExtraAnyColour reports whether t is a TapsForMana extra whose DB$ Mana
// body adds mana of any colour (Produced$ Any) or a combination of colours
// (Combo Any, Market Festival). Gorge resolves such a trigger with a KChoose
// colour ask (effects.askManaChoice) when the land is tapped; the kernel's
// fixed StatTapExtra cannot pose that ask, so the card keeps refusing with
// this reason instead of the generic "trigger mode TapsForMana".
func (lc *lowerCtx) tapExtraAnyColour(t cards.Trigger) bool {
	body := t.Effect
	if body == nil && lc.face != nil {
		body = cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(t.ParamStr(cards.PKExecute)))
	}
	if body == nil || body.API != "Mana" {
		return false
	}
	mp := effects.ManaOf(body)
	return mp.Produced != "" && mp.CountsAny && !mp.NeedsChosen
}

// tapReflectLandFilters are the ValidCard$ filters of a reflected TapsForMana
// body that reach lands only.
var tapReflectLandFilters = cards.NewNameSet("Land.Basic", "Land", "Mountain,Forest,Plains")

// tapExtraReflected lowers a reflected TapsForMana body (gorge
// pay.PaymentPlanReflectedTapMana) to a StTapReflect StatTapExtra: one mana
// of the type the tap produced. A land filter lowers (tapReflectLandFilters):
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
