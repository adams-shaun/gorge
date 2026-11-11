package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Granted ward (CR 702.21a-b): a Continuous static's AddKeyword$
// Ward:N or Ward:PayLife<N> -- an Aura's "enchanted creature has ward {2}"
// (Sheltered by Ghosts), an anthem's "each other Human you control has ward
// {1}" (Coppercoat Vanguard), Hexing Squelcher's "other creatures you control
// have ward--pay 2 life". Each instance is its own triggered ability (CR
// 113.2c), so a permanent the targeted spell or ability's controller does
// not control triggers its printed ward and then one ward per static
// granting it, each with that static's amount. gorge synthesises a granted
// ward from the derived keyword list (rules checkGrantedWardTriggers) after
// the face's own, skipping a granted keyword equal to a printed one (its
// printed map: two "Ward:2" strings are one keyword there).

// wardGrantedBits mark a ward trigger amount (stackEntry.trigAmt) as a granted
// ward's: gorge's trigger carries no face trigger index, a printed one its own.
// A granted {N} ward is N alone and no printed form sets both bits.
const wardGrantedBits = wardPowBit | wardDiscBit

// wardGrantedLifeBit marks a granted ward--pay N life. The evidence bit beside
// wardGrantedBits is unused by any printed form, so the amount carries
// N | wardGrantedBits | wardGrantedLifeBit with N in the low byte; the
// resolved ask is the printed -N one (resolveWardLife).
const wardGrantedLifeBit = wardEvBit

// wardIsGranted reports whether a ward trigger amount carries wardGrantedBits. A
// printed life ward's amount is negative and has no grant bits.
func wardIsGranted(amt int16) bool {
	return amt > 0 && amt&wardGrantedBits == wardGrantedBits
}

// wardIsGrantedLife reports whether a ward trigger amount is a granted
// ward--pay N life.
func wardIsGrantedLife(amt int16) bool {
	return amt > 0 && amt&(wardGrantedBits|wardGrantedLifeBit) == wardGrantedBits|wardGrantedLifeBit
}

// wardGranted enqueues the ward triggers of the permanent at sl (object
// ob) that entry e targeted, when the table has a granted ward.
//
//go:noinline
func (s *State) wardGranted(k *prep, sl Slot, ob *object, e *stackEntry, disguise bool) {
	p := &s.perms[sl]
	cd := &k.t.Cards[s.cardAt(sl)]
	var printed int16
	hasPrinted := false
	if disguise {
		hasPrinted = true
		printed = 2 // CR 702.168b: a face-down Disguise permanent has ward {2}
	} else if cd.Keywords&kop.KwWard != 0 && !s.genLost(k, sl) {
		hasPrinted = true
		printed = wardAmt(cd) // negative: pay N life
	}
	if hasPrinted && !(DebugDropLifeWard && printed < 0) {
		np := s.npend
		s.enqueueWard(e, p.obj, ob.ver, s.cardAt(sl), p.ctrl, e.ctrl, printed)
		s.panharmKeyword(k, sl, kop.TrigBecomesTarget, e.obj, np)
	}
	lts, lost := s.lossAt(k, sl)
	// the granted instances in gorge's derived keyword order: the layer-6
	// application order, the granting statics' timestamps
	var amt [8]int16
	var ts [8]uint16
	n := 0
	s.eachStaticC(k, sclsKw, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous ||
			a.Static.Ward == 0 && a.Static.WardLife == 0 {
			return true
		}
		// cost is the amount a printed keyword of the same text would carry
		// (ward {N} positive, ward--pay N life negative); granted is the
		// trigger amount the grant's own instance queues.
		var cost, granted int16
		if a.Static.WardLife != 0 {
			cost = -int16(a.Static.WardLife)
			granted = int16(a.Static.WardLife) | wardGrantedBits | wardGrantedLifeBit
		} else {
			cost = int16(a.Static.Ward)
			granted = cost | wardGrantedBits
		}
		if lost && s.grantTs(st) < lts {
			return true
		}
		if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			return true
		}
		if cost == printed && hasPrinted && !disguise {
			return true // gorge's printed keyword list already holds this "Ward:..."
		}
		if n == len(amt) {
			return false
		}
		t := s.grantTs(st)
		j := n
		for j > 0 && ts[j-1] > t {
			amt[j], ts[j] = amt[j-1], ts[j-1]
			j--
		}
		amt[j], ts[j] = granted, t
		n++
		return true
	})
	if DebugNoWardGrant {
		n = 0
	}
	for i := 0; i < n; i++ {
		np := s.npend
		s.enqueueWard(e, p.obj, ob.ver, s.cardAt(sl), p.ctrl, e.ctrl, amt[i])
		s.panharmKeyword(k, sl, kop.TrigBecomesTarget, e.obj, np)
	}
}

// DebugNoWardGrant drops the granted ward triggers. The ward lockstep's
// mutation test flips it to prove the pool exercises them.
var DebugNoWardGrant bool

// DebugDropLifeWard drops the printed "ward--pay N life" trigger (the
// pre-fix behaviour). The ward life lockstep's mutation test flips it.
var DebugDropLifeWard bool
