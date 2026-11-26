package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// mostCardName is ExMostCardName (gorge count_ref.go countMostCardName,
// Forge's Count$MostCardName): the greatest number of battlefield objects
// matching F that share one face name, the oracle's "N or more <things> with
// the same name as one another" (Endless Atlas, Mechanized Production, Chrome
// Replicator, Sceptre of Eternal Glory). The scan is every seat's battlefield
// (gorge AliveFrom(0)) with the filter's controller-relative words read
// against the evaluating controller, as matchPermGen does. The name read is
// the face name's card (nameCardAt), the same read the distinct-names
// aggregate (AggDistinctNames, CR 201.2) makes of gorge's o.Face().Name. The
// per-name counts are two arrays indexed by first-seen order, so no map
// iteration order is ever read. A match beyond MaxPerm slots cannot happen
// (the scan is the occupancy loop) and nearCapacity stalls a battlefield at
// MaxPerm-4 before the state can hold one more. Cold, out of line.
//
//go:noinline
func (s *State) mostCardName(k *prep, x *kop.Expr, ctx evalCtx) int {
	var names [MaxPerm]CardID
	var cnt [MaxPerm]int
	n := 0
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if !s.matchPermGen(k, x.F, sl, ctx.ctrl, ctx.srcSl, layerFull) {
			continue
		}
		nc := s.nameCardAt(sl)
		i := 0
		for ; i < n; i++ {
			if names[i] == nc {
				cnt[i]++
				break
			}
		}
		if i == n {
			names[n], cnt[n] = nc, 1
			n++
		}
	}
	best := 0
	for i := 0; i < n; i++ {
		if cnt[i] > best {
			best = cnt[i]
		}
	}
	return best
}
