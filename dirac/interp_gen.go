package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The generated-table interpreter: the ops dirac/kgen appended (OPS.md), the
// Core ops the core's switch leaves out, and the per-object loop over multi-object
// target slots. execGen returns done=false to let the core's switch run an op.

// Resolution ask kinds of the generated table (resCtx.ask), above the core's.
const (
	raGenBase           uint8 = 100 + iota
	raGenBranch               // OpChooseBranch: pick a branch
	raGenPay                  // OpUnlessPaid / OpIfPaid / OpUnlessSwitched: pay or not
	raGenPayPick              // the cards/permanents paying it
	raGenTopBottom            // OpToLibraryChoice: top (1) or bottom (0)
	raGenSearch               // OpSearchEx / OpChooseMove: a card or done
	raGenDig                  // OpDigEx: a looked-at card or done
	raGenPileSplit            // OpTwoPiles: a card for the face-down pile, or done
	raGenPileChoose           // OpTwoPiles: the opponent picks a pile
	raGenChooseEach           // OpChooseEachType: a permanent of the type being asked
	raGenCastFree             // OpCastFree: a card to cast, or done
	raGenChooseCard           // OpChooseCard
	raGenSac                  // OpSacrifice for each player
	raGenMayShuffle           // retired: gorge never asks; kept for the later values' numbers
	raGenDigArrange           // a dig's rest ordered onto the bottom
	raGenLookAck              // OpPeek: gorge's one-option look_ack (Continue) before a private look
	raGenDivide               // OpDealDamageDivided: the target dealt one more damage
	raGenResTarget            // OpChooseTargets: a mid-resolution target (DkTarget)
	raGenCopyTarget           // a spell copy's new-target election as it resolves (DkTarget; res.left = the slot)
	raGenEnchant              // OpCopyAsTokenEx of an Aura: what it enchants as it enters (CR 303.4f)
	raGenTokChoice            // a copy token's as-enters colour / type, asked before it enters (s.g.pre)
	raGenRetChoice            // a card returning from "until this leaves" exile: its as-enters choice, asked in exile (s.g.pre)
	raGenCopyTargets          // raGenCopyTarget for a declaration of several objects: one pick per object, closed by Done (res.arr, res.peekN)
	raGenReplOrder            // gorge's replacement-order ask on a noncombat damage event the op parked (s.g.nc, s.g.ro; repl_gen.go parkNC)
	raGenEnterAsk             // an as-enters choice queued mid-resolution (s.g.ask[0]), asked before the program continues (midResAsk)
	raGenForage               // a trigger's own forage cost: the either-or election (ChCostForage; act_forage.go)
	raGenConfirm              // OpChooseMove's SrConfirm: proceed with the hand move? (gorge hand_move_confirm; trig_delayedtiming.go)
	raGenTapOrUntap           // OpTapOrUntap: tap or untap the permanent (gorge taporuntap; act_discover.go)
	raGenExplore              // OpExplore's nonland reveal: the card into the graveyard or back on top (gorge explore; act_discover.go)
	raGenRearrange            // OpRearrange: the next card from the top of the looked-at ones (gorge KArrange, every card ordered; act_discover.go)
	raGenRearrShuffle         // OpRearrange C&1: shuffle that library now? (gorge arrange_mayshuffle yes/no, ChGenConfirm; act_discover.go)
	raGenBolster              // OpBolster: the tied creature with the least toughness (ChGenBolster; act_proliferate.go)
	raGenProliferate          // OpProliferate: a recipient, or Done (ChGenProlif; act_proliferate.go)
	raGenSupport              // OpSupport: a creature, or Done (ChGenBolster; act_support.go)
	raGenMayShuffleMove       // OpMayShuffleMove A.N=1: shuffle the moved cards in? (gorge objectPathMayShuffle yes/no, ChGenConfirm; act_mayshuffle.go)
	raGenEmpower              // OpEmpower with two or more Jace tokens: which one gets the loyalty counters (ChGenBolster, gorge's "counter_pick"; act_empower.go)
	raGenDemonstrate          // OpDemonstrate: copy the spell? (gorge demonstrate election, ChGenConfirm; kw_demonstrate.go)
	raGenManifestDread        // OpManifestDread: the top-two window's card to manifest (ChGenManifestDread; face_manifest.go)
)

// ChoiceKinds of the generated table's DkChoose decisions (above the core's).
const (
	ChGenBranch        ChoiceKind = 64 + iota // mode = the branch chosen
	ChGenPay                                  // mode 1 = pay / 0 = don't (an unless / if-you-do cost)
	ChGenPayPick                              // source = the card/permanent paying the cost
	ChGenTopBottom                            // mode 1 = top / 0 = bottom
	ChGenSearch                               // source = the card found
	ChGenDig                                  // source = the looked-at card taken
	ChGenPileSplit                            // source = a card put in the face-down pile
	ChGenPileChoose                           // mode 0 = the face-down pile, 1 = the face-up pile
	ChGenChooseEach                           // source = the permanent kept for the type in mode
	ChGenCastFree                             // source = the card cast without paying its mana cost
	ChGenChooseCard                           // source = the chosen card
	ChGenColor                                // mode = the colour (W U B R G = 0..4)
	ChGenType                                 // (target, mode) = the 16-bit index in gorge's creature-type vocabulary
	ChGenDivide                               // source = the target dealt one more of the divided damage (gorge's resolution ask)
	ChGenLeyline                              // source = a Leyline in the opening hand; mode 1 = put it onto the battlefield, 0 = don't
	ChGenAltAdd                               // reserved
	ChGenHiddenPick                           // source = the card picked from a graveyard / exile (as ChGenSearch)
	ChGenHandMove                             // source = the card picked from a hand (as ChGenSearch)
	ChGenPayMode                              // as ChGenPay, posed by gorge as modes (a cost inside an effect): mode 1 = pay
	ChGenMayShuffle                           // retired: gorge never asks; kept for the later values' numbers
	ChGenLookAck                              // the single Continue acknowledging a private look (gorge look_ack)
	ChGenResTarget                            // source = the object chosen for a mid-resolution target slot (OpChooseTargets); AkDone = none (up to)
	ChGenReplOrder                            // source = the replacement effect's source applied next (gorge's KReplacement order ask, CR 616.1)
	ChGenPip                                  // mode = the colour (W U B R G = 0..4) a hybrid pip is paid with (gorge's pay_* pip ask, CR 601.2b)
	ChGenEnchant                              // source = the permanent a non-cast Aura enchants as it enters (CR 303.4f; gorge's "enchant")
	ChCastDivide                              // source = the target given one more point of a divided total beyond its 1, announced while casting/activating (gorge's CR 601.2d "divide" ask)
	ChGenGift                                 // mode 1 = promise the opponent a gift / 0 = don't (gorge's gift_promise / gift_decline, CR 702.174a)
	ChCostForage                              // mode 0 = exile three graveyard cards (gorge forage_exile), 1 = sacrifice the Food in source (forage_food); CR 701.61a
	ChCostExile                               // source = a graveyard card exiled to pay a cost (gorge's exilecost; one pick per card)
	ChGenPayReturn                            // source = the permanent returned to its owner's hand to pay an unless / if-you-do cost (gorge's returncost)
	ChCostReturn                              // source = a permanent returned to its owner's hand to pay an activation cost (gorge's returncost; one pick per permanent)
	ChGenClone                                // mode 0: source = the permanent the entering spell copies; mode 1 (source = the spell): it enters as itself (gorge's ETB "clone" ask)
	ChGenConfirm                              // mode 1 = yes / 0 = no: proceed with an optional hand move (gorge's hand_move_confirm yes/no)
	ChGenReveal                               // source = a card revealed from a hand (gorge's reveal_pick, option kind "reveal")
	ChGenTapOrUntap                           // source = the permanent; mode 1 = tap / 0 = untap (gorge's taporuntap "tap" / "untap")
	ChGenExplore                              // source = the revealed library card; mode 0 = into the graveyard / 1 = back on top (gorge's explore "graveyard" / "top")
	ChGenBolster                              // source = the creature bolstered among those tied for the least toughness (gorge's "counter_pick", CR 701.39a)
	ChGenProlif                               // source = a permanent, or PlayerTarget0+p a player, given another counter of each kind (gorge's "proliferate" pick, CR 701.34a); AkDone closes the pick
	ChStation                                 // source = the creature tapped to station (gorge's KChoose "station", Min=Max=1, CR 702.184a; ctr_kinds.go)
	ChGenConvoke                              // source = the creature tapped for convoke; mode = the colour (W U B R G = 0..4) or 5 for generic, 6 for improvise generic (gorge's convoke_* pick, CR 702.51a)
	ChGenManifestDread                        // source = a card in the top-two window to manifest (gorge's "manifest_dread" pick, CR 701.62a)
	ChGenPayTap                               // source = an untapped permanent tapped to pay a trigger's own literal tapXType cost (gorge's "trigger_cost_tap"; C)
	ChWaterbend                               // source = an artifact/creature tapped for one generic of an activated ability's Waterbend cost (gorge's "waterbend_generic", CR 701.67a; act_waterbend.go)
	ChGenCasualty                             // source = the creature sacrificed for casualty (gorge's optional-sacrifice ask, answer "casualty", Min=Max=1, CR 702.153a; kw_casualty.go)
	ChDefeatCast                              // source = a defeated battle in exile; mode 1 = cast it transformed free (gorge defeat_cast_yes), 0 = leave it (defeat_cast_no); CR 310.11
	ChGenPayExile                             // source = a graveyard card exiled to pay a trigger's own ExileFromGrave cost (gorge's "exile_cost")
	ChRecruitDiscard                          // source = a hand card discarded by Recruit (gorge effRecruit's KChoose option kind "discard"; dirac/act_recruit.go)
	ChGenBargain                              // source = the artifact/enchantment/token sacrificed for Bargain (gorge's optional-sacrifice ask, answer "bargain", Min=Max=1, CR 702.166; kw_bargain.go)
	ChGenTeamwork                             // source = an untapped creature tapped for Teamwork (gorge's KChoose "teamwork", Min 1 / AllowNone / MinSum N, CR 702.194a); AkDone closes the pick, none picked declines; kw_teamwork.go
	ChRoomDoorHalf                            // mode 0 = unlock a door / 1 = lock a door (gorge effUnlockDoor's "Lock or unlock a door?", option 0 = unlock)
	ChRoomDoor                                // source = the Room; target = the canonical ordinal of the door among the candidates; mode = the door face (gorge effUnlockDoor's "Choose a Room door")
)

// sfFree marks a spell cast without paying its mana cost (OpCastFree);
// sfMayPlay one cast through a play permission.
const (
	sfHold uint8 = 1 << 3 // the resolved ability kept on the stack while its free casts are made
	// sfFbGranted: a flashback cast paying the granted instance (the mana
	// cost) of a card with a printed flashback too; set only while the cast
	// is built (cleared at finishBuild)
	sfFbGranted uint8 = 1 << 4
	sfAnyMana   uint8 = 1 << 5 // mana of any type may be spent (StAnyManaType)
	sfFree      uint8 = 1 << 6
	sfMayPlay   uint8 = 1 << 7
)

