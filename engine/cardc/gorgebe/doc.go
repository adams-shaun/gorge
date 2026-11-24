// Package gorgebe is the card compiler's gorge backend: it prints an ir.Card
// as gorge script text, loads that text through gorge's own parser, and
// builds the registry that serves each card from cardc when cardc covers it
// and from Forge otherwise, with the reason. Design of record:
// docs/design/2026-11-23-typed-script-values/PLAN.md, sections 1.3, 1.4 and 5.
//
// Nothing outside engine/cardc and cmd/cardc imports this package.
package gorgebe
