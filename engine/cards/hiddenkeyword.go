// Forge spells some combat keywords as a full English sentence rather than a
// bare head ("CARDNAME must be blocked if able.", "CARDNAME can't attack or
// block alone."). A sentence line has no colon, so KeywordHead returns the
// WHOLE sentence and the coverage walk interns `kw:<sentence>` -- a phantom
// primitive that names no engine symbol, and a keyword no matcher recognises
// by its printed head. This file is the one place the sentence spellings are
// translated into the canonical heads the engine reads, so the next such
// sentence is one table entry rather than another special case at each
// consumer.
//
// The translation is applied in engine/cards/parse.go as a printed K: line is read,
// NOT in rules: a sentence line is a PARSER artefact of Forge's script
// grammar, and every consumer of Face.Keywords (the primitive census, the
// ratchet, HasKeyword, rules' derived-keyword readers) must see the same
// canonical head. Runtime KW$ grants (a Pump/PumpAll's `KW$ HIDDEN CARDNAME
// must be blocked if able.`) never pass through the K: parser, so they keep
// the sentence form; rules reads BOTH spellings for the same meaning (see
// engine/rules/combat/restrictions.go ParseHiddenKeyword), which is why the canonical-head arm
// there is additive, not a replacement.
package cards

import "strings"

// canonicalKeywordHeads maps a Forge sentence keyword head (case-insensitive,
// already trimmed) to the canonical head the engine reads by that name. Only
// heads whose meaning is ALREADY implemented are listed: a sentence whose
// behaviour the build does not model must stay an unknown primitive, because
// renaming it would advertise support that a matcher does not deliver.
//
//   - "CARDNAME must be blocked if able." (CR 509.1a, the attacker's
//     requirement to receive at least one legal blocker) is read as the
//     MustBlock keyword, the spelling engine/rules/statics.go's
//     combat.HasMustBeBlockedKeyword accepts beside the Pump-granted sentence form.
//   - "All creatures able to block CARDNAME do so." (CR 509.1c, Lure: every
//     creature able to block the attacker must) is read as the AllMustBlock
//     keyword, the per-attacker blocking requirement askBlockers publishes as
//     a per-pair duty. It is NOT "must be blocked if able", which one blocker
//     satisfies.
//   - "CARDNAME can't attack or block." is read as CantAttackOrBlock, whose
//     two combat restrictions are both consumed by the rules readers.
var canonicalKeywordHeads = map[string]string{
	"cardname must be blocked if able.":           "MustBlock",
	"all creatures able to block cardname do so.": "AllMustBlock",
	"cardname can't attack or block.":             "CantAttackOrBlock",
	"cardname can't attack or block alone.":       "CantAttackOrBlockAlone",
	"cardname can't attack alone.":                "CantAttackAlone",
	"cardname can't block alone.":                 "CantBlockAlone",
}

// HiddenUntapNextStepKeyword is the EXACT Forge sentence a Pump/PumpAll `KW$
// HIDDEN ...` grant uses for CR 611.2b's "doesn't untap during its
// controller's next untap step" one-shot (Frost Lynx, Kashi-Tribe Elite et
// al.). The phrase never appears as a printed `K:` line, so it is not in the
// canonicalKeywordHeads table: it exists only as runtime keyword TEXT, which
// is exactly why it must be read from the derived keyword list rather than by
// HasKeyword. The sentence is matched WHOLE -- Undiscovered Paradise's
// "During your next untap step, ..." rider shares a prefix and must not
// borrow this meaning.
const HiddenUntapNextStepKeyword = "This card doesn't untap during your next untap step."

// IsHiddenUntapNextStepKeyword reports whether one derived keyword line is
// Forge's next-untap-step restriction sentence, with or without the leading
// "HIDDEN " marker the corpus spells it both ways with. This is the ONE
// reader both the grant site (effects, which stamps the one-shot flag) and
// the untap step (rules, which consumes it) call, so a future equivalent
// spelling is one arm here rather than two that can drift apart.
func IsHiddenUntapNextStepKeyword(k string) bool {
	return hiddenKeywordIs(k, HiddenUntapNextStepKeyword)
}

// hiddenKeywordIs reports whether one keyword line's head, with any leading
// "HIDDEN " marker stripped, is sentence (case-insensitive) -- the one
// reader of every runtime HIDDEN sentence.
func hiddenKeywordIs(k, sentence string) bool {
	head := strings.TrimSpace(strings.TrimPrefix(KeywordHead(k), "HIDDEN "))
	return strings.EqualFold(head, sentence)
}

// HiddenSwitchPTKeyword is the EXACT Forge sentence a Pump/PumpAll `KW$
// HIDDEN ...` grant uses for CR 613.4d's power/toughness switch (Twisted
// Image, Inside Out, Aquamoeba, Wandering Fumarole and the rest of the 28
// corpus carriers). Like the untap sentence it exists only as runtime
// keyword TEXT, never as a printed `K:` line; the Pump compiler turns it into
// a layer-7 switching effect (effects.PumpGrant.SwitchPT) instead of a
// keyword grant.
const HiddenSwitchPTKeyword = "CARDNAME's power and toughness are switched"