func (s *State) execGen(k *prep, in *kop.Instr) (ok, done bool) {
	s.refreshTy(k)
	e := &s.res.e
	if k.gen.anyTriggerAttackers && in.Obj == kop.RefTriggerAttackers {
		s.execTriggerAttackers(k, in) // trig_attackers.go
		return true, true
	}
	if k.gen.anyUnion && unionMask(in.Obj) != 0 {
		s.execUnion(k, in)
		return true, true
	}
	// a multi-object target slot: run the op once per object
	if sl, ok := tgtSlot(in.Obj); ok && !s.g.inLoop && s.xtOf(e).n > 0 && in.Op != kop.OpDealDamageDivided {
		slot := uint8(sl)
		if s.slotHasExtra(e, slot) {
			if in.Op == kop.OpSacrifice && in.C&kop.OptAsk == 0 {
				// a sacrifice over several targeted players asks each of
				// them in turn (eachPlayersSac)
				if ok, done := s.eachPlayersSac(k, in, slot); done {
					return ok, true
				}
			}
			if in.Op == kop.OpDiscard && in.F == 0 && in.C == kop.ChooseSelf {
				if ok, done := s.eachPlayersDiscard(k, in, slot); done {
					return ok, true
				}
			}
			s.execEachTarget(k, in, slot)
			return true, true
		}
	}
	switch in.Op {
	case kop.OpBounce:
		if in.C == 0 {
			return true, false // the core's
		}
		s.bounceRemember(k, in) // zone_rememberparams.go
	case kop.OpGiveGift:
		s.giveGift(k, e.ctrl)
	case kop.OpClassLevelUp:
		s.classLevelUp(k, in)
	case kop.OpCopySpell:
		return s.execCopySpell(k, in), true
	case kop.OpDemonstrate:
		return s.demonstrate(k, in), true
	case kop.OpNinjutsu:
		s.ninjutsuResolve(k) // CR 702.49b (kw_ninjutsu.go)
	case kop.OpRepeatWhile:
		// gorge effRepeat with RepeatCheckSVar$: MaxRepeat$ evaluated once
		// as the cap, the gate re-evaluated after every pass (loopBack)
		n := s.value(k, in.A)
		r := &s.res
		if n <= 0 {
			r.pc += uint8(in.B.N)
			break
		}
		if r.loopD >= 2 {
			panic("kernel: OpRepeatWhile nested deeper than 2")
		}
		if n > 255 {
			n = 255
		}
		r.loopN[r.loopD] = uint8(n)
		r.loopS[r.loopD] = r.pc + 1
		r.loopPC[r.loopD] = r.pc + 1 + uint8(in.B.N)
		r.loopD++
	case kop.OpSkipUnless:
		if !s.condEval(k, kop.CondID(in.C), s.ctxEntry(e)) {
			s.res.pc += uint8(in.A.N)
		}
	case kop.OpBranch:
		if s.g.branch != uint8(in.C) {
			s.res.pc += uint8(in.A.N)
		}
	case kop.OpChooseBranch:
		p := s.payerOf(k, in.Obj) // RefEach: the player of the repeat
		if p == 255 {
			p = e.ctrl
		}
		s.g.branch = uint8(in.A.N) // the fallback until answered
		s.res.left = uint8(in.A.N)
		any := false
		for b := uint8(0); b < uint8(in.A.N); b++ {
			if s.branchPayable(k, b, p) {
				any = true
			}
		}
		if !any {
			break // no choice can be made: the fallback branch runs
		}
		s.res.ask, s.res.askP = raGenBranch, p
		return false, true
	case kop.OpRepeatEachPlayer:
		first, second := s.eachPlayers(k, in.Obj)
		if in.C&1 == 0 && in.Obj == kop.RefEachPlayer {
			// the C=0 form keeps APNAP, also when the controller is the
			// active player (C&1: gorge RepeatEach's AliveFrom)
			first, second = s.active, 1-s.active
		}
		if in.C&kop.RepeatEachSeatOrder != 0 {
			first, second = 0, 1
		}
		if in.B.N != 0 { // Player.<q> set (B): the predicate, read before either player acts
			first, second = s.playerSet(k, in, first, second)
		}
		if first == 255 {
			s.res.pc += uint8(in.A.N)
			break
		}
		s.g.eachPl = first
		s.g.repNext = second
		s.g.repPC = s.res.pc + 1
		s.g.repEnd = s.res.pc + 1 + uint8(in.A.N)
	case kop.OpUnlessPaid, kop.OpIfPaid, kop.OpUnlessSwitched:
		return s.execPay(k, in), true
	case kop.OpDealDamageEx:
		s.execDamageEx(k, in)
	case kop.OpResolveDamageMap:
		n := s.g.ndmg
		s.g.ndmg = 0
		var codes [maxDmgMap]uint8
		var amts [maxDmgMap]int
		for i := 0; i < int(n); i++ {
			codes[i], amts[i] = s.g.dmg[i].code, int(s.g.dmg[i].amt)
		}
		s.damageBatch(k, codes[:n], amts[:n])
	case kop.OpDealDamageDivided:
		// an announced cast-time division is dealt as announced (CR 601.2d,
		// 608.2b); otherwise gorge divides at resolution
		// (effects/damage_deal.go): one repeatable pick per damage over the
		// slot's legal targets, asked only when there are two or more
		if s.dealAnnouncedDivision(k, e, uint8(tgtSlotOf(in.Obj))) {
			break
		}
		var codes [maxXTgt + 1]uint8
		m := s.divTargets(k, e, uint8(tgtSlotOf(in.Obj)), &codes)
		total := s.value(k, in.A)
		if m == 0 || total <= 0 {
			break
		}
		if m == 1 {
			amts := [1]int{total}
			s.damageBatch(k, codes[:1], amts[:])
			break
		}
		s.xtOf(e).div = [maxXTgt + 1]uint8{}
		s.g.pickLeft = uint8(min(total, 255))
		s.res.ask, s.res.askP = raGenDivide, e.ctrl
		return false, true
	case kop.OpDealDamage:
		if in.Obj == kop.RefEachPlayer {
			n := s.value(k, in.A)
			if !s.g.dmgSeqOn { // a resume (raGenReplOrder) continues the frozen sequence
				s.damageSeqStart(k, slotMask{}, true)
				if in.B.N != 0 { // Player.<q> set (B)
					s.g.dmgSeqPl = s.playerSetMask(k, in)
				}
			}
			s.damageSeqNext(k, n)
			return true, true
		}
		return true, false
	case kop.OpDiscard:
		if in.C&kop.OptAsk != 0 { // Optional$ (act_optionalask.go)
			return s.optDiscardGen(k, in, e)
		}
		if in.F == 0 && in.C != kop.ChooseOpp {
			return true, false
		}
		// a filtered discard (Pilfer: a nonland card): the core's ask offers only
		// matching cards (discardOK); with no more than N matching, they are
		// discarded without an ask
		p := s.playerRef(k, in.Obj)
		if p == 255 {
			break
		}
		n := s.value(k, in.A)
		if n < 1 {
			break // gorge discardBounds: a resolved zero discards nothing, asks nothing
		}
		pl := &s.pl[p]
		m := 0
		for i := 0; i < int(pl.handN); i++ {
			if s.discardMatch(k, in.F, pl.hand[i]) {
				m++
			}
		}
		if (m > n || in.C == kop.ChooseOpp) && m > 0 {
			// an opponent choosing from a revealed hand is always asked
			// gorge asks whenever the hand holds more than N cards, picks
			// among the matching ones (discardOK)
			s.res.ask = raDiscard
			s.res.peekI = 0
			s.res.askP = p
			if in.C == kop.ChooseOpp {
				s.res.askP = e.ctrl // the source's controller chooses (gorge discardChooser)
			}
			s.res.left = uint8(min(n, m))
			s.res.chosen = p
			return false, true
		}
		for i := 0; i < int(pl.handN); {
			if o := pl.hand[i]; s.discardMatch(k, in.F, o) && !s.park(k, o, parkDiscard, 0) {
				s.discardObj(k, o)
				continue
			}
			i++
		}
	case kop.OpPutCounter:
		s.g.lastCK = kop.CounterKind(in.C) // CounterAddedOnce reads the kind
		if in.Obj == kop.RefRemembered && s.g.nrem > 1 {
			s.putCounterRemembered(k, in)
			return true, true
		}
		return true, false
	case kop.OpUntap:
		if in.Obj == kop.RefRemembered && s.g.nrem > 1 {
			s.untapRemembered(k, in)
			return true, true
		}
		return true, false
	case kop.OpSacrifice:
		if in.C&kop.OptAsk != 0 { // Optional$ (act_optionalask.go)
			if in.Obj == kop.RefEachPlayer || in.Obj == kop.RefEachOpponent {
				first, second := s.eachPlayers(k, in.Obj)
				s.g.repNext = second
				return s.startOptSac(k, in, first), true
			}
			p := s.playerRef(k, in.Obj)
			if p == 255 {
				return true, false
			}
			return s.optSac(k, in, p, s.value(k, in.A)), true
		}
		if in.Obj == kop.RefEachPlayer || in.Obj == kop.RefEachOpponent {
			first, second := s.eachPlayers(k, in.Obj)
			s.g.repNext = second
			return s.startSac(k, in, first), true
		}
		return true, false
	case kop.OpExile, kop.OpExileEx:
		if in.Op == kop.OpExileEx && in.C&kop.MvForgetOther != 0 {
			s.forgetOtherMove(k, in) // zone_rememberparams.go
			break
		}
		o := s.objRefObj(in.Obj)
		if o == NoObj {
			break
		}
		if in.C&kop.MvFromBattlefield != 0 && s.objs[o].zone != kop.ZoneBattlefield {
			break
		}
		if in.C&kop.MvFromGraveyard != 0 && s.objs[o].zone != kop.ZoneGraveyard {
			// a trigger object exiled from the graveyard must still be there:
			// another effect already moved it (a second Promise of Tomorrow
			// after the first exiled the dying creature), so it is a new
			// object this ability cannot find (CR 400.7, 400.7j); nothing
			// moves and no exiled-with link is recorded
			break
		}
		if in.Obj == kop.RefTriggerObj && k.gen != nil && e.li == 0 && s.objs[o].zone != kop.ZoneGraveyard {
			// non-graveyard-origin fallback (MvFromGraveyard covers the
			// Origin$ Graveyard shapes above): the triggering card was not on the battlefield when it
			// triggered (Tinybones' discard trigger, the only FDN exile
			// of a trigger object, Origin$ Graveyard), so one elsewhere
			// now has moved since: back to the battlefield (the
			// discarded Reassembling Skeleton returned itself, pair 2469)
			// or exiled in response (Abyssal Harvester, pairs 4177 4280).
			// A new object: nothing moves and no stash counter is added
			// (CR 400.7, 608.2b)
			break
		}
		if !s.moveObj(k, in.Obj, o, kop.ZoneExile, in.Op == kop.OpExileEx && in.C&kop.MvRemember != 0) {
			break
		}
		if in.Op == kop.OpExileEx {
			if in.C&kop.MvRemember != 0 {
				s.remember(o)
			}
			if in.C&kop.MvForgetChanged != 0 {
				s.forgetChanged(o) // zone_forget.go
			}
			if in.C&kop.MvLKI != 0 {
				s.markRemLKI(o) // zone_rememberparams.go
			}
			if s.value(k, in.A) > 0 && kop.CounterKind(in.B.N) == kop.CtrStash {
				s.stashSet(k, o)
			}
		}
	case kop.OpReturnEx:
		if in.C>>3&kop.MvForgetOther != 0 {
			if s.forgetOtherMove(k, in) {
				return false, true
			}
			break
		}
		if s.returnsRemembered(in) {
			return !s.returnRemembered(k, in), true
		}
		if s.execReturn(k, in, NoSlot) {
			return false, true
		}
	case kop.OpReturn:
		if s.returnsRemembered(in) {
			return !s.returnRemembered(k, in), true
		}
		o := s.objRefObj(in.Obj)
		miss := o == NoObj
		if o != NoObj {
			z := s.objs[o].zone
			miss = z == kop.ZoneBattlefield || z == kop.ZoneStack || z == kop.ZoneNone || s.tokenLeftBattlefield(o) || returnLeftOrigin(in, &s.objs[o]) || s.instantStays(k, o, kop.Zone(in.C))
		}
		s.g.retMiss = miss
		if kop.Zone(in.C) != kop.ZoneBattlefield || o == NoObj || k.cards[s.objs[o].card].types&kop.TAura == 0 {
			return true, false // the core's
		}
		if miss {
			break
		}
		if s.returnAura(k, o, NoSlot) {
			return false, true
		}
		s.g.retMiss = s.objs[o].zone != kop.ZoneBattlefield // an Aura with no legal host stays where it is (CR 303.4g)
	case kop.OpToLibraryChoice:
		o := s.objRefObj(in.Obj)
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield && s.objs[o].zone != kop.ZoneStack {
			break // a spell target too (TgtPermOrSpell): its owner is asked
		}
		s.res.ask, s.res.askP = raGenTopBottom, s.objs[o].owner()
		return false, true
	case kop.OpSearchEx, kop.OpChooseMove:
		return s.startSearch(k, in), true
	case kop.OpChangeZoneAll, kop.OpChangeZoneAllEx:
		return s.execChangeZoneAll(k, in, false), true
	case kop.OpDigEx:
		return s.startDig(k, in), true
	case kop.OpDigUntil:
		s.res.left = 0 // the parked-tail marker (zone_parkedtail.go)
		return s.execDigUntil(k, in, false), true
	case kop.OpPeek:
		p := s.playerRef(k, in.Obj)
		if p == 255 {
			p = e.ctrl
		}
		if in.C != 0 {
			if in.C&kop.PeekMayReveal != 0 {
				return s.peekMayReveal(k, in, p), true // act_peekreveal.go
			}
			if in.Obj == kop.RefEachPlayer {
				// Defined$ Player: every player reveals, the resolving
				// controller first, as gorge's definedPlayers walks AliveFrom.
				first, second := s.eachPlayers(k, in.Obj)
				s.peekPublic(k, in, first)
				s.peekPublic(k, in, second)
				break
			}
			s.peekPublic(k, in, p)
			break
		}
		pl := &s.pl[p]
		n := s.value(k, in.A)
		s.g.nrem = 0
		for i := 0; i < n && i < pl.libLen() && i < maxRem; i++ {
			s.remember(pl.lib[int(pl.libLo)+i])
		}
		if s.g.nrem > 0 {
			// gorge poses the bare look's pacing ack (NoReveal$ True) first,
			// to the effect's controller (effReveal: asker = c.Controller)
			s.res.ask, s.res.askP = raGenLookAck, e.ctrl
			return false, true
		}
	case kop.OpGrantKeyword:
		if o := s.objRefObj(in.Obj); o != NoObj && s.objs[o].zone == kop.ZoneStack {
			s.grantSpellKw(o, kop.KwMask(uint16(in.C))|kop.KwMask(uint16(in.B.N))<<16)
			return true, true
		}
		// a graveyard card gains flashback with its mana cost until end of
		// turn (Sphinx of Forgotten Lore); a permanent's grant is the core's
		if kop.KwMask(uint16(in.B.N))<<16&kop.KwFlashbackOwnCost == 0 {
			return true, false
		}
		if o := s.objRefObj(in.Obj); o != NoObj && s.objs[o].zone == kop.ZoneGraveyard {
			s.touch(o)
			s.grantFB(k, o)
			return true, true
		}
		return true, false
	case kop.OpChooseTargets:
		t := tspecE(k, e, int(in.C))
		if t == nil || t.TFlags&kop.TgfGiftRange != 0 && s.giftNoSlot(t, e) {
			break
		}
		var buf [MaxOptions]uint8
		if s.targetCands(k, t, e.ctrl, s.resSrcSlot(k), e.obj, &buf) == 0 {
			break // nothing to choose: gorge asks no empty target
		}
		s.res.ask, s.res.askP = raGenResTarget, e.ctrl
		return false, true
	case kop.OpTwoPiles:
		if s.g.nrem == 0 {
			break
		}
		s.g.lookSel = 0
		s.res.ask, s.res.askP = raGenPileSplit, e.ctrl
		if s.g.nrem == 1 {
			// gorge poses no split for a one-card set: pile A is the card
			// (effects/twopiles.go), the opponent still picks (pair 1004)
			s.g.lookSel, s.resArm().pileOrd[0] = 1, 0
			s.res.ask, s.res.askP = raGenPileChoose, 1-e.ctrl
		}
		s.g.pickZ = uint8(in.C)
		return false, true
	case kop.OpChooseEachType:
		return s.startChooseEach(k, in), true
	case kop.OpSacrificeAll:
		var hit slotMask
		src := s.objRef(k, kop.RefSelf)
		for m := s.occ; m.any(); m = m.rest() {
			sl := m.first()
			if s.matchPermGen(k, in.F, sl, e.ctrl, src, layerFull) {
				hit = hit.with(sl)
			}
		}
		s.fireSacAll(k, hit)
		if in.C&kop.MvRemember != 0 {
			s.sacAllRemember(k, hit) // RememberSacrificed$ (zone_rememberwrites.go)
		} else {
			s.leaveAll(k, hit, kop.ZoneGraveyard)
		}
	case kop.OpCastFree:
		s.g.nfree, s.g.nfreeAll = 0, 0
		if in.Obj != kop.RefRemembered || in.C&0x78 != 0 || in.F != 0 {
			s.castFreeFillGen(k, in, e) // P2/P3/P4 and P1's new bits
		} else {
			for i := 0; i < int(s.g.nrem); i++ {
				o := s.g.rem[i]
				ob := &s.objs[o]
				// a spell whose mandatory targets cannot all be chosen is never
				// offered (CR 601.2c, effPlay filters through
				// castTargetsHost): it stays exiled and the trigger finishes
				if ob.zone == kop.Zone(in.C) && !k.cards[ob.card].land &&
					s.spellTargetsFeasible(k, ob.card, e.ctrl) && !(k.gen.anyCantCast && s.castForbidden(k, e.ctrl, ob.card, o)) {
					if s.g.nfree >= uint8(len(s.g.free)) {
						s.capFull(StallFree)
						break
					}
					s.g.free[s.g.nfree] = o
					s.g.nfree++
				}
			}
		}
		// a mode's index ranks in the pose-time accepted list, frozen:
		// gorge's modes decision is built once over the castable candidates
		// and a cast keeps every later card's place (gorge does not
		// re-index)
		copy(s.g.freeAll[:s.g.nfree], s.g.free[:s.g.nfree])
		s.g.nfreeAll = s.g.nfree
		if s.g.nfree == 0 {
			break
		}
		// freeP: bit 0 caster, bits 1-3 zone, bit 4 ReplaceGraveyard (the
		// freeQ drain reads them; turn_gen masks the player)
		s.g.freeP = e.ctrl | uint8(in.C&7)<<1
		if in.C&32 != 0 {
			s.g.freeP |= 16
		}
		if in.C&64 != 0 {
			s.g.freeP |= 32 // Play CopyCard$ True: the cast is of a copy (cast_copycast.go)
		}
		s.res.ask, s.res.askP = raGenCastFree, e.ctrl
		return false, true
	case kop.OpMayPlay:
		if in.Obj == kop.RefRemembered {
			n0 := s.g.nmay
			s.mayPlayRemembered(k, in, e.ctrl) // every remembered card
			if k.gen.anyMayWhileSrc {
				s.mayWhileSrcStamp(n0, e.obj) // cast_whilesource.go
				if in.A.N == kop.DurWhileValid {
					for i := n0; i < s.g.nmay; i++ {
						s.g.may[i].turn = uint16(in.F)
					}
				}
			}
			break
		}
		o := s.objRefObj(in.Obj)
		if o == NoObj || s.objs[o].zone != kop.Zone(in.C) {
			break
		}
		if s.g.nmay >= maxMay {
			s.capFull(StallMayPlay)
			break
		}
		headed := false
		mk := uint8(in.B.N)&kop.MayPlayKinds | uint8(in.B.N>>8)&kop.MayPlayAdvKind
		s.g.may[s.g.nmay] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: e.ctrl, until: grantUntil(uint8(in.A.N), mk, &headed), kind: mk, turn: s.turn}
		s.g.nmay++
		if k.gen.anyMayWhileSrc {
			s.mayWhileSrcStamp(s.g.nmay-1, e.obj) // cast_whilesource.go
		}
	case kop.OpCreateEmblem:
		p := s.playerRef(k, in.Obj)
		if p == 255 {
			p = e.ctrl
		}
		if k.gen.anyWhileHost && k.t.Emblems[in.C].WhileHost && s.hostEmblemDone(e, p, uint8(in.C)+1) {
			break // filter_combatwords.go
		}
		if s.g.nemb[p] < maxEmb {
			s.g.emb[p][s.g.nemb[p]] = uint8(in.C) + 1
			s.g.embC[p][s.g.nemb[p]] = e.card
			s.g.embO[p][s.g.nemb[p]] = e.obj
			s.g.embN[p][s.g.nemb[p]] = s.freezeEmbNum(k, in.A, s.ctxEntry(e))
			s.g.nemb[p]++
		} else {
			s.capFull(StallEmblem)
		}
	case kop.OpGainControl:
		s.opGainControl(k, in)
	case kop.OpOverride:
		if in.Obj == kop.RefRemembered && s.g.nrem > 1 {
			s.overrideRemembered(k, in) // zone_changezoneall.go: every remembered object
			break
		}
		if in.B.N == kop.OvEnterWith {
			break // applied as the return entered (execReturnObj)
		}
		o := s.objRefObj(in.Obj)
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
			break
		}
		if pc := s.res.pc; pc > 0 && s.g.retMiss {
			if p := &s.prog(k)[pc-1]; (p.Op == kop.OpReturnEx || p.Op == kop.OpReturn) && p.Obj == in.Obj {
				break // a return's StaticEffect rider: the return moved nothing (Valkyrie's Call after Infernal Vessel's own return; Brilliance Unleashed's plain return of an artifact creature already on the battlefield)
			}
		}
		if k.gen.anyWhileHost && k.t.Overrides[in.C].WhileHost {
			s.applyWhileHost(k, s.objs[o].slot, int(in.C), e) // filter_combatwords.go
			break
		}
		if in.B.N == kop.OvChosenColor {
			s.ovChosenColor(k, s.objs[o].slot, int(in.C), e.card, s.ovDurFl(in.A.N)) // static_animatechosen.go
			break
		}
		s.applyOverrideFl(k, s.objs[o].slot, int(in.C), e.card, s.ovDurFl(in.A.N))
	case kop.OpReflexiveTrigger:
		n := s.value(k, in.A)
		tobj, amt := NoObj, s.reflexAmt(k, in.B) // trig_reflexiveremembered.go: TriggerRemembered$ (B is the zero Const otherwise)
		if in.Obj != kop.RefNone {
			tobj, amt = s.reflexCapture(in.Obj) // zone_rememberwrites.go: RememberObjects$ Targeted
		}
		for i := 0; i < n; i++ {
			s.enqueue(e.obj, e.srcVer, e.card, uint8(in.C), e.ctrl, tobj, e.ctrl, amt)
		}
	case kop.OpDelayedProgram:
		o := s.objRefObj(in.Obj)
		if s.g.ndp >= maxDProg {
			s.capFull(StallDProg)
			break
		}
		if in.B.N == kop.DpMany {
			s.delayMany(e, in) // trig_delayedmany.go (B=3 also has the DpGroup bit: tested first)
			break
		}
		if in.B.N&kop.DpGroup != 0 {
			s.delayGroup(e, in) // kw_mobilize.go
			break
		}
		if in.B.N&kop.DpTrack != 0 {
			s.delayTracked(e, in) // kw_decayed.go
			break
		}
		if o == NoObj {
			// an empty remembered set still creates the delayed trigger
			// (CR 603.7; gorge effDelayedTrigger): it fires and does nothing
			s.delayEmptyGen(e, uint8(in.C))
			if in.A.N != 0 {
				s.g.dp[s.g.ndp-1].n |= dpWhen(in)
			}
			break
		}
		d := dprog{card: e.card, ab: uint8(in.C), ctrl: e.ctrl, src: e.obj, n: 1 | dpWhen(in)}
		d.objs[0], d.vers[0] = o, s.objs[o].ver
		if in.Obj == kop.RefTriggerObj && o == e.obj && s.objs[o].ver != e.srcVer+1 && !delayCopies(k, e.card, uint8(in.C)) {
			// the card that died has moved on again (exiled from the
			// graveyard before the trigger resolved): the new card it
			// remembers is gone
			d.objs[0] = NoObj
		}
		s.g.dp[s.g.ndp] = d
		s.g.dpLKI[s.g.ndp] = e.li
		s.g.ndp++
	case kop.OpClearRemembered:
		s.g.nrem = 0
		s.g.remCnt = 0
	case kop.OpClearChosen:
		s.g.nchosen = 0
	case kop.OpDoubleCounters:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			s.doubleCountersAt(k, sl, in.C)
		}
	case kop.OpRemoveCounterRem:
		s.g.remCnt = 0
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			n := s.value(k, in.A)
			s.touchSl(sl)
			p := &s.perms[sl]
			p.seeCtr(s) // gorge lists a kind removed to none at 0
			if p.ctrN(s, int(in.C)) < n {
				n = p.ctrN(s, int(in.C))
			}
			p.subCtrN(s, int(in.C), n)
			s.g.remCnt = uint16(n)
		}
	case kop.OpEnterTapped:
		if o := s.g.replaced; o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
			s.perms[s.objs[o].slot].flags |= pfTapped
			s.touch(o)
		}
	case kop.OpEnterWithCounters:
		if o := s.g.replaced; o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
			s.touch(o)
			p := &s.perms[s.objs[o].slot]
			amt := s.enterCountValue(k, in.A, s.objs[o].slot, p.ctrl)
			if k.gen.anyEntryMerge && s.entryAcc(int(in.C), amt) {
				break // ctr_entryperkind.go: joins the entry's one put of this kind
			}
			s.placeCounters(k, s.objs[o].slot, int(in.C), amt, p.ctrl, ctrPutEffect)
		}
	case kop.OpCopyAsTokenEx:
		if s.execCopyToken(k, in) {
			return false, true
		}
	case kop.OpCreateToken, kop.OpCreateTokenEx:
		p := s.playerRef(k, in.Obj)
		if p == 255 {
			p = e.ctrl
		}
		n := s.replTokens(k, p, CardID(in.C), s.value(k, in.A))
		if in.Op == kop.OpCreateTokenEx {
			s.g.tokMade = 0
			if in.B.N&kop.TokAttach != 0 {
				if ok, done := s.createAttachedToken(k, in, p, n); done {
					return ok, true
				}
			}
		}
		if n > 1 {
			if s.createTokensHeld(k, in, p, n) {
				return true, true
			}
			break
		}
		for i := 0; i < n; i++ {
			if s.nearCapacity() {
				s.stall(StallBoard)
				return true, true
			}
			o := s.newToken(CardID(in.C), p)
			tapped := in.B.N&kop.TrTapped != 0
			if in.Op == kop.OpCreateTokenEx {
				tapped = in.B.N&kop.TokTapped != 0
				s.g.tokMade++
				if in.B.N&kop.TokRememberObj != 0 {
					s.remember(o)
				}
			}
			sl := s.enter(k, o, p, tapped, kop.ZoneLibrary)
			if in.Op == kop.OpCreateTokenEx && in.B.N&(kop.TokSacAtEnd|kop.TokExileAtEnd) != 0 {
				s.tokenAtEOT(in, o)
			}
			if in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokAttacking != 0 {
				s.tokenAttacks(k, in, sl, p) // filter_combatwords.go
			}
			if k.gen.anyTokCr {
				s.fireTokenCreated(k, sl)
			}
			if in.Op == kop.OpCreateToken && in.B.N&kop.TrHasteEOT != 0 {
				s.perms[sl].eotKw |= kop.KwHaste
				s.touchSl(sl)
				s.promoKw(k, sl, kop.KwHaste)
			}
			s.res.lastTok = o
		}
	case kop.OpEndTurn:
		s.endTurnExile()
		e.flags |= sfFlashback // exiled, not put into the graveyard
		s.g.endTurn = true
	case kop.OpExileUntilLeaves:
		o := s.objRefObj(in.Obj)
		src := s.objRef(k, kop.RefSelf)
		if o == NoObj || src == NoSlot || s.objs[o].zone != kop.ZoneBattlefield {
			break
		}
		s.leave(k, s.objs[o].slot, kop.ZoneExile, false)
		if s.g.nlinks < uint8(len(s.g.links)) {
			l := &s.g.links[s.g.nlinks]
			l.src, l.sver, l.obj = e.obj, s.objs[e.obj].ver, o
			s.g.nlinks++
		} else {
			s.capFull(StallLinks)
		}
	case kop.OpChooseColor:
		// inside an as-enters program: asked before the next priority
		if o := s.g.replaced; o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
			s.queueAsk(s.objs[o].slot, 1)
		}
	case kop.OpChooseType:
		// inside an as-enters program: asked before the next priority;
		// otherwise it is a mid-resolution "choose a creature type"
		// (gorge effChooseType) and the answer is this resolution's chosen
		// subtype
		card := uint8(0)
		if in.C != 0 {
			card = resChosenCard // a card-type category (kw_madness.go)
		}
		if o := s.g.replaced; in.C == 0 && o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
			s.queueAsk(s.objs[o].slot, 2)
		} else if _, sl := s.resolutionTypeSource(); sl != NoSlot && s.perms[sl].chosen != 255 {
			// gorge effChooseType: a source that already carries a
			// ChosenType keeps it (the second activation is no-op)
			s.res.chosen = s.perms[sl].chosen
			s.res.bad |= resChosenType | card
		} else if o, sl := s.resolutionTypeSource(); sl == NoSlot && o != NoObj && s.objs[o].chosen != 255 {
			// a spell card keeps its ChosenType across the graveyard
			// (gorge apply_zone.go only clears it on a battlefield
			// leave), so a flashback re-cast reuses the first cast's
			// answer instead of asking again
			s.res.chosen = s.objs[o].chosen
			s.res.bad |= resChosenType | card
		} else {
			s.res.ask, s.res.askP = raChooseType, e.ctrl
			return false, true
		}
	case kop.OpChooseCard:
		s.g.nchosen = 0
		p := s.playerRef(k, in.Obj)
		if p == 255 {
			p = e.ctrl
		}
		if DebugChooseCardAmount2 {
			p = 1 - p // TestKchoosecardparamsMutation: the opponent picks
		}
		if c := in.C; c == chooseAttachPick || c == chooseAttachPickOpt {
			return s.attachPick(k, in, p) // tgt_attachpick.go
		}
		if in.C == choosePutCounterPick {
			return s.putCounterPick(k, in, p) // ctr_putparams.go
		}
		if s.cardCands(k, in.F, p, nil) == 0 {
			break
		}
		s.res.ask, s.res.askP = raGenChooseCard, p
		return false, true
	case kop.OpAttach:
		src := s.objRef(k, kop.RefSelf)
		dst := s.objRef(k, in.Obj)
		if src != NoSlot && dst != NoSlot && src != dst {
			s.attachTo(k, src, dst)
			s.touchSl(src)
			if in.C&kop.MvRemember != 0 {
				s.remember(s.perms[src].obj) // RememberAttached$
			}
		}
	default:
		if in.Op == kop.OpTransform { // a reserved low op, lowered
			return s.execSetState(k, in)
		}
		if in.Op >= kop.OpPutCounterAll {
			if ok, done := s.execCold1(k, in); done {
				return ok, true
			}
		}
		if in.Op >= kop.OpSkipUnless {
			panic("kernel: generated op not implemented: " + in.Op.String())
		}
		return true, false
	}
	return true, true
}

