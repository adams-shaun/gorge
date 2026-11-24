package kop

// CostAtomKind is the kind of one CostAtom. The constants are generated from
// dirac/manifest/CostAtomKind (go run ./dirac/cmd/kmanifest gen); zero is no
// atom. A kind exists only for a cost part gorge's planner prices
// (engine/rules/pay/classify.go): cost:unknown, cost:withheld and
// cost:twobrid have none.
type CostAtomKind uint8

// CostAtom is one part of a cost program: Table.CostProgs[Ability.CostProg] is
// the atoms of the ability's cost. The walker (dirac/cost_prog.go) pays them in
// gorge's payment order whatever order they are listed in.
//
//	K     what is paid (CostAtomKind)
//	N     the count: cards, permanents, counters, life
//	F     the filter of the permanents or cards the atom takes (0: the source)
//	Z     the zone the atom takes from (ZoneNone: the kind's own)
//	Ctr   the counter kind of SubCounter and AddCounter
//	Flags reserved: no bit is defined yet
type CostAtom struct {
	K     CostAtomKind
	N     ValueRef
	F     FilterID
	Z     Zone
	Ctr   CounterKind
	Flags uint8
}
