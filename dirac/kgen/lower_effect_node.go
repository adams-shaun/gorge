package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"strings"
)

// guardBlock emits a SkipIf for cond, runs fn, then patches the skip.
func (lc *lowerCtx) guardBlock(cond lCond, fn func() error) error {
	if cond.Kind == cNone {
		return fn()
	}
	at := len(lc.ab.Prog)
	lc.emit(lInstr{Op: "SkipIf", Cond: cond})
	if err := fn(); err != nil {
		return err
	}
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	return nil
}

// nodeCondition reads a node's Condition*$ params.
func (lc *lowerCtx) nodeCondition(ps *params) (lCond, error) {
	if lc.clAt < 0 {
		lc.clAt = 0
		// the Effect after a Counter RememberCountered$: its ConditionPresent$
		// is read in counterLoseEffect (the kernel's CounterLose checks the
		// source's types itself)
		return lCond{}, nil
	}
	if lc.condDone {
		lc.condDone = false
		ps.consume("ConditionCheckSVar", "ConditionSVarCompare")
		return lCond{}, nil
	}
	if ps.str("Announce") == "NumTimes" && ps.str("ConditionCheckSVar") == "NumTimes" {
		// the announced count is at least 1 exactly when the cost was paid:
		// numTimesPay (lower_numtimes.go) reads it with the rest of the shape
		return lCond{}, nil
	}
	if c, ok, err := lc.giftCondition(ps); ok || err != nil {
		return c, err
	}
	if c, ok, err := lc.teamworkCondition(ps); ok || err != nil {
		return c, err
	}
	if c, ok := ps.get("Condition"); ok {
		return lc.bareCondition(ps, c)
	}
	if c, ok, err := lc.manaSpentCondition(ps); ok || err != nil { // lower_manaspentcond.go
		return c, err
	}
	if c, ok := ps.get("ConditionActivationLimit"); ok {
		return lc.activationLimitCondition(ps, c) // lower_activationcount.go
	}
	if c, ok := ps.get("ConditionPlayerTurn"); ok {
		return lc.playerTurnCondition(ps, c)
	}
	if c, ok := ps.get("ConditionPhases"); ok {
		// a lone ConditionPhases$ (the player-turn pair is handled above);
		// any other Condition*$ key beside it refuses
		if beside := conditionKeysBesides(ps, "ConditionPhases"); len(beside) > 0 {
			return lCond{}, refuse("ConditionPhases$ beside %s$", beside[0])
		}
		return lc.phasesCondition(c)
	}
	if err := lc.notPresentAsPresent(ps); err != nil { // lower_condnotpresent.go
		return lCond{}, err
	}
	if c, ok, err := lc.effectPairCondition(ps); ok || err != nil { // lower_effectremember.go
		return c, err
	}
	if cp, ok := ps.get("ConditionPresent"); ok {
		if d, ok := ps.get("ConditionDefined"); ok && strings.TrimSpace(d) == "Collected" {
			return lc.collectedCondition(ps, cp) // lower_collectedevidence.go
		}
		if d, ok := ps.get("ConditionDefined"); ok && strings.TrimSpace(d) == "CastSA>Collected" {
			return lc.castCollectedCondition(ps, cp) // lower_collectedevidence.go
		}
		// ConditionDefined$ Self + ConditionPresent$
		// Card.wasCastFromGraveyard reads the resolving spell's own
		// graveyard-origin cast flag (gorge state.ObjectWasCastFromGraveyard
		// through effects/filter.go's wasCastFromGraveyard predicate), the
		// same read as the Count$wasCastFromGraveyard.<yes>.<no> branch head
		// in countValue. It reads the cast flag OR the cast zone:
		// a may-play permission cast from a graveyard reads true; a stack
		// copy reads false (CR 707.10). Only
		// this exact spelling is lifted here; any other use of the word (a
		// ValidCard$, a target filter, a TriggeredCard$ subject) still goes
		// to lc.filter and refuses as an unknown property.
		if d, ok := ps.get("ConditionDefined"); ok && strings.TrimSpace(d) == "Self" && strings.TrimSpace(cp) == "Card.wasCastFromGraveyard" {
			if lc.ab == nil || lc.ab.Kind != akSpell {
				// gorge reads the resolving object's persisting CastFlags: a
				// permanent's ETB/static would read true off its cast spell.
				// The kernel reads the resolving stack entry, so only a spell
				// ability has the same object.
				return lCond{}, refuse("ConditionPresent$ Card.wasCastFromGraveyard outside a spell ability")
			}
			c := lCond{Kind: cValue, A: lValue{Kind: vCastFromGrave, Sub: []lValue{cst(1), cst(0)}}, Op: opGE, B: cst(1)}
			if cmp, ok := ps.get("ConditionCompare"); ok {
				switch strings.TrimSpace(cmp) {
				case "EQ1", "GE1":
					// the default: the flag reads 1, present
				case "EQ0":
					c.Op, c.B = opEQ, cst(0)
				default:
					return lCond{}, refuse("ConditionPresent$ Card.wasCastFromGraveyard with ConditionCompare$ %q", cmp)
				}
			}
			return c, nil
		}
		// ConditionDefined$ Self + a cast-provenance word
		// reads the resolving spell's own stack entry (kernel wasCastSpell);
		// only a spell ability has that object
		d0, hasD := ps.get("ConditionDefined")
		lc.spellProvOK = hasD && strings.TrimSpace(d0) == "Self" && lc.ab != nil && lc.ab.Kind == akSpell
		// the remembered-persist leave token asks whether the remembered card
		// is still exiled with the source (a resolution-time read of the
		// source's own exile link, lower_rempersist.go)
		lc.exwOK = hasD && strings.TrimSpace(d0) == "Remembered" && strings.TrimSpace(cp) == "Card.ExiledWithSource" && lc.remPersistShape()
		if z, ok := ps.m["ConditionZone"]; ok && strings.TrimSpace(z) == "Stack" && strings.TrimSpace(cp) == "Card" && hasD && strings.TrimSpace(d0) == "TriggeredCard" {
			// "the triggering card is on the stack" (gorge conditionMetZone
			// over the defined group): Card.inZoneStack's read, the spelling
			// Goliath Daydreamer uses (lower_effectstackexile.go)
			cp = "Card.inZoneStack"
			ps.consume("ConditionZone")
		}
		spellGate := lc.targetedSpellGate(d0, hasD) // lower_condtargetspell.go
		lc.spellBaseOK = spellGate
		f, err := lc.filter(cp)
		lc.spellBaseOK = false
		lc.exwOK = false
		lc.spellProvOK = false
		if err != nil {
			return lCond{}, err
		}
		if spellGate {
			dropTargetedSpellBase(f)
		}
		c := lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
		if d, ok := ps.get("ConditionDefined"); ok {
			var ref lRef
			if r, ok, err := lc.rememberedLKIRef(d); err != nil { // lower_handmove.go
				return lCond{}, err
			} else if ok {
				ref = r
			} else if strings.TrimSpace(d) == "Sacrificed" {
				// C: the cost's sacrificed object, matched as
				// the card it is now (kernel sacrificedObj)
				if err := lc.sacrificedGroup(f); err != nil {
					return lCond{}, err
				}
				ref = lRef{Kind: rSacrificed}
			} else {
				// TriggeredAttackerLKICopy is the
				// single attacker an Attacks trigger captured, read as it now
				// stands (an Ordeal's counter is already on it). defined()
				// binds it to the triggering object and refuses the shapes
				// where gorge has no single attacker (several matches, a
				// player-event trigger).
				var err error
				lc.condDefined = true
				ref, err = lc.defined(d)
				lc.condDefined = false
				if err != nil {
					return lCond{}, err
				}
				lc.markDestroyedTargetLKI(ref, cp) // lower_remembered.go
			}
			c.Kind, c.Ref = cRefMatches, ref
		}
		if cmp, ok := ps.get("ConditionCompare"); ok {
			op, v, err := lc.cmpSpec(cmp)
			if err != nil {
				return lCond{}, err
			}
			c.Op, c.A = op, v
		}
		if c, err = lc.nodeConditionZone(ps, c, cp); err != nil { // lower_condkeys.go
			return lCond{}, err
		}
		// The plain ConditionDefined$ Remembered spelling: gorge's group is
		// the source's persistent remembered list UNIONED with the
		// resolution's own memory (rememberedWithSource, which
		// rememberedLKIGroup answers; Forge's SpellAbilityCondition.areMet
		// reads getDefinedObjects(host, "Remembered")). The kernel's
		// RefRemembered reads the resolution's own set alone, so a chain that
		// wrote nothing still meets the gate on an earlier resolution's write
		// (Nezahal, Primal Tide's second discard-three activation,
		// Aetherling's: the first exile's memory stands on the host). OR in
		// the persistent mark for the GE1 present compare. A written
		// ConditionCompare$ that lowers to GE1 (Tevesh Szat, Doom of Fools'
		// draw chain: ConditionCompare$ GE1) is the same gate: Forge's
		// presentCompare default is "GE1" (SpellAbilityVariables), so
		// spellPresentCondition compares the identical group with the
		// identical op, and the written spelling used to keep today's
		// per-resolution lowering -- which parted from gorge at the second
		// activation of the same source: no sacrifice this resolution, but
		// the host's first activation's memory stands, gorge draws and the
		// kernel refused the gate (dr-mbc-bbba pair 26). The mark is a bare
		// non-empty test, so it is ORed in only when the present filter
		// restricts nothing: a typed gate (Soul-Shackled Zombie's
		// ConditionPresent$ Card.Creature over an instant it remembered, or
		// the draw-another-if-commander tail of the same Tevesh Szat chain)
		// stays at today's per-resolution lowering.
		if c.Kind == cRefMatches && c.Ref.Kind == rRemembered &&
			c.Op == opGE && c.A.Kind == vConst && c.A.N == 1 && !c.Neg &&
			lc.remMarkFilterOK(c.F) {
			c = orCond(c, lCond{Kind: cRemMark})
		}
		// ConditionCheckSVar$ beside the present check is a conjunction
		// (gorge conditionMetCore: a denying SVar skips, a passing one leaves
		// the present group to decide). H.M.S. Bounty: ConditionPresent$
		// Whale.YouCtrl with Count$YourExtraTurns EQ0; Abzan Beastmaster:
		// Creature.YouCtrl with YourTough GEOppsTough; Triumph of Ferocity:
		// YourFerocity GE OppsFerocity, the two Count$Valid
		// <creatures>$GreatestCardPower aggregates. The SVar gate lowers
		// whenever its value and compare lower (the kernel's CondValue evalV's
		// both sides in the same resolution context, exprs.go); the extra-turn
		// count is read only against zero, and anything else refuses rather
		// than dropping the SVar gate.
		if cs, ok := ps.get("ConditionCheckSVar"); ok {
			v, err := lc.value(cs)
			if err != nil {
				return lCond{}, err
			}
			cmp, ok := ps.get("ConditionSVarCompare")
			cmp, ok = meldSVarCompareDefault(lc, cmp, ok) // Forge's default compare on a meld face (lower_meld.go)
			if !ok {
				return lCond{}, refuse("ConditionCheckSVar$ %q without ConditionSVarCompare$", cs)
			}
			op, b, err := lc.cmpSpec(cmp)
			if err != nil {
				return lCond{}, err
			}
			if v.Kind == vYourExtraTurns && (op != opEQ || !b.isConst() || b.N != 0) {
				return lCond{}, refuse("ConditionCheckSVar$ %q with ConditionSVarCompare$ %q beside ConditionPresent$ %q", cs, cmp, cp)
			}
			c = lCond{Kind: cAnd, Sub: []lCond{c, {Kind: cValue, A: v, Op: op, B: b}}}
		}
		if c, err = lc.presentSecondGroup(ps, c); err != nil { // lower_condpresent2.go
			return lCond{}, err
		}
		// A self-referencing present check ("Card.Self+Scout") is a
		// property of the source.
		return c, nil
	}
	if cs, ok := ps.get("ConditionCheckSVar"); ok {
		if lc.firstCastSVar != "" && strings.TrimSpace(cs) == lc.firstCastSVar {
			return lc.firstCastCond(ps) // lower_firstcast.go
		}
		v, err := lc.value(cs)
		if err != nil {
			return lCond{}, err
		}
		c := lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
		if cmp, ok := ps.get("ConditionSVarCompare"); ok {
			op, b, err := lc.cmpSpec(cmp)
			if err != nil {
				return lCond{}, err
			}
			c.Op, c.B = op, b
		}
		if t, ok := ps.get("ConditionOpponentTurn"); ok {
			// gorge conditions.go: ConditionOpponentTurn$ True is
			// ConditionPlayerTurn$ False, ANDed with the SVar compare
			if !strings.EqualFold(strings.TrimSpace(t), "True") {
				return lCond{}, refuse("ConditionOpponentTurn$ %q", t)
			}
			c = lCond{Kind: cAnd, Sub: []lCond{{Kind: cYourTurn, Neg: true}, c}}
		}
		return c, nil
	}
	return lCond{}, nil
}

