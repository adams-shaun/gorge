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
// synthesize a TRIGGERED ability for: Prowess, Myriad, Extort, Evolve and
// Living Weapon, whose
// bodies read nothing the walk does not already capture. Every other head returns false (fail
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
	return id == cards.KeywordHeadIDOf("Prowess") || id == cards.KeywordHeadIDOf("Myriad") ||
		id == cards.KeywordHeadIDOf("Extort") || id == cards.KeywordHeadIDOf("Evolve") ||
		id == cards.KeywordHeadIDOf("Living Weapon")
}

// GrantedMyriadPayload is the GrantTriggerPush body name a granted Myriad
// rides (events.foldGrantTriggerPush rebuilds it into DB$ Myriad), the
// sibling of the granted Prowess payload.
const GrantedMyriadPayload = "__kwMyriadGranted"

// GrantedEvolvePayload is the GrantTriggerPush body name a granted Evolve
// rides (events.foldGrantTriggerPush rebuilds it into the +1/+1 counter body).
// The "Granted" suffix, with no colon, cannot alias a printed keyword's
// "__kwEvolve" SVar.
const GrantedEvolvePayload = "__kwEvolveGranted"

// GrantedExtortPayload is the GrantTriggerPush body name a granted Extort
// rides (events.foldGrantTriggerPush rebuilds it into DB$ Extort), the
// sibling of the granted Prowess and Myriad payloads.
const GrantedExtortPayload = "__kwExtortGranted"

// GrantedLivingWeaponPayload is the GrantTriggerPush body name a granted
// Living Weapon rides (events.foldGrantTriggerPush rebuilds it into the Germ
// token and attach chain).
const GrantedLivingWeaponPayload = "__kwLivingWeaponGranted"

// GrantedKeywordPayload names the GrantTriggerPush body payload the trigger
// GrantedKeywordTrigger synthesizes for line rides: events.Apply rebuilds the
// body from that name, so a log-only replay mints the identical ability.
func GrantedKeywordPayload(line string) string {
	switch cards.KeywordHeadIDOf(cards.KeywordHead(line)) {
	case cards.KeywordHeadIDOf("Myriad"):
		return GrantedMyriadPayload
	case cards.KeywordHeadIDOf("Evolve"):
		return GrantedEvolvePayload
	case cards.KeywordHeadIDOf("Extort"):
		return GrantedExtortPayload
	case cards.KeywordHeadIDOf("Living Weapon"):
		return GrantedLivingWeaponPayload
	}
	return "__kwProwessGranted"
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
	head := cards.KeywordHead(line)
	if !GrantedTriggerHeads(head) {
		return nil
	}
	id := cards.KeywordHeadIDOf(head)
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
	if id == cards.KeywordHeadIDOf("Myriad") {
		return grantedMyriadTrigger(line, head)
	}
	if cards.KeywordHeadIDOf(head) == cards.KeywordHeadIDOf("Evolve") {
		return grantedEvolveTrigger(line, head)
	}
	if id == cards.KeywordHeadIDOf("Living Weapon") {
		return grantedLivingWeaponTrigger(line, head)
	}
	return &cards.Trigger{Mode: "SpellCast", Params: map[string]string{
		"Mode": "SpellCast", "ValidCard": "Card.nonCreature",
		"ValidActivatingPlayer": "You", "TriggerDescription": "Prowess",
	}, Effect: grantedBodySA("Pump", "Defined", "Self", "NumAtt", "+1", "NumDef", "+1")}
}

// grantedBodySA is the DB$ body a synthesized trigger carries inline: api
// and its key/value params, in pairs.
func grantedBodySA(api string, kv ...string) *cards.SA {
	p := make(map[string]string, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		p[kv[i]] = kv[i+1]
	}
	return &cards.SA{Kind: "DB", API: api, Params: p}
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

// grantedEvolveTrigger is the trigger a granted Evolve gives its holder
// (CR 702.100a, 613.1f; Tyranid Prime, Propagator Drone): the printed
// expansion's exact shape (engine/cards/kw_evolve.go), the body carried inline.
// Evolve$ True is what drives the CR 603.4 resolution recheck and the
// events.Evolved watch. Only the bare line is the keyword; any other
// spelling fails closed.
func grantedEvolveTrigger(line, head string) *cards.Trigger {
	if len(line) != len(head) {
		return nil
	}
	t, ok := cards.ParseTriggerLine("Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature.YouCtrl+Other | Evolve$ True | TriggerDescription$ Evolve")
	if !ok {
		return nil
	}
	t.Effect = grantedBodySA("PutCounter", "Defined", "Self", "CounterType", "P1P1", "CounterNum", "1")
	return &t
}

// GrantedLivingWeaponBody is the body a granted Living Weapon carries: the
// printed expansion's (engine/cards/kw_livingweapon.go) 0/0 black Phyrexian
// Germ token, remembered, then the source attached to it. The chain is built
// here once for the live queue and events.foldGrantTriggerPush alike.
func GrantedLivingWeaponBody() *cards.SA {
	body := grantedBodySA("Token", "TokenScript", "b_0_0_phyrexian_germ", "TokenOwner", "You", "RememberTokens", "True")
	body.Sub = grantedBodySA("Attach", "Defined", "Remembered", "Object", "Self")
	return body
}

// grantedLivingWeaponTrigger is the trigger a granted Living Weapon gives its
// holder (CR 702.92a, 613.1f; Ekthi, Contaminator Priest's "each Equipment you
// control has living weapon"): the printed expansion's exact shape, an
// enters-the-battlefield trigger of the holder itself, with the body carried
// inline. Only the bare line is the keyword; any other spelling fails closed.
func grantedLivingWeaponTrigger(line, head string) *cards.Trigger {
	if len(line) != len(head) {
		return nil
	}
	t, ok := cards.ParseTriggerLine("Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ Living weapon")
	if !ok {
		return nil
	}
	t.Effect = GrantedLivingWeaponBody()
	return &t
}
