package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A source that left the battlefield deals its damage with the keywords it
// last had (CR 608.2h; 702.2b deathtouch, 702.15a lifelink). gorge keeps them
// for every ability of the departing object on the stack, pending or being
// paid (effects/damage.go newDamageRider, rules/engine_damage.go
// captureSourceLifelinkLKI); the kernel keeps the two that matter to a damage
// event on the entry's skw byte, set as the permanent leaves, and falls back
// to the triggering object's snapshot when that object is the source (a dies
// trigger).
const (
	skwValid uint8 = 1 << iota // the source's keywords were read as it left
	skwDT
	skwLL
)

// skwRem is a pending trigger's skw high bit: whether its delayed program's
// remembered object was captured (stackEntry.remSet). A pending trigger carries
// it here so the struct stays two bytes smaller than a bool beside skw would
// make it.
const skwRem uint8 = 1 << 7

// skwMany marks a pending trigger fired by a many-object delayed program
// (trig_delayedmany.go): takePending stamps the objects' versions on the
// entry, nothing being able to change one between firing and stacking.
const skwMany uint8 = 1 << 6

// boolBit is bit when b, else 0.
func boolBit(b bool, bit uint8) uint8 {
	if b {
		return bit
	}
	return 0
}

// settleSrcKw stamps the departing permanent o (slot sl) keywords on every
// ability entry it sourced: the stack, the pending triggers, the resolution
// in progress and the build in progress.
//
//go:noinline
func (s *State) settleSrcKw(k *prep, sl Slot, o Obj) {
	ver := s.objs[o].ver
	var skw uint8
	read := func() uint8 {
		if skw == 0 {
			kw := s.kw(k, sl)
			skw = skwValid
			if kw&kop.KwDeathtouch != 0 {
				skw |= skwDT
			}
			if kw&kop.KwLifelink != 0 {
				skw |= skwLL
			}
		}
		return skw
	}
	for i := 0; i < min(int(s.sp), MaxStack); i++ {
		if e := &s.stack[i]; e.kind != seSpell && e.obj == o && e.srcVer == ver {
			e.skw = read()
		}
	}
	for i := uint8(0); i < s.npend; i++ {
		if p := &s.pend[i]; p.obj == o && p.srcVer == ver {
			p.skw = p.skw&(skwRem|skwMany) | read()
		}
	}
	if e := &s.res.e; s.res.active && e.kind != seSpell && e.obj == o && e.srcVer == ver {
		e.skw = read()
	}
	if s.buildLive() {
		if e := &s.bld().e; e.kind != seSpell && e.obj == o && e.srcVer == ver {
			e.skw = read()
		}
	}
}

// srcLeftKw is the keywords of the resolving ability e whose source is not on
// the battlefield: kw (the card's printed ones) with deathtouch and lifelink
// replaced by what the source last had, when known.
func (s *State) srcLeftKw(kw kop.KwMask, e *stackEntry) kop.KwMask {
	var dt, ll, ok bool
	if e.skw&skwValid != 0 {
		dt, ll, ok = e.skw&skwDT != 0, e.skw&skwLL != 0, true
	} else if e.li != 0 {
		if l := s.lk(e.li); l.obj == e.obj && l.ver == e.srcVer {
			dt, ll, ok = l.kw&kop.KwDeathtouch != 0, l.kw&kop.KwLifelink != 0, true
		}
	}
	if !ok {
		return kw
	}
	kw &^= kop.KwDeathtouch | kop.KwLifelink
	if dt {
		kw |= kop.KwDeathtouch
	}
	if ll {
		kw |= kop.KwLifelink
	}
	return kw
}
