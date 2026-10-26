package state

import "strings"

// ManaRestriction is a batch of floating mana that may be spent only for a
// Forge RestrictValid$ payment. Pool keeps the colour totals for display and
// ordinary payment; these records retain the provenance Pool deliberately
// cannot express. Amount is always positive and Color is the producing
// ManaAdd.Counter verbatim -- a bare WUBRGC letter, an "S<colour>" snow unit
// or a "<Tag><colour>" typed unit -- so its pool slot is read with
// state.ManaSlot, never ManaIndex(Color[0]).
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
	// keeps a persistent batch (and drops the ordinary ones); the TurnChange
	// fold demotes it to ordinary — the printed restriction is not
	// time-bounded, only the don't-lose clause is — so the next boundary's
	// ManaClear empties the units WITH the batch instead of leaving a
	// phantom. The pm Text suffix on the producing ManaAdd event is what
	// sets it, so a replay derives it identically.
	Persistent bool
	// Combat marks a Persistent batch whose don't-lose clause lasts only
	// "until end of combat" (CR 702.189a Firebending's shape): the same
	// " pmc" Text suffix that credits Player.CombatMana sets it. A
	// restricted spend that consumes a Combat batch's units takes them out
	// of CombatMana too, and the end-of-combat StepChange fold demotes a
	// surviving Combat batch to ordinary, so neither the tally nor the
	// batch outlives its units -- and an unrelated turn-persistent unit is
	// never emptied in its place.
	Combat bool
	// AddsCounters is the producing mana ability's AddsCounters$ rider value
	// (Opal Palace, Biophagus, Animal Attendant, Guildmages' Forum) captured
	// at PRODUCTION, when the ability is still the one that produced these
	// units: "if you spend this mana to cast [a matching spell], it enters
	// with additional counters". It is part of the batch provenance — the
	// mana-restriction Text's " ac=" segment — precisely so that the rider a
	// spent unit carries is the PRODUCING ABILITY's snapshot, never a
	// re-read of the source permanent's current face (a copied, modified or
	// text-changed permanent can gain or lose a rider between payment and
	// the spell's entry). Empty for every batch whose ability has no rider
	// and for every historical batch, so the encoding stays byte-identical.
	AddsCounters string
	// AddsKeywords is the producing ability's AddsKeywords$ rider ("if that
	// mana is spent on a Dragon creature spell, it gains haste"), snapshotted
	// at production like AddsCounters and carried by the " ak=" Text segment.
	// The zero value is no rider.
	AddsKeywords ManaKeywordRider
}

// ManaKeywordRider is one mana ability's AddsKeywords$ rider: Keywords is
// the raw AddsKeywords$ value (" & "-separated), Valid the AddsKeywordsValid$
// spell filter ("" admits every spell), Until the AddsKeywordsUntil$ value
// ("" is permanent, "UntilEOT" lasts to end of turn).
type ManaKeywordRider struct {
	Keywords string
	Valid    string
	Until    string
}

// AppendGrants appends one grant per keyword of the rider (the AddsKeywords$
// value is " & "-separated) to dst.
func (r ManaKeywordRider) AppendGrants(dst []ManaAddsKeywordGrant) []ManaAddsKeywordGrant {
	for kw := range strings.SplitSeq(r.Keywords, "&") {
		if kw = strings.TrimSpace(kw); kw != "" {
			dst = append(dst, ManaAddsKeywordGrant{Keyword: kw, Until: r.Until})
		}
	}
	return dst
}

// ManaAddsKeywordGrant is one keyword a cast earned from a spent
// AddsKeywords$ rider (CR 106.6): rules' entry hook grants it to the
// permanent the spell becomes (CR 400.7a).
type ManaAddsKeywordGrant struct {
	Keyword string
	Until   string
}

// ManaAddsCounterGrant is one AddsCounters$ mana-spend rider grant a cast
// earned: the resolved rider of the producing ABILITY -- Filter (a Forge
// spec), Kind (the counter kind, e.g. "P1P1") and Amount (an integer literal
// or the SVar BODY resolved to the producing source's table at the cast's
// payment) -- plus Count, how many of that ability's mana units the payment
// spent. rules' entry-counter plan evaluates Amount once per unit at the
// spell's battlefield entry; the rider itself is never re-read from the
// source's face, so a source that is copied, modified or loses the ability
// before entry cannot change the grant.
type ManaAddsCounterGrant struct {
	Filter string
	Kind   string
	Amount string
	Count  int32
}
