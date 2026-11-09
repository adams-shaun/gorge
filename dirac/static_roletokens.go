package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Role tokens created attached (WOE's Role cycle; CR 111.10j-r, 303.4g,
// 303.7) and gorge's Role exclusivity sweep. Cold: every path here runs
// only for a table that carries a Role token (genPrep.anyRole).

// prepRoles fills the Role prep flags: the matchup subtype index of "Role"
// and whether any card carries it (the Treasure precedent, gen.go). A table
// without Roles leaves anyRole false, so createAttachedToken and the SBA
// cost it nothing.
func prepRoles(t *kop.Table, g *genPrep) {
	for i, n := range t.Subtypes {
		if n != "Role" {
			continue
		}
		for c := range t.Cards {
			if t.Cards[c].Subtypes.Has(i) {
				g.anyRole, g.roleSub = true, uint8(i)
				return
			}
		}
	}
}

// createAttachedToken is OpCreateTokenEx with kop.TokAttach: the mint of an
// Aura token that enters attached (gorge runTokenMints: TokenCreate, the
// riders, then Attach). It creates nothing when the destination is not on
// the battlefield (gorge auraTokenWithheld, the "isn't created (CR 303.4g)"
// Note) or when no permanent on the board can be enchanted at all (gorge
// auraTokenGate, engine/rules/aura_entry.go). It does NOT check that the
// destination is a legal enchantment: gorge attaches unconditionally and
// lets the CR 704.5m SBA remove an illegal attachment. ok reports the
// instruction completed (the caller continues), done reports the op was
// consumed; a withheld token is a completed no-op, not a suspension.
//
//go:noinline
func (s *State) createAttachedToken(k *prep, in *kop.Instr, p uint8, n int) (ok, done bool) {
	if n != 1 {
		// a token doubler would mint two Roles, each attaching and sweeping
		// the other; no pool card doubles a Role.
		s.capFull(StallRoleMulti)
		return true, true
	}
	dst := s.objRef(k, kop.TokAttachDest(in.B.N))
	if dst == NoSlot {
		return true, true
	}
	var buf [MaxOptions]uint8
	if s.enchantCands(k, CardID(in.C), p, &buf) == 0 {
		return true, true
	}
	if s.nearCapacity(k) {
		s.stall(StallBoard)
		return true, true
	}
	o := s.newToken(CardID(in.C), p)
	tapped := in.B.N&kop.TokTapped != 0
	s.g.tokMade++
	if in.B.N&kop.TokRememberObj != 0 {
		s.remember(o)
	}
	if in.B.N&kop.TokImprintObj != 0 {
		s.imprint(o) // zone_imprint.go
	}
	sl := s.enter(k, o, p, tapped, kop.ZoneLibrary)
	if in.B.N&(kop.TokSacAtEnd|kop.TokExileAtEnd) != 0 {
		s.tokenAtEOT(in, o)
	}
	if k.gen.anyTokCr {
		s.fireTokenCreated(k, sl)
	}
	s.res.lastTok = o
	s.g.roleOut = 1 // must be set on EVERY Role entry: roleSBA is skipped while it is 0
	s.attachTo(k, sl, dst)
	s.touchSl(sl)
	return true, true
}

// roleSBA is gorge's Role exclusivity (engine/rules/attach.go roleSBA, CR
// 704.5z / 303.7a, gorge): a state-based action, not an
// attach-time replacement. Among the Roles attached to one bearer that the
// SAME player controls, all but the newest layer timestamp go to the
// graveyard with a plain move (no destroy, no replacement path), so their
// leave triggers fire; Roles of different controllers coexist. It reports
// whether anything left, so the SBA loop runs again.
func (s *State) roleSBA(k *prep) bool {
	ord, n := s.roleLosers(k)
	for _, q := range ord[:n] {
		s.leave(k, q, kop.ZoneGraveyard, false)
	}
	return n > 0
}

// roleLosers lists the Roles roleSBA removes, in gorge's order, without
// moving them: the SBA pass folds them into its own batch (CR 704.3) so a
// creature dying in the same check shares their departure.
func (s *State) roleLosers(k *prep) (out [MaxPerm]Slot, cnt int) {
	var roles slotMask
	for m := s.occ; m.any(); m = m.rest() {
		q := m.first()
		if s.perms[q].attached != NoSlot && s.hasSubGen(k, q, k.gen.roleSub) {
			roles = roles.with(q)
		}
	}
	if !roles.any() {
		s.g.roleOut = 0
		return
	}
	// gorge's battlefield walk (seat 0, then seat 1, zone order); pos is each
	// Role's place in it
	var ord [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < 2; p++ {
		n = s.battlefieldOrder(p, roles.and(s.ctrlM[p]), ord[:], n)
	}
	var pos [MaxPerm]uint8
	for i := 0; i < n; i++ {
		pos[ord[i]] = uint8(i)
	}
	var lose slotMask
	for m := roles; m.any(); m = m.rest() {
		q := m.first()
		for r := roles.without(q); r.any(); r = r.rest() {
			o := r.first()
			if s.perms[o].attached != s.perms[q].attached || s.perms[o].ctrl != s.perms[q].ctrl {
				continue
			}
			// q loses to o when o is newer; equal stamps keep the Role later
			// in the walk, as gorge's >= does
			if s.lstamp[o] > s.lstamp[q] || s.lstamp[o] == s.lstamp[q] && pos[o] > pos[q] {
				lose = lose.with(q)
				break
			}
		}
	}
	// gorge groups losers per bearer: bearers in order of their first Role in
	// the walk, then that bearer's losers
	var done slotMask
	for i := 0; i < n; i++ {
		b := s.perms[ord[i]].attached
		if done.has(b) {
			continue
		}
		done = done.with(b)
		for _, q := range ord[:n] {
			if lose.has(q) && s.perms[q].attached == b {
				out[cnt] = q
				cnt++
			}
		}
	}
	return
}
