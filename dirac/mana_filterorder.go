package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// faceReachOrders is faceReachFilters' walk of one set of filters: each order
// of the set's members is tried, not only battlefield order. A filter's fee is
// paid out of the mana the window has made so far (CR 605.3b), so the order
// decides what each fee eats: Bog Initiate's {1}: {B} before Overflowing
// Basin's {1},{T}: {G}{U} spends its {B} on the Basin's fee, after it the Basin's
// {G} pays the Bog's fee and the {B} stays. gorge's PotentialMana fixpoint
// reaches the mana of any order, so a face one order pays is reachable
// (Dismember's {B/P} announced beside a lone Plains, Basin and Bog).
//
//go:noinline
func (s *State) faceReachOrders(k *prep, p uint8, r *kop.ManaCost, extra int, base avail, filt *[maxFaceFilters]Slot, n int, set uint32) bool {
	var mem [maxFaceFilters]Slot
	m := 0
	for i := 0; i < n; i++ {
		if set&(1<<i) != 0 {
			mem[m] = filt[i]
			m++
		}
	}
	var idx [maxFaceFilters]uint8
	for i := range idx {
		idx[i] = uint8(i)
	}
	for {
		var ord [maxFaceFilters]Slot
		for i := 0; i < m; i++ {
			ord[i] = mem[idx[i]]
		}
		if s.faceReachSet(k, p, r, extra, base, &ord, 0, m, 1<<m-1) {
			return true
		}
		if !nextOrder(idx[:m]) {
			return false
		}
	}
}

// nextOrder steps idx to the next order in lexicographic order, reporting
// false once idx was the last.
func nextOrder(idx []uint8) bool {
	i := len(idx) - 2
	for i >= 0 && idx[i] >= idx[i+1] {
		i--
	}
	if i < 0 {
		return false
	}
	j := len(idx) - 1
	for idx[j] <= idx[i] {
		j--
	}
	idx[i], idx[j] = idx[j], idx[i]
	for l, r := i+1, len(idx)-1; l < r; l, r = l+1, r-1 {
		idx[l], idx[r] = idx[r], idx[l]
	}
	return true
}
