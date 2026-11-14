package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Teamwork N (CR 702.194, Marvel Super Heroes), copied from gorge (rules
// kw_teamwork.go's teamworkOffer, cast_begin.go's teamwork ask, cast_payment.go,
// effects count_body_heads.go's Count$Teamwork):
//
//   - the teamworked cast is gorge's "teamworked" cast mode, offered beside
//     the plain cast when the spell's targets are available under the
//     teamworked intent and the caster's untapped creatures' power totals N
//     or more. A card never has Casualty, Bargain or Kicker with it (kgen
//     refuses it), so the cast reuses AkCastCasualty and CardDef.Teamwork
//     picks the row. The offer prices the plain cast, as Casualty does.
//   - the declared intent (CR 601.2b) is sfKicked on the entry being built:
//     the kicked-range target slots (Cruel Alliance's "instead exile target
//     creature") and the charm's pick count (CardDef.ModesPickTW) read it
//     while the spell's modes and targets are announced.
//   - the tap election runs at the casualty stage, before X and targets
//     (gorge poses it after conspire, before the optional sacrifice and
//     convoke): ChGenTeamwork picks over the untapped creatures, closed by
//     Done once their total power reaches N. Done with no creature picked
//     declines (gorge: an empty answer): nothing is tapped, the intent stays
//     through the modes and targets (gorge's pc.mode), and sfKicked is
//     cleared from the pushed spell (casTWDecline). The picks are pkTapX taps, settled at
//     payment with the other costs.
//   - the paid flag is what is left of sfKicked: ExKicked (Count$Teamwork)
//     and F2Kicked (Card.Self+Teamwork) read it. A copy is never cast, so
//     copySpell clears it (CR 707.10).
//
// DebugTeamworkNoFlag is the mutation hook: the paid election drops the
// provenance bit, so TestTeamworkLockstep proves the flag is what the
// readers see.
var DebugTeamworkNoFlag bool

// teamworkOffer: p's untapped creatures' power totals CardDef.Teamwork or
// more (gorge teamworkOffer: Creature.YouCtrl, not tapped; summoning
// sickness is irrelevant to a tap that is not a {T} ability).
func (s *State) teamworkOffer(k *prep, p uint8, c CardID) bool {
	n := int(k.t.Cards[c].Teamwork)
	sum := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		if sl := m.first(); s.perms[sl].flags&pfTapped == 0 && s.isCreature(k, sl) {
			sum += s.power(k, sl)
		}
	}
	return n != 0 && sum >= n
}

// teamworkTargets reports whether the spell's targets are available to the
// plain cast and to the teamworked one (gorge castTargetsAvailable with
// the cast's mode "" and "teamworked"). They differ for a kicked-range
// slot (Cruel Alliance: any creature when teamworked, mana value 3 or
// less otherwise) and for a charm's pick count.
func (s *State) teamworkTargets(k *prep, c CardID, p uint8) (plain, tw bool) {
	ab := spellAb(k, c)
	if ab == 255 {
		ok := s.spellTargetsFeasible(k, c, p)
		return ok, ok
	}
	a := &k.t.Cards[c].Abilities[ab]
	if k.gen != nil {
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
		defer func() { s.g.probe = probe{} }()
	}
	return s.teamworkFeasible(k, a, c, p, false), s.teamworkFeasible(k, a, c, p, true)
}

// teamworkFeasible is abilityTargetsFeasible for a spell ability under one
// election: a charm needs its pick count's worth of feasible modes, every
// other slot with a non-zero minimum under the election needs a candidate.
func (s *State) teamworkFeasible(k *prep, a *kop.Ability, c CardID, p uint8, tw bool) bool {
	if a.NModes == 0 {
		return s.teamworkSlots(k, a, p, 0xff, tw)
	}
	need := charmMin(a)
	if cd := &k.t.Cards[c]; tw && cd.ModesPickTW != 0 {
		need = int(cd.ModesPickTW)
	}
	n := 0
	for m := 0; m < int(a.NModes); m++ {
		if s.teamworkSlots(k, a, p, 1<<m, tw) {
			n++
		}
	}
	return n >= need
}

