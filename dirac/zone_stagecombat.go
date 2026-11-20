package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// zone_stagecombat.go: the combat a World may hold, and the decision it is
// posed at. Staging reaches:
//
//   - a priority round at the beginning of combat (no combat marks), at the
//     declare attackers step (attackers declared, each with the player it
//     attacks) and at the declare blockers step (blocks declared: each
//     blocker with its attacker and its place in the attacker's block order);
//   - the declare attackers decision and the declare blockers decision at
//     their pose, with no pick made yet (Pose);
//   - a priority round at the first-strike damage step, the regular damage
//     step and end of combat, after the damage the position's marks and life
//     already hold. There a blocker whose attacker has left combat still counts
//     as blocking (CR 506.4) though it blocks nothing: it is Blocking with
//     Blocks == NoObj; and an attacker whose blockers have all left stays
//     blocked (CR 509.1h): Blocked with none. World.FirstStrike says whether
//     the combat had a first-strike damage step. Neither shows before damage,
//     where the position is outside the scope (ReasonCombat).

// Pose is the decision a World poses next.
type Pose uint8

const (
	PosePriority  Pose = iota // a priority round
	PoseAttackers             // the declare attackers decision, no pick made
	PoseBlockers              // the declare blockers decision, no pick made
)

// combatMarked reports whether o carries any combat mark.
func (o *WorldObj) combatMarked() bool {
	return o.Attacking || o.Blocked || o.Blocking
}

func combatBad(format string, a ...any) error { return unstageable(ReasonCombat, format, a...) }

// validateCombat checks w's combat marks and pose against its step.
func (w *World) validateCombat() error {
	var atk, blocked [MaxObj]bool
	var nBlock [MaxObj]uint8
	nAtk, nBlk := 0, 0
	for i := range w.Objs {
		o := &w.Objs[i]
		if !o.combatMarked() {
			continue
		}
		def := &w.Tbl.Cards[o.Card]
		if o.Zone != kop.ZoneBattlefield || def.Types&kop.TCreature == 0 {
			return combatBad("object %d: a combat mark off the battlefield or on a non-creature", i)
		}
		switch {
		case o.Attacking && (o.Blocking || o.Ctrl != w.Active || o.AtkSeat >= w.NSeat || o.AtkSeat == w.Active || o.Blocks != 0 || o.BlockOrd != 0):
			return combatBad("object %d: an attacker the active seat does not attack with", i)
		case o.Blocking && (o.Blocked || o.Ctrl == w.Active):
			return combatBad("object %d: a blocker of the active seat's or a blocked one", i)
		case o.Blocked && !o.Attacking:
			return combatBad("object %d: blocked and not attacking", i)
		}
		if o.Attacking {
			atk[i] = true
			blocked[i] = o.Blocked
			nAtk++
		}
		if o.Blocking {
			nBlk++
		}
	}
	for i := range w.Objs {
		if o := &w.Objs[i]; o.Blocking {
			if o.Blocks == NoObj {
				if !w.afterDamageBegins() || o.BlockOrd != 0 {
					return combatBad("object %d blocks nothing before combat damage or with a place in a block order", i)
				}
				continue
			}
			if int(o.Blocks) >= len(w.Objs) || !atk[o.Blocks] || o.BlockOrd == 0 {
				return combatBad("object %d blocks object %d, which is not attacking", i, o.Blocks)
			}
			if w.Objs[o.Blocks].AtkSeat != o.Ctrl {
				return combatBad("object %d, controlled by seat %d, blocks object %d, which attacks seat %d", i, o.Ctrl, o.Blocks, w.Objs[o.Blocks].AtkSeat)
			}
			nBlock[o.Blocks]++
		}
	}
	for i := range w.Objs {
		if o := &w.Objs[i]; atk[i] && blocked[i] != (nBlock[i] > 0) && !(blocked[i] && w.afterDamageBegins()) {
			return combatBad("object %d: blocked %v with %d blockers", i, o.Blocked, nBlock[i])
		}
	}
	if err := w.validateBlockOrder(); err != nil {
		return err
	}
	return w.validateCombatStep(nAtk, nBlk)
}

// validateBlockOrder requires each attacker's blockers to carry the places
// 1..n in its block order, each once.
func (w *World) validateBlockOrder() error {
	for a := range w.Objs {
		if !w.Objs[a].Attacking {
			continue
		}
		var seen [MaxPerm + 1]bool
		n := 0
		for i := range w.Objs {
			if o := &w.Objs[i]; o.Blocking && int(o.Blocks) == a {
				if int(o.BlockOrd) > MaxPerm || seen[o.BlockOrd] {
					return combatBad("object %d: block place %d of attacker %d", i, o.BlockOrd, a)
				}
				seen[o.BlockOrd] = true
				n++
			}
		}
		for p := 1; p <= n; p++ {
			if !seen[p] {
				return combatBad("attacker %d has no block place %d of %d", a, p, n)
			}
		}
	}
	return nil
}

// afterDamageBegins reports whether w's step is one where combat damage has
// been dealt: the damage steps and end of combat. A blocker of nothing and a
// blocked attacker with no blocker left are legal only there.
func (w *World) afterDamageBegins() bool {
	return w.Step == kop.StepFirstStrikeDamage || w.Step == kop.StepDamage || w.Step == kop.StepEndCombat
}

// damageStep reports whether w's step is one of the two combat damage steps.
func (w *World) damageStep() bool {
	return w.Step == kop.StepFirstStrikeDamage || w.Step == kop.StepDamage
}

