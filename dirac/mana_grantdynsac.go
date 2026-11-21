package dirac

// A dynamic {T}, sacrifice-self mana source (Lotus Blossom's "{T}, Sacrifice
// Lotus Blossom: Add X mana of any one colour") beside a granted mana ability
// (Gift of Paradise on a Plains): gorge's census cannot price the source (the
// probe needs a TapOnly shape), so its one prefix search activates it as a gap,
// in battlefield order among the granted sources: the Plains' printed {W}, the
// Lotus (tap, sacrifice, float), then the cast pays the rest from the pool and
// the lands (CR 605.3b). lowerDynSacGap floats the source for an ability only
// and with no grant on the board (mana_dynsacgap.go), so the grant search lists
// it as a gap kind of its own (grantDynSacGaps), the way it lists the
// sacrifice-another one (mana_grantsacx.go).

// stepDynSac is the gi of a prefix step that floats a dynamic {T}, sacrifice
// source (col the colour its X mana is asked).
const stepDynSac int16 = -11

// grantDynSacGaps: p's dynamic {T}, sacrifice-self sources the grant search
// activates as gaps, a slot mask.
func (s *State) grantDynSacGaps(k *prep, p uint8) slotMask {
	var m slotMask
	for b := s.ctrlM[SeatIx(p)].and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		pc := s.manaPCGen(k, sl)
		if !(pc.manaDyn && pc.manaSac) || s.perms[sl].flags&pfTapped != 0 {
			continue
		}
		if pc.manaAb < 0 && s.perms[sl].ovr.flags&ovCopy == 0 {
			continue
		}
		if s.canTapForManaPC(k, sl, pc) {
			m = m.with(sl)
		}
	}
	return m
}

// dynSacBranch floats candidate i's (slot sl) dynamic source on a copy of t:
// tap, the X mana in the script's colour, sacrifice (dynSacFloat), then the
// search prices the play again. A holder of a grant keeps the grant side.
func (g *grantSrch) dynSacBranch(t *State, i int, sl Slot) bool {
	if !t.occ.has(sl) || t.perms[sl].obj != g.cobj[i] || t.perms[sl].flags&pfTapped != 0 || g.np >= len(g.path) {
		return false // sacrificed or tapped by an earlier step
	}
	if !t.canTapForManaPC(g.k, sl, t.manaPCGen(g.k, sl)) {
		return false
	}
	u := t.fork()
	u.dynSacFloat(g.k, g.p, sl, g.need)
	g.path[g.np] = grantStep{i: uint8(i), gi: stepDynSac, sl: sl}
	g.np++
	var ok bool
	if g.tapsLive {
		ok = g.settle(u) // (the tap may have fired a trigger)
	} else {
		ok = g.visit(u)
	}
	t.unfork()
	if !ok {
		g.np--
	}
	return ok
}
