package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// actGrantCensus is the census an activation's mana is priced against: av,
// rebuilt for the activation's own payment when a source holds granted mana
// abilities its restriction refuses some of and admits others. A restricted grant the activation may not spend is no
// unit of it (CR 106.6; gorge's planner pool for an activation holds the
// abilities RestrictValidMatches admits for it). Marvin, Murderous Mimic beside
// Overgrown Zealot gains the Zealot's unrestricted {T}: Add one mana of any
// color and its "spend this mana only to turn permanents face up" two-mana
// one; the shared census, built with no payment, joined the two into one
// two-mana unit, so Marvin read as two mana and paid an Equip {3} that gorge's
// two-mana census cannot.
func (s *State) actGrantCensus(k *prep, sl Slot, av *avail, out *avail) *avail {
	if k.gen == nil || av.xdOn || !restrGrantTable(k.gen.grantRestr) {
		return av
	}
	d := payDesc{class: pdAct, card: s.cardAt(sl), src: sl}
	if !s.mixedGrantLive(k, s.perms[sl].ctrl, &d) {
		return av
	}
	// av is only availFor's read when nothing was added to it after (a tap-X
	// source, a filter's units, a rider): the rebuilt census would drop those
	*out = *av
	s.availFor(k, s.perms[sl].ctrl, out)
	if !sameCensus(out, av) {
		return av
	}
	*out = *av
	out.xd, out.xdOn = d, true
	s.availFor(k, s.perms[sl].ctrl, out)
	out.xdOn = false
	if out.total >= av.total {
		// no refused grant took a unit out: the shared census stands (a read
		// the payment keys that only adds units is another census)
		return av
	}
	return out
}

// restrGrantTable reports whether some granted mana ability carries a spend
// restriction.
func restrGrantTable(r []kop.FilterID) bool {
	for _, f := range r {
		if f != 0 {
			return true
		}
	}
	return false
}

// sameCensus reports whether a and b hold the same units.
func sameCensus(a, b *avail) bool {
	if a.total != b.total || a.n != b.n || a.nOne != b.nOne || a.nModal != b.nModal || a.pain != b.pain {
		return false
	}
	for i := 0; i < a.n; i++ {
		if a.mask[i] != b.mask[i] || a.cnt[i] != b.cnt[i] {
			return false
		}
	}
	for i := 0; i < int(a.nOne); i++ {
		if a.one[i] != b.one[i] || a.oneMask[i] != b.oneMask[i] || a.oneAltMask[i] != b.oneAltMask[i] || a.oneAltN[i] != b.oneAltN[i] {
			return false
		}
	}
	return true
}

// mixedGrantLive reports whether a source p controls holds several granted
// mana abilities of which d's restriction refuses some and admits others: the
// shared census joins them into one unit (availGrantJoin), so only such a
// source reads wrong for d. A source whose every grant is refused, or whose
// only grant is, keeps the shared read (Rathi Intimidator's Food token).
func (s *State) mixedGrantLive(k *prep, p uint8, d *payDesc) bool {
	var gi [maxGrants]int16
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if n := s.grantIdxs(k, sl, &gi); n > 1 {
			if adm := s.grantsAdmitted(k, sl, &gi, n, d); adm > 0 && adm < n {
				return true
			}
		}
	}
	return false
}
