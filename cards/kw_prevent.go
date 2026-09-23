// Keyword expansion for the corpus's printed `K:Prevent <english sentence>`
// keyword. Split out of keywords.go so tickets touching different keywords
// stop colliding on one file.
//
// The sentence carries no colon, so KeywordHead keeps the WHOLE line as the
// head (cards/face.go) and the registration is keyed on the three exact
// sentences the corpus prints (9 card files plus one token script at the
// current pin):
//
//	"Prevent all combat damage that would be dealt to CARDNAME."           (3 files)
//	"Prevent all combat damage that would be dealt to and dealt by
//	 CARDNAME."                                                            (Fog Bank)
//	"Prevent all damage that would be dealt to CARDNAME."                  (5 files
//	                                                                          + 1 token script)
//
// A sentence whose wording drifts from these fails closed: the head lookup
// misses, the K: line stays a raw keyword, and the coverage report names the
// bogus primitive exactly as it did before this expansion existed -- the loud
// direction, never a silently narrower prevention.

package cards

import "strings"

// kwPrevent expands one K:Prevent sentence into the printed DamageDone
// prevention replacement the engine's damage dispatch already resolves
// end to end: a bodyless `Prevent$ True` R: line (the Selfless Squire shape,
// task dponce1) is matched by rules/replacement.go's damageReplacementMatches
// -- ValidTarget$ / ValidSource$ scope it to the permanent itself,
// damageReplacementMatches' IsCombat$ gate applies the sentence's "combat"
// scoping -- and applied by applyNonMoveReplacements' Prevent$ arm, which
// stops the whole Damage event and stores the prevention Note the transcript
// and the DamagePreventedOnce trigger family read.
//
// Lifetime is the printed-keyword one, not a spell's: the minted lines are
// ordinary printed R: entries on the permanent's own face, live exactly while
// the object that carries them is where damage can be dealt to or by it. A
// prevention that outlives the creature or that covers a zone it has left
// cannot match, because the matching is by the object's own id
// (Card.Self) and combat damage events name real battlefield objects.
//
// "dealt to and dealt by" is TWO independent applicability gates over the one
// DamageDone event (the event's recipient and its dealing creature are two
// different roles), so the both-directions shape mints two lines, one per
// gate. They never compete on the same event -- one names the recipient, the
// other the source -- and when two such permanents fight each other (each
// line of one matching the other's event) the ordinary CR 616.1 damage
// replacement-order machinery already orders them.
func kwPrevent(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("R", k) {
		return
	}
	combat := strings.Contains(head, "combat damage")
	dealtBy := strings.Contains(head, "dealt by")
	mint := func(gate string) {
		line := "Event$ DamageDone | Prevent$ True | " + gate + " | Keyword$ Prevent"
		if combat {
			line += " | IsCombat$ True"
		}
		p := parseParams(line)
		p["KeywordLine"] = k
		f.Repls = append(f.Repls, Repl{Event: "DamageDone", Params: p})
	}
	mint("ValidTarget$ Card.Self")
	if dealtBy {
		mint("ValidSource$ Card.Self")
	}
}

func init() {
	registerKeyword(kwPrevent,
		"Prevent all combat damage that would be dealt to CARDNAME.",
		"Prevent all combat damage that would be dealt to and dealt by CARDNAME.",
		"Prevent all damage that would be dealt to CARDNAME.",
	)
}
