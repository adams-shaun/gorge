package state

// ManaRestriction is a batch of floating mana that may be spent only for a
// Forge RestrictValid$ payment. Pool keeps the colour totals for display and
// ordinary payment; these records retain the provenance Pool deliberately
// cannot express. Amount is always positive and Color is one WUBRGC symbol.
type ManaRestriction struct {
	Color  string
	Amount int32
	Valid  string
	// Source is the id of the permanent whose ability produced this batch,
	// when the producing event recorded one (ManaRestrictionText's optional
	// segment). Zero for every historical batch and for source-less
	// producers; the source-relative Valid predicates resolve against it.
	Source ObjID
}
