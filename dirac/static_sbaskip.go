package dirac

import "unsafe"

// The clean-scan skip of sba's per-permanent scan (sbaScan).
//
// sba runs about five times per transition and rescans every permanent each
// pass, though most passes follow a pass that found nothing and nothing has
// changed since. A scan that found no casualty, changed nothing and read
// nothing outside its key is recorded (sbaOK, sbaT, g.sbaFP); the next scan
// under the same key would find the same, so it is skipped.
//
// The key is the touch count (marks.go): every write to an object or to the
// battlefield slot it occupies, which is every counter, damage, flag,
// override, record, attachment and zone move the scan reads, and
// TestMarksCoverEveryMutation checks the marks cover them. Under a static
// that can read more (sbaBoardObjectOnly: a Dynamic or conditional amount, a
// CDA, an override, a grant, a live-valued override record) the key adds
// sbaFingerprint: the turn, step and active seat, each seat's life, lost flag
// and turn tallies, and the generated state's turn counts.
//
// A table with an emblem never skips (an emblem is a static outside the
// objects). The scan reports busy when it read something no key holds (a
// lethal-damage permanent's replacements, the legend ask, an Aura's
// legality): that scan is never recorded.
//
// The grain is the whole board, not a dirty-slot mask: a toughness reads the
// statics of other permanents, so under any static a slot is only as clean
// as the board, and about 70% of passes follow a clean one with nothing
// written since (FDN pool, 400k steps).

// sbaVerify makes every skippable scan run anyway and count, in sbaHits, the
// ones the skip would have taken and, in sbaBad, the ones it would have got
// wrong (static_sbaskip_test.go). sbaMutate breaks the skip so a test proves
// the key is load-bearing: 1 ignores the touch count, 2 the fingerprint,
// 3 never skips (the reference play).
var (
	sbaVerify bool
	sbaHits   int
	sbaBad    int
	sbaMutate uint8
)

// The recorded key's width: sbaKeyTouch when every static on the board
// reads only objects' own fields (sbaBoardObjectOnly), so the touch count is
// the whole key; sbaKeyPrint when one may read more.
const (
	sbaKeyTouch uint8 = 1 + iota
	sbaKeyPrint
)

// sbaBoardObjectOnly reports whether no static in force on the battlefield
// and no override record can read what is not an object's own fields: a Dynamic, conditional, CDA,
// override or granting one makes a derived toughness or type depend on the
// turn, the life totals and the tallies (sbaFingerprint).
func (s *State) sbaBoardObjectOnly(k *prep) bool {
	if k.gen == nil || s.fxM.any() {
		return false // an override record can hold a live value (livePT), read from the state at each P/T
	}
	for m := s.statM; m.any(); m = m.rest() {
		if k.gen.sCls[s.cardAt(m.first())]&sclsNonObj != 0 {
			return false
		}
	}
	return true
}

// sbaBoard is the pass's board scan, skipped when the last clean scan's key
// still holds.
// It fills r in place (sbaScan); a skipped scan leaves r as an empty pass's.
func (s *State) sbaBoard(k *prep, r *sbaMasks) {
	if s.anyEmb() {
		s.sbaOK = 0
		s.sbaScan(k, r)
		return
	}
	t0 := s.touchN
	hit := s.sbaOK != 0 && (s.sbaT == t0 || sbaMutate == 1) && sbaMutate != 3
	if hit && s.sbaOK == sbaKeyPrint {
		hit = s.g.sbaFP == s.sbaFingerprint() || sbaMutate == 2
	}
	if hit && !sbaVerify {
		r.die, r.gy, r.pwz, r.aur, r.anni = slotMask{}, slotMask{}, slotMask{}, slotMask{}, slotMask{}
		r.umbra.n, r.umbra.victim = 0, slotMask{}
		return
	}
	if hit {
		sbaHits++ // verify mode only: a skipping scan returned above
	}
	busy := s.sbaScan(k, r)
	clean := !busy && s.touchN == t0 && !r.die.or(r.gy).or(r.pwz).or(r.aur).or(r.anni).any()
	if hit && !clean {
		sbaBad++
	}
	// a clean scan changed nothing, so the board and fingerprint after it are the ones it ran under
	switch {
	case !clean:
		s.sbaOK = 0
	case s.sbaBoardObjectOnly(k):
		s.sbaOK, s.sbaT = sbaKeyTouch, t0
	default:
		s.sbaOK, s.sbaT, s.g.sbaFP = sbaKeyPrint, t0, s.sbaFingerprint()
	}
	return
}

// sbaFingerprint is the 48-bit hash of the non-object half of the key:
// words packed by direct loads, so a pass that hits costs a few dozen
// instructions.
func (s *State) sbaFingerprint() (fp [6]byte) {
	g := &s.g
	h := hmix(0x7362616b6579, uint64(s.turn)|uint64(s.step)<<16|uint64(s.active)<<24|uint64(s.ts)<<32|uint64(g.died)<<48|uint64(g.oppAtt)<<56)
	h = hmix(h, uint64(g.nused)|uint64(g.crime)<<8|uint64(g.dayNight)<<16|uint64(g.atkHi)<<24|
		uint64(g.attacked[0])<<32|uint64(g.attacked[1])<<40|uint64(g.attacked[2])<<48|uint64(g.attacked[3])<<56)
	for p := uint8(0); p < s.nSeats(); p++ {
		pl := &s.pl[p]
		h = hmix(h, uint64(uint16(pl.life))|uint64(uint16(pl.lifeGained))<<16|uint64(uint16(pl.lifeLost))<<32|uint64(uint16(pl.lastLost))<<48)
		h = hmix(h, uint64(b2u(pl.lost))|uint64(pl.mark&pmDrewEmpty)<<8|uint64(pl.landsPlayed)<<16|uint64(pl.drawnTurn)<<24|uint64(pl.scry)<<32|
			uint64(pl.sac[0])<<40|uint64(pl.sac[1])<<48|uint64(pl.sac[2])<<56)
		h = hmix(h, uint64(pl.disc)|uint64(pl.mulls)<<8)
	}
	if g.spent != [MaxSeats]uint16{} {
		h = hbytes(h, unsafe.Pointer(&g.spent), unsafe.Sizeof(g.spent))
	}
	if g.hist != [MaxSeats][3]uint8{} {
		h = hbytes(h, unsafe.Pointer(&g.hist), unsafe.Sizeof(g.hist))
	}
	if g.on {
		// the banks a table keeps off the fixed state (marks.go genViewHash)
		for _, c := range [...][]uint8{s.contractCold(), s.atkLastCold(), s.experCold(), s.dDoneCold()} {
			if len(c) > 0 {
				h = hbytes(h, unsafe.Pointer(&c[0]), uintptr(len(c)))
			}
		}
		if c := s.radCold(); len(c) > 0 {
			h = hbytes(h, unsafe.Pointer(&c[0]), uintptr(len(c))*2)
		}
	}
	for i := range fp {
		fp[i] = byte(h >> (8 * i))
	}
	return fp
}