// validateCombatStep checks the marks and the pose against the step: attackers
// exist from the declare attackers step on, blocks from the declare blockers
// step on, a pose has no mark of its own decision, and the first-strike flag is
// a damage step's (the first-strike damage step happens only when it is set).
func (w *World) validateCombatStep(nAtk, nBlk int) error {
	switch {
	case nAtk != 0 && w.Step != kop.StepAttackers && w.Step != kop.StepBlockers && !w.afterDamageBegins():
		return combatBad("%d attackers at step %d", nAtk, w.Step)
	case nBlk != 0 && w.Step != kop.StepBlockers && !w.afterDamageBegins():
		return combatBad("%d blockers at step %d", nBlk, w.Step)
	case w.FirstStrike && !w.damageStep():
		return combatBad("the first-strike flag at step %d", w.Step)
	case !w.FirstStrike && w.Step == kop.StepFirstStrikeDamage:
		return combatBad("the first-strike damage step of a combat with no first-strike damage step")
	}
	switch w.Pose {
	case PosePriority:
		// The step is skipped when nothing attacks, so no attacker at it means each
		// one left combat; a blocker of a departed attacker keeps blocking (CR
		// 506.4) and a view shows no mark of it.
		if w.Step == kop.StepBlockers && nAtk == 0 {
			return combatBad("the declare blockers step with no attacker: a blocker of a departed attacker is not in a view")
		}
	case PoseAttackers:
		if w.Step != kop.StepAttackers || nAtk != 0 || len(w.Stack) != 0 {
			return combatBad("the declare attackers pose with attackers, a stack or another step")
		}
	case PoseBlockers:
		if w.Step != kop.StepBlockers || nAtk == 0 || nBlk != 0 || len(w.Stack) != 0 {
			return combatBad("the declare blockers pose with no attacker, a block, a stack or another step")
		}
		return w.validateBlkDef()
	default:
		return combatBad("pose %d", w.Pose)
	}
	return nil
}

// validateBlkDef checks the defender asked at a pod's declare blockers pose: a
// seat other than the active one that some attacker attacks. At two seats it is
// the active seat's one opponent and is not read.
func (w *World) validateBlkDef() error {
	if w.NSeat == 2 {
		return nil
	}
	if w.BlkDef >= w.NSeat || w.BlkDef == w.Active {
		return combatBad("the defender %d of the declare blockers pose", w.BlkDef)
	}
	for i := range w.Objs {
		if o := &w.Objs[i]; o.Attacking && o.AtkSeat == w.BlkDef {
			return nil
		}
	}
	return combatBad("no attacker attacks the defender %d of the declare blockers pose", w.BlkDef)
}

// stageCombat marks the attackers and blockers of w on s and sets the combat
// counters as the declaration left them.
func (s *State) stageCombat(k *prep, w *World) {
	order := uint8(0)
	for i := range w.Objs {
		o := &w.Objs[i]
		if !o.Attacking {
			continue
		}
		sl := s.objs[i].slot
		p := &s.perms[sl]
		p.flags |= pfAttacking
		s.setAttackTarget(p, PlayerCode(o.AtkSeat))
		p.bord = objWord(order)
		order++
		if o.Blocked {
			p.flags |= pfBlocked
		}
		s.g.declM.set(sl)
		s.touchSl(sl)
	}
	s.nAttackers = order
	s.g.attacked[w.Active] = order
	s.declared = order != 0
	nb := uint8(0)
	for i := range w.Objs {
		o := &w.Objs[i]
		if !o.Blocking {
			continue
		}
		b := s.objs[i].slot
		bp := &s.perms[b]
		bp.flags |= pfBlocking
		if o.Blocks == NoObj { // its attacker left combat (forgetBlocked)
			bp.blocking = NoSlot
			bp.bord = 0
			s.touchSl(b)
			nb++
			continue
		}
		a := s.objs[o.Blocks].slot
		bp.blocking = a
		bp.bord = objWord(o.BlockOrd)
		s.perms[a].nblock++
		s.touchSl(b)
		s.touchSl(a)
		nb++
	}
	s.blockSeq = nb
	s.fsStep = s.anyFirstStrike(k)
	if w.damageStep() {
		s.fsStep = w.FirstStrike // a first striker that died still made the step happen
		s.declared = true        // a damage step exists only after attackers were declared
	}
}

// stageDamageStep sets what a priority round of a damage step holds beyond its
// marks, once the step and the active seat are set: the first-strike damage
// step's creatures that dealt damage in it deal no regular damage (pfDealtFirst,
// as combatDamage leaves them) and the step's division flag.
func (s *State) stageDamageStep(k *prep, w *World) {
	if w.Step != kop.StepFirstStrikeDamage {
		return
	}
	s.divFirst = true
	s.markFogFirst(k)
}

// stagePose puts s at the decision w poses: a priority round is what the
// caller has set; a pose of the attackers or blockers decision replaces it.
// The priority holder a pose leaves is the one that passed last in the round
// before it, the seat before the active one, and the round had passed NSeat
// times (the view's Priority names the deciding seat, which is not the holder
// dirac projects). A pod's blockers pose asks the defender w.BlkDef.
func (s *State) stagePose(w *World) {
	if w.Pose == PosePriority {
		return
	}
	s.lastPick = NoSlot
	s.prio = (w.Active + w.NSeat - 1) % w.NSeat
	s.passes = w.NSeat
	if w.Pose == PoseAttackers {
		s.atkArm().poseMask = s.attackingMask()
		s.ctl = ctlAttackers
		return
	}
	s.blkArm().poseMask = s.blockingMask()
	if w.NSeat > 2 {
		s.g.blkDef = w.BlkDef
	}
	s.ctl = ctlBlockers
}
