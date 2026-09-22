package state

// ManaRestriction is a batch of floating mana that may be spent only for a
// Forge RestrictValid$ payment. Pool keeps the colour totals for display and
// ordinary payment; these records retain the provenance Pool deliberately
// cannot express. Amount is always positive and Color is one WUBRGC symbol.
type ManaRestriction struct {
	Color  string
	Amount int32
	Valid  string
	// NoCounter is the producing ability's AddsNoCounter$ condition: "" (the
	// ability carries none), "True" (mana that can make its spell uncountable)
	// or "NotPermanent" (AddsNoCounter$ !Permanent: only when the spell it
	// pays for is not a permanent spell). An empty Valid is an unrestricted
	// batch that carries only this provenance — mana produced by an
	// AddsNoCounter$ ability with no RestrictValid$ (Boseiju, Who Shelters
	// All) is spendable anywhere but still marks the spell it pays for.
	NoCounter string
	// Source is the id of the permanent whose ability produced this batch,
	// when the producing event recorded one (ManaRestrictionText's optional
	// segment). Zero for every historical batch and for source-less
	// producers; the source-relative Valid predicates resolve against it.
	Source ObjID
	// Persistent marks a batch whose producing ability carries
	// PersistentMana$ True: its units do not empty as steps and phases end
	// (CR 500.4 with the card's exception) until the turn ends. ManaClear
	// keeps a persistent batch (and drops the ordinary ones); the pm Text
	// suffix on the producing ManaAdd event is what sets it, so a replay
	// derives it identically.
	Persistent bool
}