// node lowers one SA node (not its Sub).
func (lc *lowerCtx) node(sa *cards.SA, ps *params) error {
	if done, err := lc.putCounterUnion(sa, ps); done { // lower_definedheads.go
		return err
	}
	if done, err := lc.definedPlayerUnion(sa, ps); done { // lower_defineduniononplayers.go
		return err
	}
	at := len(lc.ab.Prog)
	usN := len(lc.unlessSubs)
	guard := lCond{}
	// Peek, not get: get marks Defined$ read for every node, and the
	// effect must still read (or leave unread) its own Defined$.
	ownTgt := false
	if g, ok, err := lc.ownTargetedGuard(sa, ps); ok || err != nil { // lower_targetedown.go
		if err != nil {
			return err
		}
		guard, ownTgt = g, true
	} else if d, ok := ps.m["Defined"]; ok && strings.HasPrefix(d, "Targeted.") && sa.API != "Animate" {
		// Defined$ Targeted.<qualifier> (graveexile): gorge definedSpec
		// (context.go:440-452) keeps the chain targets the qualifier
		// admits. Lower only exactly one target slot, so the node's
		// subject is that slot; a player target never matches the card
		// filter (countRefMatches yields 0), as in gorge. Animate lowers
		// this spelling itself (lower_zone.go:913), with its own guard.
		if _, ok := ps.get("ValidTgts"); ok {
			return refuse("Defined$ %q beside its own ValidTgts$", d)
		}
		if len(lc.chainT) != 1 {
			return refuse("Defined$ %q with %d target slots", d, len(lc.chainT))
		}
		t := lc.lastTarget()
		if t < 0 {
			return refuse("Defined$ %q with no target slot", d)
		}
		// The kernel keeps only a slot's first object in its target cell
		// (the rest live in xTargets), while gorge's definedSpec reads
		// every object of the slot. Lower only a slot that can never hold
		// more than one object.
		tg := lc.ab.Targets[t]
		if !tg.Min.isConst() || !tg.Max.isConst() || tg.Min.N > 1 || tg.Max.N > 1 {
			return refuse("Defined$ %q on a multi-target slot", d)
		}
		f, err := lc.filter("Card." + strings.TrimPrefix(d, "Targeted."))
		if err != nil {
			return err
		}
		guard = lCond{Kind: cRefMatches, Ref: lRef{Kind: rTarget, N: t}, F: f, Op: opGE, A: cst(1)}
		lc.targetedDefined = d
	}
	if guard.Kind == cNone {
		g, err := lc.triggeredQualGuard(sa, ps) // lower_definedheads.go
		if err != nil {
			return err
		}
		guard = g
	}
	guardAt := -1
	if guard.Kind != cNone {
		guardAt = len(lc.ab.Prog)
		lc.emit(lInstr{Op: "SkipIf", Cond: guard})
	}
	// Defined$ / TokenOwner$ Player.<q> / Opponent.<q> (B): the
	// node lowers over the plain set; the qualifier restricts it below
	var setPred *playerSetPred
	if key, ok := playerSetAPIs[sa.API]; ok {
		if v, ok := ps.m[key]; ok {
			p, matched, err := lc.playerSetSpec(sa.API, v)
			if err != nil {
				return err
			}
			if matched {
				if guardAt >= 0 {
					return refuse("%s$ %q beside a Targeted. guard", key, v)
				}
				setPred, ps.m[key] = &p, p.base
			}
		}
	}
	effAt := len(lc.ab.Prog)
	if err := lc.effect(sa, ps); err != nil {
		return err
	}
	lc.targetedDefined = ""
	if ownTgt {
		if err := lc.ownTargetedSlot(guard); err != nil { // lower_targetedown.go
			return err
		}
	}
	if setPred != nil {
		if err := lc.wrapPlayerSet(sa.API, effAt, *setPred); err != nil {
			return err
		}
	}
	// The condition is read after the effect so it may name the node's
	// own target slot (Hidetsugu's Second Rite reads the targeted player's
	// life); its SkipIf goes in front of the node's instructions. Target
	// slots are declarative, so the guard never changes target selection.
	lc.condNodeAt = effAt
	cond, err := lc.nodeCondition(ps)
	lc.condNodeAt = 0
	if err != nil {
		return err
	}
	if cond.Kind != cNone || guardAt >= 0 || setPred != nil {
		lc.markUnlessGuarded(usN) // a skip over the node alone (lower_unlesssubs.go)
	}
	if cond.Kind != cNone && guardAt < 0 && at < len(lc.ab.Prog) && lc.ab.Prog[at].Op == "IfPaid" && lc.ab.Prog[at].Skip == -1 {
		// a trigger's own "you may pay" cost (Feed the Pack's Sac<1/...>):
		// gorge's triggered-cost window pays it before the body runs, and the
		// body's conditions are checked as it resolves, so the condition
		// reads what the payment did (ConditionCheckSVar$ X, X the
		// sacrificed creature's toughness). The guard goes behind the
		// payment, over the rest of the node.
		n := len(lc.ab.Prog) - at - 1
		lc.ab.Prog = append(lc.ab.Prog, lInstr{})
		copy(lc.ab.Prog[at+2:], lc.ab.Prog[at+1:])
		lc.ab.Prog[at+1] = lInstr{Op: "SkipIf", Cond: cond, Skip: n}
		cond = lCond{}
	}
	if cond.Kind != cNone {
		n := len(lc.ab.Prog) - at
		lc.ab.Prog = append(lc.ab.Prog, lInstr{})
		copy(lc.ab.Prog[at+1:], lc.ab.Prog[at:])
		lc.ab.Prog[at] = lInstr{Op: "SkipIf", Cond: cond, Skip: n}
		if guardAt >= 0 {
			guardAt++ // the node guard moved behind the condition guard
		}
	}
	if guardAt >= 0 {
		lc.ab.Prog[guardAt].Skip = len(lc.ab.Prog) - guardAt - 1
	}
	return nil
}

