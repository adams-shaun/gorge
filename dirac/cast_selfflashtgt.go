package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Target-conditional self flash: `S:Mode$ CastWithFlash | ValidCard$
// Card.Self | ValidSA$ Spell.IsTargeting Valid <spec> | EffectZone$ All |
// Caster$ You` (Flash Photography, Timely Ward). The static is a SelfFlash
// carrier's own StatCastWithFlash ability whose Filter2 is the <spec> read
// over battlefield objects with the caster as "you" (kgen
// selfFlashTargetsFilter). gorge reads it three times, all through
// castWithFlashTargets / spellMatchesValidSA:
//
//   - the offer (selfFlashTargets): the grant holds only when some legal
//     target of the spell (costPotentialTargets) matches the spec;
//   - the target ask (flashTgtNarrow, gorge flashPermittedCandidates): a cast
//     whose off-sorcery timing rests on the grant announces only a matching
//     target;
//   - CR 601.2e (flashTgtRecheck, gorge flashGrantCoversTargets in
//     recheckIllegal): the announced targets must still match, else the cast
//     is reversed before anything is paid.
//
// Forge reads `IsTargeting Valid <spec>` over the battlefield alone (gorge
// spellIsTargetingMatchesPtr), so a player or a graveyard target never matches.

// selfFlashTargets reports whether card c, cast by p, has a legal target on
// the battlefield matching filter f (the offer's potential-target census).
// A modal spell has no census (gorge costPotentialTargets returns none for a
// Choices$ ability), so the grant never holds for it.
func (s *State) selfFlashTargets(k *prep, f kop.FilterID, p uint8, c CardID) bool {
	ab := spellAb(k, c)
	armed := k.gen != nil && s.g.probe.kind == 0
	if armed {
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
	}
	ok := false
	if ab == 255 {
		ok = s.flashTgtSlot(k, tspec(k, c, ab, 0), f, p)
	} else if a := &k.t.Cards[c].Abilities[ab]; a.NModes == 0 {
		for i := 0; i < int(a.NTargets) && !ok; i++ {
			ok = s.flashTgtSlot(k, &a.Targets[i], f, p)
		}
	}
	if armed {
		s.g.probe = probe{}
	}
	return ok
}

// flashTgtSlot: some legal candidate of target slot t matches f.
func (s *State) flashTgtSlot(k *prep, t *kop.TargetSpec, f kop.FilterID, p uint8) bool {
	if t == nil || t.Kind == kop.TgtNone || t.TFlags&kop.TgfAtResolution != 0 {
		return false
	}
	var buf [MaxOptions]TCode
	n := s.targetCands(k, t, p, NoSlot, NoObj, &buf)
	return s.flashTgtKeep(k, f, p, &buf, n) > 0
}

// flashTgtMatch: target code c is a battlefield object matching f for p.
func (s *State) flashTgtMatch(k *prep, f kop.FilterID, p uint8, c TCode) bool {
	if isPlayerTarget(c) || int(c) >= MaxObj {
		return false
	}
	ob := &s.objs[Obj(c)]
	return ob.zone == kop.ZoneBattlefield && s.matchPerm(k, f, ob.slot, p, NoSlot)
}

// flashTgtKeep keeps the candidates of buf[:n] that match f, in order, and
// returns how many.
func (s *State) flashTgtKeep(k *prep, f kop.FilterID, p uint8, buf *[MaxOptions]TCode, n int) int {
	m := 0
	for i := 0; i < n; i++ {
		if s.flashTgtMatch(k, f, p, buf[i]) {
			buf[m] = buf[i]
			m++
		}
	}
	return m
}

// flashTgtFilter is the IsTargeting filter the spell being built rests on: 0
// unless this cast is a SelfFlash carrier's, made at a time a sorcery could
// not have been cast (gorge offSorcery: the stack held nothing but this spell,
// pushed, and it is not the caster's main phase), with no instant type, no
// Flash and no other CastWithFlash grant to carry the timing (gorge
// flashGrantCoversTargets answers true for those and never rechecks).
func (s *State) flashTgtFilter(k *prep, e *stackEntry) kop.FilterID {
	if e.kind != seSpell || e.flags&sfFree != 0 {
		return 0
	}
	cd := &k.t.Cards[e.card]
	if !cd.SelfFlash || k.cards[e.card].instant || k.cards[e.card].kw&kop.KwFlash != 0 {
		return 0
	}
	below := int(s.sp)
	if s.bld().bfl&bflPushed != 0 {
		below--
	}
	if below == 0 && e.ctrl == s.active && (s.step == kop.StepMain1 || s.step == kop.StepMain2) && s.cfgx&cfgxNoSorcery == 0 {
		return 0 // announced at sorcery speed: the grant carries nothing
	}
	if s.statM.any() && k.anyFlashStat && s.castStatic(k, kop.StatCastWithFlash, e.ctrl, e.card) || k.anyEmbFlash && s.embFlash(k, e.ctrl, e.card) {
		return 0 // another grant carries the timing
	}
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCastWithFlash && a.Static.Filter2 != 0 {
			return a.Static.Filter2
		}
	}
	return 0
}

// flashTgtNarrow withholds, from a target ask of a cast resting on the
// grant, every candidate the grant does not cover (gorge
// flashPermittedCandidates: the offer admitted the cast because SOME
// matching target exists, so handing out another guarantees the reversal).
func (s *State) flashTgtNarrow(k *prep, e *stackEntry, buf *[MaxOptions]TCode, n int) int {
	if n == 0 {
		return n
	}
	f := s.flashTgtFilter(k, e)
	if f == 0 {
		return n
	}
	return s.flashTgtKeep(k, f, e.ctrl, buf, n)
}

// flashTgtRecheck is CR 601.2e for the grant: a cast resting on it must have
// announced a target matching the spec (gorge recheckIllegal /
// flashGrantCoversTargets). It reports that the cast must be reversed.
func (s *State) flashTgtRecheck(k *prep, e *stackEntry) bool {
	f := s.flashTgtFilter(k, e)
	if f == 0 {
		return false
	}
	for i := 0; i < kop.MaxTargets; i++ {
		if e.tgt[i] != NoTarget && s.flashTgtMatch(k, f, e.ctrl, e.tgt[i]) {
			return false
		}
	}
	return true
}
