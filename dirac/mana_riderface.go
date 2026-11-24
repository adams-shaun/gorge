package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A source that prints a plain one-mana {T} ability before a free bare-tap
// rider ability (Thalakos Lowlands: "{T}: Add {C}" then "{T}: Add {W} or {U}.
// It doesn't untap during your next untap step"; Grove of the Burnwillows) is
// one unit of the planner census at its plain ability's production. CR
// 601.2g's window can tap it for either ability, and gorge's xAsk reads that
// window (castWindowUnits), so the announcement prices the rider's colours as
// the other face of the same one tap: a Worldsoul Colossus whose convoke pick
// paid {G} owes {X}{W}, and the Lowlands' {W} is what leaves X above 0.

// riderFaceAlt reports the mask of the colours the rider ability of the
// plain-first source at sl adds to the window's census as an alternative of
// its plain tap: a bare-tap rider of one mana of a finite colour choice.
func (s *State) riderFaceAlt(k *prep, p uint8, sl Slot, ra *kop.Ability) (uint8, bool) {
	if !s.riderPlainFirst(k, sl) || ra.ManaEach || ra.ManaN != 1 || !riderCensusFree(ra) ||
		ra.ManaColors == 0 || ra.ManaColors == kop.ColAny || ra.ManaRider == kop.ManaRiderDeplete ||
		!s.riderActivatable(k, p, sl, ra) {
		return 0, false
	}
	alt := uint8(ra.ManaColors) & 0x3f
	return alt, alt != 0
}

// addRiderFace turns the plain-first source at sl, already counted as one unit
// of its plain production, into a two-faced unit of a's census: the plain
// {first} or one mana of alt (the exclAffords walk picks the face). It reports
// whether it did; the census is untouched when it did not.
func (a *avail) addRiderFace(first, alt uint8) bool {
	e := altExcl{first: first, own: 1, alt: alt, cnt: 1}
	for i := 0; i < int(a.nExcl); i++ {
		if x := &a.excl[i]; *x == (altExcl{first: first, own: 1, alt: alt, cnt: x.cnt}) {
			if !a.takeWide(first, 1) {
				return false
			}
			x.cnt++
			a.total++
			a.xMax++
			return true
		}
	}
	if int(a.nExcl) == len(a.excl) || !a.takeWide(first, 1) {
		return false
	}
	a.excl[a.nExcl] = e
	a.nExcl++
	a.total++
	a.xMax++
	return true
}