// slotHasExtra reports whether target slot i has objects beyond tgt[i].
func (s *State) slotHasExtra(e *stackEntry, i uint8) bool {
	for j := 0; j < int(s.xtOf(e).n); j++ {
		if s.xtOf(e).slot[j] == i {
			return true
		}
	}
	return false
}

// xtLegal rechecks extra target j at resolution.
// divTargets lists the divided slot's legal targets in pick order (the
// first in tgt[slot], then its extra objects).
func (s *State) divTargets(k *prep, e *stackEntry, slot uint8, codes *[maxXTgt + 1]uint8) int {
	m := 0
	if c := s.tgtCode(int(slot)); c != NoTarget {
		codes[m] = c
		m++
	}
	for i := 0; i < int(s.xtOf(e).n); i++ {
		if s.xtOf(e).slot[i] == slot && s.xtLegal(k, e, i) {
			codes[m] = s.xtOf(e).code[i]
			m++
		}
	}
	return m
}

// FeatDivideIndex is target code c's index among the divided slot's
// targets in pick order: while casting/activating (bsDivide, gorge's
// cast-time division ask) or among the resolving slot's legal targets
// (gorge's division option order). 0 when no division is being asked.
func (s *State) FeatDivideIndex(c uint8) int {
	if s.ctl == ctlBuild && s.bld().stage == bsDivide {
		var codes [maxXTgt + 1]uint8
		n := s.divPicks(&s.bld().e, s.bld().divSlot, &codes)
		for i := 0; i < n; i++ {
			if codes[i] == c {
				return i
			}
		}
		return 0
	}
	if s.ctl != ctlResolve || s.res.ask != raGenDivide {
		return 0
	}
	k := s.k()
	in := &s.prog(k)[s.res.pc]
	var codes [maxXTgt + 1]uint8
	m := s.divTargets(k, &s.res.e, uint8(tgtSlotOf(in.Obj)), &codes)
	for i := 0; i < m; i++ {
		if codes[i] == c {
			return i
		}
	}
	return 0
}

