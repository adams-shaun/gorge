package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// tapsForManaWatch lowers a non-Static$ `Mode$ TapsForMana` trigger (Price of
// Glory: "whenever a player taps a land for mana, if it's not that player's
// turn, destroy that land") to a TrigTaps line that only a mana tap fires
// (kop.Tf5TapsForMana). gorge's tapsMatches reads the line against a Tap event
// emitted inside a mana ability's cost (Facts.TappingForMana), never an attack,
// cost or effect tap; the Static$ True shapes, which resolve off the stack as
// part of the tap, are the tap-extra statics (lower_tapextra.go).
//
// The closed world is the shape the kernel prices:
//   - ValidCard$ over any controller's permanents by type alone (a Land) or
//     by that type plus a watcher-relative controller (Land.OppCtrl), or the
//     bare Card.Self clause: the source's own tap (Zhur-Taa Druid, whose self
//     line the kernel gates with kop.TfSelf; the builder turns the clause into
//     that flag);
//   - no Activator$ (any player's tap, Manabarbs), Activator$
//     Player.NonActive, the tapping player is not the active player
//     (effects.MatchesPlayerSpec over trigmatch.TapActor), or Activator$
//     You/Opponent, a tap by the listener's controller or by an opponent of
//     it (seat-exact, the same Trig.Whose gate); the self shape is
//     Activator$ You only;
//   - a body gorge's planner defers (tapsPlanClass == TapPlanDefer: it is no
//     vetted payer damage), so a land the line matches is a tap the planner
//     never makes (pay.PaymentPlanAbilityTier -> TierDeferred) and the
//     kernel's census leaves it out (mana_tapsdefer.go tapsWatchClass) --
//     except the vetted destroy-self row (Price of Glory, tapsDestroySelf):
//     the tapped land stays an ordinary planner unit there, the trigger
//     resolving after the payment that caused it (CR 405.6c/603.2, 605.4a).
//
// A self line's source is the tapped permanent itself, and gorge's planner
// defers it (pay.PaymentPlanTapDamageTrigger accepts Mode$ Taps only), so the
// line keeps TapPlanDefer (Zhur-Taa Druid). Its body is the literal damage row
// (kop.RefEachOpponent); a self line with another body (Forbidden Orchard's
// token) is a separate lowering. Anything else keeps the generic refusal.
func (lc *lowerCtx) tapsForManaWatch(tp *params, tr *lTrigger, t cards.Trigger, readF func(string) (*lFilter, error)) error {
	const generic = "trigger mode TapsForMana"
	// no Activator$: any player's tap (Manabarbs); Player.NonActive: only a
	// tap by a player other than the active one (Price of Glory); You/Opponent
	// (Sanctimony, Savage Firecat, Treasure Nabber, War's Toll): only a tap by
	// the listener's controller, or by any opponent of it (seat-exact) -- the
	// tapper gate Trig.Whose already carries (trig_tapsvalid.go tapsWhoseOK at
	// the fire, tapsWatchClassForeign's whoseOK in the deferral classing), the
	// gate gorge's real matcher reads the Activator$ with
	// (paymentPlanTapObservers); You with Card.Self is the source's own tap
	// (Zhur-Taa Druid, a self line)
	nonActive, selfTap := false, false
	var whose uint8
	switch v := strings.TrimSpace(tp.str("Activator")); v {
	case "":
	case "Player.NonActive":
		nonActive = true
		tp.consume("Activator")
	case "You":
		whose = whoYou
		selfTap = true
		tp.consume("Activator")
	case "Opponent":
		whose = whoOpp
		tp.consume("Activator")
	default:
		return refuse("%s", generic)
	}
	if whose != whoAny && tp.str("ValidPlayer") != "" {
		// gorge matches both specs (the Activator$ over the tapper, the
		// ValidPlayer$ over it too): an AND this lowering does not vet
		return refuse("TapsForMana ValidPlayer$ beside Activator$ You/Opponent")
	}
	vc := strings.TrimSpace(tp.str("ValidCard"))
	if vc == "" {
		return refuse("%s", generic)
	}
	var err error
	if tr.F, err = readF("ValidCard"); err != nil {
		return err
	}
	if tr.F == nil || tr.F.Players != 0 || len(tr.F.Clauses) == 0 {
		return refuse("TapsForMana ValidCard %q", vc)
	}
	self := false
	for _, c := range tr.F.Clauses {
		if c == (lClause{Flags: fSelf}) {
			self = true
			continue
		}
		// Ctrl is the watcher-relative controller the kernel's filter and
		// tapsWatchClass already read (Land.OppCtrl on Mana Web: the tap of a
		// land an opponent of the watcher controls). Any other controller
		// selector (a target, the defending player, ...) is still beyond the
		// closed world.
		if c.Owner != whoAny || (c.Ctrl != whoAny && c.Ctrl != whoYou && c.Ctrl != whoOpp) || c.Flags != 0 || c.CmpStat != cmpNone || c.Special != 0 {
			return refuse("TapsForMana ValidCard %q: beyond a type filter", vc)
		}
	}
	if self && (len(tr.F.Clauses) != 1 || !selfTap) {
		// A self TapsForMana line is the card's own tap ("whenever you tap
		// CARDNAME for mana") and names its activator: exactly the bare
		// Card.Self clause with Activator$ You (Zhur-Taa Druid).
		return refuse("TapsForMana ValidCard %q", vc)
	}
	if class, _ := tapsPlanClass(t.Effect); class != kop.TapPlanDefer {
		return refuse("TapsForMana body that is vetted payer damage: gorge prices the source as a last-resort unit")
	}
	if self {
		// The in-scope self line is the literal damage body (Zhur-Taa Druid:
		// DealDamage | Defined$ Player.Opponent, the kernel's
		// kop.RefEachOpponent). A self line with another body (Forbidden
		// Orchard's token) is a separate lowering and stays refused.
		if t.Effect == nil || t.Effect.API != "DealDamage" || strings.TrimSpace(t.Effect.ParamStr(cards.PKSubAbility)) != "" {
			return refuse("TapsForMana self line without a literal damage body")
		}
	}
	tr.Event, tr.TapsForMana, tr.TapperNonActive = "Taps", true, nonActive
	tr.Whose = whose
	if tapsDestroySelf(t) {
		// The destroy-self row (Price of Glory): the trigger resolves after
		// the payment that caused it (CR 405.6c/603.2) and its only effect is
		// to destroy the permanent whose tap already produced its mana
		// (CR 605.4a), so the tapped source stays an ordinary planner unit on
		// both engines (kop.TapPlanOrdinary; gorge pay
		// PaymentPlanTapDestroySelfTrigger, widened to Mode$ TapsForMana) and
		// the trigger still fires on its own at the tap.
		tr.TapPlan = uint8(kop.TapPlanOrdinary)
		return nil
	}
	tr.TapPlan = uint8(kop.TapPlanDefer)
	// The class stays Defer (gorge's planner never taps the source), but a
	// literal damage body naming the tapper still costs the payer life at
	// each tap, so the kernel's potential census's lethal guard reads it.
	tr.TapPainN = tapsDeferPain(t.Effect)
	return nil
}
