package dirac

import (
	"sort"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// landTypeWords are the land types of CR 205.3i, gorge's chooseLandTypes
// (engine/effects/type_choices.go): the subtypes a CR 305.7 setter drops.
var landTypeWords = map[string]bool{
	"Forest": true, "Island": true, "Mountain": true, "Plains": true, "Swamp": true,
	"Cave": true, "Desert": true, "Gate": true, "Lair": true, "Locus": true, "Mine": true, "Omenpath": true,
	"Planet": true, "Power-Plant": true, "Sphere": true, "Tower": true, "Town": true, "Urza's": true,
}

// LandTypeWords is the list, sorted, for the test that holds it to gorge's
// vocabulary.
func LandTypeWords() []string {
	out := make([]string, 0, len(landTypeWords))
	for w := range landTypeWords {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// A CR 305.7 land-subtype setter (kop.Override.LandSet; Dream Thrush's
// RemoveLandTypes$ Animate): the land's land types become the chosen basic
// land type (AddSub after the land types are dropped, layer 4), and it loses every
// ability its rules text generates (LoseAbilities), but not the intrinsic mana
// ability of the type it was set to (CR 305.6). gorge's loss is a layer-4
// effect, so the land keeps abilities another effect granted; a loss by any
// other effect (a real "loses all abilities") takes the intrinsic ability too,
// whatever its timestamp (static_landtype.go blgMask).
//
// The kernel's part is the planner view: manaPCGen answers landSetView, the
// card's plain-land facts with only the intrinsic colours (genPrep.blgIPC),
// while every ability reader sees the loss through genLost. A land whose own
// abilities are not the blgPlain shape stalls StallLandGrant, as a granted
// land type does.

// DebugLandSetLosesMana (TestDreamThrushMutation) takes the intrinsic mana
// ability away with the printed ones, as any other loss of abilities does.
var DebugLandSetLosesMana bool

// lossTs is the timestamp o's loss of abilities takes in layer 6: the record's,
// but a CR 305.7 setter's is zero. gorge's loss is part of the layer-4 effect,
// before any layer-6 grant, so no grant is older than it (abilityloss.go
// abilityLoss: lost with a zero timestamp), and it removes no keyword an
// earlier effect granted (loseAllOnset is not run for it).
func lossTs(o *kop.Override, ts uint16) uint16 {
	if o.LandSet {
		return 0
	}
	return ts
}

// landSetOnly reports whether sl lost its abilities to CR 305.7 setter
// records alone: it has one and no other effect removes its abilities. A loss
// from an Aura's lose-all static stalls StallLandGrant (which loss applies
// would need the layer walk's per-static verdicts).
func (s *State) landSetOnly(k *prep, sl Slot) bool {
	g := k.gen
	if DebugLandSetLosesMana || !s.fxM.has(sl) || !s.genLost(k, sl) {
		return false
	}
	if p := &s.perms[sl]; p.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || p.gty&gtyFaceDown != 0 {
		return false
	}
	set := false
	for i, c := 0, 0; i < int(s.g.nfx) && c < maxOvr; i++ {
		r := &s.g.fx[i]
		if r.sl != sl || r.kind != fxOverride {
			continue
		}
		c++
		if o := &k.t.Overrides[r.r.idx-1]; o.LoseAbilities {
			if !o.LandSet {
				return false
			}
			set = true
		}
	}
	if set && g.anyLoseStat {
		s.capFull(StallLandGrant)
		return false
	}
	return set
}

// landSetView is the planner view of a land set to a basic land type: the
// intrinsic ability of each basic type it now has, the card's printed one gone.
//
//go:noinline
func (s *State) landSetView(k *prep, sl Slot, c CardID) *pcard {
	var m kop.ColorMask
	if s.typesAt(k, sl)&kop.TLand != 0 {
		for col, i := range k.gen.blgSub {
			if i != subOutside && s.hasSubGen(k, sl, i) {
				m |= 1 << col
			}
		}
	}
	if m == 0 {
		return &noManaPC
	}
	if k.gen.blgIdx[c] < 0 {
		s.capFull(StallLandGrant)
		return &noManaPC
	}
	return &k.gen.blgIPC[int(k.gen.blgIdx[c])*32+int(m)]
}
