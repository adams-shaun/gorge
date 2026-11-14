package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The mechanic-bit arena. Per-mechanic masks that almost no pool uses live in
// the State's cold arrays, ms for slot width and mo for object width:
// arenaAssign gives each mechanic a table uses the next free index (a pure
// function of the table, filled into genPrep.ix), and every other mechanic
// reads false through mSl and mObj. arenaNone is past the end of any mask
// array, so the bounds test is also the off-switch: an unused mechanic touches
// no mask.

// mech names one arena mechanic. Append-only: appends
// mechExileEx and mechKicked, the object-width ones. New
// entries go before numMech and get a mechWidth row and a detector in
// arenaUses.
type mech uint8

const (
	mechGoad0           mech = iota // goad[0]: slots whose permanent player 0 goaded (CR 701.15; act_goad.go)
	mechGoad1                       // goad[1]: the same for player 1
	mechMonstrous                   // slots whose permanent is monstrous (CR 701.37b; act_monstrosity.go)
	mechRegen                       // bit sl: slot sl's regeneration shields count +4 (repl_regenerate.go)
	mechExileEx                     // slots exiled instead of dying this turn (DmgExileIfDiesThisTurn)
	mechKicked                      // slots whose permanent's spell was kicked (Kicker, Bargain, Offspring)
	mechStash                       // object mask: exiled cards with a stash counter (marks.go)
	mechFBLo                        // object mask: bit 0 of a graveyard card's granted-flashback count
	mechFBHi                        // object mask: bit 1 of that count; assigned right after mechFBLo
	mechSuspected                   // slots whose permanent is suspected (CR 701.60; act_suspect.go)
	mechMayhem                      // object mask: graveyard cards discarded this turn (CR 702.187; kw_mayhem.go)
	mechSneaked                     // slots whose permanent's spell was cast for its sneak cost (CR 702.190a; kw_sneak.go)
	mechAttacked                    // slots whose permanent was declared an attacker this turn (filter_permmarks.go)
	mechDealtTurn                   // slots whose permanent dealt damage this turn
	mechDealtAny                    // slots whose permanent has dealt damage since it entered
	mechDealtCombat                 // slots whose permanent has dealt combat damage since it entered
	mechAttackedObj                 // object mask: objects declared as attackers this turn, wherever they are now (value_turntallies.go)
	mechMustBlocked                 // slots whose permanent must be blocked this turn if able (CR 509.1c; combat_mustbeblocked.go)
	mechMustBlockedPost             // slots marked must-be-blocked while their permanent had lost all abilities: the grant postdates the loss and survives it (CR 613.7)
	mechAttacksIfAble               // slots whose permanent attacks this turn if able (CR 508.1d; combat_attacksifable.go)
	mechBlocksIfAble                // slots whose permanent blocks this turn if able (CR 509.1c; combat_blocksifable.go)
	mechNoRegen                     // slots whose creature can't be regenerated this turn (CR 701.19c; repl_noregen.go)
	mechSBAChain                    // object mask: casualties of this state-based check's passes that asked a replacement order (repl_exilemark.go)
	mechOnce2                       // slots whose permanent's second activate-only-once ability was activated (legal.go onceUsed)
	mechOnce3                       // the same for its third such ability
	mechTappedTurn                  // slots whose permanent has become tapped this turn (Taps FirstTime$; filter_permmarks.go)
	mechEscaped                     // slots whose permanent's spell was cast for its escape cost (CR 702.138a; kw_escape.go)
	mechLure                        // slots whose permanent is a lure this turn: all creatures able to block it do so (CR 509.1c; combat_lure.go)
	mechLurePost                    // slots marked lure while their permanent had lost all abilities: the grant postdates the loss and survives it (CR 613.7)
	mechProtW                       // slots whose permanent has protection from white until end of turn (a chosen-colour grant; static_colourprot.go)
	mechProtU                       // the same, blue
	mechProtB                       // black
	mechProtR                       // red
	mechProtG                       // green
	mechProtPostW                   // slots marked protected from white while their permanent had lost all abilities: the grant postdates the loss and survives it (CR 613.7)
	mechProtPostU                   // the same, blue
	mechProtPostB                   // black
	mechProtPostR                   // red
	mechProtPostG                   // green
	mechEchoOwed                    // slots of the active player's permanents that came under their control since their previous turn began, read as their upkeep's echo gate (CR 702.30a; trig_echo.go)
	mechHexUB                       // slots whose permanent has hexproof from blue and from black this turn (TurnReplHexUB; repl_turnrepl.go)
	mechAtch0                       // slots whose Aura is attached to player seat 0 (the Enchant:Player curses; static_attachtimestamp.go atchSeat)
	mechAtch1                       // the same, seat 1
	mechAtch2                       // seat 2 (read only when MaxSeats > 2)
	mechAtch3                       // seat 3
	mechGoad2                       // goad[2]: slots whose permanent seat 2 goaded (read only when MaxSeats > 2; act_goad.go)
	mechGoad3                       // goad[3]: the same, seat 3
	mechIntimidate                  // slots whose permanent has intimidate until end of turn (ExtKwMark; kw_kwmark.go)
	mechIntimidatePost              // slots marked intimidate while their permanent had lost all abilities: the grant postdates the loss and survives it (CR 613.7)
	mechInfect                      // slots whose permanent has infect until end of turn (ExtKwMark; kw_kwmark.go)
	mechInfectPost                  // slots marked infect while their permanent had lost all abilities (CR 613.7)
	mechRenowned                    // slots whose permanent is renowned (CR 702.112b; act_renown.go)
	mechAttacksPlayer               // slots whose attack duty names the opponent: a player, never their planeswalkers or battles (Encore's "attacks that opponent"; combat_attacksifable.go)
	mechRem                         // object mask: objects whose persistent remembered list is non-empty (gorge's event-backed o.Remembered, the Choose "remembered" fold), the host half of a ConditionDefined$ Remembered group gate (exprs.go CondRemMark)
	numMech
)