// answerDivide records one damage for the picked target; the last pick
// deals the division.
func (s *State) answerDivide(k *prep, c uint8) {
	e := &s.res.e
	in := &s.prog(k)[s.res.pc]
	var codes [maxXTgt + 1]uint8
	m := s.divTargets(k, e, uint8(tgtSlotOf(in.Obj)), &codes)
	x := s.xtOf(e)
	for i := 0; i < m; i++ {
		if codes[i] == c {
			x.div[i]++
		}
	}
	if s.g.pickLeft--; s.g.pickLeft > 0 {
		return
	}
	var amts [maxXTgt + 1]int
	n := 0
	for i := 0; i < m; i++ {
		if x.div[i] > 0 {
			codes[n], amts[n] = codes[i], int(x.div[i])
			n++
		}
	}
	s.damageBatch(k, codes[:n], amts[:n])
	s.continueRes(k, 0)
}

// xtLegal is extra target j of e still legal at resolution; a shared creature
// type slot (tgt_setconstraints.go) also drops the ones that no longer share.
func (s *State) xtLegal(k *prep, e *stackEntry, j int) bool {
	return s.xtLegalBase(k, e, j) && !(k.gen != nil && k.gen.anyTgtSetBf && tspecE(k, e, int(s.xtOf(e).slot[j])).TFlags&kop.TgfSharedCrType != 0 && !s.xtSetNarrowed(k, e, j))
}

