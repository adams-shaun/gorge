package mtgish

import "github.com/adams-shaun/gorge/engine/cardc/ir"

// Reference lowerings: mtgish's single-object and single-player enums to
// *ir.Ref. Every Ref_ variant resolves through Ctx.TargetSlot; an unresolved
// one is an ":unbound" refusal.

// refTarget lowers a Ref_ variant to {RefTarget, Slot}. variant is the name
// the declaration stem is matched on; key names the node being lowered, for
// the refusal.
func refTarget(c *Ctx, key, variant string) (any, error) {
	slot, ok := c.TargetSlot(variant)
	if !ok {
		return nil, &RefusalError{Key: key + ":unbound", Path: c.Src()}
	}
	return &ir.Ref{Kind: ir.RefTarget, Slot: slot}, nil
}

// refEvent is the object the triggering event concerns.
func refEvent() *ir.Ref { return &ir.Ref{Kind: ir.RefEventObject} }

func refConst1() *ir.Value { return &ir.Value{Kind: ir.VConst, N: 1} }

// refSimple registers a variant that lowers to a fixed Ref.
func refSimple(key string, mk func() *ir.Ref) {
	register(key, func(c *Ctx, n node) (any, error) { return mk(), nil })
}

// refTargetKey registers a Ref_ variant that resolves to a declared target.
func refTargetKey(key, variant string) {
	register(key, func(c *Ctx, n node) (any, error) { return refTarget(c, key, variant) })
}

// refOf registers a variant wrapping its argument: {kind, Of: c.Ref(arg)}.
func refOf(key string, kind ir.RefKind) {
	register(key, func(c *Ctx, n node) (any, error) {
		of, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Ref{Kind: kind, Of: of}, nil
	})
}

// refPass registers a variant that is just its argument's reference.
func refPass(key string) {
	register(key, func(c *Ctx, n node) (any, error) { return c.Ref(rawArgs(n)) })
}

// refAll registers a variant that is every object matching its argument.
func refAll(key string) {
	register(key, func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Ref{Kind: ir.RefAll, Filter: f}, nil
	})
}

