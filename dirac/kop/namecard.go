package kop

// The mid-resolution card-name choice (gorge effNameCard, engine/effects/
// cardflow.go: Demonic Consultation's `SP$ NameCard | Defined$ You`, Cheering
// Fanatic's trigger-body `DB$ NameCard | Defined$ You`): the resolving
// controller chooses the name of one real card of the matchup, the ask's
// options one per nameable card of the matchup in gorge's sorted-name order
// (the ChGenName decision, mode = the card's CardID, Table.NameCardsAny or
// Table.NameCards ascending; gorge's NameChoices over the matchup's name
// universe, oracle NameUniverseFor). gorge keeps the answer on the source
// object as Object.ChosenName (events.Choose Counter "name"); the kernel
// keeps it in a battlefield source's chosen byte as that CardID, or in the
// resolution's own answer for a source still on the stack (the spell is
// gone from the stack before its chain ends, and gorge clears the name when
// the object leaves the battlefield, events apply_zone.go). A source that
// already carries a chosen name is a no-op: gorge's effNameCard reads the
// guard `o.ChosenName != ""` and returns, so a permanent that chose a name
// as it entered (ExtChooseName) never asks again, and neither does a second
// ask inside one chain.
//
// The named card is read by PredNamedCard (Card.NamedCard: DigUntil's
// Valid$, a ReduceCost static's ValidCard$) and cleared by ExtClearNamedCard
// (gorge `DB$ Cleanup | ClearNamedCard$ True`, Cheering Fanatic), which
// empties the source's chosen name so the next ask asks again. The frozen
// read beside such a clear is the host-bound emblem's (gorge
// state.ContinuousEffect.ChosenName; kernel static_emblemcold.go).

// ExtNameCard (a manifest entry, dirac/manifest/ExtOp/ExtNameCard) asks the
// resolving controller for a card name and stores the answer on the
// resolving source (a battlefield source's chosen byte, else the
// resolution's own answer). B.N: 0 every nameable card of the matchup, 1 the
// nonland ones (gorge ValidCards$ Card.nonLand).
//
// ExtClearNamedCard (dirac/manifest/ExtOp/ExtClearNamedCard) empties the
// resolving source's chosen card name (gorge `DB$ Cleanup |
// ClearNamedCard$ True`; Cheering Fanatic's next attack asks anew). A
// battlefield source's chosen byte goes back to "none"; a stack source's
// resolution answer is dropped.