func (s *State) xtLegalBase(k *prep, e *stackEntry, j int) bool {
	c := s.xtOf(e).code[j]
	if c == NoTarget {
		return false
	}
	if c >= PlayerTarget0 {
		return !s.pl[c-PlayerTarget0].lost
	}
	if s.objs[c].ver != s.xtOf(e).ver[j] {
		return false
	}
	t := tspecE(k, e, int(s.xtOf(e).slot[j]))
	if t == nil {
		return false
	}
	src := NoSlot
	if e.kind != seSpell {
		ob := &s.objs[e.obj]
		if ob.zone == kop.ZoneBattlefield && ob.ver == e.srcVer {
			src = ob.slot
		}
	}
	return s.targetOK(k, t, e.ctrl, src, e.obj, c)
}

// execEachTarget runs in once per object of target slot i.
func (s *State) execEachTarget(k *prep, in *kop.Instr, i uint8) {
	e := &s.res.e
	saveT, saveV, saveBad := e.tgt[i], e.tver[i], s.res.bad
	if k.gen.anyBatch {
		defer s.batchClose(s.batchOpen())
	}
	var hr holdRec
	if in.Op == kop.OpReturn && kop.Zone(in.C) == kop.ZoneBattlefield || in.Op == kop.OpReturnEx && kop.Zone(in.C&7) == kop.ZoneBattlefield {
		hr = s.holdEntries() // gorge's zone batch: the targets enter together (CR 603.6a)
	} else {
		hr = s.holdOpen(k)
	}
	defer s.holdClose(k, hr)
	if s.leaveTogether(k, in, i) {
		return
	}
	s.g.inLoop = true
	s.exec(k, in)
	for j := 0; j < int(s.xtOf(e).n); j++ {
		if s.xtOf(e).slot[j] != i || !s.xtLegalRes(k, e, j, i) {
			continue
		}
		e.tgt[i], e.tver[i] = s.xtOf(e).code[j], s.xtOf(e).ver[j]
		s.res.bad &^= 1 << i
		s.exec(k, in)
	}
	e.tgt[i], e.tver[i], s.res.bad = saveT, saveV, saveBad
	s.g.inLoop = false
}

// eachPlayersSac handles an OpSacrifice whose target slot holds several
// player targets (Defined$ Targeted over "any number of target players"):
// gorge asks the first player to choose the permanent(s) and only then the
// next, the earlier choices already in the graveyard (CR 608.2c, 701.21a),
// so the ask runs through the repNext chain startSac already walks. Running
// it as execEachTarget would drop the suspend and resolve the rest of the
// program before the first answer. done=false leaves an object-target slot
// on the execEachTarget path.
func (s *State) eachPlayersSac(k *prep, in *kop.Instr, slot uint8) (ok, done bool) {
	var ps [2]uint8
	np, over := s.slotPlayers(k, slot, &ps)
	if over {
		s.capFull(StallSac) // more seats than this state models
		return true, true
	}
	if np == 0 {
		return false, false
	}
	s.g.repNext = 255
	if np == 2 {
		s.g.repNext = ps[1]
	}
	return s.startSac(k, in, ps[0]), true
}

// eachPlayersDiscard is eachPlayersSac's discard: the first targeted player
// answers, and the next is asked with those picks still in hand.
func (s *State) eachPlayersDiscard(k *prep, in *kop.Instr, slot uint8) (ok, done bool) {
	var ps [2]uint8
	np, over := s.slotPlayers(k, slot, &ps)
	if np == 0 || over {
		return false, false
	}
	next := uint8(255)
	if np == 2 {
		next = ps[1]
	}
	return s.discardChain(k, in, ps[0], next), true
}

// slotPlayers lists the seats target slot holds as player targets, in pick order;
// the same seat twice is one. over: more seats than ps holds.
func (s *State) slotPlayers(k *prep, slot uint8, ps *[2]uint8) (np int, over bool) {
	var codes [maxXTgt + 1]uint8
	m := s.divTargets(k, &s.res.e, slot, &codes)
	for i := 0; i < m; i++ {
		c := codes[i]
		if !isPlayerTarget(c) {
			continue
		}
		p := c - PlayerTarget0
		dup := false
		for j := 0; j < np; j++ {
			if ps[j] == p {
				dup = true
			}
		}
		if dup {
			continue
		}
		if np == len(ps) {
			return np, true
		}
		ps[np] = p
		np++
	}
	return np, false
}

// eachPlayers expands an each-player ref into APNAP order (255 = none).
func (s *State) eachPlayers(k *prep, r kop.Ref) (uint8, uint8) {
	switch r {
	case kop.RefEachPlayer:
		// gorge's Defined$ Player / "each player" set is AliveFrom(controller):
		// the resolving controller first, then the other seat. (RefEachOpponent
		// is the controller's opponent; DamageAll's "each player" order is
		// irrelevant to its simultaneous damage.)
		return s.res.e.ctrl, 1 - s.res.e.ctrl
	case kop.RefEachOpponent:
		return 1 - s.res.e.ctrl, 255
	case kop.RefOppNonRemCtrl:
		return s.oppNonRemCtrl(), 255 // filter_defined.go
	case kop.RefSrcOwner:
		return s.srcOwner(), 255
	}
	return s.playerRef(k, r), 255
}

// loopBackGen jumps back to an OpRepeatEachPlayer body for its next player.
func (s *State) loopBackGen() {
	if s.g.repNext != 255 && s.res.pc == s.g.repEnd && s.g.repEnd != 0 {
		s.g.eachPl = s.g.repNext
		s.g.repNext = 255
		s.res.pc = s.g.repPC
	}
}

func (s *State) remember(o Obj) {
	if !s.inSet(&s.g.rem, s.g.nrem, o) {
		if s.g.nrem >= maxRem {
			s.capFull(StallRemember)
			return
		}
		s.g.rem[s.g.nrem] = o
		s.g.nrem++
	}
}

// moveObj moves object o (named by ref r) to zone z of its owner; it
// reports whether it moved. commit: the resolving spell exiling itself
// moves now instead of at finishResolve, because a later instruction of
// the same resolution reads it from exile (RememberChanged$ then
// Defined$ Remembered: Esper Origins' return, CR 702.34a, 406.2).
func (s *State) moveObj(k *prep, r kop.Ref, o Obj, z kop.Zone, commit bool) bool {
	ob := &s.objs[o]
	switch ob.zone {
	case kop.ZoneStack:
		if commit && z == kop.ZoneExile && o == s.res.e.obj && s.res.e.kind == seSpell {
			// finishResolve's stack exit, now; sfFlashback stays unset (the
			// cast-from-graveyard provenance is beginCast's, cast.go)
			if k.gen != nil && k.gen.anyFaces {
				z = s.stackExitFace(k, o, z, true) // face_castface.go
			}
			from := s.detach(o)
			if z == kop.ZoneLibrary {
				s.omenToLibrary(o) // face_omen.go
			} else {
				s.placeFromK(k, o, z, false, from, false)
			}
			return true
		}
		if z == kop.ZoneExile && o == s.res.e.obj && s.res.e.kind == seSpell {
			// the resolving spell exiles itself ("Exile CARDNAME":
			// Restorative Burst, Retrieve, Eldritch Evolution; gorge
			// ChangeZone Origin$ Stack): it rests in exile instead of
			// the graveyard, as OpEndTurn's exile does
			s.res.e.flags |= sfFlashback
		}
		return false
	case kop.ZoneNone:
		return false
	case kop.ZoneBattlefield:
		if r == kop.RefSelf && ob.ver != s.res.e.srcVer && s.res.e.kind != seSpell {
			return false
		}
		if z == kop.ZoneGraveyard {
			s.leaveParked(k, ob.slot)
		} else {
			s.leave(k, ob.slot, z, false)
		}
	default:
		if s.tokenLeftBattlefield(o) {
			return false // CR 111.8 (zone_tokenleft.go)
		}
		from := s.detach(o)
		s.placeFromK(k, o, z, false, from, false)
	}
	return true
}

func (s *State) execDamageEx(k *prep, in *kop.Instr) {
	if in.C&kop.DmgTrackExcess != 0 {
		s.g.excess = 0 // an unbound read is 0, never a stale record
	}
	code := s.refCode(k, in.Obj)
	if code == NoTarget {
		return
	}
	n := s.value(k, in.A)
	if in.C&kop.DmgMapped != 0 {
		if s.g.ndmg < maxDmgMap && n > 0 {
			s.g.dmg[s.g.ndmg] = struct{ code, amt uint8 }{code, uint8(n)}
			s.g.ndmg++
		} else if n > 0 {
			s.capFull(StallDmgMap)
		}
		return
	}
	if isObjTarget(code) && s.objs[code].zone == kop.ZoneBattlefield {
		sl := s.objs[code].slot
		lethal := 0
		if in.C&kop.DmgTrackExcess != 0 {
			lethal = s.excessLethalGen(k, sl)
			if lethal < 0 || DebugExcessNoLethal {
				lethal = 0
			}
		}
		if in.C&kop.DmgExileIfDiesThisTurn != 0 && in.C&kop.DmgExileIfDealt == 0 {
			s.markExileEx(k, sl)
		}
		dealt := s.damageTo(k, code, n)
		if in.C&kop.DmgTrackExcess != 0 {
			// CR 120.10: the excess of the damage dealt, after prevention and
			// replacement (Tomik's +1), over what was lethal before it landed
			s.g.excess = int16(max(dealt-lethal, 0))
		}
		if dealt > 0 {
			s.damageDealt(k, in, Obj(code)) // cast_mayplaylimit.go
		}
		return
	}
	s.damageTo(k, code, n)
}

// markExileEx records a turn-scoped "exile instead of dying" effect on a slot.
// Each is its own CR 616.1 replacement in gorge, asked about at the death
// (exileOrderAsk), so each keeps the object that made it. A full table stalls.
func (s *State) markExileEx(k *prep, sl Slot) {
	s.mSlSet(k, mechExileEx, sl)
	for i := range s.cold.exMark {
		if r := &s.cold.exMark[i]; !r.live {
			*r = exMarkRec{sl: sl, src: s.res.e.obj, live: true}
			return
		}
	}
	if len(s.cold.exMark) != 0 {
		s.capFull(StallReplLeave)
	}
}

// dieZone is where a permanent that goes to the graveyard goes (exile when an effect said so
// this turn); two or more competing replacements stall (StallReplLeave) unless
// a parked move's ask has settled them (repl_exilemark.go).
func (s *State) dieZone(k *prep, sl Slot) kop.Zone {
	if k.gen != nil && k.gen.anyFinality && s.perms[sl].ctrN(s, int(kop.CtrFinality)) > 0 {
		// CR 122.1h: a permanent with a finality counter is exiled instead,
		// a game rule gorge applies at the move boundary before the
		// replacements (finalityReplacementApplies)
		return kop.ZoneExile
	}
	exileN := 0
	if k.gen != nil && k.gen.anyRepl&rbExileOnLeave != 0 {
		exileN = s.exileOnLeaveN(k, sl)
	}
	// a mark lasts whatever the object is by then: a planeswalker with no
	// loyalty dies too (CR 700.4; gorge registerReplaceDying matches any object)
	exileEx := s.mSl(k, mechExileEx, sl)
	graveLib := k.gen != nil && k.gen.anyRepl&rbGraveLib != 0 && !s.genLost(k, sl) &&
		s.graveRedirect(k, s.perms[sl].card, kop.ZoneGraveyard) == kop.ZoneLibrary
	// Each matching replacement is a separate CR 616.1 effect. Gorge asks
	// the affected player to order them when two or more apply, even when
	// they agree on the destination.
	n := exileN
	if exileEx {
		n += s.exMarkCount(sl)
	}
	if graveLib {
		n++
	}
	if len(s.cold.parked) != 0 && s.takeAnswered(s.perms[sl].obj) {
		return kop.ZoneExile // settled by the ask, even with a source gone since
	}
	if n > 1 {
		s.capFull(StallReplLeave)
		if exileN > 0 || exileEx {
			return kop.ZoneExile
		}
		return s.graveRedirect(k, s.perms[sl].card, kop.ZoneGraveyard)
	}
	if exileN > 0 || exileEx {
		return kop.ZoneExile
	}
	return kop.ZoneGraveyard
}

