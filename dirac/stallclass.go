package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Stall classes, and refusing a pair before its game. Cold code.
//
// A stall is one of two things. Capacity: a fixed array or budget ran out, gorge
// has no such limit, and the game was correct up to there. Those are declared
// limits, always counted and never a failure. Unmodelled: gorge does something at
// this point the kernel does not. Those are rule gaps, and a lockstep run fails on
// them. The class belongs to the reason, so a site that stops for a rule gap must
// not borrow a capacity reason's name: the five reasons below exist because sites
// did.

// Reasons for what used to stop under a capacity reason's name.
const (
	StallOutsideType   = 66 // a read of an outside type another source chose: equal or not is unknown (act_choosetype.go; was StallBoard)
	StallExileOnMoved  = 67 // an ExileOnMoved$ permission over two or more remembered cards: gorge ends the whole effect when one leaves (value_validagg.go; was StallMayPlay)
	StallMultiRetarget = 69 // a change-targets ask with a target no declaration holds or more than 8 answers (act_stackrewrite.go)
	StallL6Walk        = 71 // a global lose-all override beside a subtype lord the layer-4 walk reads: the loss check would re-enter the walk (gen l6Stall; static_layer6.go; was StallTables)
	StallLowering      = 72 // a state kgen's lowering is meant to rule out: a read no prep walk provided for, a broken table, an unreachable shape (was StallCtrAdded, StallAddPhase, StallTables, StallBoard)
)

func init() {
	stallNames[StallOutsideType] = "outsidetype"
	stallNames[StallExileOnMoved] = "exileonmoved"
	stallNames[StallMultiRetarget] = "multiretarget"
	stallNames[StallL6Walk] = "l6walk"
	stallNames[StallLowering] = "lowering"
	for _, r := range stallCapacityReasons {
		stallClasses[r] = StallCapacity
	}
	for _, r := range stallUnmodelledReasons {
		stallClasses[r] = StallUnmodelled
	}
	kop.PairGuard = pairRefusal
	kop.PairGuardN = pairRefusalN
}

// StallClass is what kind of stop a stall reason is.
type StallClass uint8

const (
	// stallUnclassed is a reason nobody classed. TestEveryStallHasAClass fails on
	// one, and StallClassOf's callers treat it as unmodelled: a new reason cannot
	// pass as a declared limit by being forgotten.
	stallUnclassed StallClass = iota
	StallCapacity
	StallUnmodelled
)

var stallClasses [NumStall]StallClass

// StallClassOf is reason r's class. StallNone and a reason nobody classed are
// neither capacity nor unmodelled.
func StallClassOf(r uint8) StallClass {
	if int(r) >= len(stallClasses) {
		return stallUnclassed
	}
	return stallClasses[r]
}

// StallIsUnmodelled reports whether reason r is not a declared limit: an
// unmodelled reason, or one nobody classed. False for StallNone.
func StallIsUnmodelled(r uint8) bool {
	return r != StallNone && StallClassOf(r) != StallCapacity
}