// IsHiddenSwitchPTKeyword reports whether one keyword line is Forge's P/T
// switch sentence, with or without the leading "HIDDEN " marker.
func IsHiddenSwitchPTKeyword(k string) bool {
	return hiddenKeywordIs(k, HiddenSwitchPTKeyword)
}

// CanonicalKeywordLine rewrites a keyword line whose head is a known Forge
// sentence spelling to its canonical head, preserving any parameter the line
// carries after its first colon. A line whose head is not in the table -- and
// a line already carrying the canonical head -- is returned unchanged, so the
// function is idempotent and cannot loop.
func CanonicalKeywordLine(k string) string {
	head := KeywordHead(k)
	canon, ok := canonicalKeywordHeads[strings.ToLower(head)]
	if !ok || canon == head {
		return k
	}
	if i := strings.IndexByte(k, ':'); i >= 0 {
		return canon + k[i:]
	}
	return canon
}

// HiddenKeyword is the dense code for one derived keyword line's combat
// meaning. It is the ONE boundary conversion for the combat restriction
// reader (engine/rules/combat.ParseHiddenKeyword) and for the Effect-delivery
// whitelist (engine/effects): Forge delivers these as keyword TEXT, either as
// a printed K: line canonicalised by canonicalKeywordHeads or as a runtime
// KW$/AddHiddenKeyword$ grant that keeps the sentence spelling. Both spellings
// are matched here, once, and every reader dispatches on the code.
type HiddenKeyword uint16

const (
	HiddenKeywordNone HiddenKeyword = iota
	HiddenKeywordCantAttackOrBlock
	HiddenKeywordCantAttack
	HiddenKeywordCantBlock
	HiddenKeywordMustBlock
	HiddenKeywordUntapNextStep
	HiddenKeywordCantAttackOrBlockAlone
	HiddenKeywordCantAttackAlone
	HiddenKeywordCantBlockAlone
	HiddenKeywordAllMustBlock
	HiddenKeywordCount
)

// hiddenKeywordHeadCodes resolves one already-lower-cased, trimmed head to its
// code. The table carries the canonical heads and the printed Forge sentence
// spellings; the "HIDDEN " marker is stripped before the lookup.
var hiddenKeywordHeadCodes = NewStrCodes(
	StrEntry[HiddenKeyword]{Key: "cantattackorblock", Val: HiddenKeywordCantAttackOrBlock},
	StrEntry[HiddenKeyword]{Key: "cardname can't attack or block.", Val: HiddenKeywordCantAttackOrBlock},
	StrEntry[HiddenKeyword]{Key: "cantattack", Val: HiddenKeywordCantAttack},
	StrEntry[HiddenKeyword]{Key: "cardname can't attack.", Val: HiddenKeywordCantAttack},
	StrEntry[HiddenKeyword]{Key: "cantblock", Val: HiddenKeywordCantBlock},
	StrEntry[HiddenKeyword]{Key: "cardname can't block.", Val: HiddenKeywordCantBlock},
	StrEntry[HiddenKeyword]{Key: "decayed", Val: HiddenKeywordCantBlock},
	StrEntry[HiddenKeyword]{Key: "mustblock", Val: HiddenKeywordMustBlock},
	StrEntry[HiddenKeyword]{Key: "cardname must be blocked if able.", Val: HiddenKeywordMustBlock},
	StrEntry[HiddenKeyword]{Key: "this card doesn't untap during your next untap step.", Val: HiddenKeywordUntapNextStep},
	StrEntry[HiddenKeyword]{Key: "cantattackorblockalone", Val: HiddenKeywordCantAttackOrBlockAlone},
	StrEntry[HiddenKeyword]{Key: "cardname can't attack or block alone.", Val: HiddenKeywordCantAttackOrBlockAlone},
	StrEntry[HiddenKeyword]{Key: "cantattackalone", Val: HiddenKeywordCantAttackAlone},
	StrEntry[HiddenKeyword]{Key: "cardname can't attack alone.", Val: HiddenKeywordCantAttackAlone},
	StrEntry[HiddenKeyword]{Key: "cantblockalone", Val: HiddenKeywordCantBlockAlone},
	StrEntry[HiddenKeyword]{Key: "cardname can't block alone.", Val: HiddenKeywordCantBlockAlone},
	StrEntry[HiddenKeyword]{Key: "allmustblock", Val: HiddenKeywordAllMustBlock},
	StrEntry[HiddenKeyword]{Key: "all creatures able to block cardname do so.", Val: HiddenKeywordAllMustBlock},
)

// HiddenKeywordOf resolves one keyword line to its HiddenKeyword code,
// case-insensitively, or HiddenKeywordNone when the line is outside the
// vocabulary. The optional leading "HIDDEN " marker is stripped without a
// strings call (the codeshape ratchet).
func HiddenKeywordOf(k string) HiddenKeyword {
	head := KeywordHead(k)
	if len(head) >= len("HIDDEN ") && head[:len("HIDDEN ")] == "HIDDEN " {
		head = head[len("HIDDEN "):]
	}
	if head == "" {
		return HiddenKeywordNone
	}
	return hiddenKeywordHeadCodes.Code(lowerASCIIString(head))
}
