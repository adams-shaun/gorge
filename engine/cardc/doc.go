// Package cardc is the root of the typed card interface: engine/cardc/ir holds
// the types, and each reader of a card source (engine/cardc/mtgish today) lowers
// that source into them. Nothing that plays a game may import these packages
// yet; internal/archtest's TestNothingImportsCardc holds the allowed edges.
package cardc