var stallCapacityReasons = [...]uint8{
	StallBoard, StallPending, StallStack, StallTables, StallLoop, StallDelayed, StallRemember, StallMayPlay,
	StallDProg, StallEmblem, StallDmgMap, StallXTgt, StallUsed, StallKwRepX, StallLinks, StallFree, StallAsk,
	StallCounters, StallLKI, StallTapPick, StallRegen, StallLook, StallGrants, StallActs, StallLinger, StallSac,
	StallEntered, StallConvoke, StallDelve, StallCastRing, StallPT, StallActRing, StallCharmOpts, StallRefLatch, StallZone,
	StallOptions, StallSrcMark, StallDmgLatch, StallReplCands, StallReplPlusBatch, StallPrevCol, StallColGrant, StallRestrX,
	StallObsSrc, StallDynMana, StallTapExtra, StallExiledWith, StallBreach, StallNetLock, StallPanharmGrant, StallExileGrant, StallCharmPick,
	StallCombatHit, StallFinality, StallPersistMana, StallOptMoves, StallMustBlock, StallRestrSrc, StallFirebend,
	StallImprint, StallExHaste, StallEffEtbCtr, StallAtkGrp, StallParadigm, StallFiltX, StallPlanAlts,
	StallCtrAdded,     // a fifth record of genState.ctrAdd; its two non-capacity sites moved to StallLowering
	StallTwoSeat,      // a two-seat-only table in a larger game (stallclass.go): a declared limit
	StallAddPhase,     // a fifth genState.xph entry or a 256th grant; its one non-capacity site moved to StallLowering
	StallUntapLock,    // a fifth source-bound untap lock at once (genState.ul; act_untaplock.go)
	StallTurnRepl,     // the exile-instead table is full of Malicious Eclipse scope records (repl_turnrepl.go)
	StallEmbRem,       // a fifth object in an emblem's remembered set (static_emblemrem.go)
	StallDmgAmt,       // a fifth reader-card object damaged in one turn (genState.dmgAmt; value_dmgamount.go)
	StallDmgMark,      // a second mark of one controller on one player while the first is live (genState.dmk; repl_dmgmark.go)
	StallChooseNumber, // a mid-resolution ChooseNumber's present Max$ resolves above the chosen byte's 254 (repl_choosenumber.go)
	StallDmgBatch,     // a ninth DamageDealtOnce line of one DamageAll sweep's batch latched (armRes.dmgB; trig_dmgbatch.go)
}

var stallUnmodelledReasons = [...]uint8{
	StallInfect, StallReplPlus, StallLifeRepl, StallDrawRepl, StallProlif, StallCtrOrder,
	StallShield, StallReplCtr, StallReplLeave, StallCopyFace, StallRoleMulti, StallDefenderPW,
	StallTokAtkAsk, StallEnterDefender, StallEndureN, StallLinkCtr, StallNumTimes, StallAegis, StallSacOther,
	StallCopyCast, StallNetMana, StallTokenPT, StallStaticAsk, StallHandSize, StallMayRaise, StallUncounterOrder,
	StallEntryOrder, StallCopyAllAsk, StallCloneLayer, StallTokRewrite,
	StallGravePower, // no longer raised: FeatCDAPT evaluates every lowered CDA amount
	StallLandGrant,  // gorge offers both productions of a land with a granted basic type; pcard holds one (static_landtype.go:148,161)
	StallOutsideType, StallExileOnMoved, StallMultiRetarget, StallL6Walk, StallLowering, StallPayLifeShared, StallNoPrevent,
	StallDmgMarkOrder, // a damage event the per-player mark and another replacement both match: gorge asks the CR 616.1 order, which the mark has no candidate for (repl_dmgmark.go)
	StallFbGrantStamp, // a stamped flashback grant beside an own-cost flashback static: gorge offers both granted instances (kw_fbgrant.go)
	StallRestrDefer,   // a non-spell payment only a Taps-deferred restricted source funds: gorge pays it by its prefix search (mana_tapsdefer.go)
	StallDiamondMix,   // a Lion's Eye Diamond beside another census-gap kind: gorge's one prefix search activates both (mana_diamondmix.go)
	StallStackKwGrant, // a granted convoke/improvise on a Harmonize or alternative-cost cast: gorge's announcement for those routes is unverified (kw_convoke.go)
	// a requirement- or lure-maximising block declaration with a two-attacker blocker ready: gorge offers its second block too (combat_extrablock.go)
	StallExtraBlockReq,
	// a card with the pay-before-entering replacement entered other than by its cast spell resolving: gorge asks its controller with the move pending, which the kernel's entry does not model (repl_paybeforeetb.go)
	StallPayETB,
	// an attack tax beside a part its walk does not model: an attack requirement, a Taps watcher, a mana source of another shape (combat_attacktax.go)
	StallAtkTax,
	StallBlkTax,            // a block tax beside a part its source walk does not model (combat_blocktax.go)
	StallChoosePlayerOrder, // an as-enters opponent choice beside another object's enters replacement: gorge asks the CR 616.1 order (repl_chooseplayer.go)
	StallPrevNext,          // a depleting prevent-the-next-N shield and another damage replacement match one event, or a shield amount past 255 (repl_prevnext.go)
	StallOffZonePT,         // an off-battlefield P/T read of a card a live AffectedZone$ All P/T grant boosts (static_offzonept.go)
	StallZeroAltShared,     // a {0} may-play alternative beside a free, pay-life or otherwise priced permission, or a second one: two of gorge's casts would share one mode (cast_mayzeroalt.go)
}

