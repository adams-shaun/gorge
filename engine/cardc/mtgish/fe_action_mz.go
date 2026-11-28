package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Action variants M-Z and the satellite enums they use (EnterFlag,
// PutCountersAction, RemoveCountersAction,
// RevealTheTopNumberCardsOfLibraryAction, SearchLibraryAction,
// DamageToRecipients).

// damagePart is what DamageToRecipients lowers to.
type damagePart struct {
	n  *ir.Value
	to *ir.Ref
}

// counterWord lowers a CounterType argument to its counter word.
func counterWord(c *Ctx, x any) (string, error) {
	v, err := c.lower(x)
	if err != nil {
		return "", err
	}
	w, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s lowers to %T, want a counter word", qualified(x), v)
	}
	return w, nil
}

// chooseOf is a choice of n cards (nil n: any number) matching argument i.
func chooseOf(c *Ctx, n node, i int, cnt *ir.Value) (*ir.Ref, error) {
	f, err := c.Filter(argN(n, i))
	if err != nil {
		return nil, err
	}
	return &ir.Ref{Kind: ir.RefChoose, Filter: f, N: cnt}, nil
}

// moveObj registers a move of the object argument 0 to zone to, with word.
// flags names the argument holding the EnterFlag list, or -1.
func moveObj(key string, to ir.Zone, word string, flags int) {
	register(key, func(c *Ctx, n node) (any, error) {
		r, err := c.Ref(argN(n, 0))
		if err != nil {
			return nil, err
		}
		st := &ir.Step{Verb: ir.VerbMove, Obj: r, To: to, Word: word}
		if flags >= 0 {
			if st.Enter, err = c.Enters(list(argN(n, flags))); err != nil {
				return nil, err
			}
		}
		return st, nil
	})
}

// moveAll registers a move of every object matching filter argument 0.
func moveAll(key string, to ir.Zone, word string) {
	register(key, func(c *Ctx, n node) (any, error) {
		r, err := refAllOf(c, n, 0)
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbMove, Obj: r, To: to, Word: word}, nil
	})
}

// moveOne registers a move of one chosen object matching filter argument 0.
func moveOne(key string, to ir.Zone, flags int) {
	register(key, func(c *Ctx, n node) (any, error) {
		r, err := chooseOf(c, n, 0, refConst1())
		if err != nil {
			return nil, err
		}
		st := &ir.Step{Verb: ir.VerbMove, Obj: r, To: to}
		if flags >= 0 {
			if st.Enter, err = c.Enters(list(argN(n, flags))); err != nil {
				return nil, err
			}
		}
		return st, nil
	})
}

// block is the steps as one *ir.Step: itself when there is one, else a
// VerbDo block.
func block(ss []*ir.Step) *ir.Step {
	if len(ss) == 1 {
		return ss[0]
	}
	return &ir.Step{Verb: ir.VerbDo, Then: &ir.Program{Steps: ss}}
}

// mayStep is the steps of action x made optional.
func mayStep(c *Ctx, x any) (*ir.Step, error) {
	ss, err := c.Steps(x)
	if err != nil {
		return nil, err
	}
	st := block(ss)
	st.Optional = true
	return st, nil
}

// ownedByYou is f and the owner fact "you own it".
func ownedByYou(f *ir.Filter) *ir.Filter {
	own := &ir.Filter{Subject: f.Subject, Zones: f.Zones, Op: ir.FPred, Pred: ir.POwnedBy, Ref: &ir.Ref{Kind: ir.RefYou}}
	return &ir.Filter{Subject: f.Subject, Zones: f.Zones, Op: ir.FAll, Args: []*ir.Filter{f, own}}
}

func foundRef() *ir.Ref { return &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcFound} }

func topRef(of *ir.Ref, n *ir.Value) *ir.Ref { return &ir.Ref{Kind: ir.RefTopOfLibrary, Of: of, N: n} }