func init() {
	// Permanent.
	refSimple("Permanent.ThisPermanent", func() *ir.Ref { return &ir.Ref{Kind: ir.RefSelf} })
	refSimple("Permanent.HostPermanent", func() *ir.Ref {
		return &ir.Ref{Kind: ir.RefAttachedTo, Of: &ir.Ref{Kind: ir.RefSelf}}
	})
	for _, v := range []string{"Ref_TargetPermanent", "Ref_TargetPermanent1", "Ref_TargetPermanent2"} {
		refTargetKey("Permanent."+v, v)
	}
	for _, v := range []string{"ThatEnteringPermanent", "Trigger_ThatCreature", "Trigger_ThatPermanent"} {
		refSimple("Permanent."+v, refEvent)
	}

	// Player.
	refSimple("Player.You", func() *ir.Ref { return &ir.Ref{Kind: ir.RefYou} })
	refOf("Player.ControllerOfPermanent", ir.RefControllerOf)
	refOf("Player.ControllerOfSpell", ir.RefControllerOf)
	refOf("Player.OwnerOfPermanent", ir.RefOwnerOf)
	refOf("Player.PlayerOrControllerOfPermanent", ir.RefPlayerOrControllerOf)
	register("Player.ControllerOfTargetPermanent", func(c *Ctx, n node) (any, error) {
		return refControllerOfTarget(c, "Player.ControllerOfTargetPermanent", "Ref_TargetPermanent")
	})
	register("Player.ControllerOfTargetSpell", func(c *Ctx, n node) (any, error) {
		return refControllerOfTarget(c, "Player.ControllerOfTargetSpell", "Ref_TargetSpell")
	})
	refTargetKey("Player.Ref_TargetPlayer", "Ref_TargetPlayer")
	refSimple("Player.TheActivePlayer", func() *ir.Ref { return &ir.Ref{Kind: ir.RefActivePlayer} })
	refSimple("Player.Trigger_DefendingPlayer", func() *ir.Ref { return &ir.Ref{Kind: ir.RefDefendingPlayer} })
	refSimple("Player.Trigger_ThatPlayer", func() *ir.Ref { return &ir.Ref{Kind: ir.RefEventPlayer} })
	refSimple("Player.ActionPlayer", func() *ir.Ref { return &ir.Ref{Kind: ir.RefIterPlayer} })

	// PlayerOrPermanent.
	refTargetKey("PlayerOrPermanent.Ref_AnyTarget", "Ref_AnyTarget")
	refTargetKey("PlayerOrPermanent.Ref_TargetPlayerOrPermanent", "Ref_TargetPlayerOrPermanent")

	// Spell, Card, CardInGraveyards, CardInExile.
	refSimple("Spell.ThisSpell", func() *ir.Ref { return &ir.Ref{Kind: ir.RefSelf} })
	refTargetKey("Spell.Ref_TargetSpell", "Ref_TargetSpell")
	refSimple("Spell.Trigger_ThatSpell", refEvent)
	refSimple("Card.ThisCard", func() *ir.Ref { return &ir.Ref{Kind: ir.RefSelf} })
	refSimple("CardInGraveyards.ThisGraveyardCard", func() *ir.Ref { return &ir.Ref{Kind: ir.RefSelf} })
	refTargetKey("CardInGraveyards.Ref_TargetGraveyardCard", "Ref_TargetGraveyardCard")
	refSimple("CardInGraveyards.Trigger_ThatGraveyardCard", refEvent)
	refSimple("CardInExile.TheCardExiledThisWay", func() *ir.Ref {
		return &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcExiled}
	})
	for _, v := range []string{"TheChosenExiledCard", "TheExiledCardChosenThisWay"} {
		refSimple("CardInExile."+v, func() *ir.Ref { return &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcChosen} })
	}

	// DamageRecipient, SingleDamageRecipient.
	refAll("DamageRecipient.EachPermanent")
	refAll("DamageRecipient.EachPlayer")
	refPass("DamageRecipient.Permanent")
	refPass("DamageRecipient.Player")
	for _, v := range []string{"Ref_AnyTarget", "Ref_AnyTargets", "Ref_TargetPlayerOrPermanent"} {
		refTargetKey("DamageRecipient."+v, v)
	}
	refPass("SingleDamageRecipient.Permanent")

	// Exilable. A nil Of is the acting player.
	refPass("Exilable.Permanent")
	refPass("Exilable.Spell")
	refPass("Exilable.CardInGraveyards")
	refAll("Exilable.EachPermanent")
	refAll("Exilable.EachCardInGraveyards")
	refSimple("Exilable.ACardFromHand", func() *ir.Ref {
		return &ir.Ref{
			Kind: ir.RefChoose,
			Filter: &ir.Filter{
				Subject: ir.SubjCard,
				Zones:   ir.MakeZoneSet(ir.ZoneHand),
				Op:      ir.FPred,
				Pred:    ir.PAny,
			},
			N: refConst1(),
		}
	})
	register("Exilable.ACardInGraveyards", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Ref{Kind: ir.RefChoose, Filter: f, N: refConst1()}, nil
	})
	refSimple("Exilable.TheTopCardOfLibrary", func() *ir.Ref {
		return &ir.Ref{Kind: ir.RefTopOfLibrary, N: refConst1()}
	})
	register("Exilable.TheTopNumberCardsOfLibrary", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Ref{Kind: ir.RefTopOfLibrary, N: v}, nil
	})
	register("Exilable.TheTopCardOfEachPlayersLibraries", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Ref{
			Kind: ir.RefTopOfLibrary,
			Of:   &ir.Ref{Kind: ir.RefAll, Filter: f},
			N:    refConst1(),
		}, nil
	})
	register("Exilable.AllCardsFromPlayersLibrary", func(c *Ctx, n node) (any, error) {
		pl, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Ref{Kind: ir.RefAll, Filter: &ir.Filter{
			Subject: ir.SubjCard,
			Zones:   ir.MakeZoneSet(ir.ZoneLibrary),
			Op:      ir.FPred,
			Pred:    ir.POwnedBy,
			Ref:     pl,
		}}, nil
	})
}

// refControllerOfTarget is {RefControllerOf, Of: target(variant)}.
func refControllerOfTarget(c *Ctx, key, variant string) (any, error) {
	t, err := refTarget(c, key, variant)
	if err != nil {
		return nil, err
	}
	return &ir.Ref{Kind: ir.RefControllerOf, Of: t.(*ir.Ref)}, nil
}
