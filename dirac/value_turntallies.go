package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// three this-turn records gorge reads from its turn ledger.
//
//   - Food sacrificed: player.sac[sacFood], beside the Permanent and Artifact
//     tallies (noteSac; gorge sacrificedThisTurn matches the spec against the
//     card where it went, a token copy skipped as for the other two).
//   - Combat damage to a player, by source (gorge combatHit, rules/combat.go,
//     rules/host_turn_ledger.go): every landed hit records the player, the
//     source's ObjID and its card/controller at the hit. The kernel keeps
//     the two shapes gorge reads. genState.cdLeg bit p: player p was hit by a
//     source matching Creature.Legendary (Blitzball; gorge matches the hit's
//     snapshot, the printed face unless a layer-4 entry exists for the ID at
//     the read: the kernel reads the source's types at the hit, which is the
//     same unless that entry appears or ends between the hit and the read,
//     GORGE-FINDINGS.md). genState.cdh: for a source whose card reads CtrlHitBySrc
//     against itself (CardDef.CombatHitRec, Steel Hellkite), the players it
//     hit, keyed by Obj (gorge matches by ObjID, so the record survives the
//     source leaving and returning). Both clear at the turn start.
//   - Attackers declared this turn: the object-width arena mask
//     mechAttackedObj (gorge walks AttackersDeclaredThisTurn and matches the
//     live object wherever it is now). Set after the declaration's triggers.

// genState/player tally indexes: player.sac.
const sacFood = 2 // the sacrificed card's printed subtypes include Food (tokens count)

// StallCombatHit: a fifth source that reads CtrlHitBySrc dealt combat
// damage to a player this turn (genState.cdh holds four).
const StallCombatHit = 130

func init() { stallNames[StallCombatHit] = "combathit" }

// DebugTurnTalliesNoReset (TestTurntalliesMutation): the turn start keeps the
// Food tally, the combat-hit records and the attacked-this-turn marks.
var DebugTurnTalliesNoReset bool

// prepTurnTallies reads the table for the three records: a recording card, the
// legendary-hit expression, the Food cards (g.ix gives the attacked mask).
func prepTurnTallies(t *kop.Table, p *prep, g *genPrep) {
	g.anyAttackedObj = g.ix[mechAttackedObj] != arenaNone
	for i := range t.Cards {
		if t.Cards[i].CombatHitRec {
			p.cards[i].cdRec = true
			g.anyCombatHit = true
		}
	}
	food := -1
	for i, n := range t.Subtypes {
		if n == "Food" {
			food = i
			break
		}
	}
	for i := range t.Exprs {
		switch x := &t.Exprs[i]; {
		case x.Kind == kop.ExOppsLostLife && x.N == 1:
			g.anyCombatHit, g.cdLegOn = true, true
		case x.Kind == kop.ExSacrificedThisTurn && x.N == kop.SacrificedFood && food >= 0:
			for c := range t.Cards {
				if t.Cards[c].Subtypes.Has(food) {
					p.cards[c].food = true
				}
			}
		}
	}
}

// turnTalliesUses reports the arena mechanics the table's expressions reach
// (arenaUses).
func turnTalliesUses(t *kop.Table, u *[numMech]bool) {
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExAttackedTurn {
			u[mechAttackedObj] = true
		}
	}
}

// turnTalliesNewTurn ends the combat-hit records and the attacked marks as
// the turn begins.
func (s *State) turnTalliesNewTurn(k *prep) {
	if DebugTurnTalliesNoReset {
		return
	}
	s.g.cdh = [4]struct {
		o  Obj
		pl uint8
	}{}
	s.g.ncdh = 0
	s.g.cdLeg = 0
	s.mObjZero(k, mechAttackedObj)
}

// markAttackedObj marks every declared attacker (finishAttacks, after the
// declaration's triggers; a permanent put onto the battlefield attacking is
// not declared).
//
//go:noinline
func (s *State) markAttackedObj(k *prep) {
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		if sl := m.first(); s.perms[sl].flags&pfAttacking != 0 {
			s.mObjSet(k, mechAttackedObj, s.perms[sl].obj)
		}
	}
}

// attackedTurn is ExAttackedTurn: the marked objects matching x.F where each
// now is, a battlefield permanent through the permanent matcher and one in
// another zone as a card (gorge matchesZoneSpecCtx over the live object).
//
//go:noinline
func (s *State) attackedTurn(k *prep, x *kop.Expr, ctx evalCtx) int {
	if k.gen == nil {
		return 0
	}
	ix := k.gen.ix[mechAttackedObj]
	if int(ix) >= len(s.cold.mo) {
		return 0
	}
	n := 0
	b := &s.cold.mo[ix]
	for w := 0; w < ObjWords; w++ {
		for m := b[w]; m != 0; m &= m - 1 {
			o := Obj(w*64 + bits.TrailingZeros64(m))
			ob := &s.objs[o]
			if ob.zone == kop.ZoneBattlefield {
				if s.matchPermGen(k, x.F, ob.slot, ctx.ctrl, ctx.srcSl, layerFull) {
					n++
				}
			} else if s.matchCardGen(k, x.F, ob.card, ob.owner(), ctx.ctrl, o) {
				n++
			}
		}
	}
	return n
}

// noteCombatHits records the landed combat hits on players of one damage pass
// (combatEventsGen, after prevention: an event with an amount left, to a
// player).
//
//go:noinline
func (s *State) noteCombatHits(k *prep, r *dmgRec) {
	for i := 0; i < r.nev; i++ {
		ev := &r.ev[i]
		if ev.n <= 0 || ev.dst != NoSlot || !s.occ.has(ev.src) {
			continue
		}
		// toDefender gives a player recipient (dst NoSlot) only to an attacker
		// whose target is a player: PlayerTarget0 or PlayerTarget0+1
		pl := CodeSeat(s.perms[ev.src].atk)
		if pl > 1 {
			continue
		}
		if k.gen.cdLegOn {
			if ty := s.typesAt(k, ev.src); ty&kop.TCreature != 0 && ty&kop.TLegendary != 0 {
				s.g.cdLeg |= 1 << pl
			}
		}
		if k.cards[s.cardAt(ev.src)].cdRec {
			s.noteCombatHitSrc(s.perms[ev.src].obj, pl)
		}
	}
}

// noteCombatHitSrc sets player pl's bit in the cdh entry of source o.
func (s *State) noteCombatHitSrc(o Obj, pl uint8) {
	for i := 0; i < int(s.g.ncdh); i++ {
		if s.g.cdh[i].o == o {
			s.g.cdh[i].pl |= 1 << pl
			return
		}
	}
	if int(s.g.ncdh) >= len(s.g.cdh) {
		s.capFull(StallCombatHit)
		return
	}
	s.g.cdh[s.g.ncdh].o, s.g.cdh[s.g.ncdh].pl = o, 1<<pl
	s.g.ncdh++
}

// combatHitBySrc is CtrlHitBySrc for a permanent whose controller is
// ctrl: that player was hit this turn by the resolving entry's source (gorge
// dealtCombatDamageBySource; an ability's source, matched by ObjID). Outside
// a resolution there is no source and nothing matches.
func (s *State) combatHitBySrc(ctrl uint8) bool {
	if !s.res.active {
		return false
	}
	src := s.res.e.obj
	for i := 0; i < int(s.g.ncdh); i++ {
		if s.g.cdh[i].o == src {
			return s.g.cdh[i].pl>>ctrl&1 != 0
		}
	}
	return false
}