// goadMechs names each seat's goad mech in seat order: mechGoad0 and
// mechGoad1 first, mechGoad2 and mechGoad3 appended past mechAtch3, so the
// per-seat goad mech cannot be read by arithmetic off mechGoad0.
var goadMechs = []mech{mechGoad0, mechGoad1, mechGoad2, mechGoad3}

// mech widths: a slot mask indexes the permanent's slot; an object mask
// indexes its Obj.
const (
	wSlot uint8 = iota
	wObj
)

var mechWidth = [numMech]uint8{
	mechGoad0:           wSlot,
	mechGoad1:           wSlot,
	mechMonstrous:       wSlot,
	mechRegen:           wSlot,
	mechExileEx:         wSlot,
	mechKicked:          wSlot,
	mechStash:           wObj,
	mechFBLo:            wObj,
	mechFBHi:            wObj,
	mechSuspected:       wSlot,
	mechMayhem:          wObj,
	mechSneaked:         wSlot,
	mechAttacked:        wSlot,
	mechDealtTurn:       wSlot,
	mechDealtAny:        wSlot,
	mechDealtCombat:     wSlot,
	mechAttackedObj:     wObj,
	mechMustBlocked:     wSlot,
	mechMustBlockedPost: wSlot,
	mechAttacksIfAble:   wSlot,
	mechBlocksIfAble:    wSlot,
	mechAttacksPlayer:   wSlot,
	mechNoRegen:         wSlot,
	mechSBAChain:        wObj,
	mechOnce2:           wSlot,
	mechOnce3:           wSlot,
	mechTappedTurn:      wSlot,
	mechEscaped:         wSlot,
	mechLure:            wSlot,
	mechLurePost:        wSlot,
	mechProtW:           wSlot,
	mechProtU:           wSlot,
	mechProtB:           wSlot,
	mechProtR:           wSlot,
	mechProtG:           wSlot,
	mechProtPostW:       wSlot,
	mechProtPostU:       wSlot,
	mechProtPostB:       wSlot,
	mechProtPostR:       wSlot,
	mechProtPostG:       wSlot,
	mechEchoOwed:        wSlot,
	mechHexUB:           wSlot,
	mechAtch0:           wSlot,
	mechAtch1:           wSlot,
	mechAtch2:           wSlot,
	mechAtch3:           wSlot,
	mechGoad2:           wSlot,
	mechGoad3:           wSlot,
	mechIntimidate:      wSlot,
	mechIntimidatePost:  wSlot,
	mechInfect:          wSlot,
	mechInfectPost:      wSlot,
	mechRenowned:        wSlot,
	mechRem:             wObj,
}

var mechName = [numMech]string{"goad0", "goad1", "monstrous", "regen", "exileEx", "kicked", "stash", "fbLo", "fbHi", "suspected", "mayhem", "sneaked", "attacked", "dealtTurn", "dealtAny", "dealtCombat", "attackedObj", "mustBlocked", "mustBlockedPost", "attacksIfAble", "blocksIfAble", "noRegen", "sbaChain", "once2", "once3", "tappedTurn", "escaped", "lure", "lurePost", "protW", "protU", "protB", "protR", "protG", "protPostW", "protPostU", "protPostB", "protPostR", "protPostG", "echoOwed", "hexUB", "atch0", "atch1", "atch2", "atch3", "goad2", "goad3", "intimidate", "intimidatePost", "infect", "infectPost", "renowned", "attacksPlayer", "rem"}

