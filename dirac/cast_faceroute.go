package dirac

// gorge's prices a face route through the potential
// planner as well as the floating pool, so a face cast from untapped
// sources is a lowered play: the witness sources are tapped at priority,
// before the cast asks its targets (rules potentialModeCastPlan +
// payexec lowering). The kernel offers the same cast from its mana census
// (face_castface.go, kw_sneak.go); these helpers tap the same
// witness so the two engines project the same pool and taps while the cast
// is built.

// DebugFacePoolOnly restores the old pool-only face offer (the floating
// pool alone pays, FeatPoolPays): the lockstep mutation proves the
// untapped-source offers are load-bearing. Test-only.
var DebugFacePoolOnly bool

// lowerFaceCast taps a face cast's witness sources at priority when the
// floating pool alone does not pay it (gorge's potential plan). A
// pool-payable face cast stays a direct option (d.Options), with no
// priority-time taps.
func (s *State) lowerFaceCast(k *prep, p uint8, o Obj) {
	if s.cfg&cfgGorgeUnpriced == 0 || s.FeatPoolPays(p, o, 0) {
		return
	}
	// The witness prices the probe face (buildCost reads s.bld().e.card), but
	// gorge's hand-reserve census reads the object as it lies in hand: the
	// front face (Twinmaw Stormbrood's {W}, not Charring Bite's {R}), or a
	// Room's up door (Faces[1-Face]) when it shows door 1.
	ob := &s.objs[o]
	face := ob.card
	if cd := &k.t.Cards[face]; (cd.Face != 0 || roomDef(k, face)) && len(cd.Faces) > 0 {
		// A Room showing door 1 shows Faces[1-Face] in hand (its def's Face
		// is the door up); every other face card shows its front.
		if roomDef(k, face) {
			ob.card = cd.Faces[1-cd.Face] // seat-ok: the other face of a two-faced card
		} else {
			ob.card = cd.Faces[0]
		}
		defer func() { ob.card = face }()
	}
	s.bld().bfl |= bflLowered
	s.lowerWitness(k, p)
}

// lowerSneakCast is lowerFaceCast for a sneak cast: buildCost prices the
// sneak cost once beginSneak has marked the entry (trigPlSneak), and the
// pool check is the sneak cost's.
func (s *State) lowerSneakCast(k *prep, p uint8, o Obj, c CardID) {
	if s.cfg&cfgGorgeUnpriced == 0 {
		return
	}
	cost := s.buildCost(k)
	// gorge's planner prices the granted delve's credit (offerPlay's choke
	// point), so the pool gate prices the credited cost too; a twobrid cost
	// is leaf-credited by the gate's avail (gateDelveCredit)
	cost = s.gateDelveCredit(k, p, c, o, false, cost)
	if s.sneakPoolPays(k, p, c, o, &cost) {
		return
	}
	s.bld().bfl |= bflLowered
	s.lowerWitness(k, p)
}