// auraEntry is the CR 303.4f/g check of card o entering the battlefield
// under ctrl other than by resolving: for an Aura, the one permanent it
// can enchant (host), an ask posed for two or more (ask; raGenEnchant), or
// none, where it stays in its zone (stay).
func (s *State) auraEntry(k *prep, o Obj, ctrl uint8) (host Slot, ask, stay bool) {
	c := s.objs[o].card
	if k.cards[c].types&kop.TAura == 0 || k.t.Cards[c].Enchant.Kind != kop.TgtObject {
		return NoSlot, false, false
	}
	var buf [MaxOptions]uint8
	switch s.enchantCands(k, c, ctrl, &buf) {
	case 0:
		return NoSlot, false, true
	case 1:
		return s.objs[buf[0]].slot, false, false
	}
	s.res.ask, s.res.askP = raGenEnchant, ctrl
	return NoSlot, true, false
}

// returnAura is the core's OpReturn of an Aura card to the battlefield under its
// owner's control, as execReturnSimple enters any plain return (auraEntry: the
// owner is who chooses what it enchants, CR 303.4f); host != NoSlot is the
// answered enchant ask. It reports whether the ask is posed.
func (s *State) returnAura(k *prep, o Obj, host Slot) bool {
	ctrl := s.objs[o].owner()
	if host == NoSlot {
		var ask, stay bool
		if host, ask, stay = s.auraEntry(k, o, ctrl); ask || stay {
			return ask
		}
	}
	from := s.detach(o)
	if k.gen != nil && k.gen.anyMayLimit {
		s.countMayPlayLand(k, o, from)
	}
	var sl Slot
	s.enterOnto(host)
	if from == kop.ZoneGraveyard {
		sl = s.enterFromGrave(k, o, ctrl, false)
	} else {
		sl = s.enter(k, o, ctrl, false, from)
	}
	s.attachedTo(k, sl, host)
	return false
}

// execReturn moves the instruction's object to its zone. An Aura card put
// onto the battlefield enchants what its controller chooses as it enters
// (CR 303.4f): with two or more legal permanents it waits in its zone for
// the "enchant" ask and true is returned (applyResolveGen answers with
// host, its chosen permanent); with none it stays where it is (CR
// 303.4g).
func (s *State) execReturn(k *prep, in *kop.Instr, host Slot) bool {
	return s.execReturnObj(k, in, host, s.returningObj(in))
}

// execReturnObj is execReturn for object o (named by in.Obj).
func (s *State) execReturnObj(k *prep, in *kop.Instr, host Slot, o Obj) bool {
	if in.C>>3&kop.MvAlsoSelf != 0 {
		return s.returnWithSelf(k, in, host)
	}
	s.g.retMiss = true
	e := &s.res.e
	if o == NoObj {
		return false
	}
	ob := &s.objs[o]
	if ob.zone == kop.ZoneBattlefield || ob.zone == kop.ZoneStack || ob.zone == kop.ZoneNone || s.tokenLeftBattlefield(o) || s.instantStays(k, o, kop.Zone(in.C)) {
		return false
	}
	if ob.zone == kop.ZoneLibrary || in.B.N&kop.RetFromLibrary != 0 {
		if in.B.N&kop.RetFromLibrary == 0 || ob.zone != kop.ZoneLibrary {
			return false // Origin$ Library gates the move to a library card; every other return skips one
		}
	}
	if in.Obj == kop.RefTriggerObj && o == e.obj && e.kind == seTrigger && ob.ver != e.srcVer+1 {
		return false // the card that died has moved on (exiled from the graveyard in response): a new object
	}
	if in.Obj == kop.RefTriggerObj && o != e.obj && e.kind == seTrigger && s.trigObjMoved(e, ob) {
		// another card that died (Valkyrie's Call's "that card", gorge
		// Origin$ Graveyard) that has changed zones since is a new object
		// (CR 400.7, gorge; pairs 69834 30137 35683 68528)
		return false
	}
	if in.Obj == kop.RefSelf && e.kind == seAbility && o == e.obj && ob.ver != e.srcVer && s.activatedFrom(k, e) == kop.ZoneGraveyard {
		// the card that comes back to the graveyard after the activation
		// (returned by an earlier activation, then died) is a new object:
		// "return this card" finds nothing (CR 400.7; gorge's
		// incarnation stamp, the 13 slice pairs 88 134 ...)
		return false
	}
	if in.Obj == kop.RefSelf && e.kind == seAbility && o == e.obj && ob.zone == kop.ZoneExile && s.activatedFrom(k, e) == kop.ZoneGraveyard {
		// "return this card from your graveyard": a source exiled in
		// response (Abyssal Harvester) stays exiled (pairs 3630 7853). gorge
		// checks the zone only, so a Skeleton that returned and died again
		// meanwhile is still returned (CR 400.7 says not)
		return false
	}
	z := kop.Zone(in.C & 7)
	fl := in.C >> 3
	ctrl := ob.owner()
	if fl&kop.MvUnderYourControl != 0 {
		ctrl = e.ctrl
	}
	if in.B.N&kop.RetUnderOpponent != 0 {
		ctrl = 1 - e.ctrl // GainControl$ ChosenPlayer: the one opponent (kop.RetUnderOpponent)
	}
	if z == kop.ZoneBattlefield && fl&kop.MvTransformed != 0 {
		s.enterTransformed(k, o)
	}
	if z == kop.ZoneBattlefield && host == NoSlot && in.B.N&kop.RetAttachTo != 0 {
		var stay bool
		if host, stay = s.czAttachHost(k, in, o, ctrl); stay { // AttachedTo$ (zone_originalternative.go)
			return s.res.ask == raGenCzBearer // the filter bearer's ask (tgt_attachedto.go)
		}
	}
	if z == kop.ZoneBattlefield && host == NoSlot {
		var ask, stay bool
		if host, ask, stay = s.auraEntry(k, o, ctrl); ask || stay {
			return ask
		}
	}
	if z == kop.ZoneBattlefield && s.cloneReturnPark(k, o, ctrl) {
		return true // the clone election is asked with the card still in its zone (copy_cloneenter.go)
	}
	if z == kop.ZoneBattlefield && host == NoSlot && fl&kop.MvTransformed == 0 && s.entryParkOp(k, in, o, ctrl) {
		return true // the CR 616.1 entry-order ask, the card still in its zone (repl_entryorder.go)
	}
	from := s.detach(o)
	if fl&kop.MvRemember != 0 {
		s.remember(o) // RememberChanged$ (gorge eventRemember)
	}
	if fl&kop.MvForgetChanged != 0 {
		s.forgetChanged(o) // zone_forget.go
	}
	if from == kop.ZoneLibrary && in.B.N&kop.RetShuffleOwed != 0 {
		s.shuffleLib(s.objs[o].owner()) // a Defined$ library fetch shuffles only when the script owes one (gorge moveDefinedLibraryObjects)
	}
	if z != kop.ZoneBattlefield {
		s.placeFromK(k, o, z, false, from, false)
		return false
	}
	if k.gen != nil && k.gen.anyMayLimit {
		s.countMayPlayLand(k, o, from)
	}
	if n := s.value(k, in.A); n > 0 {
		s.g.enterCtr.n, s.g.enterCtr.kind = uint8(n), kop.CounterKind(in.B.N)
	}
	var sl Slot
	switch {
	case s.enterWithOverride(k, in):
		sl = s.enterAssimilated(k, o, ctrl, from)
	case s.g.retAt != 0:
		sl = s.enterNF(k, o, ctrl, fl&kop.MvTapped != 0, from, 0)
		if from == kop.ZoneGraveyard {
			s.fireLeavesGraveyardSelf(k, sl, o)
		}
		s.g.held |= 1 << (s.perms[sl].ts & 7)
	default:
		s.enterOnto(host)
		sl = s.enterMoved(k, o, ctrl, fl&kop.MvTapped != 0, from)
	}
	s.g.enterCtr.n = 0
	s.g.retMiss = false
	if fl&kop.MvAttacking != 0 {
		s.enterAttacking(k, sl, ctrl)
	}
	if host != NoSlot && s.occ.has(host) {
		s.attachedTo(k, sl, host)
	} else if host == NoSlot && in.B.N&kop.RetAttachTo != 0 && in.F != 0 {
		return s.czBearerAfter(k, in, o) // an Equipment's filter bearer is asked after it enters (tgt_attachedto.go)
	}
	return false
}

