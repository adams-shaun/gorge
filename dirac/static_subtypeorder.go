package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugSubtypeReplaceLast is the mutation switch of
// TestLayer4SubtypeTimestampOrderMutation: a repeated layer-4 subtype
// addition merges into its older record again, keeping the older timestamp,
// so a later subtype replacement resolves after it
// (blk-a16-layer4-subtype-timestamp-order's bug).
var DebugSubtypeReplaceLast bool

// subtypeAddStep reports whether o adds a subtype in layer 4: a
// timestamp-ordered effect (CR 613.1d, 613.7) whose every application gets
// the newest timestamp, so a repeat must take the newest record position
// instead of merging into the older one. A subtype replacement (HasSub or
// RemoveCreatureSub) is already ordered this way by applyOverridePT's merge
// predicate; this covers the addition half.
func subtypeAddStep(o *kop.Override) bool {
	if DebugSubtypeReplaceLast {
		return false
	}
	return o.AddSub.Any() || o.AllCreatureTypes
}

// DebugSubtypeOverridePairs is a test hook: how many battlefield permanents
// carry two or more layer-4 subtype overrides (records and override statics)
// with different timestamps. A four-seat lockstep test uses it to prove its
// pod reached the ordering the mutation test pins, rather than passing empty.
func (s *State) DebugSubtypeOverridePairs() int {
	k := s.k()
	n := 0
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		cnt, distinct := 0, false
		var first uint16
		s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
			if !(o.HasSub || o.AddSub.Any() || o.RemoveCreatureSub || o.AllCreatureTypes) {
				return
			}
			cnt++
			if cnt == 1 {
				first = ts
			} else if ts != first {
				distinct = true
			}
		})
		if cnt >= 2 && distinct {
			n++
		}
	}
	return n
}
