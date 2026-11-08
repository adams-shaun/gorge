package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Granted ward (CR 702.21a-b): a Continuous static's AddKeyword$
// Ward:N -- an Aura's "enchanted creature has ward {2}" (Sheltered by
// Ghosts), an anthem's "each other Human you control has ward {1}"
// (Coppercoat Vanguard). Each instance is its own triggered ability (CR
// 113.2c), so a permanent the targeted spell or ability's controller does
// not control triggers its printed ward and then one ward per static
// granting it, each with that static's amount. gorge synthesises a granted
// ward from the derived keyword list (rules checkGrantedWardTriggers) after
// the face's own, skipping a granted keyword equal to a printed one (its
// printed map: two "Ward:2" strings are one keyword there).

// wardGrantedBits mark a ward trigger amount (stackEntry.trigAmt) as a granted
// ward's: gorge's trigger carries no face trigger index, a printed one its own.
// A granted ward is {N} alone and no printed form sets both bits.
const wardGrantedBits = wardPowBit | wardDiscBit

// wardIsGranted reports whether a ward trigger amount carries wardGrantedBits. A
// life ward's amount is negative, its upper bits set.
func wardIsGranted(amt int16) bool {
	return amt > 0 && amt&wardGrantedBits == wardGrantedBits
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
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Ward == 0 {
			return true
		}
		if lost && s.grantTs(st) < lts {
			return true
		}
		if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			return true
		}
		if int16(a.Static.Ward) == printed && hasPrinted && !disguise {
			return true // gorge's printed keyword list already holds this "Ward:N"
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
		amt[j], ts[j] = int16(a.Static.Ward), t
		n++
		return true
	})
	if DebugNoWardGrant {
		n = 0
	}
	for i := 0; i < n; i++ {
		np := s.npend
		s.enqueueWard(e, p.obj, ob.ver, s.cardAt(sl), p.ctrl, e.ctrl, amt[i]|wardGrantedBits)
		s.panharmKeyword(k, sl, kop.TrigBecomesTarget, e.obj, np)
	}
}

// DebugNoWardGrant drops the granted ward triggers. The ward lockstep's
// mutation test flips it to prove the pool exercises them.
var DebugNoWardGrant bool

// DebugDropLifeWard drops the printed "ward--pay N life" trigger (the
// pre-fix behaviour). The ward life lockstep's mutation test flips it.
var DebugDropLifeWard bool
