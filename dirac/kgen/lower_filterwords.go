package kgen

import (
	"math/bits"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// withKwBit is the KwMask bit index of a with<Keyword> filter word's
// keyword (the words filter.go admits all have one).
func withKwBit(p string) int {
	name := strings.TrimPrefix(p, "with")
	switch name {
	case "FirstStrike":
		name = "First Strike"
	case "DoubleStrike":
		name = "Double Strike"
	}
	return bits.TrailingZeros32(uint32(kwBits[name]))
}

// cmpFaceDown is an explicit high comparison kind, clear of
// the cmp iota tail: the filter word faceDown (no operator), a face-down
// battlefield permanent (gorge wordFaceDown). It lowers to kop.PredFaceDown.
// faceUp and !faceDown are not read and keep refusing.
const cmpFaceDown uint8 = 200

// cmpWord7 is an explicit high comparison kind, clear of
// the cmp iota tail: the five filter words noName, greatestPower,
// attackingAlone and inZone<Zone>, lowered to kop.PredWord7 with the sub-kind
// in CmpN (kop.W7*; the zone in CmpN>>8 for inZone).
const cmpWord7 uint8 = 203

// filterWordZones are the zones inZone<Z> may name (gorge filter_word.go's
// inZone words; Command is not lowered).
var filterWordZones = [...]string{"Battlefield", "Graveyard", "Hand", "Library", "Exile", "Stack"}

// filterWordPred maps a filter word to its PredWord7 PredN, or !ok.
func filterWordPred(p string) (n int, ok bool) {
	switch p {
	case "noName":
		return int(kop.W7NoName), true
	case "greatestPower":
		return int(kop.W7GreatestPower), true
	case "attackingAlone":
		return int(kop.W7AttackingAlone), true
	case "!IsImprinted":
		// gorge's '!' of the word (kop.W7NotImprinted: the negation of the
		// source's persistent imprint read, the store and zone gate
		// included; Mirror of Life Trapping's TrigReturn pick). The word
		// itself maps in filterWordPred3, refused outside a chain that
		// writes the imprinted set (imprintedFilterGate); so is its
		// negation.
		return int(kop.W7NotImprinted), true
	}
	z, found := strings.CutPrefix(p, "inRealZone") // gorge wordInZone: the live read inZone<X> is (filter_word.go:401-415)
	if !found {
		z, found = strings.CutPrefix(p, "inZone")
	}
	if !found {
		return filterWordPred2(p) // lower_filterwords.go
	}
	z = strings.TrimSpace(z) // parseZone trims, so "inRealZone Graveyard" reads too (gorge's Mimic Vat spelling)
	for _, w := range filterWordZones {
		if w == z {
			return int(kop.W7InZone) | int(thisTurnEnteredZoneWords[z])<<8, true
		}
	}
	return 0, false
}

// filterWordPred2 maps six exact filter-word spellings to their
// PredWord7 sub-kinds (kop.W7FaceUp ... kop.W7SharesNameLegend). Any other
// spelling of the same words stays unknown: gorge reads only these
// arguments (effects/filter_word.go, filter_attachment.go).
func filterWordPred2(p string) (n int, ok bool) {
	switch p {
	case "faceUp":
		return int(kop.W7FaceUp), true
	case "IsNotChosenType":
		return int(kop.W7NotChosenType), true
	case "attacking Opponent":
		return int(kop.W7AttackingOpp), true
	case "EnchantedBy Aura.YouCtrl":
		return int(kop.W7EnchantedByYours), true
	case "doesNotShareNameWith OtherYourBattlefield":
		return int(kop.W7NoNameTwin), true
	case "sharesNameWith Valid Permanent.Legendary":
		return int(kop.W7SharesNameLegend), true
	}
	return filterWordPred3(p) // lower_filterwords.go
}

// filterWordPred3 maps three exact filter-word spellings to their
// PredWord7 sub-kinds (kop.W7GreatestPowerTgt, kop.W7CrewedBySrc,
// kop.W7WithWarp). withWarp is gorge's printed-keyword read (effects/filter.go
// keywordPredicates registers Warp); only that spelling lowers: withoutWarp
// and hasKeywordWarp stay unknown properties. withDisguise
// (kop.W7WithDisguise) is the same read for Disguise; withoutDisguise and
// hasKeywordDisguise stay unknown. withFlashback (kop.W7WithFlashback) is the
// same read for Flashback, with the same two unknown spellings. withInfect
// (kop.W7WithInfect) is the same read for Infect; withoutInfect and
// hasKeywordInfect stay unknown.
func filterWordPred3(p string) (n int, ok bool) {
	switch p {
	case "greatestPowerControlledByTargeted":
		return int(kop.W7GreatestPowerTgt), true
	case "CrewedBySourceThisTurn":
		return int(kop.W7CrewedBySrc), true
	case "withWarp":
		return int(kop.W7WithWarp), true
	case "withDisguise":
		return int(kop.W7WithDisguise), true
	case "withFlashback":
		return int(kop.W7WithFlashback), true
	case "withInfect":
		return int(kop.W7WithInfect), true
	case "withDevoid":
		return int(kop.W7WithDevoid), true // the same read for Devoid (a spell restriction's Spell.withDevoid)
	case "withSkulk":
		return int(kop.W7WithSkulk), true // lower_filterwords.go: the same read for Skulk
	case "Snow":
		// gorge hasType Snow: the printed supertype, never while face down
		// (kop.W7Snow); TypeMask and FilterFlags2 have no bit left for it
		return int(kop.W7Snow), true
	case "nonSnow":
		return int(kop.W7NonSnow), true
	case "attackingSame":
		// the source's own defender (gorge attackingSame; Kitesail
		// Skirmisher). Read where a source slot exists: a spell's source is
		// on the stack and attacks nothing, so the word matches nobody there
		return int(kop.W7AttackingSame), true
	case "EnchantedPlayerCtrl":
		// the player the source Aura enchants controls the candidate (gorge
		// wordEnchantedPlayerCtrl; kop.W7EnchPlayerCtrl). Battlefield only
		// (build.go refuses it elsewhere)
		return int(kop.W7EnchPlayerCtrl), true
	case "ManaCostPartialBlue":
		return int(kop.W7CostBlue), true
	case "NoAbilities":
		return int(kop.W7NoAbilities), true
	case "IsCommander":
		return int(kop.W7IsCommander), true
	case "ThisTurnEnteredFrom_Battlefield":
		return int(kop.W7EnteredFromBF), true
	case "ThisTurnEnteredFrom_Library":
		return int(kop.W7EnteredFromLib), true
	case "BottomLibrary":
		return int(kop.W7BottomLibrary), true
	case "IsImprinted":
		return int(kop.W7Imprinted), true // gated to a chain that writes the set (lower_imprintzone.go)
	case "sharesCreatureTypeWith Commander":
		// the evaluating controller's commanders (kop.W7CrCommander; Path of
		// Ancestry's rider): needs no trigger object
		return kop.W7SharesCrCommanderN(), true
	case "sharesCreatureTypeWith TriggeredAttacker", "sharesCreatureTypeWith TriggeredAttackerLKICopy":
		// the Attacks trigger's attacker (lowerCtx.referentGate holds it to
		// an Attacks trigger's body, where trigObj is that attacker)
		return int(kop.W7SharesCrTrigObj), true
	case "sharesCreatureTypeWith TriggeredCard", "sharesCreatureTypeWith TriggeredCardLKICopy":
		// the zone-change trigger's moving card (gorge sharesTypeReferents
		// reads TriggerCard for both spellings, its current types wherever it
		// now is). lowerCtx.referentGate holds it to a Dies or ETB trigger's
		// body, where trigObj is that card
		return int(kop.W7SharesCrTrigObj), true
	case "blockingTriggeredAttacker":
		// a creature blocking the become-blocked trigger's attacker
		// (lowerCtx.referentGate holds it to an AttackerBlocked trigger's
		// body, where trigObj is that attacker)
		return int(kop.W7BlockingTrigObj), true
	case "blockingSource":
		// a creature blocking the filter's source (gorge blockingSource: the
		// source's BlockedBy names it)
		return int(kop.W7BlockingSrc), true
	case "blockedBySource":
		// a creature the filter's source is blocking (gorge blockedBySource:
		// its BlockedBy names the source)
		return int(kop.W7BlockedBySrc), true
	}
	return attachedToWord(p)
}

// attachedToWord maps gorge's two-token "AttachedTo <class>[.<qual>]"
// (effects/filter_attachment.go attachedToArg: the object is attached to a
// battlefield permanent of that class meeting the qualifier, read against
// the bearer) to kop.W7AttachedTo. The classes lowered are Card and
// Permanent (any permanent), Creature and Land; the qualifiers Other (not
// the filter's source) and YouCtrl. A referent (Targeted, ParentTarget, a
// trigger's card) or any other class or qualifier stays unknown.
func attachedToWord(p string) (n int, ok bool) {
	arg, found := strings.CutPrefix(p, "AttachedTo ")
	if !found {
		return 0, false
	}
	class, qual, _ := strings.Cut(arg, ".")
	var bits int
	switch class {
	case "Card", "Permanent":
	case "Creature":
		bits = kop.AttToCreature
	case "Land":
		bits = kop.AttToLand
	default:
		return 0, false
	}
	switch qual {
	case "":
	case "Other":
		bits |= kop.AttToOther
	case "YouCtrl":
		bits |= kop.AttToYouCtrl
	case "EnchantedBy", "EquippedBy":
		// the attached-bearer qualifiers' one body (gorge attachedBy: the
		// bearer is what the filter's own source is attached to; With Great
		// Power, Baki's Curse)
		bits |= kop.AttToSrcBearer
	default:
		return 0, false
	}
	return int(kop.W7AttachedTo) | bits<<8, true
}

// cmpNoNameGrave is the CmpN placeholder of a clause's lone
// "doesNotShareNameWith YourGraveyard": the word lowers only beside
// "doesNotShareNameWith OtherYourBattlefield" in the same clause, as
// kop.W7NoNameTwinGrave. build refuses the placeholder left alone.
const cmpNoNameGrave = -int(kop.W7NoNameTwinGrave)

// noNameTwinGrave reads the two doesNotShareNameWith halves Guardian Project
// writes in one clause (gorge doesNotShareNameWithKey's YourGraveyard and
// OtherYourBattlefield keys) into the one PredWord7 sub-kind they make
// together; either order. handled is false for any other property.
func (c *lClause) noNameTwinGrave(p string) (handled bool, err error) {
	var other int
	switch p {
	case "doesNotShareNameWith YourGraveyard":
		other = int(kop.W7NoNameTwin)
	case "doesNotShareNameWith OtherYourBattlefield":
		other = cmpNoNameGrave
	default:
		return false, nil
	}
	if c.CmpStat == cmpWord7 && c.CmpN == other {
		c.CmpN = int(kop.W7NoNameTwinGrave)
		return true, nil
	}
	if p == "doesNotShareNameWith YourGraveyard" {
		return true, c.setCmp(cmpWord7, 0, cmpNoNameGrave, "", "")
	}
	return false, nil
}

// faceUpDisguise reads the faceUp and withDisguise words Expose the Culprit
// writes in one clause (Creature.faceUp+YouCtrl+withDisguise) into the one
// PredWord7 sub-kind they make together (kop.W7FaceUpDisguise); either order.
// A lone withDisguise is kop.W7WithDisguise. handled is false for any other
// property or a lone faceUp.
func (c *lClause) faceUpDisguise(p string) (handled bool, err error) {
	switch p {
	case "withDisguise":
		if c.CmpStat == cmpWord7 && c.CmpN == int(kop.W7FaceUp) {
			c.CmpN = int(kop.W7FaceUpDisguise)
			return true, nil
		}
	case "faceUp":
		if c.CmpStat == cmpWord7 && c.CmpN == int(kop.W7WithDisguise) {
			c.CmpN = int(kop.W7FaceUpDisguise)
			return true, nil
		}
	}
	return false, nil
}

// rewriteFilterWords runs, in lc.filter's order, every expression rewrite a
// filter parse applies before parseFilter; the first named refusal wins
// (each word's gate lives on its own function here).
func (lc *lowerCtx) rewriteFilterWords(expr string) (string, error) {
	for _, rw := range []func(string) (string, error){
		lc.spawnerTargetCtrl, lc.spawnerDefendingCtrl, lc.parentTargetCtrl,
	} {
		e, err := rw(expr)
		if err != nil {
			return "", err
		}
		expr = e
	}
	return expr, nil
}

// parentTargetCtrl rewrites the filter word "ControlledBy ParentTarget"
// (gorge controlReferentPlayers ParentTarget, engine/effects/
// trigger_referents.go: the parent chain link's chosen PLAYER targets -- an
// object target binds nothing, the tail skips it -- Yosei, the Morning
// Star's dying trigger, Cloak and Dagger Entwined, The Fall of Kroog, Yes
// Man's ImmediateTrigger condition) onto TargetedPlayerCtrl's resolution
// player-target set (kop.F2TgtPlayerCtrl, battlefield only). The kernel bit
// reads every player target of the resolving entry, so the rewrite is exact
// only while the parent's slot is the entry's only player-targeting one: a
// second earlier link targeting a player would widen the kernel's set past
// gorge's parent-only read, and a parent whose filter admits no player
// binds nothing on either side -- both shapes refuse with a named reason.
// The longer "ControlledBy ParentTargetedController" spelling is a
// different, unmodelled referent and keeps its unknown-property refusal.
// The boundary the parse-time gate cannot see: the filter's own link also
// declaring player targets (a Player alternative beside the ParentTarget
// clause in the same ValidTgts$); no corpus carrier has it, and the
// TargetedPlayerCtrl word itself reads the same entry-wide set.
func (lc *lowerCtx) parentTargetCtrl(expr string) (string, error) {
	const word = "ControlledBy ParentTarget"
	if !strings.Contains(expr, word) {
		return expr, nil
	}
	t := lc.lastTarget()
	if t < 0 {
		return expr, refuse("filter %q: ControlledBy ParentTarget without an earlier target", expr)
	}
	pt := lc.ab.Targets[t]
	if pt.F == nil || pt.F.Players == 0 {
		return expr, refuse("filter %q: ControlledBy ParentTarget: the parent link's target admits no player", expr)
	}
	for _, i := range lc.chainT[:len(lc.chainT)-1] {
		if f := lc.ab.Targets[i].F; f != nil && f.Players != 0 {
			return expr, refuse("filter %q: ControlledBy ParentTarget beside another player-targeting link", expr)
		}
	}
	return rewriteExactToken(expr, word, "TargetedPlayerCtrl"), nil
}

// rewriteExactToken replaces every occurrence of word in s that is the whole
// token (the next byte does not continue an identifier), so a longer
// spelling that merely starts with word keeps its own parser fate.
func rewriteExactToken(s, word, repl string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, word)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := i + len(word)
		if end < len(s) && isIdentByte(s[end]) {
			b.WriteString(s[:end])
			s = s[end:]
			continue
		}
		b.WriteString(s[:i])
		b.WriteString(repl)
		s = s[end:]
	}
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// spawnerTargetCtrl rewrites the filter word "ControlledBy
// Spawner>TriggeredTarget" (gorge controlReferentPlayers spawnerChain: the
// spawning trigger's target player) to trigPlayerCtrlWord inside an
// ImmediateTrigger body spawned by a combat-damage-to-a-player trigger whose
// source is the card itself (lc.spawnDmg, set by immediateTrigger): the
// player a creature deals combat damage to is the damaged seat, at any seat
// count (CR 506.4; the kernel's trigger player, dmgSeat/fireDmg; seat-exact).
// Anywhere else the word refuses with a named reason.
func (lc *lowerCtx) spawnerTargetCtrl(expr string) (string, error) {
	const word = "ControlledBy Spawner>TriggeredTarget"
	if !strings.Contains(expr, word) {
		return expr, nil
	}
	if !lc.spawnDmg {
		return expr, refuse("filter %q: Spawner>TriggeredTarget outside an ImmediateTrigger spawned by Card.Self's combat damage to a player", expr)
	}
	return strings.ReplaceAll(expr, word, trigPlayerCtrlWord), nil
}

