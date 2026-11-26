package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Granted toxic N (CR 702.164; gorge ToxicValue sums every Toxic:<N> entry of
// the object's derived keywords, and 702.164c makes the instances
// cumulative). The printed N is CardDef.Toxic; toxicAt adds every grant in
// force, read beside it, so combat damage poisons for the total. A grant
// comes three ways, none adding a State field:
//
//   - a Continuous static's kop.Static.Toxic (AddKeyword$ Toxic:N: Karumonix's
//     Rats, Necrogen Communion, Prosthetic Injector), a layer-6 grant: one
//     older than a loss of the recipient's abilities grants nothing, and each
//     live static adds its own N;
//   - an override record's kop.Override.Toxic (an Animate's Keywords$ Toxic:N:
//     Monument to Perfection), read under the same layer-6 rule;
//   - OpExt ExtKwMark KwMarkToxic with the amount in B (a Pump / PumpAll KW$
//     Toxic:N until end of turn: Aspirant's Ascent, Plague Nurse): a count in
//     perm.spare's three toxic bits, cleared in the cleanup step. A mark made
//     while the permanent had lost its abilities (the later grant that
//     outlives the loss, CR 613.7) or past the count's seven stalls
//     (StallToxicGrant): a declared limit, never a dropped grant.
//
// All behind genPrep.anyToxicGrant. DebugNoGrantedToxic (the mutation switch)
// makes the read ignore every grant.

const (
	spareToxicShift       = 5
	spareToxic      uint8 = 7 << spareToxicShift // perm.spare's granted-toxic count (bits 5-7)
)

// StallToxicGrant: an until-end-of-turn toxic grant that the three spare bits
// cannot hold exactly, a count past seven or a grant to a permanent that has
// lost its abilities (gorge sums each grant by its layer-6 timestamp).
const StallToxicGrant = 82

func init() { stallNames[StallToxicGrant] = "toxicgrant" }

// DebugNoGrantedToxic (TestGrantedToxicMutation): toxicAt ignores every granted
// toxic, so a poison counter a grant owed is never placed.
var DebugNoGrantedToxic bool

// ToxicStats counts the poison counters granted toxic placed in a lockstep
// game (DebugToxicStats; TestGrantedToxicLockstep reads it so a green game is
// one where a grant decided the poison; nothing in the engine reads it).
type ToxicStats struct {
	GrantedPoison int64 // poison counters from granted toxic (the total less the printed N)
}

// DebugToxicStats, when set, receives the counts.
var DebugToxicStats *ToxicStats

// tableGrantsToxic reports whether some part of t grants toxic.
func tableGrantsToxic(t *kop.Table) bool {
	scan := func(abs []kop.Ability) bool {
		for j := range abs {
			a := &abs[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Toxic != 0 {
				return true
			}
			for i := range a.Prog {
				if in := &a.Prog[i]; in.Op == kop.OpExt && in.Ext() == kop.ExtKwMark && in.C&kop.KwMarkToxic != 0 {
					return true
				}
			}
		}
		return false
	}
	for i := range t.Cards {
		if scan(t.Cards[i].Abilities) {
			return true
		}
	}
	for i := range t.Emblems {
		if scan(t.Emblems[i].Abilities) {
			return true
		}
	}
	for i := range t.Overrides {
		if t.Overrides[i].Toxic != 0 {
			return true
		}
	}
	return false
}

// toxicMark is ExtKwMark's toxic grant: n more toxic on the permanent at sl
// until the cleanup step.
func (s *State) toxicMark(k *prep, sl Slot, n int) {
	if n <= 0 {
		return
	}
	p := &s.perms[sl]
	c := int(p.spare&spareToxic>>spareToxicShift) + n
	if c > int(spareToxic>>spareToxicShift) || s.genLost(k, sl) {
		s.capFull(StallToxicGrant)
		return
	}
	p.spare = p.spare&^spareToxic | uint8(c)<<spareToxicShift
	s.touchSl(sl)
}

// toxicGranted is the toxic N the permanent at sl is granted: the marks (not
// while it has lost its abilities), every live static's and every live
// override record's, the last two not when older than the loss.
func (s *State) toxicGranted(k *prep, sl Slot) int {
	lts, lost := s.lossAt(k, sl)
	n := 0
	if !lost {
		n = int(s.perms[sl].spare & spareToxic >> spareToxicShift)
	}
	s.eachStatic(k, func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatContinuous || ab.Static.Toxic == 0 ||
			lost && s.grantTs(st) < lts {
			return true
		}
		if s.statCond(k, st) && s.matchPermGen(k, ab.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			n += int(ab.Static.Toxic)
		}
		return true
	})
	if s.fxM.has(sl) {
		s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
			if o.Toxic != 0 && (!lost || ts >= lts) {
				n += int(o.Toxic)
			}
		})
	}
	return n
}
