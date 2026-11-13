package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ProjObj is one object's canonical projection.
type ProjObj struct {
	Card     CardID
	Base     CardID // the printed card of a ProjBase permanent (Card is the card it copies)
	Owner    uint8
	Ctrl     uint8 // battlefield controller (owner elsewhere)
	Zone     kop.Zone
	ZoneIdx  uint8 // position in an ordered zone: library from the top, hand/graveyard/exile in arrival order, battlefield in its controller's list (entry order)
	Token    bool
	Flags    uint8 // ProjTapped | ProjAttacking | ProjBlocking | ProjSick
	Flags2   uint8 // ProjSuspected
	Damage   uint8
	Counters [kop.NumCounterKinds2]uint16
	// CountersX is the unfeaturized kinds' counts, CountersX[k-CtrCharge]
	// (kcounters: outside Counters so the feature layout stays fixed)
	CountersX [kop.NumCounterKindsX]uint16
	Power     int16 // derived P/T (battlefield creatures only; Mossborn Hydra passes 127, pair 5419)
	Tough     int16
	Attached  Obj   // the object this Aura/Equipment is attached to
	Blocks    Obj   // blocker: the attacker it blocks
	Attacks   TCode // attacker: the defender (PlayerTarget* or a planeswalker Obj); NoTarget otherwise
}

// Projection flag bits.
const (
	ProjTapped uint8 = 1 << iota
	ProjAttacking
	ProjBlocking
	ProjSick
	ProjCopy      // a token created as a copy (CopyAsToken), or a permanent copying another
	ProjMonstrous // the permanent's CR 701.37b monstrous designation (act_monstrosity.go)
	ProjFaceDown  // a face-down permanent or face-down spell (CR 708.2)
	ProjBase      // a battlefield permanent wearing another card's copiable values by a copy effect (Assimilation Aegis); Base is its printed card, which gorge projects (zone_exilethenattach.go)
)

// Projection flag bits of ProjObj.Flags2 (Flags is full).
const (
	ProjSuspected uint8 = 1 << iota // the permanent's CR 701.60 suspected designation (act_suspect.go)
	ProjNameless                    // a token copy of a face-down permanent, which has no name (blankCopy)
)

// ProjStack is one stack entry.
type ProjStack struct {
	Kind    uint8 // 0 spell, 1 activated ability, 2 triggered ability
	Source  Obj
	Card    CardID
	Ability uint8 // ability index in the CardDef; 255 = permanent spell
	Ctrl    uint8
	Targets [kop.MaxTargets]TCode
	// a generated table's multi-object target slots: the objects beyond
	// the first of slot ExtraSlot[i], in pick order
	Extra     [maxXTgt]TCode
	ExtraSlot [maxXTgt]uint8
	NExtra    uint8
	Mode      uint8
	X         uint8
}

// Projection is the canonical, comparable view of a State for the oracle:
// fixed arrays in a deterministic order (objects by Obj index, the stack
// bottom to top). Two States in the same game position project equal.
type Projection struct {
	NObj   Obj
	Objs   [MaxObj]ProjObj
	Life   [MaxSeats]int16
	NStack uint8
	Stack  [MaxStack]ProjStack
	Turn   uint16
	Step   kop.Step
	Active uint8 // the starting player before the first turn (gorge view.Project)
	Prio   uint8
	Draws  uint64 // RNG draws (gorge Engine.RNGDraws)
	Lands  [MaxSeats]uint8
	Pool   [MaxSeats][kop.NumManaKinds]uint8
	LibN   [MaxSeats]uint8
	HandN  [MaxSeats]uint8
	GraveN [MaxSeats]uint8
	ExileN [MaxSeats]uint8
	Over   bool
	Winner int8
	// CmdCast is each seat's commander casts from the command zone (CR
	// 903.8); CmdDmg[p][q] the combat damage player p was dealt by seat q's
	// commander (CR 903.10a). A partner pair's two tallies are summed here
	// (saturating); FeatCommanderCastN / FeatCommanderDamageN keep them apart.
	CmdCast [MaxSeats]uint8
	CmdDmg  [MaxSeats][MaxSeats]uint8
	// Chars is each battlefield object's derived characteristics, by Obj
	// index; only ProjectChars (the oracle's) fills it.
	Chars [MaxObj]ProjChars
	// NSeat is the seat count of a game of more than two seats and Lost
	// each seat's loss (CR 800.4a); both stay zero at two seats, where a
	// loss ends the game.
	NSeat uint8
	Lost  [MaxSeats]bool
}