// spawnerDefendingCtrl rewrites the filter word "ControlledBy
// Spawner>TriggeredDefendingPlayer" (gorge controlReferentPlayers
// spawnerChain: the spawning Attacks trigger's DefendingPlayer, resolved
// against the immediate trigger's role-riding instance, engine/effects/
// trigger_referents.go) to the plain "ControlledBy
// TriggeredDefendingPlayer" word inside an ImmediateTrigger body spawned by
// an Attacks trigger whose source is the card itself (lc.spawnAtk, set by
// immediateTrigger). The seat is seat-exact at any seat count: the reflexive
// entry inherits the parent Attacks trigger's defSeat as it is queued
// (OpReflexiveTrigger, interp_gen.go), which defendingSeatF2 reads. Anywhere
// else the word refuses with a named reason.
func (lc *lowerCtx) spawnerDefendingCtrl(expr string) (string, error) {
	const word = "ControlledBy Spawner>TriggeredDefendingPlayer"
	if !strings.Contains(expr, word) {
		return expr, nil
	}
	if !lc.spawnAtk {
		return expr, refuse("filter %q: Spawner>TriggeredDefendingPlayer outside an ImmediateTrigger spawned by Card.Self's Attacks trigger", expr)
	}
	return strings.ReplaceAll(expr, word, "ControlledBy TriggeredDefendingPlayer"), nil
}