// initStall is the reason a game of nseat players on this table stalls at its
// start, StallNone when it does not: a pair of cards the kernel cannot play
// together, or a two-seat-only card in a larger game, read off the table. Init
// uses it as a guard for a hand-built table; pairRefusalN refuses a built
// matchup with it, so the two cannot disagree.
func (g *genPrep) initStall(nseat uint8) uint8 {
	switch {
	case nseat > 2 && g.twoSeatOnly:
		return StallTwoSeat // prepTwoSeat, below
	case g.infectStall:
		return StallInfect // kw_infect.go prepInfect
	case g.anyRepl&(rbDblLife|rbPlusLife) == rbDblLife|rbPlusLife:
		return StallLifeRepl // gorge asks the order of a doubler and a plus-N
	case g.l6Stall:
		return StallL6Walk // static_layer6.go prepL6
	case g.anyRepl&rbPrevMill != 0 && g.anyRepl&(rbDblDmg|rbPlusDmg|rbPrevCol) != 0:
		return StallReplPlus // repl_preventmill.go: gorge asks the order of a prevent-and-mill and an amount change
	case g.anyRepl&rbSelfDmg != 0 && g.anyRepl&rbSelfDmgCompete != 0:
		return StallReplPlus // repl_selfdamage.go: gorge asks the order of a self-damage replacement and another damage replacement
	case g.dhMix:
		return StallDiamondMix // mana_diamondmix.go prepDiamondMix
	case g.fbStampOwnCost:
		return StallFbGrantStamp // kw_fbgrant.go prepFbStamp
	case g.atkTaxStall:
		return StallAtkTax // combat_attacktax.go prepAtkTax
	case g.blkTaxStall:
		return StallBlkTax // combat_blocktax.go prepBlkTax
	case g.chPlOrder:
		return StallChoosePlayerOrder // repl_chooseplayer.go prepChoosePlayer
	}
	return StallNone
}

// pairRefusal is kop.PairGuard: why a game on t would stall at its start, and which
// cards carry the two halves. It prepares a throwaway copy of t's header, the same
// prepare Init runs on the table, so it refuses exactly what Init would stall. It
// keeps nothing: a caller may still edit the table before playing it. tablesMu
// serialises it with registerTable's prepare. A table prepare panics on is not
// refused: Init raises that.
func pairRefusal(t *kop.Table) *kop.PairRefusal { return pairRefusalN(t, 2) }

