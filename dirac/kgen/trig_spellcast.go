package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

func init() {
	triggerMode("SpellCast", trigSpellCast)
	triggerMode("SpellCastOrCopy", trigSpellCast)
}

// trigSpellCast lowers the Mode$ SpellCast and SpellCastOrCopy trigger arms, moved out of lowerTriggered (trigger_reg.go).
func trigSpellCast(lc *lowerCtx, a *trigArm) error {
	t, tp, tr, readF := a.t, a.tp, a.tr, a.readF
	var err error
	if t.Mode == "SpellCastOrCopy" {
		// gorge trigmatch: the cast (PutOnStack) or a spell copy
		// (StackCopy) matches; the cast-count gates read casts only
		tr.Event, tr.OrCopy = "SpellCast", true
		for _, k := range []string{"ActivatorThisTurnCast", "TargetsValid", "OpponentTurn"} {
			if _, ok := tp.get(k); ok {
				return refuse("SpellCastOrCopy with %s$", k)
			}
		}
	}
	if v, ok := tp.get("Static"); ok {
		// gorge resolves a Static$ True SpellCast trigger off the
		// stack by the trigger drain, before the CR 603.3b ordering
		// ask (engine/rules/static_trigger.go): no priority pass, no
		// order choice. Only a body PROVABLY ask-free may resolve off
		// the stack — gorge's staticTriggerAskFree refuses a line
		// with an optional/unless/cost rider, a target, a Charm or an
		// asking body, and so does the body check after the chain
		// lowers. Every other mode keeps refusing
		// Static$ as unread.
		if t.Mode != "SpellCast" || v != "True" {
			return refuse("%s Static$ %q", t.Mode, v)
		}
		for _, k := range [...]string{"OptionalDecider", "Optional", "UnlessCost", "UnlessSwitch", "UnlessResolveSubs", "UnlessPayer", "Cost"} {
			if strings.TrimSpace(t.Params[k]) != "" {
				return refuse("static SpellCast trigger %s$ %q (an off-stack resolution cannot ask)", k, t.Params[k])
			}
		}
		tr.Static = true
	}
	lc.spellProvOK = true // the cast-provenance words read the spell cast
	// a "Chosen" bound reads the number the source chose as it entered (lower_choosenumber.go)
	lc.chosenNumOK = lc.granted == 0 && tp.str("TriggerZones") == "Battlefield" && entersChoosingNumber(lc.face)
	tr.F, err = readF("ValidCard")
	lc.spellProvOK, lc.chosenNumOK = false, false
	if err != nil {
		return err
	}
	w, ok := whoOfPlayer(tp.str("ValidActivatingPlayer"))
	if !ok {
		w, ok = castNonActive(tr, t.Mode, tp.str("ValidActivatingPlayer"))
		if ok && tp.is("OpponentTurn", "True") {
			return refuse("ValidActivatingPlayer %q with OpponentTurn$", tp.str("ValidActivatingPlayer"))
		}
	}
	if !ok {
		return refuse("ValidActivatingPlayer %q", tp.str("ValidActivatingPlayer"))
	}
	tr.Whose = w
	if tp.is("OpponentTurn", "True") {
		tr.OnlyOppTurn = true
	}
	if v, ok := tp.get("ValidSA"); ok {
		// "whenever you cast a spell using mana produced by CARDNAME"
		// (gorge trigmatch validSAManaFromCardSelf): the cast's payment
		// tapped this very source. Only the exact spelling, on a plain
		// SpellCast of a permanent source (kernel Tf2ManaFromSelf).
		// "Spell.ManaSpent <op><n>" is the spend gate (spentGateOf).
		// "Spell.MayPlaySource" (gorge validSAMayPlaySourceClause) reads
		// the cast's FlagMayPlay: the may-play permission provenance,
		// the flag-only approximation of Forge's host-specific
		// MayPlaySource. It lowers as kernel
		// Tf3MayPlaySrc, a trigger flag read off the spell's stack
		// entry as the trigger fires — a filter clause would be
		// dropped by fireSelfCast's TfSelf filter stripping, so one
		// flag covers the Card.Self carriers (Undead Sprinter) too.
		if took, err := lc.validSAHead(v, t.Mode, tr); took {
			if err != nil {
				return err
			}
		} else if gate, ok := spentGateOf(v, false); ok && t.Mode == "SpellCast" {
			tr.SpentGate = gate
		} else if v == "Spell.MayPlaySource" && t.Mode == "SpellCast" {
			tr.MayPlaySrc = true
		} else if v != "Spell.ManaFromCard.StrictlySelf" || t.Mode != "SpellCast" {
			return refuse("spell-ability filter ValidSA %q%s", v, validSAWhy(v)) // the T12 spell-ability filter guard (T13c's words)
		} else {
			tr.ManaFromSelf = true
		}
	}
	if v, ok := tp.get("ValidSAonCard"); ok {
		// gorge spellValidSAonCardMatches: the same mana-spent
		// comparison over the cast spell, X naming the spell's printed
		// mana value. Only that spelling, on a plain SpellCast, as the
		// sole spend gate (kernel spentGateOK).
		gate, ok := spentGateOf(v, true)
		if !ok || t.Mode != "SpellCast" || tr.SpentGate != 0 {
			return refuse("ValidSAonCard %q on %s", v, t.Mode)
		}
		tr.SpentGate = gate
	}
	if v, ok := tp.get("ActivatorThisTurnCast"); ok {
		// gorge CompareIntCount over activatorCastCount (the caster's
		// spells this turn that match ValidCard$, this one included): the
		// six operators, a literal 1..127
		cmp, n, ok := trigCmp(v)
		if !ok {
			return refuse("ActivatorThisTurnCast %q", v)
		}
		switch vc := t.ParamStr(cards.PKValidCard); {
		case vc == "" || vc == "Card" || vc == "Card.YouCtrl":
		case tr.CastHand || strings.Contains(vc, "wasCast") || strings.Contains(vc, "CastSa"):
			return refuse("ActivatorThisTurnCast over cast provenance %q", vc)
		default:
			tr.NthFiltered = true
			for _, c := range tr.F.Clauses {
				if why := castRingClauseInexact(c); why != "" {
					return refuse("ActivatorThisTurnCast over ValidCard %q: %s", vc, why)
				}
			}
		}
		if cmp != uint8(kop.DmgEQ) {
			tr.NthCmp = cmp
		}
		tr.NthSpell = n
	}
	if v, ok := tp.get("TargetsValid"); ok {
		if tr.OrTargets {
			return refuse("TargetsValid beside an IsTargeting ValidSA")
		}
		// gorge TargetShapeMatches / targetMatchesTargetsValid: the
		// trigger fires if ANY target of the spell's chain matches ANY
		// comma alternative, a player target through the player spec,
		// an object target through the object spec. Lowered for the
		// object alternatives that are on the battlefield (Card.Self,
		// or an explicit inZoneBattlefield), a zone-free type
		// alternative (Danitha, Spear of Agony's Creature.OppCtrl, which
		// also matches a creature SPELL on the stack: effects
		// matchesBase reads the type in any zone), and one You or
		// Opponent player alternative (kernel castTargetsPlayer). The
		// zone-free alternatives carry kop.ZoneAny on Filter2, the marker
		// the kernel's castTargetsAll reads for an off-battlefield
		// target through the card matcher. A line mixing a battlefield
		// and a zone-free object alternative stays refused: the kernel
		// would read the battlefield one in any zone too.
		if err := lc.targetsValid(tr, v, false); err != nil {
			return err
		}
	}
	if v, ok := tp.get("Increment"); ok {
		// gorge kwIncrement's Increment$ marker (trigmatch
		// IncrementHolds): the source is a creature and the cast's spend
		// exceeds its current power/toughness, read as the trigger fires
		// and rechecked on resolution (kernel Tf2Increment). Only the exact spelling, on a
		// plain SpellCast (gorge's SpellCastOrCopy carries it on the cast
		// event only; the kernel's copy arm reads casts only).
		if t.Mode != "SpellCast" || v != "True" {
			return refuse("Increment %q on %s", v, t.Mode)
		}
		tr.Increment = true
	}
	return nil
}
