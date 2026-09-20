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
	// WhenSpent is the TriggersWhenSpent$ SVar name of the producing
	// ability's trigger definition (task mordorparams1, Path of Ancestry's
	// "When that mana is spent to cast ... scry 1"): the batch is otherwise
	// an ordinary spendable batch (an empty Valid, the Boseiju provenance
	// shape), and rules' cast-payment path reads it off the consumed record
	// to queue the "when you spend this mana" trigger against the paying
	// spell. Empty for every historical batch and every ability without
	// the parameter.
	WhenSpent string
}