// spawnAttacked reports whether ab is a triggered ability whose trigger is
// `Mode$ Attacks | ValidCard$ Card.Self` (Ace, Fearless Rebel's Nitro-9).
func spawnAttacked(ab *lAbility) bool {
	if ab == nil || ab.Trigger == nil || ab.Latent {
		return false
	}
	t := ab.Trigger
	if t.Event != "Attacks" || t.F2 != nil || t.F == nil || len(t.F.Clauses) != 1 {
		return false
	}
	c := t.F.Clauses[0]
	return c.Flags&fSelf != 0 && c.Ctrl == whoAny
}

// spawnDamagedOpp reports whether ab is a triggered ability whose trigger is
// a DamageDone trigger on this card dealing combat damage to a player.
func spawnDamagedOpp(ab *lAbility) bool {
	if ab == nil || ab.Trigger == nil || ab.Latent {
		return false
	}
	t := ab.Trigger
	if t.Event != "DamageDone" || t.Combat != 1 || t.F2 == nil || t.F2.Players != plBoth || len(t.F2.Clauses) != 0 || t.F == nil || len(t.F.Clauses) != 1 {
		return false
	}
	c := t.F.Clauses[0]
	return c.Flags&fSelf != 0 && c.Ctrl == whoAny
}

// the four per-permanent mark words (kernel PredPermMark).