// pairRefusalN is kop.PairGuardN: pairRefusal for a game of nseat players.
func pairRefusalN(t *kop.Table, nseat int) (r *kop.PairRefusal) {
	defer func() {
		if recover() != nil {
			r = nil
		}
	}()
	tablesMu.Lock()
	defer tablesMu.Unlock()
	tt := new(kop.Table) // prepare fills in an empty Filters
	*tt = *t
	pp := prepare(tt)
	g := pp.gen
	if g == nil {
		return nil
	}
	why := g.initStall(uint8(nseat))
	if why == StallNone {
		return nil
	}
	var a, b func(c *kop.CardDef, i int) bool
	var text string
	switch why {
	case StallTwoSeat:
		text = "a lowering that reads the one opponent of a two-player game, in a game of more seats"
		a = func(c *kop.CardDef, _ int) bool { return cardTwoSeat(c) }
		b = func(*kop.CardDef, int) bool { return false }
	case StallInfect:
		text = "infect, toxic, wither or poison beside a damage replacement order ask, a counter-added trigger or proliferate: gorge orders the conversion differently"
		a = func(c *kop.CardDef, i int) bool {
			return c.Infect || c.Toxic != 0 || c.Wither || abilityOn(func(x *kop.Ability) bool {
				return x.Kind == kop.AbStatic && x.Static.Kind == kop.StatContinuous && x.Static.Kw&kop.KwWither != 0 ||
					progHas(x, func(in *kop.Instr) bool {
						return in.Op == kop.OpPoison ||
							in.Op == kop.OpGrantKeyword && (kop.KwMask(uint16(in.C))|kop.KwMask(uint16(in.B.N))<<16)&kop.KwWither != 0
					})
			})(c, i)
		}
		b = func(c *kop.CardDef, i int) bool {
			return g.replBits[i]&(rbDblDmg|rbPrevNC|rbPlusDmg|rbPrevCol|rbPlProt) != 0 || abilityOn(func(x *kop.Ability) bool {
				return x.Kind == kop.AbTriggered && x.Trig.Event == kop.TrigCounterAddedOnce ||
					progHas(x, func(in *kop.Instr) bool {
						return in.Op == kop.OpProliferate || in.Ext() == kop.ExtPreventColor || in.Ext() == kop.ExtPreventDamageScope || in.Ext() == kop.ExtPreventNext
					})
			})(c, i)
		}
	case StallLifeRepl:
		text = "a life-gain doubler beside a life-gain plus-N: gorge asks the CR 616.1 order and they do not commute"
		a = func(_ *kop.CardDef, i int) bool { return g.replBits[i]&rbDblLife != 0 }
		b = func(_ *kop.CardDef, i int) bool { return g.replBits[i]&rbPlusLife != 0 }
	case StallReplPlus:
		if g.anyRepl&rbPrevMill == 0 || g.anyRepl&(rbDblDmg|rbPlusDmg|rbPrevCol) == 0 {
			text, a, b = selfDamagePair(g)
			break
		}
		text = "a prevent-and-mill damage replacement beside a damage doubler, plus-N or colour prevention: gorge asks the CR 616.1 order and the mill changes with it"
		a = func(_ *kop.CardDef, i int) bool { return g.replBits[i]&rbPrevMill != 0 }
		b = func(c *kop.CardDef, i int) bool {
			return g.replBits[i]&(rbDblDmg|rbPlusDmg) != 0 || abilityOn(func(x *kop.Ability) bool {
				return progHas(x, func(in *kop.Instr) bool { return in.Ext() == kop.ExtPreventColor || in.Ext() == kop.ExtPreventNext })
			})(c, i)
		}
	case StallL6Walk:
		text = "a global lose-all override beside a subtype lord the layer-4 walk reads: the loss check would re-enter the walk"
		a = abilityOn(func(x *kop.Ability) bool {
			return x.Kind == kop.AbStatic && x.Static.Kind == kop.StatOverride && x.Static.Override > 0 &&
				t.Overrides[x.Static.Override-1].LoseAbilities && !filterAttachedOnly(t, x.Static.Filter)
		})
		b = abilityOn(func(x *kop.Ability) bool {
			return x.Kind == kop.AbStatic && x.Static.Kind == kop.StatContinuous && (x.Static.AddSub.Any() || x.Static.AddChosen) && filterReadsSub(t, x.Static.Filter)
		})
	case StallFbGrantStamp:
		text = "a stamped flashback grant beside a flashback static with its own cost: gorge offers both granted instances, the kernel prices one (CR 702.34a)"
		a = abilityOn(fbOwnCostStatic)
		b = abilityOn(func(x *kop.Ability) bool {
			return progHas(x, func(in *kop.Instr) bool {
				return in.Op == kop.OpGrantKeyword && (kop.KwMask(uint16(in.C))|kop.KwMask(uint16(in.B.N))<<16)&kop.KwFlashbackOwnCost != 0
			})
		})
	case StallAtkTax:
		text, a, b = atkTaxStallPair(t, g) // combat_attacktax.go
	case StallBlkTax:
		text, a, b = blkTaxStallPair(t, g) // combat_blocktax.go
	case StallChoosePlayerOrder:
		text, a, b = choosePlayerStallPair() // repl_chooseplayer.go
	case StallDiamondMix:
		text = "a Lion's Eye Diamond beside another mana census-gap kind (a filter, a restricted or any-colour restricted source, a counter-cost or tap-another source, a deferred source, a granted mana ability, an each-colour source): gorge's one prefix search activates both and the kernel models the Diamond alone"
		a = abilityOn(kop.DiscardHandManaAb)
		b = func(_ *kop.CardDef, i int) bool { return dhMixGap(tt, g, pp, i) }
	}
	r = &kop.PairRefusal{Reason: StallName(why), Why: text}
	for _, side := range []func(*kop.CardDef, int) bool{a, b} {
		for i := range t.Cards {
			if side(&t.Cards[i], i) {
				r.Cards = append(r.Cards, kop.CardID(i))
			}
		}
	}
	return r
}

