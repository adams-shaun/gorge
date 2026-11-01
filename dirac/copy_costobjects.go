package dirac

// Cost-paid objects of a spell or ability and its copies (CR 707.10, gorge
// 36a8c9724). gorge records the objects a cast or activation paid as
// a cost (sacrificedLKI, castExiled, castRevealed, castCounterPut) and a
// stack copy inherits every record from its original (emit.go
// inheritCopyRecord): a copied Fling deals the ORIGINAL's sacrificed
// creature's power. Here the sacrificed object's record is the build
// entry's LKI index (b.e.li, the pool, zone_lki.go), which the stack entry
// takes on push and execCopySpell copies with the rest of the entry; the
// kicked flag and X ride along the same way. The kernel lowers no Exiled$,
// Revealed$ or counter-put ref, so those records have no reader here.

// snapSacPick records the first permanent a cast or activation sacrifices
// for a picked cost (Fling's Sac<1/Creature>, Ayli's "another creature")
// as the entry's last known information, unless a "sacrifice this" cost
// already did (RefSacrificed; gorge sacrificedLKI).
//
//go:noinline
func (s *State) snapSacPick(o Obj) {
	if s.bld().e.li == 0 {
		s.bld().e.li = s.snapLKI(o)
	}
}

// Copy exceptions as copiable values (CR 707.9b, gorge): see
// copy_exceptions.go (the layer-1 exception records, CR 613).