// permMarkWord maps a filter property word, with or without the ! prefix,
// to PredPermMark's PredN.
func permMarkWord(p string) (n int, ok bool) {
	if len(p) > 0 && p[0] == '!' {
		n, ok = permMarkWord(p[1:])
		return n | kop.PermMarkNot, ok
	}
	switch p {
	case "attackedThisTurn":
		return kop.PermMarkAttacked, true
	case "dealtDamageThisTurn":
		return kop.PermMarkDealtTurn, true
	case "dealtDamagetoAny":
		return kop.PermMarkDealtAny, true
	case "dealtCombatDamagetoAny":
		return kop.PermMarkDealtCombat, true
	}
	return 0, false
}

// cmpPowerGTBase is an explicit high comparison kind,
// clear of the cmp iota tail: the filter word powerGTbasePower (gorge
// numericPred over currentPower and basePowerValue). It lowers to
// kop.PredPowerGTBase. Any other two-characteristic spelling
// (powerLTbasePower, toughnessGTbaseToughness, ...) keeps refusing.
const cmpPowerGTBase uint8 = 202

// cmpNotF2 is an explicit high comparison kind, clear of
// the cmp iota tail: the '!' negation of one of the three F2 flag words
// (gorge matchPredicate negates any positive word). CmpN is the negated
// Flags2 bit's index; it lowers to kop.PredNotF2. FilterFlags2 is full, so
// the negations cannot be F2 bits of their own.
const cmpNotF2 uint8 = 201

