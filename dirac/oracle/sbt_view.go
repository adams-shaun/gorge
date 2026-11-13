package oracle

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// sbt_view.go builds the gorge view.View the sbt agent's seat reads from a
// kernel position: exactly the fields sb-tactical (gorge
// bots/spellbench/builtins), heuristic.BoardFromView and botpolicy read --
// the step, the life totals and zone sizes, every public card with its
// derived power, toughness, keywords, tap/sick/combat state, counters and
// mana production, the deciding seat's own hand and pool, and the stack.
// Card strings come from SBTCards (gorge's own projection of each card).

// sbtObjID is kernel object o's gorge object id in the agent's views: the
// canonical instance plus one (gorge's genesis ids for deck cards; tokens
// keep their creation order).
func sbtObjID(o uint8) state.ObjID { return state.ObjID(o) + 1 }

// sbtKernelObj inverts sbtObjID.
func sbtKernelObj(id state.ObjID) dirac.Obj { return dirac.Obj(id - 1) }

// sbtStackID is the gorge id of an ability on the stack at position i.
func sbtStackID(i int) state.ObjID { return state.ObjID(0x10000 + i) }

var sbtSteps = [...]string{
	kop.StepUntap: "untap", kop.StepUpkeep: "upkeep", kop.StepDraw: "draw", kop.StepMain1: "main1",
	kop.StepBeginCombat: "begin-combat", kop.StepAttackers: "declare-attackers", kop.StepBlockers: "declare-blockers",
	kop.StepFirstStrikeDamage: "combat-damage", kop.StepDamage: "combat-damage", kop.StepEndCombat: "end-combat",
	kop.StepMain2: "main2", kop.StepEnd: "end", kop.StepCleanup: "cleanup",
}

func sbtPhase(s kop.Step) string {
	switch s {
	case kop.StepUntap, kop.StepUpkeep, kop.StepDraw:
		return "beginning"
	case kop.StepMain1:
		return "main1"
	case kop.StepMain2:
		return "main2"
	case kop.StepEnd, kop.StepCleanup:
		return "ending"
	}
	return "combat"
}

// canon.CounterNames are gorge's counter kinds for the kernel's counter array.
var sbtManaSyms = [6]string{"W", "U", "B", "R", "G", "C"}

