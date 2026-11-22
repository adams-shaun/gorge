package dirac

// maxFiltPermSet is the largest filter set whose activation orders filtMaxX
// tries in full: 4! = 24 orders. A larger set keeps the slot order alone.
const maxFiltPermSet = 4

// activateOrders is the largest X any way of activating the filters of set
// (a bit set over filt[:n]) leaves, over every order the set's filters can be
// activated in. The fee of a filter is paid from the mana made so far (CR
// 605.3b), so the order decides which units a fee takes and which colours
// are left for the cost: Desolate Mire's {1},{T}: {W}{B} first and Mystic
// Skull's {1},{T}: any colour after it leaves {R} and {B} of one floating {R}
// (Enrage's {X}{R} at X = 1), while the Skull first spends the {R} and the
// Mire's fee then takes the Skull's mana. gorge's castWindowPays searches the
// window for a funding order, so the walk's greedy slot order alone priced
// one X short. The slot order is the first one tried, so the result is never
// below the one-order walk's.
func (w *filtWalk) activateOrders(t avail, set int, filt *[maxFaceFilters]Slot, n int) int {
	var ord [maxFaceFilters]Slot
	m := 0
	for i := 0; i < n; i++ {
		if set>>i&1 != 0 {
			ord[m] = filt[i]
			m++
		}
	}
	// a copy of the walk reads ord: storing &ord through w would move it to the heap
	wk := *w
	wk.filt = &ord
	full := 1<<m - 1
	best := wk.activate(t, filtPos{set: full})
	if m > 1 && m <= maxFiltPermSet {
		// Heap's algorithm over ord[:m], the identity order already run
		var c [maxFiltPermSet]int
		for i := 0; i < m; {
			if c[i] < i {
				if i&1 == 0 {
					ord[0], ord[i] = ord[i], ord[0]
				} else {
					ord[c[i]], ord[i] = ord[i], ord[c[i]]
				}
				best = max(best, wk.activate(t, filtPos{set: full}))
				c[i]++
				i = 0
			} else {
				c[i] = 0
				i++
			}
		}
	}
	return best
}
