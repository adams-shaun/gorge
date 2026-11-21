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
//   - "CARDNAME must be blocked by two or more creatures if able." (CR
//     509.1c, Gorm the Great) is read as MustBeBlockedByTwo: one more
//     requirement unit the defending player obeys by declaring a second
//     blocker against the attacker, counted by the declare-blockers solver
//     beside the plain MustBlock unit.
var canonicalKeywordHeads = map[string]string{
	"cardname must be blocked if able.":                          "MustBlock",
	"cardname must be blocked by two or more creatures if able.": "MustBeBlockedByTwo",
	"all creatures able to block cardname do so.":                "AllMustBlock",
	"cardname can't attack or block.":                            "CantAttackOrBlock",
	"cardname can't attack or block alone.":                      "CantAttackOrBlockAlone",
	"cardname can't attack alone.":                               "CantAttackAlone",
	"cardname can't block alone.":                                "CantBlockAlone",
	// CR 509.1c / 508.1c-d: the combat requirement and restriction sentences
	// whose behaviour combat.ParseHiddenKeyword reads (engine/rules/combat/restrictions.go).
	"cardname must be blocked by exactly one creature if able.":                "MustBeBlockedByOne",
	"cardname can only attack alone.":                                          "CanOnlyAttackAlone",
	"cardname can't attack unless a creature with greater power also attacks.": "CantAttackUnlessGreaterPower",
	"cardname can't block unless a creature with greater power also blocks.":   "CantBlockUnlessGreaterPower",
	"cardname can't attack unless at least two other creatures attack.":        "CantAttackUnlessTwoOthers",
	"cardname can't block unless at least two other creatures block.":          "CantBlockUnlessTwoOthers",
	"cardname can't attack unless a black or green creature also attacks.":     "CantAttackUnlessBlackOrGreen",
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
	HiddenKeywordMustBeBlockedByTwo
	HiddenKeywordMustBeBlockedByOne
	HiddenKeywordMustBeBlockedByAll
	HiddenKeywordCanOnlyAttackAlone
	HiddenKeywordCantAttackUnlessGreaterPower
	HiddenKeywordCantBlockUnlessGreaterPower
	HiddenKeywordCantAttackUnlessTwoOthers
	HiddenKeywordCantBlockUnlessTwoOthers
	HiddenKeywordCantAttackUnlessBlackOrGreen
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
	StrEntry[HiddenKeyword]{Key: "mustbeblockedbytwo", Val: HiddenKeywordMustBeBlockedByTwo},
	StrEntry[HiddenKeyword]{Key: "cardname must be blocked by two or more creatures if able.", Val: HiddenKeywordMustBeBlockedByTwo},
	StrEntry[HiddenKeyword]{Key: "mustbeblockedbyone", Val: HiddenKeywordMustBeBlockedByOne},
	StrEntry[HiddenKeyword]{Key: "cardname must be blocked by exactly one creature if able.", Val: HiddenKeywordMustBeBlockedByOne},
	// MustBeBlockedByAll:<blocker filter>:<text> is the printed Lure with a
	// blocker filter (Marble Priest, Talruum Piper); the filter rides the
	// keyword line, read by combat.ParseHiddenKeyword.
	StrEntry[HiddenKeyword]{Key: "mustbeblockedbyall", Val: HiddenKeywordMustBeBlockedByAll},
	StrEntry[HiddenKeyword]{Key: "canonlyattackalone", Val: HiddenKeywordCanOnlyAttackAlone},
	StrEntry[HiddenKeyword]{Key: "cardname can only attack alone.", Val: HiddenKeywordCanOnlyAttackAlone},
	StrEntry[HiddenKeyword]{Key: "cantattackunlessgreaterpower", Val: HiddenKeywordCantAttackUnlessGreaterPower},
	StrEntry[HiddenKeyword]{Key: "cardname can't attack unless a creature with greater power also attacks.", Val: HiddenKeywordCantAttackUnlessGreaterPower},
	StrEntry[HiddenKeyword]{Key: "cantblockunlessgreaterpower", Val: HiddenKeywordCantBlockUnlessGreaterPower},
	StrEntry[HiddenKeyword]{Key: "cardname can't block unless a creature with greater power also blocks.", Val: HiddenKeywordCantBlockUnlessGreaterPower},
	StrEntry[HiddenKeyword]{Key: "cantattackunlesstwoothers", Val: HiddenKeywordCantAttackUnlessTwoOthers},
	StrEntry[HiddenKeyword]{Key: "cardname can't attack unless at least two other creatures attack.", Val: HiddenKeywordCantAttackUnlessTwoOthers},
	StrEntry[HiddenKeyword]{Key: "cantblockunlesstwoothers", Val: HiddenKeywordCantBlockUnlessTwoOthers},
	StrEntry[HiddenKeyword]{Key: "cardname can't block unless at least two other creatures block.", Val: HiddenKeywordCantBlockUnlessTwoOthers},
	StrEntry[HiddenKeyword]{Key: "cantattackunlessblackorgreen", Val: HiddenKeywordCantAttackUnlessBlackOrGreen},
	StrEntry[HiddenKeyword]{Key: "cardname can't attack unless a black or green creature also attacks.", Val: HiddenKeywordCantAttackUnlessBlackOrGreen},
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

// IsHiddenRuleEffectKeyword reports whether one derived keyword line is one
// of the attack/block restriction or requirement sentences the CR applies as
// a game-rule effect rather than an ability of the object: CR 613.11's class
// ("say that a creature must attack this turn if able" -- the can't-attack,
// can't-block, must-be-blocked and alone/unless siblings) is applied after
// all other continuous effects, so a LATER layer-6 "loses all abilities"
// (CR 613.1f) cannot end it, exactly as it cannot end a Pacifism. It DOES end
// a granted ability line (a granted flying) and every printed line, whose
// sentences the printed K: parser already canonicalised to heads.
//
// UntapNextStep is deliberately outside the class: the untap step consumes
// the one-shot state flag stamped at grant time
// (engine/effects registerPumpEffects), not the keyword text, so the line's
// loss to an ability removal is not observable. The whitelist is explicit, so
// a future sentence meaning an ABILITY starts as one (a removal ends it)
// instead of silently surviving every removal.
func IsHiddenRuleEffectKeyword(k string) bool {
	switch HiddenKeywordOf(k) {
	case HiddenKeywordCantAttackOrBlock, HiddenKeywordCantAttack, HiddenKeywordCantBlock,
		HiddenKeywordMustBlock, HiddenKeywordCantAttackOrBlockAlone, HiddenKeywordCantAttackAlone,
		HiddenKeywordCantBlockAlone, HiddenKeywordAllMustBlock, HiddenKeywordMustBeBlockedByTwo,
		HiddenKeywordMustBeBlockedByOne, HiddenKeywordMustBeBlockedByAll, HiddenKeywordCanOnlyAttackAlone,
		HiddenKeywordCantAttackUnlessGreaterPower, HiddenKeywordCantBlockUnlessGreaterPower,
		HiddenKeywordCantAttackUnlessTwoOthers, HiddenKeywordCantBlockUnlessTwoOthers,
		HiddenKeywordCantAttackUnlessBlackOrGreen:
		return true
	}
	return false
}