// abilityOn is true of a card with an ability pred holds of.
func abilityOn(pred func(*kop.Ability) bool) func(*kop.CardDef, int) bool {
	return func(c *kop.CardDef, _ int) bool {
		for j := range c.Abilities {
			if pred(&c.Abilities[j]) {
				return true
			}
		}
		return false
	}
}

func progHas(a *kop.Ability, pred func(*kop.Instr) bool) bool {
	for j := range a.Prog {
		if pred(&a.Prog[j]) {
			return true
		}
	}
	return false
}

// StallTwoSeat: a game of three or more seats on a table whose lowering reads
// a two-player identity (CardDef.TwoSeat, or a kernel operand that names "the
// opponent" in the singular). It is a declared limit: the two-seat game plays
// the table, a larger game refuses it at its start (initStall).
const StallTwoSeat = 154

func init() { stallNames[StallTwoSeat] = "twoseat" }

// prepTwoSeat sets genPrep.twoSeatOnly when any card of t is two-seat only.
func prepTwoSeat(t *kop.Table, g *genPrep) {
	if sp, _ := prepSpeed(t); MaxSeats > 2 && sp {
		g.twoSeatOnly = true // genState.speed packs two seats' speed, 3 bits each (kw_speed.go)
		return
	}
	for i := range t.Cards {
		if cardTwoSeat(&t.Cards[i]) {
			g.twoSeatOnly = true
			return
		}
		if MaxSeats > 2 && skipUntapCard(&t.Cards[i]) {
			g.twoSeatOnly = true // perm.cseen holds two seats' skip-untap marks (act_skipuntap.go)
			return
		}
		if MaxSeats > 2 && t.Cards[i].Types&kop.TBattle != 0 {
			g.twoSeatOnly = true // a battle's protector is the one opponent (face_battle.go): gorge asks among the opponents (CR 310.10)
			return
		}
	}
}

// cardTwoSeat reports whether c plays only in a two-seat game (kop.CardTwoSeat).
func cardTwoSeat(c *kop.CardDef) bool { return kop.CardTwoSeat(c) }

// skipUntapCard reports whether a program of c carries OpSkipUntap.
func skipUntapCard(c *kop.CardDef) bool {
	for i := range c.Abilities {
		for j := range c.Abilities[i].Prog {
			if c.Abilities[i].Prog[j].Op == kop.OpSkipUntap {
				return true
			}
		}
	}
	return false
}