// Project writes s's canonical projection into p.
func (s *State) Project(p *Projection) {
	k := s.k()
	*p = Projection{}
	p.NObj = s.nobj
	for i := 0; i < int(s.nobj); i++ {
		s.projObj(k, Obj(i), &p.Objs[i])
	}
	s.projRest(p, true)
}

// ProjectDelta brings p, a projection of the State whose epoch s carries
// (taken when SetEpoch was called), up to s: the objects marked changed
// (and every battlefield object when all is set: a caller whose derived
// P/T may have moved with another object's change), the zone positions
// and every non-object field -- library positions excepted: they stay as
// the base's (FeatZoneIndex reads one).
func (s *State) ProjectDelta(p *Projection, all bool) {
	k := s.k()
	n0 := int(p.NObj)
	p.NObj = s.nobj
	for w, m := range s.chg {
		for ; m != 0; m &= m - 1 {
			if i := Obj(w<<6 + bits.TrailingZeros64(m)); int(i) < int(s.nobj) {
				s.projChanged(k, i, &p.Objs[i])
			}
		}
	}
	for i := n0; i < int(s.nobj); i++ {
		s.projObj(k, Obj(i), &p.Objs[i])
	}
	if all {
		for m := s.occ; m.any(); m = m.rest() {
			o := s.perms[m.first()].obj
			s.projObj(k, o, &p.Objs[o])
		}
	}
	for i := int(s.nobj); i < n0; i++ {
		p.Objs[i] = ProjObj{}
	}
	p.Stack = [MaxStack]ProjStack{}
	s.projRest(p, false)
}

// projChanged re-projects changed object i, cheaply when it was and is in
// a library (only its card can differ: a re-deal).
func (s *State) projChanged(k *prep, i Obj, po *ProjObj) {
	if ob := &s.objs[i]; ob.zone == kop.ZoneLibrary && po.Zone == kop.ZoneLibrary && ob.of&ofToken == 0 {
		po.Card = ob.card
		return
	}
	s.projObj(k, i, po)
}

// ProjectSince brings p, a projection of this State's line of play as of
// touch count since (Touches), up to s: the objects written since (and
// every battlefield object when all is set), the zone positions but the
// library's, and every non-object field.
func (s *State) ProjectSince(p *Projection, since uint32, all bool) {
	k := s.k()
	n0 := int(p.NObj)
	p.NObj = s.nobj
	for w, m := range s.chg {
		for ; m != 0; m &= m - 1 {
			i := Obj(w<<6 + bits.TrailingZeros64(m))
			if int(i) < int(s.nobj) && (int(i) >= n0 || s.ChangedSince(i, since)) {
				s.projChanged(k, i, &p.Objs[i])
			}
		}
	}
	if all {
		for m := s.occ; m.any(); m = m.rest() {
			o := s.perms[m.first()].obj
			s.projObj(k, o, &p.Objs[o])
		}
	}
	for i := int(s.nobj); i < n0; i++ {
		p.Objs[i] = ProjObj{}
	}
	p.Stack = [MaxStack]ProjStack{}
	s.projRest(p, false)
}

