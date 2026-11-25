package dirac

// Object birth order. gorge's object ids are creation order and the payment
// planner's last tie-break compares them (pay.CompareSteps), so the source it
// draws first is the oldest. Once the object table fills, the kernel mints a
// token by reusing the lowest free id (zone_objreuse.go), so a new object can
// have a smaller id than an older one. The planner must still see creation
// order: the cold region keeps each object's rank (element 0 is the next
// rank), stamped when the object is first created and again when a reused id
// is minted.
var coldBorn = newCold[uint16]("born", func(g *genPrep) int { return MaxObj + 1 })

// birthOf is o's creation rank. A table with no cold region has no reused id
// either, so its raw object id is the rank; an object a fixture wrote into the
// table without minting its rank (an unstamped 0) keeps the raw id too.
func (s *State) birthOf(o Obj) uint16 {
	b := coldBorn.of(s)
	if len(b) == 0 || b[int(o)+1] == 0 {
		return uint16(o)
	}
	return b[int(o)+1]
}

// mintBirth gives o the next creation rank: a fresh object is the newest, and
// a reused object id (newToken) starts a new object, as a fresh gorge ObjID
// would.
func (s *State) mintBirth(o Obj) {
	b := coldBorn.of(s)
	if len(b) == 0 {
		return
	}
	b[0]++
	b[int(o)+1] = b[0]
}