// unless wraps fn in an "unless a player pays" block: fn runs unless payer
// pays (switched: fn runs only if payer pays).
func (lc *lowerCtx) unless(ps *params, fn func() error) error {
	uc, ok := ps.get("UnlessCost")
	if !ok {
		return fn()
	}
	var in lInstr
	ri, isRes, rerr := lc.resUnlessToken(strings.TrimSpace(uc)) // lower_costtokens.go
	if rerr != nil {
		return rerr
	}
	if !isRes {
		if ri, isRes, rerr = lc.resUnlessDiscard(strings.TrimSpace(uc)); rerr != nil { // lower_costtokens.go
			return rerr
		}
	}
	if isRes {
		in = ri
	} else if strings.TrimSpace(uc) == "DefinedCost_ChosenCard" {
		// Forge's DefinedCost_ChosenCard (Tariff): the chosen card's mana
		// VALUE as one generic cost. gorge's unlessDefinedCost renders it
		// at the ask; the kernel's ExMVOf over RefChosen reads the same
		// card as the cost is offered.
		in = lInstr{CS: "Mana", A: lValue{Kind: vMVOf, Ref: lRef{Kind: rChosen}}, B: cst(0)}
	} else if _, isVar := lc.face.SVars[strings.TrimSpace(uc)]; isVar {
		// an SVar-sized generic mana cost (Dazzling Denial's Y: {4} if you
		// control a Bird, else {2}); gorge evaluates it as the ask is posed
		v, err := lc.value(strings.TrimSpace(uc))
		if err != nil {
			return err
		}
		in = lInstr{CS: "Mana", A: v, B: cst(0)}
	} else {
		lc.wbOK = lc.ab.Kind != akLatent
		lc.unlessParse = lc.ab.Kind != akLatent
		cost, err := lc.costUnless(uc)
		lc.unlessParse = false
		lc.wbOK = false
		if err != nil {
			return err
		}
		if cost.SubCtr > 0 {
			// gorge's unless SubCounter drains the resolving
			// ability's host, and the corpus's only carrier pays as the
			// controller (UnlessPayer$ You). Any other payer is refused.
			if p, ok := ps.get("UnlessPayer"); !ok || p != "You" {
				return refuse("SubCounter unless cost without UnlessPayer$ You")
			}
		}
		lc.unlessCost = lc.ab.Kind != akLatent
		in, err = lc.unlessPayInstr(cost, lRef{}, cost.SubCtr > 0) // lower_unlesschain.go
		lc.unlessCost = false
		if err != nil {
			return err
		}
	}
	if ps.is("UnlessSwitched", "True") {
		in.Op = "UnlessSwitched"
	} else {
		in.Op = "UnlessPaid"
	}
	// UnlessPayer$ is read here, before fn: the node's own unread check
	// must not name a key the unless wrapper consumes (Carrion Rats' DB$
	// Effect static). The payer is still bound after the node, where the
	// default's own target slot exists.
	up := "TargetedController"
	if p, ok := ps.get("UnlessPayer"); ok {
		up = p
	}
	gate, whenPaid, err := unlessSubsMode(ps)
	if err != nil {
		return err
	}
	if gate && lc.depth == 0 {
		return refuse("UnlessResolveSubs$ outside a chain")
	}
	at, nTgt := len(lc.ab.Prog), len(lc.ab.Targets)
	lc.emit(in)
	if err := fn(); err != nil {
		return err
	}
	// gorge unlessProceed (Forge's default): UnlessPayer$ defaults to
	// TargetedController, the controller of the node's target -- bound
	// after the node so its own target slot exists (the key itself was
	// read before fn, above).
	if up == "ThisTargetedController" {
		// gorge's unlessPayerTargetsCodes maps ThisTargetedController to the
		// same selector code as TargetedController (effects/unless.go), so it
		// reads the node's OWN target set. A chained link that declared
		// exactly one target lowers exactly like the default: its own slot's
		// controller, or NO payer when that optional target answered empty
		// (gorge: unlessProceed returns the unpaid
		// orientation for a non-nil empty ownTargets, with no fallback ask).
		// The head SA's own target is bound at announcement (E), and a
		// slot allowing several targets names several payers gorge walks in
		// order: both stay refused.
		if lc.head || len(lc.ab.Targets)-nTgt != 1 {
			return refuse("ThisTargetedController unless payer outside a chained single own target")
		}
		if t := lc.ownTarget(); t < 0 {
			return refuse("ThisTargetedController unless payer without its own target slot")
		}
		if mx := lc.ab.Targets[len(lc.ab.Targets)-1].Max; !mx.isConst() || mx.N != 1 {
			return refuse("ThisTargetedController unless payer on a multi-target slot")
		}
		up = "TargetedController"
	}
	payer, err := lc.defined(up)
	if err != nil {
		return err
	}
	if gate && payer.Kind == rEachPlayer {
		return refuse("UnlessResolveSubs$ with an each-player payer")
	}
	lc.ab.Prog[at].Obj = payer
	lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
	if gate {
		lc.unlessSubs = append(lc.unlessSubs, usGate{at: at, nodeEnd: len(lc.ab.Prog),
			same: ps.is("UnlessSwitched", "True") == whenPaid, nTgt: len(lc.ab.Targets)})
	}
	return nil
}