// arenaNone is the index of a mechanic the table does not use.
const arenaNone = 0xFF

// arenaUses reports which mechanics t's programs and triggers reach, by the
// predicates the prep flags use (prepGoad, prepMonstrosity).
func arenaUses(t *kop.Table) (u [numMech]bool) {
	permMarkUses(t, &u)    // filter_permmarks.go
	turnTalliesUses(t, &u) // value_turntallies.go
	addPhaseUses(t, &u)    // act_addphase.go
	for i := range t.Conds {
		if t.Conds[i].Kind == kop.CondRemMark {
			u[mechRem] = true // a persistent-remembered gate (exprs.go CondRemMark)
		}
	}
	for i := range t.Cards {
		if cd := &t.Cards[i]; cd.Keywords&kop.KwKicker != 0 || cd.HasMultikicker || cd.Bargain || cd.Offspring || cd.OptEvidence != 0 && cd.Types&(kop.TInstant|kop.TSorcery) == 0 {
			u[mechKicked] = true
		}
		if cd := &t.Cards[i]; cd.Types&kop.TAura != 0 && cd.Enchant.Kind == kop.TgtPlayer {
			for sd := 0; sd < MaxSeats; sd++ {
				u[mechAtch0+mech(sd)] = true // a player-attached Aura's seat (static_attachtimestamp.go)
			}
		}
		once := 0
		if t.Cards[i].Mayhem {
			u[mechMayhem] = true
		}
		if t.Cards[i].HasSneak && !u[mechSneaked] {
			u[mechSneaked] = sneakFilterRead(t) // cold scan, once per table (kw_sneak.go)
		}
		if t.Cards[i].Escape && !u[mechEscaped] {
			u[mechEscaped] = escapeFilterRead(t) // cold scan, once per table (kw_escape.go)
		}
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Cost.Flags&kop.CostOncePerGame != 0 {
				once++
			}
			if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplExileOnLeave {
				u[mechSBAChain] = true // a state-based batch can park its casualties (repl_exilemark.go)
			}
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigBecomeMonstrous {
				u[mechMonstrous] = true
			}
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.Flags2&kop.Tf2FirstTime != 0 {
				u[mechTappedTurn] = true
			}
			if a.Kind == kop.AbTriggered && a.Trig.Cond != 0 && t.Conds[a.Trig.Cond].Kind == kop.CondEchoOwed {
				u[mechEchoOwed] = true // an echo trigger's gate (trig_echo.go)
			}
			for _, in := range a.Prog {
				switch in.Op {
				case kop.OpGoad:
					for sd := 0; sd < MaxSeats; sd++ {
						u[goadMechs[sd]] = true // goad[sd]: seat sd may goad
					}
				case kop.OpMonstrosity:
					u[mechMonstrous] = true
				case kop.OpRegenerate:
					u[mechRegen] = true
				case kop.OpDealDamageEx:
					if in.C&kop.DmgExileIfDiesThisTurn != 0 {
						u[mechExileEx] = true
					}
					if in.C&(kop.DmgNoRegen|kop.DmgNoRegenTarget) != 0 {
						u[mechNoRegen] = true // repl_noregen.go
					}
				case kop.OpExileEx:
					if kop.CounterKind(in.B.N) == kop.CtrStash {
						u[mechStash] = true // the only stashSet site (interp_gen.go)
					}
				case kop.OpExt:
					mustBlockUses(&in, &u) // combat_mustbeblocked.go
					colProtUses(&in, &u)   // static_colourprot.go
					kwMarkUses(&in, &u)    // kw_kwmark.go
					if in.Ext() == kop.ExtAttacksIfAble {
						u[mechAttacksIfAble] = true // combat_attacksifable.go
						if in.C&kop.AttacksOpponent != 0 {
							u[mechAttacksPlayer] = true
						}
					}
					if in.Ext() == kop.ExtBlocksIfAble {
						u[mechBlocksIfAble] = true // combat_blocksifable.go
					}
					if in.Ext() == kop.ExtExileExMark || in.Ext() == kop.ExtTurnRepl && in.C == kop.TurnReplExileOpp {
						u[mechExileEx] = true
					}
					if in.Ext() == kop.ExtTurnRepl && in.C == kop.TurnReplHexUB {
						u[mechHexUB] = true // repl_turnrepl.go
					}
					if in.Ext() == kop.ExtSuspect && in.C&1 == 0 { // seat-ok: an instruction flag bit
						u[mechSuspected] = true // a suspect; an un-suspect or a read needs no mask
					}
					if in.Ext() == kop.ExtRenown {
						u[mechRenowned] = true // the only writer; a read alone needs no mask (act_renown.go)
					}
				case kop.OpGrantKeyword:
					// grantFB is the only writer of the flashback planes, so a pool
					// without such a grant reads 0 grants (cast.go's b.fbGrants too)
					if kop.KwMask(uint16(in.B.N))<<16&kop.KwFlashbackOwnCost != 0 {
						u[mechFBLo], u[mechFBHi] = true, true
					}
				}
			}
		}
		if once >= 2 {
			u[mechOnce2] = true
		}
		if once >= 3 {
			u[mechOnce3] = true
		}
	}
	u[mechSBAChain] = u[mechSBAChain] || u[mechExileEx]
	return u
}