// projObj writes object i's projection (ZoneIdx left 255, elsewhere: projRest
// sets it for an object an ordered zone lists).
func (s *State) projObj(k *prep, i Obj, po *ProjObj) {
	ob := &s.objs[i]
	*po = ProjObj{ZoneIdx: 255}
	po.Card, po.Owner, po.Ctrl, po.Zone, po.Token = ob.card, ob.owner(), ob.owner(), ob.zone, ob.of&ofToken != 0
	po.Attached, po.Blocks, po.Attacks = NoObj, NoObj, TCode(NoTarget)
	if po.Token && k.anyInit && ob.card == k.gen.dungeon {
		po.Token = false // a dungeon is no token (act_initiative.go)
	}
	if ob.of&ofCopy != 0 {
		po.Flags |= ProjCopy // a copy token stays one after it ceases (gorge's CopyOf)
	}
	if ob.of&ofPending != 0 {
		po.Zone, po.ZoneIdx = kop.ZoneLibrary, 255 // gorge's minted token, in no zone list (zone 0)
	}
	if s.g.linkHide != 0 && ob.zone == kop.ZoneExile && s.hideHas(i) {
		po.Flags |= ProjFaceDown // a Hideaway card is exiled face down (kw_hideaway.go)
	}
	if k.gen != nil && k.gen.anyFaceExile && ob.zone == kop.ZoneExile && s.faceDownExiled(i) {
		po.Flags |= ProjFaceDown // a Dig's face-down may-look exile (zone_exilefacedown.go)
	}
	if k.gen != nil && k.gen.anyForetell && ob.zone == kop.ZoneExile && s.foretold(i) {
		po.Flags |= ProjFaceDown // a foretold card is exiled face down (kw_foretell.go)
	}
	if ob.zone == kop.ZoneExile && s.g.on && s.mObj(k, mechStash, i) {
		po.Counters[kop.CtrStash] = 1 // Tinybones' stash counter on an exiled card
	}
	if k.gen != nil && k.gen.anySuspend && ob.zone == kop.ZoneExile {
		if n := s.suspendTime(i); n != 0 {
			projCtr(po, uint8(kop.CtrTime), uint16(n)) // a suspended card's time counters (kw_suspend.go)
		}
	}
	if s.res.ask == raGenRetChoice && s.g.pre.obj == i && ob.zone != kop.ZoneBattlefield {
		po.Ctrl = s.res.askP // a card entering under another player, its as-enters choice asked first (cloneReturnPark): gorge changed control before the move
	}
	if s.g.pre.state == 1 && s.g.pre.land && s.g.pre.obj == i {
		po.Ctrl = s.prio // a land played from another player's card (Tinybones), its as-enters choice asked first: gorge changed its control at the play
	}
	if s.res.ask == raGenRetChoice && s.g.pre.obj == i && s.g.pre.state != 0 {
		po.Ctrl = s.res.askP // a GainControl$ return's as-enters choice (Coalstoke Gearhulk): gorge folds the control onto the card before its entry (emitEntryMove)
	}
	if s.res.ask == raGenEnchant && ob.zone != kop.ZoneBattlefield && s.res.askP != ob.owner() && s.enchantReturnObj(k) == i {
		po.Ctrl = s.res.askP // a GainControl$ return of an Aura asks its host under the new controller (Meathook Massacre II): gorge folds the control onto the card before the move (emitEntryMove, CR 303.4f)
	}
	if ob.zone == kop.ZoneStack {
		for j := 0; j < int(s.sp); j++ {
			if s.stack[j].obj == i && s.stack[j].kind == seSpell {
				po.Ctrl = s.stack[j].ctrl // a spell cast from another player's card (Tinybones)
				if s.stack[j].faceDown() {
					po.Flags |= ProjFaceDown
				}
			}
		}
	}
	if ob.zone == kop.ZoneBattlefield || k.phasing && ob.zone.Phased() {
		sl := ob.slot
		pm := &s.perms[sl]
		po.Card = s.cardAt(sl)
		aegis := k.gen != nil && (k.gen.anyAegis || k.gen.anyCloneCopy || k.gen.anyBecomeCopy) && pm.ovr.flags&ovCopy != 0 // tables: gorge projects every copy effect's permanent as its printed card (an ETB copy too), as it does an Aegis bearer
		if aegis {
			po.Base = pm.card
			po.Flags |= ProjBase
		}
		po.Ctrl = pm.ctrl
		if pm.flags&pfTapped != 0 {
			po.Flags |= ProjTapped
		}
		if pm.flags&pfAttacking != 0 {
			po.Flags |= ProjAttacking
			po.Attacks = pm.atk
		}
		if pm.flags&pfBlocking != 0 {
			po.Flags |= ProjBlocking
			if a := s.blockProjected(sl); a != NoSlot {
				po.Blocks = s.perms[a].obj
			}
		}
		if pm.ovr.flags&ovCopy != 0 && !aegis {
			po.Flags |= ProjCopy
		}
		if pm.flags&pfSick != 0 {
			po.Flags |= ProjSick
		}
		if k.gen != nil && k.gen.anyMonstrosity && s.mSl(k, mechMonstrous, sl) {
			po.Flags |= ProjMonstrous
		}
		if k.gen != nil && k.gen.anySuspect && s.suspected(k, sl) {
			po.Flags2 |= ProjSuspected
		}
		if s.faceDown(k, sl) {
			if s.blankCopy(sl) {
				po.Flags2 |= ProjNameless
			} else {
				po.Flags |= ProjFaceDown
			}
		}
		po.Damage = pm.damage
		po.Counters = [kop.NumCounterKinds2]uint16{}
		po.Counters[kop.CtrP1P1] = pm.p1
		for j, c := range pm.ck {
			projCtr(po, c, pm.cn[j])
		}
		if pm.ck[ctrSlots-1] != 0 && !DebugCtrOverProjOff {
			for _, e := range s.ctrOverOf(pm.obj, 0) {
				projCtr(po, e.kind, e.n)
			}
		}
		if s.g.on && s.mObj(k, mechStash, i) {
			po.Counters[kop.CtrStash]++ // gorge keeps the stash counter on a card played from exile
		}
		if hid := k.phasing && s.hiddenAt(sl); s.isCreature(k, sl) || hid && s.faceDown(k, sl) {
			pw, tg := s.pt(k, sl)
			if hid {
				pw, tg = s.phasedPT(k, sl) // kw_phasing.go
			}
			po.Power, po.Tough = int16(pw), int16(tg)
		}
		if pm.attached != NoSlot {
			po.Attached = s.perms[pm.attached].obj
		} else if s.g.lingerM.has(sl) {
			po.Attached = s.lingerHost(sl) // its host left; gorge unattaches at the next state-based check
		}
	}

}