// teamworkSlots: every mandatory slot of the modes has a candidate under
// the election (a kicked-range slot reads its kicked bounds when tw).
func (s *State) teamworkSlots(k *prep, a *kop.Ability, p uint8, modes uint8, tw bool) bool {
	for i := 0; i < int(a.NTargets); i++ {
		t := &a.Targets[i]
		if t.TFlags&kop.TgfAtResolution != 0 {
			continue
		}
		if tm := a.TgtModes[i]; tm != 0 && tm&modes == 0 {
			continue
		}
		lo := staticSlotMin(t)
		if tw && t.TFlags&kop.TgfKickedRange != 0 {
			lo = t.MinKicked
		}
		if lo != 0 && !s.hasTarget(k, t, p, NoSlot, NoObj) {
			return false
		}
	}
	return true
}

// twModes is the charm pick count the entry being built is announced with
// when it is a teamworked cast (0: the ability's own ModesPick).
func (s *State) twModes(k *prep) uint8 {
	if k.entwine {
		if n, ok := s.entwineModes(k); ok {
			return n // the entwined cast's every feasible mode (kw_entwine.go)
		}
	}
	if e := &s.bld().e; e.kind == seSpell && e.flags&sfKicked != 0 {
		return k.t.Cards[e.card].ModesPickTW
	}
	return 0
}

// teamworkOptions poses the next creature pick of the tap election; false =
// the election is closed (every creature picked) and the build moves on.
// gorge's decision is one KChoose (Min 1, Max the candidates, AllowNone,
// MinSum N over each creature's power); the kernel walks it as sequential
// picks the way tapPowerOptions does: a creature is offered while the
// answer stays completable, Done once the picks reach N (or none were made).
//
//go:noinline
func (s *State) teamworkOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	floor := int(k.t.Cards[e.card].Teamwork)
	sum := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if ob := &s.objs[s.bld().picks[i]]; s.bld().pickKs[i] == pkTapX && ob.zone == kop.ZoneBattlefield {
			sum += s.power(k, ob.slot)
		}
	}
	var cand [MaxPerm]Slot
	var pw [MaxPerm]int
	nc, rest := 0, 0
	for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
		sl := m.first()
		if s.picked(s.perms[sl].obj) || s.perms[sl].flags&pfTapped != 0 || !s.isCreature(k, sl) {
			continue
		}
		cand[nc], pw[nc] = sl, s.power(k, sl)
		if pw[nc] > 0 {
			rest += pw[nc]
		}
		nc++
	}
	if nc == 0 || int(s.bld().pickN) >= len(s.bld().picks) {
		if nc > 0 {
			s.capFull(StallTapPick)
		}
		s.teamworkFinish()
		return false
	}
	d.N = 0
	for i := 0; i < nc; i++ {
		others := rest
		if pw[i] > 0 {
			others -= pw[i]
		}
		if sum+pw[i]+others >= floor {
			d.add(MakeAction(AkChoose, TCode(s.perms[cand[i]].obj), 0, 0))
		}
	}
	if s.bld().pickN == 0 || sum >= floor {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChGenTeamwork, e.ctrl
	return true
}

// answerTeamwork records the chosen creature as a tap pick; the election
// stays open (teamworkOptions poses the next pick or closes it).
func (s *State) answerTeamwork(a Action) {
	s.bld().picks[s.bld().pickN] = Obj(a.Source())
	s.bld().pickKs[s.bld().pickN] = pkTapX
	s.bld().pickN++
}

// casTWDecline is build.cas once the Teamwork election declined (or the
// mutation hook dropped the paid flag): the entry keeps sfKicked, the
// declared intent (CR 601.2b), through the modes and targets, and
// teamworkClear drops it from the pushed spell.
const casTWDecline uint8 = 3

// teamworkFinish closes the election: with no creature picked the cast
// declines the teamwork (gorge: an empty answer leaves teamworkPaid false
// but pc.mode stays "teamworked" for the targets); otherwise the bit stays
// as the paid provenance.
func (s *State) teamworkFinish() {
	b := s.bld()
	b.cas = 0
	if b.pickN == 0 || DebugTeamworkNoFlag {
		b.cas = casTWDecline
	}
	s.syncBuild()
}

// teamworkClear drops the intent bit from the pushed spell of a declined
// election, so it carries no paid provenance (casualtyTrigger).
func (s *State) teamworkClear() {
	s.stack[s.sp-1].flags &^= sfKicked
}

// FeatTeamworkCard reports whether o's card has Teamwork (the oracle's
// cast mode choice; kw_teamwork.go).
func (s *State) FeatTeamworkCard(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].Teamwork != 0
}
