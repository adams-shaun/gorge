// Package cost is the cost vocabulary every rules subsystem shares: the
// parsed Cost and its CostPart components, the Forge cost-string parsers
// (ParseCost, the strict ParseUnlessCost), the client-notation and prose
// renderers (FormatCost, CostPhrase) and the pure predicates and arithmetic
// over a Cost (CMC, WithX, Plus, Priceable, HasNonMana, the mana-symbol
// helpers).
//
// It is a leaf layer of the rules engine: it imports only state, holds no *rules.Engine and reads
// no game state. internal/archtest TestCostVocabularyIsALeaf pins its import
// set. Payment -- pip expansion, the resolveMana search, the cast-flow
// stages -- stays in package rules until the engine/rules/pay extraction.
//
// Package rules reaches this package through one bridge file,
// engine/rules/cost_vocab.go, which aliases the types and forwards the functions
// under their historical rules names, so the move left rules' call sites
// untouched.
package cost
