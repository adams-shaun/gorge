package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("ChangeX", effChangeX)
}

// effChangeX implements DB$ ChangeX -- a mid-resolution rewrite of the {X} a
// stack object was cast or activated with. The two corpus carriers:
//
//	Unbound Flourishing "Whenever you cast a permanent spell with a mana cost
//	that contains {X}, double the value of X."
//	  SVar:TrigDouble:DB$ ChangeX | Defined$ TriggeredSpellAbility | Value$
//	  TriggeredSpellAbility>Count$xPaid/Twice
//	Glava, Five-Advents Mage "...you may have the value of X become 5."
//	  SVar:TrigSetX:DB$ ChangeX | Defined$ TriggeredSpellAbility | Value$ 5
//
// The trigger's Remembered carries the cast/activation stack object
// (TriggeredSpellAbility), whose pay-time CastInfo already set its X; the
// rewrite emits one XChange event per defined object (the spell object on
// the stack for a cast arm; the activated-ability wrapper re-derived from
// the source permanent for an activation arm), and every
// downstream reader (resolution's ctx.X, the ETB replacement ctx,
// Count$xPaid) picks the new value up fresh -- nothing else needs plumbing.
// CR 601.2c: the trigger resolves above the spell it names, after targets
// were announced; UF's doubling adds no payment, so only the binding changes.
//
// Degraded shapes are loud but harmless: a Value$ the grammar cannot resolve
// (an unknown ref -- the CastSA adamant family's shape fails closed in
// evalCountExprOK) or a Defined$ object that is missing or no longer on the
// stack (the spell was countered out from under the trigger) each emit one
// Note and rewrite nothing. A rewrite whose new value equals the old is
// emitted unconditionally -- one branch-free event, and a value nobody could
// observe differently either way.
func effChangeX(h Host, c *Ctx, sa *cards.SA) {
	value, ok := NumResolved(h, c, sa, "Value", 0)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "ChangeX value unresolvable"})
		return
	}
	g := h.Game()
	any := false
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil {
			continue
		}
		if o.Zone == state.ZStack {
			// The SPELL arm: a cast trigger's Remembered carries the spell
			// object itself (the PutOnStack event's Obj).
			any = true
			h.Emit(events.Event{Kind: events.XChange, Obj: t.Obj, Amount: value})
			continue
		}
		// The ACTIVATION arm: an AbilityPush event's Obj is the SOURCE
		// PERMANENT (the ability stack object is minted inside events.Apply,
		// off the event), so a SpellAbilityCast trigger's Remembered names
		// the battlefield permanent its activation was paid on -- derive the
		// ability object the trigger fired on here (changeXAbilityObject).
		if id, found := changeXAbilityObject(g, o.ID); found {
			any = true
			h.Emit(events.Event{Kind: events.XChange, Obj: id, Amount: value})
		}
	}
	if !any {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "ChangeX found no stack object"})
	}
}

// changeXAbilityObject re-derives the activated-ability stack object a
// SpellAbilityCast trigger fired on: the topmost ability wrapper on the stack
// whose Source is the triggering permanent, skipping the trigger wrappers the
// same permanent minted (state.TriggerOf -- the one classifier view's
// StackView.Kind and rules' TargetType$ share, so the three cannot disagree).
//
// Approximation, stated: with SEVERAL activations of the same permanent on
// the stack below their queued triggers, every trigger binds the topmost
// (most recent) one -- the event stream cannot carry the minted object's id,
// so per-trigger provenance beyond "an ability of this permanent" does not
// exist. Single-activation resolution -- the shape every corpus carrier and
// this build's tests exercise -- is exact.
func changeXAbilityObject(g *state.Game, perm state.ObjID) (state.ObjID, bool) {
	for i := len(g.Stack) - 1; i >= 0; i-- {
		o := g.Obj(g.Stack[i])
		if o == nil || o.Card != nil || o.Ability == nil || o.Source != perm {
			continue
		}
		if _, isTrig := state.TriggerOf(g, o); isTrig {
			continue
		}
		return o.ID, true
	}
	return 0, false
}
