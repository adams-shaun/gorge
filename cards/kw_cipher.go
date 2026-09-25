// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwCipher mints the SVar the engine's spell-resolution tail runs for a
// Cipher spell, and nothing else: no triggered ability is synthesized.
//
// CR 702.99a: "Cipher is a static ability that functions while the spell
// with cipher is on the stack. 'Then you may exile this spell card encoded on
// a creature you control.'" The word "Then" makes the encode part of the
// resolving spell's own instruction (CR 608.2), NOT a separately counterable
// triggered ability: opponents get no priority between the spell resolving
// and the encode offer, cannot counter the encode, and cannot move the spell
// card out of a resting zone to deny it. A trigger delivered through the
// trigger queue (the shape `addKeywordTrigger` would mint) is a second stack
// object with its own priority round, which is exactly the defect this
// expansion no longer creates.
//
// The expansion is one SVar (`__kwCipher`) holding the api:Cipher primitive
// (effects/cipher.go). rules/stack.go's resolveTop splices a fresh copy of
// the resolving spell's ability chain with this body appended as its LAST
// instruction (rules/cipher.go's cipherTailFor), so the ordinary
// suspension/continuation machinery runs it as the resolution's tail: a
// mid-resolution ask parks the spell on the stack and the answered decision
// re-enters the same chain, and a completed resolution leaves the stack only
// afterwards. Because the tail rides the resolution, a spell that is
// countered or fizzles (never resolves) never reaches it -- CR 702.99c --
// without any ResolvedOnly$ gate.
//
// The combat-damage copy trigger the resulting association grants is rules'
// business (rules/trigger_granted.go's checkCipherTriggers plus the queue's
// __kwCipher: payload, exactly the granted-keyword shape Conspire and
// Demonstrate use). The trailing colon on that queue payload is what keeps it
// from aliasing this SVar.
func kwCipher(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	f.setSVar("__kwCipher", "DB$ Cipher | Defined$ Self")
}

func init() { registerKeyword(kwCipher, "Cipher") }
