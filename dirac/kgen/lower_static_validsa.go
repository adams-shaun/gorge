package kgen

import (
	"slices"
	"strings"
)

// ValidSA$ on a static (the census row `other:static spell-ability filter
// ValidSA ...`). Every static that reads ValidSA$ in its own handler
// (CantBeActivated, ManaConvert, CantTarget, the cost statics) never reaches
// the shared unread-params refusal; the carriers that do are two unrelated
// shapes. Measured with `go run ./dirac/cmd/kgen -census-std -all-blockers
// -tsv <file>`: 25 rows over all sets, 18 in the ALL group, before the port
// (the CantAttack/CantBlock rows are the six cards below, two rows each) and
// 9 rows, 6 in ALL (the Continuous ones only) after:
//
//	Mode          carriers                                          gorge reader
//	CantAttack    Bound in Gold, Faith's Fetters, Intercessor's     INERT: CantAttackParamsReadableForRules
//	(6 cards)     Arrest, Katabatic Winds, Realmbreaker's Grasp,    (engine/effects/restrictions.go)
//	              Serra Bestiary                                    reads and ignores it
//	CantBlock     the same six                                      none: combat's BlockRestricted CantBlock
//	(6 cards)                                                       loop runs no whitelist
//	Continuous    Brokkos Apex of Forever, Detective's Phoenix,     mayPlayParamValidSA (staticline.go)
//	(6 cards)     Ninja Teen, Sabin Master Monk, Tenacious          accepts only Spell; the Mutate, Blitz,
//	              Underdog, Timeline Culler                         Warp, Bestow, Sneak shapes need cast context
//
// The first two rows are one compound line `Mode$ CantAttack,CantBlock,
// [CantCrew,]CantBeActivated`: the parse-time split gives every mode the SAME
// Params map, so ValidSA$ (the CantBeActivated half's spare-mana filter) is
// also on the CantAttack and CantBlock statics. gorge ignores it there, so the
// lowering must too: readInertValidSA marks it read, any value, and the
// statics lower exactly as before. It is not a gate and the kernel gets no
// word for it. The CantBeActivated half lowers separately (lower_cantact.go).
//
// Continuous MayPlay$ + ValidSA$ Spell.<alt-cost> is NOT ported: it selects
// which alternative-cost cast the graveyard permission grants and has no plain
// match without alt-cost cast context. Its refusal now names the mode
// (`static spell-ability filter ValidSA Continuous "_"`), so the census
// isolates it from the compound-line carriers. The prefix `other:static
// spell-ability filter ` keeps classTier's M5.

// readInertValidSA marks ValidSA$ read without using its value: the shared
// Params map of a compound CantAttack/CantBlock/CantBeActivated line carries
// the sibling mode's filter, which gorge reads and ignores on these two modes.
func (p *params) readInertValidSA() { p.consume("ValidSA") }

// staticUnreadRefuse refuses a static's unread params; when the static's
// spell-ability filter (ValidSA$) is among them the refusal names the guard
// and the mode (T12: no kernel word reads it yet, T13c's FwSA* family), for
// the census to count it.
func staticUnreadRefuse(mode string, sp *params, left []string) error {
	if slices.Contains(left, "ValidSA") {
		return refuse("static spell-ability filter ValidSA %s %q", mode, strings.TrimSpace(sp.m["ValidSA"]))
	}
	return refuse("static unread params %v", left)
}