// projRest writes the zone positions (library positions only with lib)
// and the non-object fields.
func (s *State) projRest(p *Projection, lib bool) {
	// battlefield positions: each controller's list, in entry order
	for pi := 0; pi < int(s.nSeats()); pi++ {
		var order [MaxPerm]Slot
		n := 0
		pm := s.ctrlM[pi].and(s.occ)
		if s.k().phasing {
			pm = pm.or(s.phasedCtrl(uint8(pi))) // a phased-out permanent keeps its place in the list
		}
		for m := pm; m.any(); m = m.rest() {
			sl := m.first()
			j := n
			for j > 0 && s.perms[order[j-1]].ts > s.perms[sl].ts {
				order[j] = order[j-1]
				j--
			}
			order[j] = sl
			n++
		}
		for i := 0; i < n; i++ {
			p.Objs[s.perms[order[i]].obj].ZoneIdx = uint8(i)
		}
	}
	for pi := 0; pi < int(s.nSeats()); pi++ {
		pl := &s.pl[pi]
		for i := pl.libLo; lib && i < pl.libHi; i++ {
			p.Objs[pl.lib[i]].ZoneIdx = i - pl.libLo
		}
		for i := 0; i < int(pl.handN); i++ {
			p.Objs[pl.hand[i]].ZoneIdx = uint8(i)
		}
		for i := 0; i < int(pl.graveN); i++ {
			p.Objs[pl.grave[i]].ZoneIdx = uint8(i)
		}
		for i := 0; i < int(pl.exileN); i++ {
			p.Objs[pl.exile[i]].ZoneIdx = uint8(i)
		}
		p.Life[pi] = pl.life
		p.Lands[pi] = s.landsProj(uint8(pi))
		p.Pool[pi] = s.poolProj(uint8(pi))
		p.LibN[pi] = uint8(pl.libLen())
		p.HandN[pi] = pl.handN
		p.GraveN[pi] = pl.graveN
		p.ExileN[pi] = pl.exileN
		// a seat's two commanders project as one sum, as the oracle folds
		// gorge's per-commander lists
		p.CmdCast[pi] = satAdd8(pl.cmdCast, s.cmdCast2(uint8(pi)))
		for q := range p.CmdDmg[pi] {
			p.CmdDmg[pi][q] = satAdd8(pl.cmdDmg[q], s.cmdDmg2(uint8(pi), uint8(q)))
		}
	}
	if n := s.nSeats(); n > 2 {
		p.NSeat = n
		for q := uint8(0); q < n; q++ {
			p.Lost[q] = s.out(q)
		}
	}
	k := s.k()
	sp := s.sp
	if s.res.active && sp > 0 && s.stack[sp-1].kind == seSpell && s.objs[s.stack[sp-1].obj].zone != kop.ZoneStack {
		sp-- // the resolving spell left the stack mid-resolution (moveObj's self-exile); gorge's g.Stack has no such object
	}
	p.NStack = sp
	for i := 0; i < int(sp); i++ {
		e := &s.stack[i]
		p.Stack[i] = ProjStack{Kind: e.kind, Source: e.obj, Card: e.card, Ability: e.ab, Ctrl: e.ctrl, Targets: e.tgt, Mode: e.mode, X: e.x}
		if e.x != 0 && e.kind == seTrigger {
			p.Stack[i].X = projTrigX(k, e) // a bound X is not the object's own (trig_xpaid.go)
		}
		if k.gen != nil && k.gen.anyStackTgt {
			for j, c := range e.tgt {
				if isStackTarget(c) {
					p.Stack[i].Targets[j] = s.stackTgtProj(c, e.tver[j]) // its live index (tgt_stackability.go)
				}
			}
		}
		if e.ab == abDelayed {
			p.Stack[i].Ability = 255 // gorge reports delayed triggers with no ability index
		}
		if e.ab == abWard && wardIsGranted(e.trigAmt) {
			p.Stack[i].Ability = 255 // a granted ward has no face trigger index
		}
		if e.ab == abWard && e.kind == seTrigger && s.objs[e.obj].zone == kop.ZoneBattlefield && s.faceDown(k, s.objs[e.obj].slot) {
			p.Stack[i].Ability = 255 // a face-down permanent's ward is the cloak/disguise one, not its real face's trigger (gorge 255)
		}
		if k.anySaga && e.kind != seSpell && isChapterAbility(k, e.card, e.ab) {
			// a chapter ability is a K:Chapter SVar, not a Face.Triggers
			// entry: gorge's abilityIndex reports 255 for it
			p.Stack[i].Ability = 255
		}
		if e.kind == seTrigger && int(e.ab) < len(k.t.Cards[e.card].Abilities) && k.t.Cards[e.card].Abilities[e.ab].AFlags&kop.AfGift != 0 {
			p.Stack[i].Ability = 255 // a gift trigger is the keyword's, no Face.Triggers entry (kw_gift.go)
		}
		if k.anyGate && e.kind != seSpell && isModeGrant(k, e.card, e.ab) {
			p.Stack[i].Ability = 255 // a Siege's granted trigger is no Face.Triggers entry (face_siegemode.go)
		}
		if e.xi != 0 {
			x := s.xtOf(e)
			p.Stack[i].NExtra = x.n
			p.Stack[i].Extra = x.code
			p.Stack[i].ExtraSlot = x.slot
		}
	}
	p.Turn, p.Step, p.Active, p.Prio = s.turn, s.step, s.active, s.prio
	if s.turn == 0 && !s.over {
		// the pregame (mulligans, Leylines): gorge's view names the
		// starting player active (view.Project), while its Game.Active
		// is still the zero value (the oracle maps it back)
		p.Active = s.startPl
	}
	p.Draws = s.rng.Draws
	p.Over, p.Winner = s.over, s.win
}

// RNGDraws is the number of RNG values consumed (gorge Engine.RNGDraws).
func (s *State) RNGDraws() uint64 { return s.rng.Draws }
