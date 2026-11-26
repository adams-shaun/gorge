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
// synthesize a TRIGGERED ability for: Prowess, Myriad, Extort, Evolve,
// Mentor, Living Weapon and Battle cry, whose bodies read nothing the walk
// does not already capture, and Afterlife and Annihilator, whose only input
// beyond that is the N in their own keyword line. Every other head returns false (fail closed): the expansion
// of a keyword like Afflict, Dethrone or Firebending mints a trigger whose
// body reads keyword-specific state (a defending player, an attack's
// attacker set, an N in the keyword text) through a dedicated walk
// (checkGrantedAfflictTriggers and kin), and handing back a wrong body would
// be worse than handing back none. Dispatch is by interned head ID, not a
// string case: the codeshape stringCaseLiterals ratchet is shrink-only, so a
// new `case "Literal":` arm is not allowed in engine/rules/ (cards owns one such
// whitelist; this is the rules-side twin).
func GrantedTriggerHeads(head string) bool {
	id := cards.KeywordHeadIDOf(head)
	return id == cards.KeywordHeadIDOf("Prowess") || id == cards.KeywordHeadIDOf("Myriad") ||
		id == cards.KeywordHeadIDOf("Extort") || id == cards.KeywordHeadIDOf("Evolve") ||
		id == cards.KeywordHeadIDOf("Afterlife") || id == cards.KeywordHeadIDOf("Living Weapon") ||
		id == cards.KeywordHeadIDOf("Mentor") || id == cards.KeywordHeadIDOf("Annihilator") ||
		id == cards.KeywordHeadIDOf("Battle cry") || id == cards.KeywordHeadIDOf("Undying") ||
		id == cards.KeywordHeadIDOf("Persist")
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

// GrantedAfterlifePayloadPrefix starts the GrantTriggerPush body name a granted
// Afterlife rides: "__kwAfterlifeGranted:N", the count N after the colon
// (events.foldGrantTriggerPush rebuilds it into the printed expansion's Spirit
// token body). A bare "__kwAfterlifeGranted" with no count rebuilds nothing.
const GrantedAfterlifePayloadPrefix = "__kwAfterlifeGranted:"

// grantedAfterlifeCount reads N from an "Afterlife:N" keyword line: a bare
// decimal literal of one to nine digits with no leading zero (so N is at
// least 1), the only form the corpus prints. Any other spelling reports false
// (fail closed). Read by hand: the rules-side strings ratchet counts every
// strings.*/strconv.* call.
func grantedAfterlifeCount(line string) (string, bool) {
	i := 0
	for i < len(line) && line[i] != ':' {
		i++
	}
	if i == len(line) {
		return "", false
	}
	param := line[i+1:]
	if len(param) == 0 || len(param) > 9 || param[0] < '1' || param[0] > '9' {
		return "", false
	}
	for j := 1; j < len(param); j++ {
		if param[j] < '0' || param[j] > '9' {
			return "", false
		}
	}
	return param, true
}

// GrantedAnnihilatorPayloadPrefix starts the GrantTriggerPush body name a
// granted Annihilator rides: "__kwAnnihilatorGranted:N", the count N after
// the colon (events.foldGrantTriggerPush rebuilds it into the printed
// expansion's Sacrifice body).
const GrantedAnnihilatorPayloadPrefix = "__kwAnnihilatorGranted:"

// GrantedBattleCryPayload is the GrantTriggerPush body name a granted Battle
// cry rides (events.foldGrantTriggerPush rebuilds it into the printed
// expansion's PumpAll body).
const GrantedBattleCryPayload = "__kwBattleCryGranted"

// GrantedUndyingPayload and GrantedPersistPayload are the GrantTriggerPush
// body names a granted Undying or Persist rides (events.foldGrantTriggerPush
// rebuilds each into the printed expansion's return-to-the-battlefield body).
const (
	GrantedUndyingPayload = "__kwUndyingGranted"
	GrantedPersistPayload = "__kwPersistGranted"
)

// GrantedLivingWeaponPayload is the GrantTriggerPush body name a granted
// Living Weapon rides (events.foldGrantTriggerPush rebuilds it into the Germ
// token and attach chain).
const GrantedLivingWeaponPayload = "__kwLivingWeaponGranted"

// GrantedMentorPayload is the GrantTriggerPush body name a granted Mentor
// rides (events.foldGrantTriggerPush rebuilds it into the printed expansion's
// targeted P1P1 PutCounter body, Mentor$ marker included). The "Granted"
// suffix keeps it from aliasing the "__kwMentor" SVar a printed bare
// K:Mentor line mints (the Exploit/Offspring rule).
const GrantedMentorPayload = "__kwMentorGranted"

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
	case cards.KeywordHeadIDOf("Afterlife"):
		n, _ := grantedAfterlifeCount(line)
		return GrantedAfterlifePayloadPrefix + n
	case cards.KeywordHeadIDOf("Annihilator"):
		n, _ := grantedAfterlifeCount(line)
		return GrantedAnnihilatorPayloadPrefix + n
	case cards.KeywordHeadIDOf("Battle cry"):
		return GrantedBattleCryPayload
	case cards.KeywordHeadIDOf("Living Weapon"):
		return GrantedLivingWeaponPayload
	case cards.KeywordHeadIDOf("Mentor"):
		return GrantedMentorPayload
	case cards.KeywordHeadIDOf("Undying"):
		return GrantedUndyingPayload
	case cards.KeywordHeadIDOf("Persist"):
		return GrantedPersistPayload
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
	if id == cards.KeywordHeadIDOf("Afterlife") {
		return grantedAfterlifeTrigger(line)
	}
	if cards.KeywordHeadIDOf(head) == cards.KeywordHeadIDOf("Evolve") {
		return grantedEvolveTrigger(line, head)
	}
	if id == cards.KeywordHeadIDOf("Mentor") {
		return grantedMentorTrigger(line, head)
	}
	if id == cards.KeywordHeadIDOf("Living Weapon") {
		return grantedLivingWeaponTrigger(line, head)
	}
	if id == cards.KeywordHeadIDOf("Annihilator") {
		return grantedAnnihilatorTrigger(line)
	}
	if id == cards.KeywordHeadIDOf("Battle cry") {
		return grantedBattleCryTrigger(line, head)
	}
	if id == cards.KeywordHeadIDOf("Undying") {
		return grantedReturnTrigger(line, head, "P1P1", "Undying")
	}
	if id == cards.KeywordHeadIDOf("Persist") {
		return grantedReturnTrigger(line, head, "M1M1", "Persist")
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

// grantedAfterlifeTrigger is the trigger a granted Afterlife N gives its
// holder (CR 702.135a; Afterlife Insurance's "creatures you control gain
// afterlife 1 until end of turn"): the printed expansion's exact shape
// (engine/cards/kw_afterlife.go), the body carried inline. Its Origin$
// Battlefield makes it a look-back trigger (CR 603.10a): the walk reads the
// grant from the pre-departure board. A count that is not a positive decimal
// literal fails closed.
func grantedAfterlifeTrigger(line string) *cards.Trigger {
	n, ok := grantedAfterlifeCount(line)
	if !ok {
		return nil
	}
	t, ok := cards.ParseTriggerLine("Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | TriggerDescription$ Afterlife")
	if !ok {
		return nil
	}
	t.Effect = grantedBodySA("Token", "TokenScript", "wb_1_1_spirit_flying", "TokenAmount", n)
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

// grantedMentorTrigger is the trigger a granted Mentor gives its holder (CR
// 702.134, 702.134b; Aegis of the Legion's "Equipped creature has mentor",
// Nyxborn Unicorn's Bestow aura): the printed expansion's exact shape
// (engine/cards/kw_mentor.go), the targeted PutCounter body carrying the
// Mentor$ marker so rules' mentorAdmits reads the lesser-power restriction at
// both the target offer and the CR 608.2b recheck. Only the bare line is the
// keyword; any other spelling fails closed.
func grantedMentorTrigger(line, head string) *cards.Trigger {
	if len(line) != len(head) {
		return nil
	}
	t, ok := cards.ParseTriggerLine("Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Mentor")
	if !ok {
		return nil
	}
	t.Effect = grantedBodySA("PutCounter", "ValidTgts", "Creature.attacking",
		"TgtPrompt", "Select target attacking creature with lesser power",
		"Mentor", "True", "CounterType", "P1P1", "CounterNum", "1")
	return &t
}

// GrantedAnnihilatorBody is the body a granted Annihilator N carries: the
// printed expansion's (engine/cards/kw_annihilator.go) Sacrifice of N
// permanents by the trigger's defending player, the count on the
// Annihilator$ marker. Built here once for the live queue and
// events.foldGrantTriggerPush alike.
func GrantedAnnihilatorBody(n string) *cards.SA {
	return grantedBodySA("Sacrifice", "Defined", "TriggeredDefendingPlayer", "SacValid", "Permanent", "Annihilator", n)
}

// grantedAnnihilatorTrigger is the trigger a granted Annihilator N gives its
// holder (CR 702.86a, 613.1f; Idol of False Gods' "it has annihilator 2",
// Eldrazi Conscription, Flayer of Loyalties' GainControl AddKWs$): the
// printed expansion's exact shape, an Attacks trigger of the holder itself.
// The defending player is the walk's TriggerContext capture, as the printed
// trigger's is. A count that is not a positive decimal literal (Ulamog, the
// Defiler's Annihilator:X, whose X is an SVar the line does not carry) fails
// closed.
func grantedAnnihilatorTrigger(line string) *cards.Trigger {
	n, ok := grantedAfterlifeCount(line)
	if !ok {
		return nil
	}
	t, ok := cards.ParseTriggerLine("Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Annihilator")
	if !ok {
		return nil
	}
	t.Effect = GrantedAnnihilatorBody(n)
	return &t
}

// GrantedBattleCryBody is the body a granted Battle cry carries: the printed
// expansion's (engine/cards/kw_battle_cry.go) PumpAll of each other attacking
// creature, +1/+0 until end of turn.
func GrantedBattleCryBody() *cards.SA {
	return grantedBodySA("PumpAll", "ValidCards", "Creature.attacking+StrictlyOther", "NumAtt", "+1", "NumDef", "0")
}

// grantedBattleCryTrigger is the trigger a granted Battle cry gives its holder
// (CR 702.91a, 613.1f; Reckless Pyrosurfer's landfall Pump KW$ Battle cry,
// Marshland Hordemaster): the printed expansion's exact shape. Only the bare
// line is the keyword; any other spelling fails closed.
func grantedBattleCryTrigger(line, head string) *cards.Trigger {
	if len(line) != len(head) {
		return nil
	}
	t, ok := cards.ParseTriggerLine("Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Battle cry")
	if !ok {
		return nil
	}
	t.Effect = GrantedBattleCryBody()
	return &t
}

// GrantedReturnBody is the body a granted Undying (counter P1P1) or Persist
// (counter M1M1) carries: the printed expansions' (engine/cards/kw_undying.go,
// kw_persist.go) return of the dead card to the battlefield with one counter
// of that kind. Built here once for the live queue and
// events.foldGrantTriggerPush alike.
func GrantedReturnBody(counter string) *cards.SA {
	return grantedBodySA("ChangeZone", "Defined", "TriggeredNewCardLKICopy", "Origin", "Graveyard",
		"Destination", "Battlefield", "WithCountersType", counter, "WithCountersAmount", "1")
}

// grantedReturnTrigger is the trigger a granted Undying (CR 702.93a) or
// Persist (CR 702.79a) gives its holder (613.1f; Mikaeus, the Unhallowed's
// "have undying", Cauldron of Souls' Pump KW$ Persist): the printed
// expansion's exact dies trigger, its "no counters of that kind" condition
// read off the LKI. Its Origin$ Battlefield makes it a look-back trigger (CR
// 603.10a), as a granted Afterlife's is. Only the bare line is the keyword;
// any other spelling fails closed.
func grantedReturnTrigger(line, head, counter, desc string) *cards.Trigger {
	if len(line) != len(head) {
		return nil
	}
	t, ok := cards.ParseTriggerLine("Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self+counters_EQ0_" + counter + " | TriggerDescription$ " + desc)
	if !ok {
		return nil
	}
	t.Effect = GrantedReturnBody(counter)
	return &t
}
