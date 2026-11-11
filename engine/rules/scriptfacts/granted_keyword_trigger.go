package scriptfacts

// The granted-keyword triggered-ability synthesizer, the triggered sibling of
// cards.GrantedKeywordAbility. A layer-6 AddKeyword$ grant lands its keyword in
// the recipient's DERIVED keyword list (CR 613.1f), but keyword expansion
// (engine/cards/keywords.go expandKeywords) only reads a face's PRINTED keyword list,
// so no trigger is ever minted for the grant. This file is the one dispatcher
// the granted-trigger walk calls: it synthesizes the triggered ability for the
// heads whose body is a fixed, self-contained rule from the keyword LINE alone.
//
// It lives here rather than cards on purpose: cards/CompilerFingerprint
// hashes every engine/cards/*.go source file, so adding this there would invalidate
// the compiled corpus cache on every machine. The synthesizer needs no
// compiler state beyond cards' exported types, so this leaf is its home.

import "github.com/adams-shaun/gorge/engine/cards"

// GrantedTriggerHeads is the set of keyword heads GrantedKeywordTrigger can
// synthesize a TRIGGERED ability for. Every other head returns false (fail
// closed): the expansion of a keyword like Afflict, Dethrone or Firebending
// mints a trigger whose body reads keyword-specific state (a defending player,
// an attack's attacker set, an N in the keyword text) through a dedicated walk
// (checkGrantedAfflictTriggers and kin), and handing back a wrong body would be
// worse than handing back none. Dispatch is by interned head ID, not a
// string case: the codeshape stringCaseLiterals ratchet is shrink-only, so a
// new `case "Literal":` arm is not allowed in engine/rules/ (cards owns one such
// whitelist; this is the rules-side twin).
func GrantedTriggerHeads(head string) bool {
	id := cards.KeywordHeadIDOf(head)
	return id == cards.KeywordHeadIDOf("Prowess") || id == cards.KeywordHeadIDOf("Extort")
}

// GrantedKeywordTrigger synthesizes the triggered ability a keyword LINE
// grants. The line is the full "Head:param..." text exactly as it sits in a
// layer-6 AddKeyword$ grant's derived entry ("Prowess", "Extort"). Each
// trigger reuses the printed expansion's exact shape (engine/cards/kw_prowess.go
// and kw_extort.go) so the granted body is byte-identical to the printed
// one's -- a granted keyword and a printed K: line are the same rule. The body
// is carried inline (Effect) rather than as an Execute$ SVar name: a granted
// keyword lives in the layer system, never on a face, so there is no table to
// resolve a name against. A line whose head mints no triggered ability returns
// nil (the totality stance every synthesizer takes).
func GrantedKeywordTrigger(line string) *cards.Trigger {
	id := cards.KeywordHeadIDOf(cards.KeywordHead(line))
	if id == cards.KeywordHeadIDOf("Extort") {
		// CR 702.101a: gorge kwExtort's SpellCast trigger. Its body is
		// DB$ Extort (effExtort), which asks the caster for the {W/B}
		// payment and drains on a pay. The line is parsed rather than
		// built as a map literal: the rules-side codeshape ratchet counts
		// map[string] types, and this trigger is the printed expansion's
		// exact line.
		t, ok := cards.ParseTriggerLine("Mode$ SpellCast | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | TriggerDescription$ Extort")
		if !ok {
			return nil
		}
		t.Effect = &cards.SA{Kind: "DB", API: "Extort"}
		return &t
	}
	if id != cards.KeywordHeadIDOf("Prowess") {
		return nil
	}
	return &cards.Trigger{Mode: "SpellCast", Params: map[string]string{
		"Mode": "SpellCast", "ValidCard": "Card.nonCreature",
		"ValidActivatingPlayer": "You", "TriggerDescription": "Prowess",
	}, Effect: &cards.SA{Kind: "DB", API: "Pump", Params: map[string]string{
		"Defined": "Self", "NumAtt": "+1", "NumDef": "+1",
	}}}
}
