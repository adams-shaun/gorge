package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// gainedIntrUnit is the mana of sl's gained CR 305.6 intrinsic abilities: the
// colours of every unrestricted, tap-able intrinsic basic-land ability a
// GainsAbilitiesOf$ static gives it (Marvin, Murderous Mimic beside an
// animated Forest and Island), and the largest amount. gorge's planner holds
// each such gained ability as a real alternative of the recipient
// (pay.PaymentAbility's intrinsic branch), so the source is a creature unit
// of that many colours, flexible over exactly them, where a static grant or a
// gained authored ability leaves it a classless position (planInertGrant).
// mask 0: the source has none.
func (s *State) gainedIntrUnit(k *prep, sl Slot) (mask, n uint8) {
	g := k.gen
	if g == nil || len(g.grantPC) == 0 || !g.uses.has(useGainedMana) {
		return 0, 0
	}
	var gi [maxGrants]int16
	ng := s.grantIdxs(k, sl, &gi)
	for i := 0; i < ng; i++ {
		j := gi[i]
		r := g.grantOf[j]
		a := &k.t.Cards[r.card].Abilities[r.ab]
		if g.grantRestr[j] != 0 || a.AFlags&kop.AfLatent != 0 || !gainedManaPlanned(&k.t.Cards[r.card], a) ||
			!gainedManaIntrinsic(&k.t.Cards[r.card], a, k.t.Subtypes) {
			continue
		}
		gp := s.grantView(g, j, sl)
		if !s.canTapForManaPC(k, sl, gp) {
			continue // the {T} cost: a tapped, lost or once-blocked source makes nothing (CR 302.6)
		}
		mask |= s.srcMaskAt(gp, sl)
		n = max(n, s.srcYieldAt(k, sl, gp))
	}
	return mask, n
}

// planGainedIntr registers, in the pass its shape belongs to (a fixed one
// colour unit before the choice-shaped ones, as plan's loop orders them), the
// real unit of a source whose gained intrinsic abilities gorge's planner
// lists: its colours, its flexibility the count of them (pay Alt.Flex over
// every eligible alternative), a creature when the source is one. It reports
// whether sl was such a source, whatever pass this is.
func (s *State) planGainedIntr(k *prep, pk *planner, i int, sl Slot, pass int) bool {
	mask, n := s.gainedIntrUnit(k, sl)
	if mask == 0 || n == 0 {
		return false
	}
	choice := bits.OnesCount8(mask) > 1
	if pass == 2 || (pass == 0) == choice || pk.nu >= maxUnits {
		return true
	}
	cr := s.isCreature(k, sl)
	fl := uint8(bits.OnesCount8(mask))
	ti := -1
	for t := 0; t < pk.nt; t++ {
		ty := &pk.types[t]
		if ty.mask == mask && ty.dmask == mask && ty.n == n && ty.creat == cr && !ty.sac && !ty.sacOnly &&
			ty.pair == 0 && ty.pain == 0 && ty.first == 0 && ty.printed == 0 && ty.flex == fl &&
			ty.xv == ([kop.NumManaKinds]uint8{}) && !ty.rb && !ty.inert {
			ti = t
			break
		}
	}
	if ti < 0 {
		if pk.nt == maxTypes {
			return true // no room for another shape (no pool board reaches it)
		}
		ti = pk.nt
		pk.nt++
		pk.types[ti] = utype{mask: mask, dmask: mask, n: n, creat: cr, flex: fl}
		if int(n) > pk.maxN {
			pk.maxN = int(n)
		}
	}
	pk.types[ti].avail++
	pk.units[pk.nu] = sl
	if pk.bfl {
		pk.uobj[pk.nu] = Obj(i) // battlefield order (plan's hybrid census)
	} else {
		pk.uobj[pk.nu] = s.perms[sl].obj
	}
	pk.utyp[pk.nu] = uint8(ti)
	pk.rkind[pk.nu] = 0
	pk.rAb[pk.nu] = -1 // the gained ability's number names nothing on sl's own card (planGrantUnit's)
	pk.nu++
	for c := 0; c < 5; c++ {
		if mask&(1<<c) != 0 {
			pk.colSrc[c]++
		}
	}
	return true
}
