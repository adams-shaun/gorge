package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// an Effect-delivered MayPlayLimit$ grant counts per GRANT
// and per TURN (gorge, engine/rules/mayplay.go mayPlayCapOpen /
// mayPlayEffectKey / mayPlayKeyLimitReached: the plays stamped with the
// grant's own key, Source + registration Timestamp, since the last
// TurnChange), not per player. The records one OpMayPlay resolution appends
// are one grant. They sit next to each other in the pool, and the first of
// them carries mayUntilHead in its until byte (Dur* values are below 16);
// the grant is that record and the non-head limited records after it. Every
// record of the grant counts the plays it has taken this turn in the
// mayUntilUsed bits; the cap stays in the kind's MayPlayLimit bits. The
// count resets when a record outlives the turn (cleanupGen). 0 B: the bits
// ride until.
const (
	mayUntilHead = 0x80 // first record of its limited grant
	mayUntilUsed = 0x70 // plays the grant has taken this turn (0..7)
	mayUntilOne  = 0x10
	mayUntilDur  = 0x0F // the Dur* value
)

// dur is the record's Dur* value without the grant bits.
func (r mayPlay) dur() uint8 { return r.until & mayUntilDur }

// grantUntil is the until byte of a new record: u as the op names it, with
// the head bit when kind carries a MayPlayLimit cap and this is the first
// such record of the resolution (*headed).
func grantUntil(u, kind uint8, headed *bool) uint8 {
	if kind&kop.MayPlayLimitMask != 0 && !*headed {
		*headed = true
		return u | mayUntilHead
	}
	return u
}

// spendGrant takes one use of the limited grant record i belongs to: the
// used count of every record of the grant rises by one. A record with no
// cap is untouched.
//
//go:noinline
func (s *State) spendGrant(i int) {
	if s.mayAt(i).kind&kop.MayPlayLimitMask == 0 {
		return
	}
	lo := i
	for lo > 0 && s.mayAt(lo).until&mayUntilHead == 0 && s.mayAt(lo-1).kind&kop.MayPlayLimitMask != 0 {
		lo--
	}
	hi := i
	for hi+1 < int(s.g.nmay) && s.mayAt(hi+1).until&mayUntilHead == 0 && s.mayAt(hi+1).kind&kop.MayPlayLimitMask != 0 {
		hi++
	}
	// only limited records carry the count: a record with no cap compares
	// its until byte exactly (Warp, Plot, Prepared), so it is never raised
	for j := lo; j <= hi; j++ {
		if m := s.mayAt(j); m.kind&kop.MayPlayLimitMask != 0 && m.until&mayUntilUsed != mayUntilUsed {
			m.until += mayUntilOne
		}
	}
}

// removeMay spends record i (a cast or land play used it): its grant loses
// a use, the record leaves the pool, and a grant head passes its mark on to
// the next record of the same grant.
func (s *State) removeMay(i int) {
	if s.mayAt(i).kind&kop.MayPlayLimitMask != 0 {
		s.spendGrant(i)
		if s.mayAt(i).until&mayUntilHead != 0 && i+1 < int(s.g.nmay) {
			if nx := s.mayAt(i + 1); nx.until&mayUntilHead == 0 && nx.kind&kop.MayPlayLimitMask != 0 {
				nx.until |= mayUntilHead
			}
		}
	}
	s.mayShiftDown(i)
	s.g.nmay--
}

// damageDealt is execDamageEx's tail for an object the damage actually
// reached (gorge, effects/damage_deal.go: RememberDamaged$ and the
// Remembered selector of ReplaceDyingDefined$ name only recipients dealt
// damage above 0 after prevention and replacement, CR 120.1).
//
//go:noinline
func (s *State) damageDealt(k *prep, in *kop.Instr, o Obj) {
	if in.C&kop.DmgRemember != 0 {
		s.remember(o)
	}
	if in.C&kop.DmgExileIfDealt != 0 && s.objs[o].zone == kop.ZoneBattlefield {
		s.markExileEx(k, s.objs[o].slot)
	}
	if in.C&kop.DmgNoRegen != 0 {
		s.markNoRegen(k, o) // repl_noregen.go
	}
}
