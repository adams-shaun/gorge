package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugNoYourTurns (TestTownLockstepTurnsMutation): ExYourTurns' count
// reads 0.
var DebugNoYourTurns bool

// DebugNoRedNC (TestOjerLockstepMutation): ExRedNCDmg reads 0.
var DebugNoRedNC bool

// DebugNoExtraTurnBegun (TestBountyMutation): the extra-turn-begun bit is
// never set, so ExYourExtraTurns stays 0.
var DebugNoExtraTurnBegun bool

// exprCold evaluates the rarely used Expr kinds, out of line of State.expr
// the controller's life lost this turn, an SVar-named modifier's sum,
// a player's hand size, exiled cards matching a filter, the opponents who
// lost life this turn and an object's mana value.
//
//go:noinline
func (s *State) exprCold(k *prep, x *kop.Expr, ctx evalCtx) int {
	switch x.Kind {
	case kop.ExVoteTally:
		// the StoreVoteNum option branch being resolved (act_vote.go): the
		// per-option tally the vote wrote after the last voter. Only a
		// flagged vote branch sets voteCur, so no other read sees a value.
		return int(s.resArm().voteCounts[s.resArm().voteCur])
	case kop.ExOppsAttacked:
		return s.exOppsAttacked(x.N, ctx) // value_oppsattacked.go
	case kop.ExTargetPlayers:
		// TargetedPlayer$Amount: every player target the slot holds, the first
		// and each extra object (gorge targetedPlayers; Hollow Marauder counts
		// the opponents it targets, three at four seats).
		e := ctx.e
		i, ok := tgtSlot(x.Ref)
		if e == nil || !ok {
			return 0
		}
		return s.tgtPlayerCount(k, e, uint8(i))
	case kop.ExScriedThisTurn:
		return int(s.pl[SeatIx(ctx.ctrl)].scry>>(4*(x.N&1))) & 15 // seat-ok: the scry nibble an operand selects
	case kop.ExDilemmasFaced:
		return int(s.g.dilemmas[SeatIx(ctx.ctrl)])
	case kop.ExCastBattlefield:
		// Count$LastStateBattlefieldWithFallback: the
		// count frozen at beginCast into the spell entry's trigMV
		// (value_castbattlefield.go). gorge's fallback for an uncast source is
		// the live battlefield; kgen lowers the head only in a spell's own
		// program, so only the frozen arm is reached in a built table.
		e := ctx.e
		if e == nil {
			return 0
		}
		frozen := int(e.trigMV)
		if e.kind != seSpell {
			return int(s.castBFLiveCount(k, ctx.ctrl, ctx.srcSl, k.t.Cards[e.card].CastBF))
		}
		if DebugCastBFStats {
			live := int(s.castBFLiveCount(k, ctx.ctrl, ctx.srcSl, k.t.Cards[e.card].CastBF))
			if frozen >= 1 {
				castBFFrozenN++
			} else {
				castBFFrozen0++
			}
			if live != frozen {
				castBFFrozenLive++
			}
		}
		if DebugCastBFLive {
			return int(s.castBFLiveCount(k, ctx.ctrl, ctx.srcSl, k.t.Cards[e.card].CastBF))
		}
		return frozen
	case kop.ExSacrificedThisTurn:
		return s.sacrificedThisTurn(k, x, ctx)
	case kop.ExDiscardedThisTurn:
		return s.discardedThisTurn(ctx)
	case kop.ExRingTempted:
		return s.ringCountOf(ctx.ctrl) // the controller's temptation count (act_ring.go)
	case kop.ExYourTurns:
		if x.N == 2 {
			return s.ownMainPhase(ctx.ctrl) // value_countheads.go
		}
		if x.N == 1 {
			if s.active == ctx.ctrl {
				return 1
			}
			return 0
		}
		if DebugNoYourTurns {
			return 0
		}
		return int(s.g.yourTurns[SeatIx(ctx.ctrl)])
	case kop.ExYourExtraTurns:
		return int(s.g.extraTurnBegun>>SeatIx(ctx.ctrl)) & 1
	case kop.ExRedNCDmg:
		if DebugNoRedNC {
			return 0
		}
		return int(s.g.redNC[SeatIx(ctx.ctrl)])
	case kop.ExRemLKIPT:
		return s.remLKIPT() // zone_rememberlki.go
	case kop.ExExiledWithN:
		return s.exwArm(k, x, ctx) // value_countheads.go
	case kop.ExCountHand:
		if x.N == 1 {
			return s.countLibrary(k, x, ctx) // value_countheads.go
		}
		return s.countHand(k, x, ctx) // cast_altfree.go
	case kop.ExVoid:
		if s.voidHolds(k) {
			return s.evalV(k, x.Sub[0], ctx)
		}
		return s.evalV(k, x.Sub[1], ctx)
	case kop.ExIfCastOwnMain:
		if s.castInOwnMain(ctx) { // value_ifcastmain.go
			return s.evalV(k, x.Sub[0], ctx)
		}
		return s.evalV(k, x.Sub[1], ctx)
	case kop.ExChosenMode:
		return s.chosenModeOf(ctx.srcSl) // face_siegemode.go
	case kop.ExChosenNumber:
		return s.chosenNumberOfFor(ctx.src, ctx.srcSl) // repl_choosenumber.go
	case kop.ExUnlockedDoors:
		return s.unlockedDoors(k, ctx.ctrl) // value_countheads.go
	case kop.ExTotalCommanderCastFromCommandZone:
		return s.cmdCastTotal(ctx.ctrl) // value_cmdcast.go
	case kop.ExCmdGreatestMV:
		return s.cmdGreatestMV(k, ctx.ctrl) // value_cmdmv.go
	case kop.ExRevolt:
		if s.revoltHolds(ctx.ctrl) { // zone_leftzonering.go
			return 1
		}
		return 0
	case kop.ExMod:
		return s.exprMod(k, x, ctx) // value_countheads.go
	case kop.ExPow:
		return s.exprPow(k, x, ctx) // value_countheads.go
	case kop.ExLeftZone:
		return s.leftZone(k, x, ctx) // zone_leftzonering.go
	case kop.ExCtrAdded:
		return s.ctrAdded(k, x, ctx) // ctr_addedthisturn.go
	case kop.ExLowestLife:
		return s.lowestLife() // value_lowestlife.go
	case kop.ExLifeLostThisTurn:
		return s.lifeLostOf(k, x.Ref, ctx) // value_lifelost.go
	case kop.ExLifeLostLastTurn:
		return int(s.pl[ctx.ctrl].lastLost)
	case kop.ExSum:
		return s.evalV(k, x.Sub[0], ctx) + s.evalV(k, x.Sub[1], ctx)
	case kop.ExHandSizeOf:
		if p := s.refPlayer(k, x.Ref, ctx); p != 255 {
			if x.N == 1 {
				return s.handCards(p) // Count$ValidHand Card: a token waiting in the hand is no card (CR 111.6)
			}
			return int(s.pl[p].handN) // N 0: Count$CardsInYourHand, the raw zone length
		}
		return 0
	case kop.ExCountExile:
		if x.N == 1 {
			return s.exileTypeAgg(k, x, ctx) // zone_exiledwithstatic.go
		}
		return s.countExile(k, x, ctx)
	case kop.ExPhaseCount:
		return s.phaseCount(x) // act_addphase.go
	case kop.ExStepIn:
		if x.N&(int16(1)<<s.step) != 0 { // the bitmask names the current step
			return 1
		}
		return 0
	case kop.ExAttackedTurn:
		return s.attackedTurn(k, x, ctx) // value_turntallies.go
	case kop.ExOppsLostLife:
		return s.oppsLostLife(ctx.ctrl, x.N) // N 1 legendary hit, N 2 any hit (value_turntallies.go), N 3 any damage (value_oppsdamaged.go)
	case kop.ExNotOwnLibMV:
		return s.notOwnLibMV(k) // value_objectmv.go
	case kop.ExRemEachOwnNonLand:
		return s.remEachOwnNonLand(k) // value_remeachown.go
	case kop.ExCounterKinds:
		return s.counterKinds(k, x, ctx) // value_counterkinds.go
	case kop.ExMVOf:
		if DebugMVOfReplacedZero && x.Ref == kop.RefReplaced {
			return 0 // mutation hook
		}
		if x.Ref == kop.RefSacrificed {
			return s.sacrificedMV(ctx)
		}
		if x.Ref == kop.RefRemembered {
			return s.rememberedMVAgg(k, x.N == kop.AggGreatestMV)
		}
		if x.Ref == kop.RefLibTop {
			return s.libTopMV(k, ctx) // zone_toplibrary.go
		}
		if _, ok := tgtSlot(x.Ref); ok {
			return s.targetMV(k, x.Ref, ctx)
		}
		if o := s.refObjCtx(x.Ref, ctx); o != NoObj {
			return s.refObjMV(k, x.Ref, o, ctx) // value_objectmv.go
		}
		return 0
	case kop.ExRefCount:
		// gorge evalRefProperty Valid: the live object (MatchesSpecCtx), a
		// battlefield permanent or the card in its new zone. N 1 is the
		// Sacrificed$Valid type read's sacrifice-time snapshot.
		return s.countRefMatchesN(k, x.Ref, x.F, ctx, x.N == 1)
	case kop.ExTriggerSpellMVLKI:
		if e := ctx.e; e != nil && e.trigObj != NoObj {
			return int(e.trigMV) // snapshotted at trigger queue time
		}
		return 0
	case kop.ExCountAtLeast:
		// gorge evalCompare: Count$Compare <count> GE<n>.<then>.<else>
		if s.countPerm(k, x.F, ctx) >= int(x.N) {
			return s.evalV(k, x.Sub[0], ctx)
		}
		return s.evalV(k, x.Sub[1], ctx)
	case kop.ExResolvedThisTurn:
		return s.resolvedThisTurn(k)
	case kop.ExThisTurnCast:
		return s.thisTurnCast(k, x, ctx)
	case kop.ExSelfPrepared:
		return s.selfPrepared(x, ctx) // face_prepare.go
	case kop.ExCommittedCrime:
		return s.committedCrime(k, x, ctx)
	case kop.ExMonarch:
		return s.exMonarch(k, x, ctx) // act_monarch.go
	case kop.ExBlessing:
		return s.exBlessing(k, x, ctx) // kw_ascend.go
	case kop.ExThisTurnActGrave:
		return s.thisTurnActGrave(ctx)
	case kop.ExThisTurnActExhaust:
		return s.thisTurnActExhaust(ctx)
	case kop.ExActivations:
		if x.N == 1 {
			return s.activationsOfSource(ctx) // value_activations.go
		}
		return s.activationsOfResolving() // value_activations.go
	case kop.ExThisTurnCastSpec:
		return s.thisTurnCastSpec(k, x, ctx)
	case kop.ExRollResult:
		return int(s.g.roll)
	case kop.ExCompare:
		return s.exprCompare(k, x, ctx)
	case kop.ExGraveTypes:
		if x.N == 1 {
			return s.rememberedTypes(k) // Remembered$CardTypes (value_refheads.go)
		}
		return s.graveTypes(k, ctx.ctrl, ctx.uncounted)
	case kop.ExDrawnThisTurn:
		return int(s.pl[ctx.ctrl].drawnTurn) // gorge CardsDrawnThisTurn
	case kop.ExParty:
		return s.partySize(k, ctx.ctrl)
	case kop.ExYourSpeed:
		if x.N == 1 {
			return s.maxSpeedProp(ctx.ctrl) // kw_maxspeed.go
		}
		return s.speedOf(ctx.ctrl) // kw_speed.go
	case kop.ExDevotion:
		return s.devotion(k, kop.ColorMask(x.N), ctx.ctrl)
	case kop.ExValidAgg:
		return s.validAgg(k, x, ctx)
	case kop.ExGraveAgg:
		return s.graveAgg(k, x, ctx)
	case kop.ExOppExtreme:
		return s.oppExtreme(k, x, ctx)
	case kop.ExPlExtreme:
		return s.plExtreme(k, x, ctx) // value_plrextreme.go
	case kop.ExHighestOppPoison:
		if k.gen != nil && k.gen.anyPoisonHigh {
			return s.highestOppPoison(ctx.ctrl)
		}
		return 0
	case kop.ExCastManaSpent:
		if e := ctx.e; e != nil && e.kind == seSpell {
			return s.spentOfSpell(e) // recordSpent; cast_enterrecord.go
		} else if e != nil && e.kind == seTrigger && int(e.ab) < len(k.t.Cards[e.card].Abilities) &&
			k.t.Cards[e.card].Abilities[e.ab].Trig.Flags2&kop.Tf2SpentBound != 0 {
			// bound as the self-ETB trigger queued (bindTrigX); a source that has
			// left keeps what it was as it left (CR 608.2h), and one back as a
			// new object was not cast for this spend
			if ob := &s.objs[e.obj]; ob.zone == kop.ZoneBattlefield && ob.ver != e.srcVer {
				return 0
			}
			return int(e.x)
		}
		return 0
	case kop.ExDmgAmountReceived:
		return s.dmgAmountReceived(ctx.src) // value_dmgamount.go
	case kop.ExTgtSpent:
		// the resolving spell's target spell's spend, snapshotted as the
		// resolution started (snapTgtSpent; 0 with no snapshot, as gorge's
		// absent target spend)
		if s.g.tgtSpent == 0 {
			return 0
		}
		return int(s.g.tgtSpent) - 1
	case kop.ExConverge:
		return s.exConverge(k, ctx) // mana_converge.go
	case kop.ExAdamant:
		return s.exAdamant(k, x, ctx) // kw_evoke.go
	case kop.ExNumColorsOf:
		return s.refNumColors(k, x.Ref, ctx) // trig_colours.go
	case kop.ExChromaOf:
		return s.refChroma(k, x.Ref, int(x.N), ctx) // trig_colours.go
	case kop.ExPoisonOf, kop.ExEnergyOf, kop.ExProduct, kop.ExAttackersAll, kop.ExSrcRemCounters, kop.ExStaticExileKw:
		return s.exprKvf(k, x, ctx) // filter_valueheads.go
	}
	panic("kernel: expr kind not implemented")
}