// notF2Bit names the F2 bit of a '!'-negatable flag word: targetedBy,
// ThisTurnEntered and wasDealtDamageThisTurn (the kernel's dmgTurn record
// outlives a zone change, as gorge's does on the graveyard object). Any
// other negated word is not read here and keeps refusing.
func notF2Bit(w string) (int, bool) {
	var f kop.FilterFlags2
	switch w {
	case "targetedBy":
		f = kop.F2TargetedBy
	case "ThisTurnEntered":
		f = kop.F2EnteredThisTurn
	case "wasDealtDamageThisTurn":
		f = kop.F2WasDamaged
	default:
		return 0, false
	}
	return bits.TrailingZeros32(uint32(f)), true
}

// filterReadsNotBlocking reports whether a clause of f reads the negated
// filter word !blocking (kop.W7NotBlocking).
func filterReadsNotBlocking(f *lFilter) bool {
	if f == nil {
		return false
	}
	for _, c := range f.Clauses {
		if c.CmpStat == cmpWord7 && c.CmpN == int(kop.W7NotBlocking) {
			return true
		}
	}
	return false
}

// cutNon is the word a non<word> property negates; nonSnow is no negation
// but its own word (filterWordPred3, kop.W7NonSnow).
func cutNon(p string) (string, bool) {
	if p == "nonSnow" {
		return "", false
	}
	return strings.CutPrefix(p, "non")
}
