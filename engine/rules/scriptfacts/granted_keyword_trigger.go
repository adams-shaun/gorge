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
// synthesize a TRIGGERED ability for: Prowess and Myriad, whose bodies read
// nothing the walk does not already capture. Every other head returns false (fail
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
	return id == cards.KeywordHeadIDOf("Prowess") || id == cards.KeywordHeadIDOf("Myriad")
}

// GrantedMyriadPayload is the GrantTriggerPush body name a granted Myriad
// rides (events.foldGrantTriggerPush rebuilds it into DB$ Myriad), the
// sibling of the granted Prowess payload.
const GrantedMyriadPayload = "__kwMyriadGranted"

// GrantedKeywordPayload names the GrantTriggerPush body payload the trigger
// GrantedKeywordTrigger synthesizes for line rides: events.Apply rebuilds the
// body from that name, so a log-only replay mints the identical ability.
func GrantedKeywordPayload(line string) string {
	if cards.KeywordHeadIDOf(cards.KeywordHead(line)) == cards.KeywordHeadIDOf("Myriad") {
		return GrantedMyriadPayload
	}
	return "__kwProwessGranted"
}

// GrantedKeywordTrigger synthesizes the triggered ability a keyword LINE
// grants. The line is the full "Head:param..." text exactly as it sits in a
// layer-6 AddKeyword$ grant's derived entry ("Prowess"). The trigger reuses
// the printed expansion's exact shape (engine/cards/kw_prowess.go: Mode$ SpellCast |
// ValidCard$ Card.nonCreature | ValidActivatingPlayer$ You, body
// DB$ Pump | Defined$ Self | NumAtt$ +1 | NumDef$ +1) so the granted body is
// byte-identical to the printed one's -- a granted Prowess and a printed
// K:Prowess are the same rule. The body is carried inline (Effect) rather than
// as an Execute$ SVar name: a granted keyword lives in the layer system, never
// on a face, so there is no table to resolve a name against. A line whose head
// mints no triggered ability returns nil (the totality stance every
// synthesizer takes).
func GrantedKeywordTrigger(line string) *cards.Trigger {
	head := cards.KeywordHead(line)
	if !GrantedTriggerHeads(head) {
		return nil
	}
	if cards.KeywordHeadIDOf(head) == cards.KeywordHeadIDOf("Myriad") {
		return grantedMyriadTrigger(line, head)
	}
	return &cards.Trigger{Mode: "SpellCast", Params: map[string]string{
		"Mode": "SpellCast", "ValidCard": "Card.nonCreature",
		"ValidActivatingPlayer": "You", "TriggerDescription": "Prowess",
	}, Effect: &cards.SA{Kind: "DB", API: "Pump", Params: map[string]string{
		"Defined": "Self", "NumAtt": "+1", "NumDef": "+1",
	}}}
}

// grantedMyriadTrigger is the trigger a granted Myriad gives its holder
// (CR 702.116a; Blade of Selves' "Equipped creature has myriad", Legion
// Loyalty): the printed expansion's shape (engine/cards/kw_myriad.go: Mode$
// Attacks | ValidCard$ Card.Self, body DB$ Myriad) without its Myriad$ True
// marker, which pins a PRINTED line's Execute$ SVar to the Myriad body
// (trigmatch.AttacksMatchedAttackers) and has no SVar to read here: the body
// rides inline and the payload GrantedMyriadPayload. Only the bare line is
// the keyword (the printed K:Myriad has no parameter); any other spelling
// fails closed. The body reads the defending player from the trigger's
// Remembered (effects.myriadOpponents), which the granted walk captures from
// the DeclareAttackers event exactly as the printed path does.
func grantedMyriadTrigger(line, head string) *cards.Trigger {
	if len(line) != len(head) {
		return nil
	}
	t, ok := cards.ParseTriggerLine("Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Myriad")
	if !ok {
		return nil
	}
	t.Effect = &cards.SA{Kind: "DB", API: "Myriad"}
	return &t
}