// execChangeZoneAll moves the instruction's cards; false when it suspends at a card's
// as-enters ask, with that card and the later ones still in their zones and the batch and
// its held enters checks open (gorge's zone batch). resume runs it again from the answer:
// the cards already moved have left what it walks.
func (s *State) execChangeZoneAll(k *prep, in *kop.Instr, resume bool) (done bool) {
	e := &s.res.e
	fl := k.t.Filters[in.F]
	if (fl.Zone == kop.ZoneGraveyard || in.Op == kop.OpChangeZoneAllEx && in.C>>3&kop.MvGraveExile != 0) && k.gen.anyBatch {
		old := s.batchOpen() // one graveyard-leave batch
		if resume {
			old &^= batchOpenMask
		}
		defer func() {
			if done {
				s.batchClose(old)
			}
		}()
	}
	z := kop.Zone(in.C & 7)
	mv := int16(0)
	if in.Op == kop.OpChangeZoneAllEx {
		mv = in.C >> 3
	}
	if mv&kop.MvForgetOther != 0 || z == kop.ZoneExile && in.B.N&kop.CzUntilLeaves != 0 {
		s.czall3(k, in, mv, z) // zone_changezoneall.go
		return true
	}
	only := uint8(255)
	if in.Obj != kop.RefNone {
		only = s.playerRef(k, in.Obj)
		if only == 255 {
			return true
		}
	}
	src := s.objRef(k, kop.RefSelf)
	if (fl.Zone == kop.ZoneNone || fl.Zone == kop.ZoneBattlefield) && mv&kop.MvGraveExile == 0 {
		var hit slotMask
		for m := s.occ; m.any(); m = m.rest() {
			sl := m.first()
			if (only == 255 || s.perms[sl].ctrl == only) && s.matchPermGen(k, in.F, sl, e.ctrl, src, layerFull) {
				hit = hit.with(sl)
			}
		}
		// one simultaneous departure (CR 603.10a), seat 0's battlefield
		// list then seat 1's (gorge effChangeZoneAll: AliveFrom(0) inside
		// one departure batch)
		if mv&kop.MvRemember != 0 {
			s.rememberLeaveAll(k, hit, z) // zone_rememberparams.go
			return true
		}
		s.leaveAll(k, hit, z)
		return true
	}
	// cards in a zone of each player, seat 0 first (gorge
	// changeZoneAllPlayers: AliveFrom(0), not APNAP); a sweep onto the
	// battlefield is one entering event (CR 603.6a, gorge): the
	// entrants' enters checks are held to the end (entBatch). A dual
	// Graveyard,Exile origin is zone-major -- every player's Graveyard, then
	// every player's Exile (gorge effChangeZoneAll's `for _, z := range from`
	// outer loop) -- not one player's two zones.
	var eb entBatch
	stamp := s.ts
	var rid sweepRiders // MvEntryRiders (zone_sweepriders.go)
	if mv&kop.MvEntryRiders != 0 && z == kop.ZoneBattlefield {
		rid = s.sweepRidersOf(k, in)
	}
	nz, zbase := 1, fl.Zone
	if mv&kop.MvGraveExile != 0 {
		nz, zbase = 2, kop.ZoneGraveyard // graveexile: Graveyard then Exile
		if DebugIgnoreGraveExile {
			nz = 1
		}
	}
	for zi := 0; zi < nz; zi++ {
		zsrc := zbase
		if zi == 1 {
			zsrc = kop.ZoneExile
		}
		for p := uint8(0); p < 2; p++ {
			if only != 255 && p != only {
				continue
			}
			var list [MaxDeck]Obj
			var listFrom [MaxDeck]kop.Zone
			n := 0
			pl := &s.pl[p]
			switch zsrc {
			case kop.ZoneGraveyard:
				for i := 0; i < int(pl.graveN); i++ {
					list[n] = pl.grave[i]
					n++
				}
			case kop.ZoneExile:
				for i := 0; i < int(pl.exileN); i++ {
					list[n] = pl.exile[i]
					n++
				}
			case kop.ZoneHand:
				for i := 0; i < int(pl.handN); i++ {
					list[n] = pl.hand[i]
					n++
				}
			case kop.ZoneLibrary:
				for i := int(pl.libLo); i < int(pl.libHi); i++ {
					list[n] = pl.lib[i]
					n++
				}
			}
			nl := 0 // library-bound cards, compacted into list[:nl]
			for i := 0; i < n; i++ {
				o := list[i]
				if !s.matchCardGen(k, in.F, s.objs[o].card, p, e.ctrl, o) || s.tokenLeftBattlefield(o) || s.instantStays(k, o, z) {
					continue
				}
				if z == kop.ZoneBattlefield && s.occ.full() {
					// a mass return onto a full battlefield: the kernel's
					// capacity, a draw like the token makers' (enter panics)
					s.fireEntries(k, &eb)
					s.stall(StallBoard)
					return true
				}
				ctrl := p
				if mv&kop.MvUnderYourControl != 0 {
					ctrl = e.ctrl
				}
				// CR 303.4f/g, in gorge's move order (Forge's per-card moveTo): an
				// Aura card enters attached to the one host it can enchant, waits
				// for its controller's pick among two or more with the cards
				// before it already entered, and stays where it is with none
				host := NoSlot
				if z == kop.ZoneBattlefield && k.gen != nil {
					if s.digAuraStays(k, o, ctrl) {
						continue
					}
					if host = s.digAuraHost(k, o, ctrl); host == NoSlot && s.digAuraPark(k, o, ctrl) {
						if !resume {
							s.g.held, s.g.retTS = 0, stamp
						}
						s.holdBatch(&eb)
						return false
					}
				}
				if z == kop.ZoneBattlefield && s.cloneReturnPark(k, o, ctrl) {
					if !resume {
						s.g.held, s.g.retTS = 0, stamp
					}
					s.holdBatch(&eb)
					return false
				}
				from := s.detach(o)
				if mv&kop.MvRemember != 0 {
					s.remember(o) // RememberChanged$, in move order (gorge emitMove)
				}
				if z == kop.ZoneBattlefield && k.gen != nil && k.gen.anyMayLimit {
					s.countMayPlayLand(k, o, from)
				}
				if z == kop.ZoneBattlefield {
					if mv&kop.MvEntryRiders != 0 {
						s.armEntryCounters(&rid)
					}
					sl := s.enterHeld(k, &eb, o, ctrl, mv&kop.MvTapped != 0, from)
					if s.g.pre.obj == o {
						s.g.pre = preAsk{}
					}
					if host != NoSlot && s.occ.has(host) {
						s.attachTo(k, sl, host)
					}
					if mv&kop.MvEntryRiders != 0 {
						s.g.enterCtr.n = 0
						s.entryOverride(k, &rid, sl)
						s.entryAtEOT(&rid, o)
					}
					if from == kop.ZoneGraveyard {
						s.fireLeavesGraveyardSelf(k, sl, o)
					}
				} else if z == kop.ZoneLibrary {
					list[nl] = o // placed below, in gorge's order
					listFrom[nl] = from
					nl++
				} else {
					s.placeFromK(k, o, z, false, from, false)
				}
			}
			if nl > 0 {
				s.libTopInOrder(list[:nl], listFrom[:nl])
			}
		}
	}
	if resume {
		s.holdBatch(&eb)
		s.fireHeld(k, s.g.retTS)
		return true
	}
	s.fireEntries(k, &eb)
	return true
}

// execDigUntil reveals until in.A cards match and moves them and the rest; false when
// it suspends at a found card's as-enters ask. resume re-runs it from that ask: the
// walk stopped at s.g.pickP with s.g.pickLeft found cards still to enter over s.g.nlook
// revealed ones. A card found for the battlefield enters while it and the revealed
// cards are still in the library, each ask answered in turn, and the rest move after
// the last answer (gorge effDigUntil's emit order, Forge DigUntilEffect).
func (s *State) execDigUntil(k *prep, in *kop.Instr, resume bool) bool {
	first, second := s.eachPlayers(k, in.Obj)
	for _, p := range [2]uint8{first, second} {
		if p == 255 || resume && p != s.g.pickP {
			continue
		}
		resumed := resume
		resume = false
		pl := &s.pl[p]
		want := s.value(k, in.A)
		scan := MaxDeck
		if resumed {
			want = int(s.g.pickLeft)
			scan = want + int(s.g.nlook)
		}
		found := 0
		var rev [MaxDeck]Obj
		nr := 0
		var fnd [MaxDeck]Obj
		nf := 0
		for i := 0; (found < want || resumed) && i < scan && int(pl.libLo)+i < int(pl.libHi); i++ {
			o := pl.lib[int(pl.libLo)+i]
			if found < want && s.matchCardGen(k, in.F, s.objs[o].card, p, s.res.e.ctrl, NoObj) && nf < len(fnd) {
				fnd[nf] = o
				nf++
				found++
			} else {
				rev[nr] = o
				nr++
			}
		}
		fl := in.C >> 3
		fz := kop.Zone(in.C & 7)
		rz := kop.Zone(fl & 7)
		entering := fz == kop.ZoneBattlefield && fl&kop.DuFoundWithRevealed == 0
		if !entering {
			s.digPop(pl, nr+nf, rev[:nr], fnd[:nf])
		}
		if !resumed {
			if fl&kop.DuRememberFound != 0 {
				s.digUntilRemember(p == first, fnd[:nf])
			}
			if fl&kop.DuDugMark != 0 {
				s.g.digLo = pl.exileN // the dug cards start here (act_discoverbottom.go)
			}
		}
		if fl&kop.DuFoundWithRevealed != 0 {
			for i := 0; i < nf; i++ {
				rev[nr] = fnd[i]
				nr++
			}
			nf = 0
		}
		for i := 0; i < nf; i++ {
			if entering {
				s.g.pickP, s.g.nlook, s.g.pickLeft = p, uint8(nr), uint8(nf-i)
				if s.cloneReturnPark(k, fnd[i], p) {
					return false
				}
				if !s.digEnter(k, p, fnd[i], fl&kop.DuFoundTapped != 0, fl&kop.DuFoundAttacking != 0) {
					return true
				}
				if s.g.pre.obj == fnd[i] {
					s.g.pre = preAsk{}
				}
				s.g.pickLeft--
				if s.parkTail(k, tailDigUntil) {
					return false
				}
				continue
			}
			s.digMove(k, p, fnd[i], fz, fnd[i+1:nf], rev[:nr])
		}
		if entering {
			s.digPop(pl, nr, rev[:nr], nil)
		}
		if fl&kop.DuRandomOrder != 0 {
			// gorge's DigUntil: a forward Fisher-Yates drawing once per
			// position (len(rest) draws, the last one Rand(1))
			for i := 0; i < nr; i++ {
				j := i + s.rng.IntN(nr-i)
				rev[i], rev[j] = rev[j], rev[i]
			}
		}
		for i := 0; i < nr; i++ {
			o := rev[i]
			if rz == kop.ZoneLibrary {
				if s.objs[o].zone == kop.ZoneLibrary {
					pl.libRemove(o) // put back for a Progenitus shuffle (digMove)
				}
				s.objs[o].zone = kop.ZoneLibrary
				s.objs[o].ver++
				pl.libBottom(o)
			} else {
				s.digMove(k, p, o, rz, rev[i+1:nr], nil)
			}
		}
	}
	return true
}

// digPop takes the n cards off the top of pl's library into no zone: a and b.
func (s *State) digPop(pl *player, n int, a, b []Obj) {
	for _, o := range a {
		s.touch(o)
		s.objs[o].zone = kop.ZoneNone
	}
	for _, o := range b {
		s.touch(o)
		s.objs[o].zone = kop.ZoneNone
	}
	pl.libLo += uint8(n)
}

// execCopyToken creates the instruction's token copies. A copy of an Aura
// enchants what its controller chooses as it enters (CR 303.4f): with more
// than one legal permanent the token waits outside every zone (gorge's
// minted token, zone 0) for the "enchant" ask, and true is returned; with
// none it is not created (CR 303.4g). The tokens still owed wait in
// s.g.pickLeft.
func (s *State) execCopyToken(k *prep, in *kop.Instr) bool {
	e := &s.res.e
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return false
	}
	c := s.objs[o].card
	if s.objs[o].zone == kop.ZoneBattlefield {
		c = s.cardAt(s.objs[o].slot)
	}
	v := s.value(k, in.A)
	if v <= 0 && in.B.N&(kop.CpSacAtEndDly|kop.CpExileAtEnd) != 0 {
		// X = 0 makes no copy, but the AtEOT$ rider's delayed trigger is
		// created all the same (CR 603.7): it goes on the stack at the end
		// step and does nothing (gorge scheduleAtEOTNone)
		s.delayEmpty(e)
	}
	return s.copyTokens(k, in, c, s.replTokens(k, e.ctrl, c, v))
}

// copyTokens creates n token copies of card c for the resolving entry's
// controller (execCopyToken).
func (s *State) copyTokens(k *prep, in *kop.Instr, c CardID, n int) bool {
	e := &s.res.e
	if src := s.objRefObj(in.Obj); src != NoObj && s.faceDownAt(k, src) {
		for i := 0; i < n; i++ {
			if s.nearCapacity() {
				s.stall(StallBoard)
				return false
			}
			t := s.newToken(c, e.ctrl)
			s.objs[t].of |= ofCopy
			s.copyTokenEnter(k, in, t, NoSlot, nil)
		}
		return false
	}
	aura := k.cards[c].types&kop.TAura != 0 && k.t.Cards[c].Enchant.Kind == kop.TgtObject
	// one resolution's copies enter together (gorge effCopyPermanent's zone
	// batch, CR 603.6a): each one's "another enters" sees the others
	var eb entBatch
	defer s.fireEntries(k, &eb)
	for i := 0; i < n; i++ {
		if s.nearCapacity() {
			s.stall(StallBoard)
			return false
		}
		host := NoSlot
		if aura {
			var buf [MaxOptions]uint8
			switch s.enchantCands(k, c, e.ctrl, &buf) {
			case 0:
				continue // CR 303.4g: the Aura token isn't created
			case 1:
				host = s.objs[buf[0]].slot
			default:
				t := s.newToken(c, e.ctrl)
				s.objs[t].of |= ofCopy | ofPending
				s.res.lastTok = t
				s.g.pickLeft = uint8(n - i - 1)
				s.res.ask, s.res.askP = raGenEnchant, e.ctrl
				return true
			}
		}
		kind := s.enterChoiceKind(k, c)
		if kind == 0 && k.gen.anyBecomeCopy {
			kind = s.cloneTokenAsk(k, in, c) // ctr_entryperkind.go: a copy of a clone card is asked the clone election
		}
		if kind != 0 {
			// an as-enters choice (Heraldic Banner's colour): gorge's
			// minted token waits outside every zone (zone 0) for it, as a
			// permanent spell does on the stack (preEnterAsk)
			t := s.newToken(c, e.ctrl)
			s.objs[t].of |= ofCopy | ofPending
			s.res.lastTok = t
			s.g.pickLeft = uint8(n - i - 1)
			s.g.pre = preAsk{obj: t, kind: kind}
			s.res.ask, s.res.askP = raGenTokChoice, e.ctrl
			return true
		}
		t := s.newToken(c, e.ctrl)
		s.objs[t].of |= ofCopy
		s.copyTokenEnter(k, in, t, host, &eb)
	}
	return false
}