func fixedMove(key string, mk func() *ir.Step) { stepFixed(key, mk) }

func init() {
	register("Action.MayAction", func(c *Ctx, n node) (any, error) { return mayStep(c, rawArgs(n)) })
	register("Action.MayActions", func(c *Ctx, n node) (any, error) {
		p, err := c.StepList(list(rawArgs(n)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbDo, Optional: true, Then: p}, nil
	})
	for _, k := range []string{"MayCost", "MustCost"} {
		opt := k == "MayCost"
		register("Action."+k, func(c *Ctx, n node) (any, error) {
			co, err := c.Cost(rawArgs(n))
			if err != nil {
				return nil, err
			}
			return &ir.Step{Verb: ir.VerbPayCost, Optional: opt, Cost: co}, nil
		})
	}
	stepN("Action.MillNumberCards", ir.VerbMill)

	damage := func(c *Ctx, src, amt, to any) (*ir.Step, error) {
		s, err := c.Ref(src)
		if err != nil {
			return nil, err
		}
		v, err := c.Value(amt)
		if err != nil {
			return nil, err
		}
		o, err := c.Ref(to)
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbDealDamage, Source: s, N: v, Obj: o}, nil
	}
	for _, k := range []string{"PermanentDealsDamage", "SpellDealsDamage"} {
		register("Action."+k, func(c *Ctx, n node) (any, error) {
			return damage(c, argN(n, 0), argN(n, 1), argN(n, 2))
		})
	}
	register("Action.PermanentDealsMultipleDamage", func(c *Ctx, n node) (any, error) {
		s, err := c.Ref(argN(n, 0))
		if err != nil {
			return nil, err
		}
		p := &ir.Program{}
		for _, x := range list(argN(n, 1)) {
			v, err := c.lower(x)
			if err != nil {
				return nil, err
			}
			d, ok := v.(*damagePart)
			if !ok {
				return nil, fmt.Errorf("%s lowers to %T, want a damage part", qualified(x), v)
			}
			p.Steps = append(p.Steps, &ir.Step{Verb: ir.VerbDealDamage, Source: s, N: d.n, Obj: d.to})
		}
		return p, nil
	})
	register("DamageToRecipients.DamageToRecipients", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		r, err := c.Ref(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &damagePart{n: v, to: r}, nil
	})

	register("Action.PlayerAction", func(c *Ctx, n node) (any, error) {
		who, err := argRef(c, n, 0)
		if err != nil {
			return nil, err
		}
		ss, err := c.Steps(argN(n, 1))
		if err != nil {
			return nil, err
		}
		for _, s := range ss {
			if s.Who == nil {
				s.Who = who
			}
		}
		return &ir.Program{Steps: ss}, nil
	})
	register("Action.PlayerActions", func(c *Ctx, n node) (any, error) {
		who, err := argRef(c, n, 0)
		if err != nil {
			return nil, err
		}
		p, err := c.StepList(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbDo, Who: who, Then: p}, nil
	})
	register("Action.PlayerMayAction", func(c *Ctx, n node) (any, error) {
		who, err := argRef(c, n, 0)
		if err != nil {
			return nil, err
		}
		st, err := mayStep(c, argN(n, 1))
		if err != nil {
			return nil, err
		}
		if st.Who == nil {
			st.Who = who
		}
		return st, nil
	})
	register("Action.PlayerMayCost", func(c *Ctx, n node) (any, error) {
		who, err := argRef(c, n, 0)
		if err != nil {
			return nil, err
		}
		co, err := c.Cost(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbPayCost, Optional: true, Who: who, Cost: co}, nil
	})

	// Moves.
	// "A card from your graveyard": the graveyard is its owner's, so the filter carries the owner fact.
	register("Action.PutACardFromGraveyardIntoHand", func(c *Ctx, n node) (any, error) {
		r, err := chooseOf(c, n, 0, refConst1())
		if err != nil {
			return nil, err
		}
		r.Filter = ownedByYou(r.Filter)
		return &ir.Step{Verb: ir.VerbMove, Obj: r, To: ir.ZoneHand}, nil
	})
	moveOne("Action.PutACardFromHandOnBattlefield", ir.ZoneBattlefield, 1)
	register("Action.PutEachCardFromEachPlayersGraveyardOntoTheBattlefield", func(c *Ctx, n node) (any, error) {
		cs, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		pls, err := c.Filter(argN(n, 1))
		if err != nil {
			return nil, err
		}
		en, err := c.Enters(list(argN(n, 2)))
		if err != nil {
			return nil, err
		}
		f := &ir.Filter{Op: ir.FAll, Args: []*ir.Filter{
			withZone(cs, ir.ZoneGraveyard),
			{Subject: ir.SubjCard, Op: ir.FPred, Pred: ir.POwnedBy, Ref: &ir.Ref{Kind: ir.RefAll, Filter: pls}},
		}}
		return &ir.Step{Verb: ir.VerbMove, Obj: &ir.Ref{Kind: ir.RefAll, Filter: f}, To: ir.ZoneBattlefield, Enter: en}, nil
	})
	moveAll("Action.PutEachGraveyardCardIntoHand", ir.ZoneHand, "")
	moveAll("Action.PutEachPermanentIntoItsOwnersHand", ir.ZoneHand, "")
	moveAll("Action.PutEachPermanentOnBottomOfOwnersLibraryInOrderOfOwnersChoice", ir.ZoneLibrary, "bottom-any-order")
	moveObj("Action.PutExiledCardOntoBattlefield", ir.ZoneBattlefield, "", 1)
	moveObj("Action.PutGraveyardCardIntoHand", ir.ZoneHand, "", -1)
	moveObj("Action.PutGraveyardCardOntoBattlefield", ir.ZoneBattlefield, "", 1)
	register("Action.PutNumCardsFromHandOnTopOfLibraryAnyOrder", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbMove, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: anyCardIn(ir.ZoneHand), N: v},
			To: ir.ZoneLibrary, Word: "top-any-order"}, nil
	})
	moveObj("Action.PutPermanentIntoItsOwnersHand", ir.ZoneHand, "", -1)
	moveObj("Action.PutPermanentOnBottomOfOwnersLibrary", ir.ZoneLibrary, "bottom", -1)
	register("Action.PutTheTopCardOfPlayersLibraryOnTheBottomOfTheirLibrary", func(c *Ctx, n node) (any, error) {
		pl, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbMove, Obj: topRef(pl, refConst1()), To: ir.ZoneLibrary, Word: "bottom"}, nil
	})
	fixedMove("Action.PutTopOfLibraryInHand", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbMove, Obj: topRef(nil, refConst1()), To: ir.ZoneHand}
	})
	register("Action.ReturnDeadGraveyardCardToBattlefield", func(c *Ctx, n node) (any, error) {
		en, err := c.Enters(list(rawArgs(n)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbMove, Obj: &ir.Ref{Kind: ir.RefEventObject}, From: ir.ZoneGraveyard,
			To: ir.ZoneBattlefield, Enter: en}, nil
	})
	register("Action.ReturnEachCardFromPlayersGraveyardToBattlefield", func(c *Ctx, n node) (any, error) {
		gs, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		pl, err := argRef(c, n, 1)
		if err != nil {
			return nil, err
		}
		en, err := c.Enters(list(argN(n, 2)))
		if err != nil {
			return nil, err
		}
		f := &ir.Filter{Op: ir.FAll, Args: []*ir.Filter{gs, {Subject: ir.SubjCard, Op: ir.FPred, Pred: ir.POwnedBy, Ref: pl}}}
		return &ir.Step{Verb: ir.VerbMove, Obj: &ir.Ref{Kind: ir.RefAll, Filter: f}, To: ir.ZoneBattlefield, Enter: en}, nil
	})
	moveObj("Action.ReturnGraveyardCardToHand", ir.ZoneHand, "", -1)
	moveObj("Action.ReturnPermanentToTopOrBottomOfLibrary", ir.ZoneLibrary, "top-or-bottom", -1)
	fixedMove("Action.ShuffleHandIntoLibrary", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbMove, Obj: &ir.Ref{Kind: ir.RefAll, Filter: anyCardIn(ir.ZoneHand)},
			To: ir.ZoneLibrary, Word: "shuffle"}
	})

	for _, k := range []string{"PutCounters", "RemoveCounters"} {
		register("Action."+k, func(c *Ctx, n node) (any, error) { return c.StepList(list(rawArgs(n))) })
	}

	register("Action.Reflexive_CreateTokens_WhenYouDo", func(c *Ctx, n node) (any, error) {
		var steps []*ir.Step
		for _, t := range list(argN(n, 0)) {
			ss, err := c.Steps(t)
			if err != nil {
				return nil, err
			}
			steps = append(steps, ss...)
		}
		en, err := c.Enters(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		if en != nil {
			for _, s := range steps {
				e := *en
				s.Enter = &e
			}
		}
		body, err := c.NestedBody(argN(n, 2))
		if err != nil {
			return nil, err
		}
		steps = append(steps, &ir.Step{Verb: ir.VerbDelayedTrigger, Abilities: []*ir.Ability{
			{Kind: ir.AbTriggered, Trigger: &ir.Trigger{Event: ir.EvReflexive}, Body: body}}})
		return &ir.Program{Steps: steps}, nil
	})

	stepObj("Action.RegeneratePermanent", ir.VerbRegenerate)
	stepObj("Action.SacrificePermanent", ir.VerbSacrifice)
	stepObj("Action.TapPermanent", ir.VerbTap)
	stepObj("Action.TransformPermanent", ir.VerbTransform)
	stepObj("Action.UntapPermanent", ir.VerbUntap)
	stepAllObj("Action.SacrificeEachPermanent", ir.VerbSacrifice)
	stepN("Action.Surveil", ir.VerbSurveil)
	stepFixed("Action.TakeAnExtraTurn", func() *ir.Step { return &ir.Step{Verb: ir.VerbExtraTurn} })
	stepFixed("Action.Shuffle", func() *ir.Step { return &ir.Step{Verb: ir.VerbShuffle} })
	stepFixed("Action.RevealHand", func() *ir.Step { return &ir.Step{Verb: ir.VerbRevealHand} })
	stepFixed("Action.RevealACardFromHand", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbReveal, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: anyCardIn(ir.ZoneHand), N: refConst1()}}
	})
	stepFixed("Action.RevealTopCardOfLibrary", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbReveal, Obj: topRef(nil, refConst1())}
	})
	register("Action.RevealCardsFromTheTopOfLibraryUntilACardOfTypeIsRevealed", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		p, err := c.StepList(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbRevealUntil, Filter: f, Then: p}, nil
	})
	for k, verb := range map[string]ir.Verb{"Discard": ir.VerbDiscard, "Exile": ir.VerbExile} {
		register("Action.RevealHandAndPlayerChoosesACardTo"+k, func(c *Ctx, n node) (any, error) {
			pl, err := argRef(c, n, 0)
			if err != nil {
				return nil, err
			}
			f, err := c.Filter(argN(n, 1))
			if err != nil {
				return nil, err
			}
			return &ir.Step{Verb: ir.VerbRevealHand, Then: &ir.Program{Steps: []*ir.Step{{Verb: verb,
				Obj: &ir.Ref{Kind: ir.RefChoose, Of: pl, Filter: withZone(f, ir.ZoneHand), N: refConst1()}}}}}, nil
		})
	}
	register("Action.RevealTheTopNumberCardsOfLibrary", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		p, err := c.StepList(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbReveal, Obj: topRef(nil, v), Then: p}, nil
	})

	register("Action.SacrificeAPermanent", func(c *Ctx, n node) (any, error) {
		r, err := chooseOf(c, n, 0, refConst1())
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbSacrifice, Obj: r}, nil
	})
	register("Action.SacrificeNumberPermanents", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		r, err := chooseOf(c, n, 1, v)
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbSacrifice, Obj: r}, nil
	})
	register("Action.SearchLibrary", func(c *Ctx, n node) (any, error) {
		p, err := c.StepList(list(rawArgs(n)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbSearch, Then: p}, nil
	})
	register("Action.Unless", func(c *Ctx, n node) (any, error) {
		cd, err := c.Cond(argN(n, 0))
		if err != nil {
			return nil, err
		}
		p, err := c.StepList(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CNot, Args: []*ir.Cond{cd}}, Then: p}, nil
	})

	registerMZSatellites()
}