// card fills cv for object o (its projection po).
func (a *SBT) card(s *dirac.State, o uint8, cv *view.CardView) {
	c := a.c
	po := &a.rs.kp.Objs[o]
	cid := po.Card
	*cv = view.CardView{ID: sbtObjID(o), Controller: state.PlayerID(po.Ctrl), Owner: state.PlayerID(po.Owner), IsToken: po.Token}
	if po.Flags&dirac.ProjFaceDown != 0 {
		// CR 708.2: a face-down permanent's face is hidden from every viewer
		// but its controller (gorge view.mayLookFaceDown). The controller
		// still sees the printed card, marked FaceDown.
		cv.FaceDown = true
		if po.Ctrl != uint8(a.v.Viewer) {
			return
		}
	}
	cd := c.card(cid)
	if cd == nil {
		return
	}
	cv.Name, cv.Types, cv.ManaCost, cv.SpellAPI, cv.PrintedPT = cd.name, cd.types, cd.manaCost, cd.spellAPI, cd.pt
	cv.Produces = cd.prod
	ob := dirac.Obj(o)
	if po.Zone != kop.ZoneBattlefield {
		v := c.view(cid)
		cv.Power, cv.Toughness = v.pow, v.tou
		if p, t, ok := s.FeatCDAPT(ob); ok {
			cv.Power, cv.Toughness = int32(p), int32(t)
		}
		cv.Keywords = v.kw
		return
	}
	cv.Tapped = po.Flags&dirac.ProjTapped != 0
	cv.SummonSick = po.Flags&dirac.ProjSick != 0
	cv.Attacking = po.Flags&dirac.ProjAttacking != 0
	cv.Damage = int32(po.Damage)
	cv.Power, cv.Toughness = int32(po.Power), int32(po.Tough)
	if po.Attached != dirac.NoObj {
		cv.AttachedTo = sbtObjID(uint8(po.Attached))
	}
	cv.ActivatedThisTurn = int32(s.FeatActivations(ob))
	if cv.Attacking && dirac.TCode(po.Attacks) >= dirac.PlayerTarget0 && dirac.TCode(po.Attacks) != dirac.NoTarget {
		pl := state.PlayerID(dirac.TCode(po.Attacks) - dirac.PlayerTarget0)
		cv.AttackingPlayer = &pl
	}
	cv.Blocked = s.FeatBlocked(ob)
	// the type line: an animated land is a creature, a creature an effect
	// made a non-creature is not (sb-tactical counts creatures by the line)
	isCre := s.IsCreature(ob)
	if isCre != cd.creature {
		if isCre {
			cv.Types += " Creature"
		} else {
			cv.Types = "Land"
		}
	}
	granted, printed := s.FeatGranted(ob)
	if printed {
		if v := c.view(cid); len(v.kw) > 0 {
			cv.Keywords = v.kw
		}
	}
	if granted != 0 {
		kws := append([]string(nil), cv.Keywords...)
		for m := uint32(granted); m != 0; m &= m - 1 {
			if n := feat.KeywordNames[bits.TrailingZeros32(m)]; n != "" {
				kws = append(kws, n)
			}
		}
		cv.Keywords = kws
	}
	for k, n := range po.Counters {
		if n == 0 || k >= len(canon.CounterNames) || canon.CounterNames[k] == "" {
			continue
		}
		if cv.Counters == nil {
			cv.Counters = map[string]int32{}
		}
		cv.Counters[canon.CounterNames[k]] = int32(n)
	}
	for x, n := range po.CountersX {
		if n == 0 {
			continue
		}
		if cv.Counters == nil {
			cv.Counters = map[string]int32{}
		}
		cv.Counters[kop.CtrXNames[x]] = int32(n)
	}
}