// copyTokenEnter puts copy token t onto the battlefield, attached to host
// when it is an Aura's.
func (s *State) copyTokenEnter(k *prep, in *kop.Instr, t Obj, host Slot, eb *entBatch) {
	tmpl := NoSlot
	if src := s.objRefObj(in.Obj); src != NoObj && s.objs[src].zone == kop.ZoneBattlefield {
		tmpl = s.objs[src].slot
	}
	s.copyTokenEnterFrom(k, in, t, host, tmpl, eb)
}

// copyTokenEnterFrom is copyTokenEnter with the copied permanent's slot tmpl
// (NoSlot: none on the battlefield) given. eb, when not nil, holds its enters check.
func (s *State) copyTokenEnterFrom(k *prep, in *kop.Instr, t Obj, host, tmpl Slot, eb *entBatch) {
	e := &s.res.e
	fl := in.B.N & 0xff
	face := uint8(0)
	if tmpl != NoSlot && s.faceDown(k, tmpl) {
		face = 1 // CR 707.2: its copiable values are the face-down characteristics
	}
	early := !k.gen.anyBecomeCopy && (in.C != 0 || tmpl != NoSlot && s.fxM.has(tmpl))
	if early {
		s.g.entExc = entExcOn | uint32(tmpl)<<8 | uint32(uint8(in.C))
	}
	sl := s.enterNF(k, t, e.ctrl, false, kop.ZoneLibrary, face)
	if face != 0 {
		s.perms[sl].gty |= s.perms[tmpl].gty & gtyProvMask
	}
	s.touch(t)
	// a token that elected a copy as it entered (ctr_entryperkind.go): the
	// elected template is its basis and replaces the copy's own exceptions
	elected := k.gen.anyBecomeCopy && s.perms[sl].ovr.flags&ovCopy != 0
	if (k.gen.anyCloneCopy || k.gen.anyBecomeCopy) && tmpl != NoSlot && s.perms[tmpl].ovr.flags&ovCopy != 0 {
		if elected {
			s.cloneTokenElected(sl, t, tmpl) // ctr_entryperkind.go: the printed card is the copy's, the election stays the basis
		} else {
			s.cloneTokenBasis(sl, t, tmpl) // a copy of a copy: gorge copyTokenBasis (copy_clone.go)
		}
	}
	// the copy exceptions, the template's first (CR 707.9b: they are
	// copiable values), then the op's own (copy_exceptions.go). The entry's
	// triggers read the token with them: it enters with its exceptions. A
	// table with an as-enters copy election adds them here, once it is known.
	if !early && (in.C != 0 || tmpl != NoSlot && s.fxM.has(tmpl)) && !elected {
		s.copyExcAdd(k, sl, tmpl, uint8(in.C))
	}
	if host != NoSlot && s.occ.has(host) {
		s.attachTo(k, sl, host) // a copy that enters attached is attached when its enters check looks (CR 303.4f, 603.6a)
	}
	if eb != nil && eb.n < len(eb.sl) {
		eb.sl[eb.n], eb.o[eb.n] = sl, t
		eb.n++
	} else if s.anyETB(k) {
		s.fireObj(k, kop.TrigETB, sl, t, e.ctrl, 0)
	}
	if in.B.N>>kop.CopyPumpShift != 0 {
		s.copyPumpEnter(k, sl, in) // PumpKeywords$: a permanent, non-copiable grant
	}
	if fl&(kop.CpSacAtEnd|kop.CpExileAtEnd) != 0 {
		act := kop.OpSacrifice
		// AtEOTTrig$ Sacrifice is the token's own granted trigger: its source
		// stays the token. Only the AtEOT$ rider is sourced on its creator
		// (CR 603.7d-e; gorge copypermanent.go scheduleAtEOTRider).
		creator := NoObj
		if fl&kop.CpExileAtEnd != 0 {
			act = kop.OpExile // AtEOT$ Exile (gorge scheduleAtEOT, __kwWarpExile)
			creator = e.obj
		}
		if s.ndly < MaxDelayed {
			s.dly[s.ndly] = delayed{act: uint8(act), ctrl: e.ctrl, obj: t, ver: s.objs[t].ver, creator: creator}
			s.ndly++
		} else {
			s.capFull(StallDelayed)
		}
	}
	if fl&kop.CpSacAtEndDly != 0 {
		s.sacAtEndUntracked(t, e.ctrl) // trig_leavereturn.go
	}
	if fl&kop.CpRemember != 0 {
		s.remember(t)
	}
	s.res.lastTok = t
	if k.gen.anyTokCr {
		s.fireTokenCreated(k, sl) // a copy token's entry is its creation
	}
}

// enchantCands writes the battlefield permanents an Aura of card c
// entering under p's control may enchant (its enchant ability's filter, not
// targeting: hexproof and shroud do not count, protection does, CR
// 303.4f, 702.16c) into out, and returns their number.
func (s *State) enchantCands(k *prep, c CardID, p uint8, out *[MaxOptions]uint8) int {
	t := &k.t.Cards[c].Enchant
	n := 0
	for m := s.occ; m.any() && n < MaxOptions; m = m.rest() {
		sl := m.first()
		if s.objs[s.perms[sl].obj].of&ofPending != 0 {
			continue
		}
		if s.matchPerm(k, t.Filter, sl, p, NoSlot) && s.kw(k, sl)&kop.KwProtectionAll == 0 &&
			!(k.gen.anyProtCol && s.protPrinted(k, sl)&k.t.Cards[c].Colors != 0) &&
			!(k.gen.anyProtF && s.protQuality(k, sl, NoSlot, c)) {
			out[n] = uint8(s.perms[sl].obj)
			n++
		}
	}
	return n
}

// NoSubtypeIdx is a matchup filter's "absent subtype" sentinel minus one.
const NoSubtypeIdx = 254

// damageBatch deals damage from the resolving entry's source to several
// recipients as one simultaneous event (a Drakuseth damage map, damage
// divided at targeting): lifelink gains the total once, a DamageDealtOnce line
// fires once with the total, and every other damage trigger fires per recipient.
func (s *State) damageBatch(k *prep, codes []uint8, amts []int) {
	kw := s.srcKw(k)
	inf := k.gen.anyInfect && s.srcInfect(k)
	wit := k.gen.anyInfect && s.srcWither(k, kw) // wither's creature half
	total, toPl, plM := 0, false, uint8(0)
	var dn [maxXTgt + 1]int16 // each recipient's dealt amount, for the per-referent pass
	for i, code := range codes {
		n := amts[i]
		if n > 0 && k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbProtAll|rbProtCol|rbPlusDmg|rbShield|rbPrevCol) != 0 {
			n = s.replDamage(k, s.res.e.ctrl, s.objRef(k, kop.RefSelf), s.res.e.card, code, false, n)
		}
		if n <= 0 {
			continue
		}
		if code >= PlayerTarget0 {
			if inf {
				s.addPoison(code-PlayerTarget0, n) // CR 702.90b
			} else {
				s.loseLife(k, code-PlayerTarget0, n)
			}
			toPl = true
			plM |= 1 << (code - PlayerTarget0)
		} else {
			ob := &s.objs[code]
			if ob.zone != kop.ZoneBattlefield {
				continue
			}
			s.touch(Obj(code))
			p := &s.perms[ob.slot]
			if s.typesAt(k, ob.slot)&(kop.TPlaneswalker|kop.TBattle) != 0 && !s.isCreature(k, ob.slot) {
				s.dmgCounters(k, ob.slot, p, n)
			} else if (inf || wit) && s.isCreature(k, ob.slot) {
				s.infectCounters(k, ob.slot, n) // CR 702.90b
			} else {
				p.damage = uint8(min(int(p.damage)+n, 255))
			}
			if kw&kop.KwDeathtouch != 0 {
				p.markDT(1) // a planeswalker too (gorge effects/damage.go, combat.go: any damaged permanent)
			}
			if k.gen.dmgBy != nil {
				s.noteDmgBy(k, s.objRef(k, kop.RefSelf), ob.slot) // DamagedBy marks (filter_combatwords.go)
			}
		}
		total += n
		dn[i] = int16(n)
		s.res.lastDmg = int16(n)
	}
	if total == 0 {
		return
	}
	if kw&kop.KwLifelink != 0 {
		s.gainLife(k, s.res.e.ctrl, total)
	}
	if k.gen.anyPermMark {
		s.markDealt(k, s.objRef(k, kop.RefSelf), false) // filter_permmarks.go
	}
	if s.subs[kop.TrigDamageDone].or(s.subs[kop.TrigDamageDealtOnce]).any() {
		agg := refLatch{agg: true} // the once-per-batch lines take the total, the rest wait for the pass below
		if sl := s.objRef(k, kop.RefSelf); sl != NoSlot {
			s.fireDamageEvGen(k, sl, int16(total), toPl, false, nil, NoTarget, &agg)
		} else if s.res.e.obj != NoObj { // a spell, or an ability whose source is gone (pair 3334)
			s.fireDamageSrc(k, s.res.e.obj, s.res.e.ctrl, int16(total), plM, NoTarget, &agg)
		}
		s.fireDamageRefs(k, codes, dn[:len(codes)]) // one Damage event per recipient
	}
}

// discardMatch reports whether hand card o matches the discard's filter.
// gorge's discardEligible (engine/effects/cardflow.go:442) matches
// DiscardValid$ with matchesInOwnZone (filter.go:3382): a bare Permanent
// base is a permanent card in hand (CR 110.4a), which is
// what kgen's permanent-card type union matches.
func (s *State) discardMatch(k *prep, f kop.FilterID, o Obj) bool {
	if f == 0 {
		return true
	}
	for f != 0 {
		if s.matchCardOne(k, &k.t.Filters[f], s.objs[o].card, s.objs[o].owner(), s.res.e.ctrl, o) {
			return true
		}
		f = k.t.Filters[f].Or
	}
	return false
}

// discardOK: hand card o may be picked by the discard being resolved (its
// filter, when it has one).
func (s *State) discardOK(k *prep, o Obj) bool {
	in := &s.prog(k)[s.res.pc]
	if in.Op != kop.OpDiscard || in.F == 0 {
		return true
	}
	return s.discardMatch(k, in.F, o)
}

// activatedFrom is the zone ability entry e's ability is activated from
// (its ActZone; the battlefield when none is set).
func (s *State) activatedFrom(k *prep, e *stackEntry) kop.Zone {
	cd := &k.t.Cards[e.card]
	if int(e.ab) < len(cd.Abilities) && cd.Abilities[e.ab].ActZone != kop.ZoneNone {
		return cd.Abilities[e.ab].ActZone
	}
	return kop.ZoneBattlefield
}