func registerMZSatellites() {
	// EnterFlag.
	register("EnterFlag.EntersNormally", func(c *Ctx, n node) (any, error) { return &ir.Enter{}, nil })
	register("EnterFlag.EntersTapped", func(c *Ctx, n node) (any, error) { return &ir.Enter{Tapped: true}, nil })
	register("EnterFlag.EntersUnderOwnersControl", func(c *Ctx, n node) (any, error) { return &ir.Enter{UnderOwner: true}, nil })
	register("EnterFlag.EntersUnderPlayersControl", func(c *Ctx, n node) (any, error) {
		r, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Enter{Controller: r}, nil
	})
	register("EnterFlag.EntersWithACounter", func(c *Ctx, n node) (any, error) {
		w, err := counterWord(c, rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Enter{Counter: w, N: refConst1()}, nil
	})
	register("EnterFlag.EntersWithNumberCounters", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		w, err := counterWord(c, argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Enter{Counter: w, N: v}, nil
	})
	register("EnterFlag.EntersWithLayerEffect", func(c *Ctx, n node) (any, error) {
		es, err := c.Effects(list(rawArgs(n)))
		if err != nil {
			return nil, err
		}
		return &ir.Enter{Effects: es}, nil
	})

	// PutCountersAction.
	register("PutCountersAction.ACounterOfTypeOnPermanent", func(c *Ctx, n node) (any, error) {
		w, err := counterWord(c, argN(n, 0))
		if err != nil {
			return nil, err
		}
		r, err := argRef(c, n, 1)
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbPutCounters, Obj: r, N: refConst1(), Word: w}, nil
	})
	register("PutCountersAction.ACounterOfTypeOnEachPermanent", func(c *Ctx, n node) (any, error) {
		w, err := counterWord(c, argN(n, 0))
		if err != nil {
			return nil, err
		}
		r, err := refAllOf(c, n, 1)
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbPutCounters, Obj: r, N: refConst1(), Word: w}, nil
	})
	register("PutCountersAction.NumberCountersOfTypeOnPermanent", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		w, err := counterWord(c, argN(n, 1))
		if err != nil {
			return nil, err
		}
		r, err := argRef(c, n, 2)
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbPutCounters, Obj: r, N: v, Word: w}, nil
	})
	register("PutCountersAction.DoubleCountersOfTypeOnPermanent", func(c *Ctx, n node) (any, error) {
		w, err := counterWord(c, argN(n, 0))
		if err != nil {
			return nil, err
		}
		r, err := argRef(c, n, 1)
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbDoubleCounters, Obj: r, Word: w}, nil
	})
	stepAllObj("PutCountersAction.DoubleAllCountersOnEachPermanent", ir.VerbDoubleCounters)

	register("RemoveCountersAction.AllCountersOfTypeFromEachPermanent", func(c *Ctx, n node) (any, error) {
		w, err := counterWord(c, argN(n, 0))
		if err != nil {
			return nil, err
		}
		r, err := refAllOf(c, n, 1)
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbRemoveCounters, Obj: r, Word: w}, nil
	})

	// SearchLibraryAction.
	findOne := func(key string, f func(c *Ctx, n node) (*ir.Ref, error)) {
		register("SearchLibraryAction."+key, func(c *Ctx, n node) (any, error) {
			r, err := f(c, n)
			if err != nil {
				return nil, err
			}
			return &ir.Step{Verb: ir.VerbChoose, Obj: r}, nil
		})
	}
	// A search of a hidden zone for cards with a stated quality never obliges the
	// searcher to find them (CR 701.23b), so each of these is "up to".
	upTo := func(r *ir.Ref, err error) (*ir.Ref, error) {
		if r != nil {
			r.UpTo = true
		}
		return r, err
	}
	findOne("FindACardOfType", func(c *Ctx, n node) (*ir.Ref, error) { return upTo(chooseOf(c, n, 0, refConst1())) })
	findOne("FindAGenericCard", func(c *Ctx, n node) (*ir.Ref, error) {
		return &ir.Ref{Kind: ir.RefChoose, Filter: anyCardIn(ir.ZoneLibrary), N: refConst1()}, nil
	})
	findN := func(c *Ctx, n node) (*ir.Ref, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		return upTo(chooseOf(c, n, 1, v))
	}
	findOne("FindNumberCardsOfType", findN)
	findOne("FindUptoNumberCardsOfType", findN)
	foundMove := func(key string, to ir.Zone, word string, flags bool) {
		register("SearchLibraryAction."+key, func(c *Ctx, n node) (any, error) {
			st := &ir.Step{Verb: ir.VerbMove, Obj: foundRef(), To: to, Word: word}
			if flags {
				var err error
				if st.Enter, err = c.Enters(list(rawArgs(n))); err != nil {
					return nil, err
				}
			}
			return st, nil
		})
	}
	foundMove("PutFoundCardsIntoHand", ir.ZoneHand, "", false)
	foundMove("PutFoundCardsOntoBattlefield", ir.ZoneBattlefield, "", true)
	foundMove("PutSetAsideCardsOnTopOfLibrary", ir.ZoneLibrary, "top", false)
	stepFixed("SearchLibraryAction.RevealFoundCards", func() *ir.Step { return &ir.Step{Verb: ir.VerbReveal, Obj: foundRef()} })
	stepFixed("SearchLibraryAction.Shuffle", func() *ir.Step { return &ir.Step{Verb: ir.VerbShuffle} })
	stepFixed("SearchLibraryAction.SetAsideFoundCards", func() *ir.Step { return &ir.Step{Verb: ir.VerbDo} })

	// RevealTheTopNumberCardsOfLibraryAction.
	register("RevealTheTopNumberCardsOfLibraryAction.MayPutAnyNumberOfCardsOfTypeOntoTheBattlefield", func(c *Ctx, n node) (any, error) {
		r, err := chooseOf(c, n, 0, nil)
		if err != nil {
			return nil, err
		}
		en, err := c.Enters(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbMove, Optional: true, Obj: r, To: ir.ZoneBattlefield, Enter: en}, nil
	})
	stepFixed("RevealTheTopNumberCardsOfLibraryAction.PutTheCardFoundThisWayIntoHand", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbMove, Obj: foundRef(), To: ir.ZoneHand}
	})
	stepFixed("RevealTheTopNumberCardsOfLibraryAction.PutTheRemainingCardsIntoGraveyard", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbMove, To: ir.ZoneGraveyard}
	})
	stepFixed("RevealTheTopNumberCardsOfLibraryAction.PutTheRemainingCardsOnTheBottomOfLibraryInARandomOrder", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbMove, To: ir.ZoneLibrary, Word: "bottom-random"}
	})
}