// buildView projects s for seat me into a.v (reusing its storage).
// potential is the seat's PotentialActions (priority only).
func (a *SBT) buildView(s *dirac.State, me uint8, potential []decision.PotentialAction) *view.View {
	kp := &a.rs.kp
	v := &a.v
	players := v.Players[:0]
	*v = view.View{Viewer: state.PlayerID(me), Turn: int32(kp.Turn), Active: state.PlayerID(kp.Active),
		Priority: state.PlayerID(kp.Prio), Over: kp.Over, Players: players}
	if int(kp.Step) < len(sbtSteps) {
		v.Step = sbtSteps[kp.Step]
	}
	v.Phase = sbtPhase(kp.Step)
	// zone lists in gorge's order: the battlefield in entry order, the
	// others in arrival order (the projection's zone index)
	for p := uint8(0); p < 2; p++ {
		pv := view.PlayerView{ID: state.PlayerID(p), Life: int32(kp.Life[p]), LibrarySize: int(kp.LibN[p]),
			HandSize: int(kp.HandN[p]), GraveyardSize: int(kp.GraveN[p])}
		if kp.Over && kp.Winner >= 0 && uint8(kp.Winner) != p {
			pv.Lost = true
		}
		pool := map[string]int32{}
		for k := 0; k < 6 && k < kop.NumManaKinds; k++ {
			if n := kp.Pool[p][k]; n > 0 {
				pool[sbtManaSyms[k]] = int32(n)
			}
		}
		pv.Pool = pool
		if p == me {
			pv.PotentialActions = potential
		}
		v.Players = append(v.Players, pv)
	}
	a.zone = a.zone[:0]
	for o := 0; o < int(kp.NObj); o++ {
		po := &kp.Objs[o]
		switch po.Zone {
		case kop.ZoneBattlefield, kop.ZoneGraveyard, kop.ZoneExile:
		case kop.ZoneHand:
			if po.Owner != me {
				continue
			}
		default:
			continue
		}
		a.zone = append(a.zone, uint8(o))
	}
	// insertion sort by (zone, seat, index): a few dozen objects
	key := func(o uint8) uint32 {
		po := &kp.Objs[o]
		seat := po.Owner
		if po.Zone == kop.ZoneBattlefield {
			seat = po.Ctrl
		}
		return uint32(po.Zone)<<16 | uint32(seat)<<8 | uint32(po.ZoneIdx)
	}
	z := a.zone
	for i := 1; i < len(z); i++ {
		for j := i; j > 0 && key(z[j]) < key(z[j-1]); j-- {
			z[j], z[j-1] = z[j-1], z[j]
		}
	}
	for _, o := range z {
		po := &kp.Objs[o]
		var cv view.CardView
		a.card(s, o, &cv)
		switch po.Zone {
		case kop.ZoneBattlefield:
			pv := &v.Players[po.Ctrl&1]
			pv.Battlefield = append(pv.Battlefield, cv)
		case kop.ZoneHand:
			pv := &v.Players[po.Owner&1]
			pv.Hand = append(pv.Hand, cv)
		case kop.ZoneGraveyard:
			pv := &v.Players[po.Owner&1]
			pv.Graveyard = append(pv.Graveyard, cv)
		case kop.ZoneExile:
			pv := &v.Players[po.Owner&1]
			pv.Exile = append(pv.Exile, cv)
		}
	}
	// BlockedBy: each blocker's attacker
	for o := 0; o < int(kp.NObj); o++ {
		po := &kp.Objs[o]
		if po.Zone != kop.ZoneBattlefield || po.Flags&dirac.ProjBlocking == 0 || po.Blocks == dirac.NoObj {
			continue
		}
		att := sbtObjID(uint8(po.Blocks))
		for p := range v.Players {
			for i := range v.Players[p].Battlefield {
				if cv := &v.Players[p].Battlefield[i]; cv.ID == att {
					cv.BlockedBy = append(cv.BlockedBy, sbtObjID(uint8(o)))
				}
			}
		}
	}
	for i := 0; i < int(kp.NStack); i++ {
		e := &kp.Stack[i]
		sv := view.StackView{Controller: state.PlayerID(e.Ctrl)}
		// CR 708.4: a face-down spell has no name, types or abilities on
		// the stack; every viewer but its controller sees a blank band
		// (gorge view.stackViews).
		hiddenFD := e.Kind == 0 && int(e.Source) < int(kp.NObj) &&
			kp.Objs[e.Source].Flags&dirac.ProjFaceDown != 0 && kp.Objs[e.Source].Ctrl != me
		if !hiddenFD {
			if cd := a.c.card(e.Card); cd != nil {
				sv.Name = cd.name
			}
		}
		switch e.Kind {
		case 0:
			sv.Kind, sv.ID = "spell", sbtObjID(uint8(e.Source))
			if int(e.Source) < int(kp.NObj) && !hiddenFD {
				cv := new(view.CardView)
				a.card(s, uint8(e.Source), cv)
				sv.Card = cv
			}
		case 1:
			sv.Kind, sv.ID, sv.Source = "ability", sbtStackID(i), sbtObjID(uint8(e.Source))
		default:
			sv.Kind, sv.ID, sv.Source = "trigger", sbtStackID(i), sbtObjID(uint8(e.Source))
		}
		sv.X = int32(e.X)
		for _, t := range e.Targets {
			switch {
			case dirac.TCode(t) == dirac.NoTarget:
			case dirac.TCode(t) >= dirac.PlayerTarget0:
				sv.Targets = append(sv.Targets, view.TargetView{Player: state.PlayerID(dirac.TCode(t) - dirac.PlayerTarget0), IsPlayer: true})
			default:
				sv.Targets = append(sv.Targets, view.TargetView{Obj: sbtObjID(t)})
			}
		}
		v.Stack = append(v.Stack, sv)
	}
	if v.Stack == nil {
		v.Stack = []view.StackView{}
	}
	return v
}
