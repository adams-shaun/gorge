// Package cardc is the card compiler's root: it lowers mtgish's typed card
// definitions into gorge's IR. The design of record is
// docs/design/2026-11-23-typed-script-values/PLAN.md.
//
// Nothing outside engine/cardc and cmd/cardc may import it
// (internal/archtest's TestNothingImportsCardc).
package cardc
