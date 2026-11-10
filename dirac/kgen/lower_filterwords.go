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
	}
	z, found := strings.CutPrefix(p, "inZone")
	if !found {
		return filterWordPred2(p) // lower_filterwords.go
	}
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
// hasKeywordDisguise stay unknown.
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
	case "ManaCostPartialBlue":
		return int(kop.W7CostBlue), true
	case "NoAbilities":
		return int(kop.W7NoAbilities), true
	case "IsCommander":
		return int(kop.W7IsCommander), true
	case "ThisTurnEnteredFrom_Battlefield":
		return int(kop.W7EnteredFromBF), true
	case "BottomLibrary":
		return int(kop.W7BottomLibrary), true
	case "IsImprinted":
		return int(kop.W7Imprinted), true // gated to a chain that writes the set (lower_imprintzone.go)
	case "sharesCreatureTypeWith TriggeredAttacker", "sharesCreatureTypeWith TriggeredAttackerLKICopy":
		// the Attacks trigger's attacker (lowerCtx.referentGate holds it to
		// an Attacks trigger's body, where trigObj is that attacker)
		return int(kop.W7SharesCrTrigObj), true
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

// spawnerTargetCtrl rewrites the filter word "ControlledBy
// Spawner>TriggeredTarget" (gorge controlReferentPlayers spawnerChain: the
// spawning trigger's target player) to OppCtrl inside an ImmediateTrigger
// body spawned by a combat-damage-to-a-player trigger whose source is the
// card itself (lc.spawnDmg, set by immediateTrigger): two-player, the
// player a creature deals combat damage to is the one opponent (CR 506.4).
// Anywhere else the word refuses with a named reason.
func (lc *lowerCtx) spawnerTargetCtrl(expr string) (string, error) {
	const word = "ControlledBy Spawner>TriggeredTarget"
	if !strings.Contains(expr, word) {
		return expr, nil
	}
	if !lc.spawnDmg {
		return expr, refuse("filter %q: Spawner>TriggeredTarget outside an ImmediateTrigger spawned by Card.Self's combat damage to a player", expr)
	}
	return strings.ReplaceAll(expr, word, "OppCtrl"), nil
}

// spawnDamagedOpp reports whether ab is a triggered ability whose trigger is
// `Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Player |
// CombatDamage$ True`.
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