// giftCondition reads "if the gift was (wasn't) promised" (CR 702.174):
// ConditionPresent$ Card.Self+PromisedGift on the spell itself
// (ConditionZone$ Stack), compared EQ1 or EQ0. Any other PromisedGift
// condition shape refuses.
func (lc *lowerCtx) giftCondition(ps *params) (lCond, bool, error) {
	cp := strings.TrimSpace(ps.m["ConditionPresent"])
	if !strings.Contains(cp, "PromisedGift") {
		return lCond{}, false, nil
	}
	ps.consume("ConditionPresent")
	if cp != "Card.Self+PromisedGift" || !lc.out.Gift || ps.str("ConditionZone") != "Stack" {
		return lCond{}, false, refuse("gift condition %q", cp)
	}
	if _, ok := ps.get("ConditionDefined"); ok {
		return lCond{}, false, refuse("gift condition with ConditionDefined$")
	}
	switch ps.str("ConditionCompare") {
	case "EQ1", "GE1":
		return lCond{Kind: cPromisedGift}, true, nil
	case "EQ0":
		return lCond{Kind: cPromisedGift, Neg: true}, true, nil
	}
	return lCond{}, false, refuse("gift condition compare %q", ps.str("ConditionCompare"))
}

// teamworkCondition reads "if this spell was cast using teamwork" (CR
// 702.194b): ConditionPresent$ Card.Self+Teamwork on the spell itself
// (ConditionZone$ Stack), the resolving entry's paid provenance (CondKicked
// reads sfKicked, which a Teamwork card sets only for a paid tap). Any other
// Teamwork condition shape refuses.
func (lc *lowerCtx) teamworkCondition(ps *params) (lCond, bool, error) {
	cp := strings.TrimSpace(ps.m["ConditionPresent"])
	if !strings.Contains(cp, "Teamwork") {
		return lCond{}, false, nil
	}
	ps.consume("ConditionPresent")
	if cp != "Card.Self+Teamwork" || lc.out.Teamwork == 0 || ps.str("ConditionZone") != "Stack" ||
		lc.ab == nil || lc.ab.Kind != akSpell {
		return lCond{}, false, refuse("teamwork condition %q", cp)
	}
	if _, ok := ps.get("ConditionDefined"); ok {
		return lCond{}, false, refuse("teamwork condition with ConditionDefined$")
	}
	switch ps.str("ConditionCompare") {
	case "", "EQ1", "GE1":
		return lCond{Kind: cKicked}, true, nil
	}
	return lCond{}, false, refuse("teamwork condition compare %q", ps.str("ConditionCompare"))
}