// arenaAssign gives each used mechanic the next free arena index of its
// width in enum order (arenaNone for the unused). nms and nmo are the slot and
// object masks in use. mechFBHi directly follows mechFBLo and they are used
// together, so its index is mechFBLo's + 1, which the flashback helpers rely on.
func arenaAssign(t *kop.Table) (ix [numMech]uint8, nms, nmo uint8) {
	u := arenaUses(t)
	for m := mech(0); m < numMech; m++ {
		ix[m] = arenaNone
		switch {
		case !u[m]:
		case mechWidth[m] == wSlot:
			ix[m] = nms
			nms++
		default:
			ix[m] = nmo
			nmo++
		}
	}
	return ix, nms, nmo
}

// prepArena fills g.ix; coldMS and coldMO size the masks from it.
func prepArena(t *kop.Table, g *genPrep) {
	g.ix, _, _ = arenaAssign(t)
}

// coldMS and coldMO are the slot- and object-width masks a table's arena
// assigned.
var (
	coldMS = newCold[slotMask]("ms", func(g *genPrep) int { return arenaCount(g, wSlot) }).view(func(c *coldArrays) *[]slotMask { return &c.ms })
	coldMO = newCold[objBits]("mo", func(g *genPrep) int { return arenaCount(g, wObj) }).view(func(c *coldArrays) *[]objBits { return &c.mo })
)

// arenaCount is how many masks of width w g.ix assigned.
func arenaCount(g *genPrep, w uint8) int {
	n := 0
	for m, i := range g.ix {
		if i != arenaNone && mechWidth[m] == w {
			n++
		}
	}
	return n
}

// mSl reports whether mechanic m's mask holds sl; false when the table does
// not use m (and for a hand-written table, which has no genPrep).
func (s *State) mSl(k *prep, m mech, sl Slot) bool {
	if k.gen == nil {
		return false
	}
	i := k.gen.ix[m]
	return int(i) < len(s.cold.ms) && s.cold.ms[i].has(sl)
}

// mSlSet sets sl in mechanic m's mask (a no-op when the table does not use m).
func (s *State) mSlSet(k *prep, m mech, sl Slot) {
	if k.gen == nil {
		return
	}
	if i := k.gen.ix[m]; int(i) < len(s.cold.ms) {
		s.cold.ms[i].set(sl)
	}
}

// mSlClr clears sl in mechanic m's mask.
func (s *State) mSlClr(k *prep, m mech, sl Slot) {
	if k.gen == nil {
		return
	}
	if i := k.gen.ix[m]; int(i) < len(s.cold.ms) {
		s.cold.ms[i].clr(sl)
	}
}

// mSlZero empties mechanic m's whole mask.
func (s *State) mSlZero(k *prep, m mech) {
	if k.gen == nil {
		return
	}
	if i := k.gen.ix[m]; int(i) < len(s.cold.ms) {
		s.cold.ms[i] = slotMask{}
	}
}

// arenaClrSlot clears slot sl in every arena mask: sl is taking a new
// permanent, so no mechanic's mark for the previous one survives (the one
// loop that replaces the per-mechanic clears at enter).
func (s *State) arenaClrSlot(sl Slot) {
	ms := s.cold.ms
	for i := range ms {
		ms[i] = ms[i].without(sl)
	}
	s.coldClrSlot(sl) // every perSlot cold region's record (statecopy.go)
	for i := range s.cold.exMark {
		if r := &s.cold.exMark[i]; r.sl == sl {
			r.live = false // the last permanent's exile-instead records end with it (repl_exilemark.go)
		}
	}
}

// mSlMask returns mechanic m's mask (empty when the table does not use m).
func (s *State) mSlMask(k *prep, m mech) slotMask {
	if k.gen == nil {
		return slotMask{}
	}
	if i := k.gen.ix[m]; int(i) < len(s.cold.ms) {
		return s.cold.ms[i]
	}
	return slotMask{}
}