// tgtPlayerCount counts the player targets of target slot i on entry e: the
// first and every extra object of a multi-object slot, an illegal one no
// longer counted (CR 608.2b; the resolving entry's bad mask and xtLegalRes).
func (s *State) tgtPlayerCount(k *prep, e *stackEntry, i uint8) int {
	t := e.tgt[i]
	if e == &s.res.e {
		t = s.tgtCode(int(i))
	}
	n := 0
	if isPlayerTarget(t) {
		n++
	}
	if e.xi == 0 {
		return n
	}
	xt := s.xtOf(e)
	for j := 0; j < int(xt.n); j++ {
		if xt.slot[j] != i || !isPlayerTarget(xt.code[j]) {
			continue
		}
		if e == &s.res.e && !s.xtLegalRes(k, e, j, i) {
			continue
		}
		n++
	}
	return n
}

// oppsLostLife is ExOppsLostLife: the number of p's opponents who lost life
// this turn (mode 0), who were dealt combat damage by a legendary creature
// (mode 1, genState.cdLeg bit q) or by any source (mode 2, cdLeg bit 4+q),
// or who were dealt any damage (mode 3, a living opponent's plDmg bit).
// A seat that has left the game is no opponent (gorge opponentGroup walks
// the living).
func (s *State) oppsLostLife(p uint8, mode int16) int {
	c := 0
	for q := uint8(0); q < s.nSeats(); q++ {
		if q == p || s.pl[q].lost {
			continue
		}
		switch mode {
		case 1:
			if s.g.cdLeg>>q&1 != 0 {
				c++
			}
		case 2:
			if s.g.cdLeg>>(4+q)&1 != 0 {
				c++
			}
		case 3:
			if s.oppDamaged(q) { // value_oppsdamaged.go
				c++
			}
		default:
			if s.pl[q].lifeLost > 0 {
				c++
			}
		}
	}
	return c
}

// oppsLifeLost is the life p's opponents lost this turn (gorge
// LifeOppsLostThisTurn sums over the opponents).
func (s *State) oppsLifeLost(p uint8) int {
	v := 0
	for q := uint8(0); q < s.nSeats(); q++ {
		if q != p {
			v += int(s.pl[q].lifeLost)
		}
	}
	return v
}

// countExile is ExCountExile: the exiled cards matching x.F in every living
// seat's exile, or with N 2 only the cards player x.Ref owns (Ravenloft
// Adventurer's defending player, Etrata's damaged one).
func (s *State) countExile(k *prep, x *kop.Expr, ctx evalCtx) int {
	only := uint8(255) // N 2: only the cards player Ref owns (Ravenloft Adventurer's defending player, Etrata's damaged one)
	if x.N == 2 {
		if only = s.refPlayer(k, x.Ref, ctx); only == 255 {
			return 0
		}
	}
	n := 0
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) || only != 255 && pp != only {
			continue
		}
		pl := &s.pl[pp]
		for j := 0; j < int(pl.exileN); j++ {
			o := pl.exile[j]
			if s.matchCardGenAt(k, x.F, s.objs[o].card, pp, ctx.ctrl, o, &ctx) {
				n++
			}
		}
	}
	return n
}
