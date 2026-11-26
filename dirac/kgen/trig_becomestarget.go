package kgen

import (
	"strings"
)

func init() {
	triggerMode("BecomesTarget", trigBecomesTarget)
}

// trigBecomesTarget lowers the Mode$ BecomesTarget trigger arm, moved out of lowerTriggered (trigger_reg.go).
func trigBecomesTarget(lc *lowerCtx, a *trigArm) error {
	tp, tr := a.tp, a.tr
	var err error
	switch {
	case tp.is("Ward", "True") && tp.str("ValidTarget") == "Card.Self":
		tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
		tr.Event = "BecomesTargetByOpp"
	case tp.is("Valiant", "True") && tp.str("ValidTarget") == "Card.Self" &&
		tp.str("ValidSource") == "SpellAbility.YouCtrl":
		// Valiant (CR 603.2 ability word; gorge
		// trigmatch.valiantFirstTimeTargetedByYouThisTurn): the first
		// targeting this turn by a spell or ability its controller
		// controls; the kernel's per-turn mark is in kw_valiant.go
		tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
		tr.Valiant = true
	case tp.str("ValidSource") == "SpellAbility.OppCtrl" &&
		tp.str("ValidTarget") == "Card.Self" &&
		!tp.is("Ward", "True") && !tp.is("Valiant", "True") && !tp.is("FirstTime", "True"):
		// "Whenever this becomes the target of a spell or ability an
		// opponent controls" (CR 603.2; gorge trigmatch
		// becomesTargetMatches with ValidTarget$ Card.Self and
		// ValidSource$ SpellAbility.OppCtrl). This is the watcher form
		// whose filter is Self: gorge matches ValidTarget$ against the
		// TargetsChosen event's chosen Ids, so the trigger object is the
		// targeted permanent itself (becomesTargetReferent). The kernel
		// fires it through btWatchers, whose objTrigGen self match
		// restricts a TfSelf line to the targeted permanent
		// (kernel trig_becomestarget.go). FirstTime$ would need
		// gorge's per-object once-a-turn gate, which the kernel does not
		// model here: refuse it rather than fire every time.
		tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
		tr.Whose = whoOpp
		tr.Watch = true
	case tp.str("ValidSource") == "SpellAbility.OppCtrl" &&
		tp.str("ValidTarget") != "" && !strings.Contains(tp.str("ValidTarget"), "Card.Self"):
		// a watcher (Pawpatch Recruit, Surrak, Elusive Hunter): each
		// object an opponent's spell or ability targets that matches
		// ValidTarget$ (gorge becomesTargetMatches, one TargetsChosen
		// event per chosen target); the trigger object is the targeted
		// one (gorge becomesTargetReferent). A creature-spell
		// alternative (Spell.Creature+YouCtrl) is a flag on the stack
		// target walk (kernel Tf2SpellToo).
		var alts []string
		for _, a := range strings.Split(tp.str("ValidTarget"), ",") {
			if a == "Spell.Creature+YouCtrl" {
				tr.SpellToo = true
				continue
			}
			alts = append(alts, a)
		}
		if len(alts) == 0 {
			return refuse("BecomesTarget shape")
		}
		if tr.F, err = lc.btTargetList(tr, alts); err != nil { // lower_bt_source.go
			return err
		}
		tr.Whose = whoOpp
		tr.Watch = true
	case isBTSpellSourceLine(tp):
		// the spell source forms: lower_bt_source.go
		if err := lc.lowerBTSpellSource(tp, tr); err != nil {
			return err
		}
	case tp.str("ValidSource") == "" && tp.str("ValidTarget") == "Card.Self" &&
		!tp.is("Ward", "True") && !tp.is("Valiant", "True"):
		// "Whenever this becomes the target of a spell or ability" with
		// no source restriction (CR 603.2; gorge becomesTargetMatches
		// with ValidTarget$ Card.Self and no ValidSource$): every
		// TargetsChosen naming the permanent fires it, its controller's
		// own included. A TfSelf line with no Whose gate: the kernel's
		// becameTarget fires it through the plain per-object walk
		// (fireObjGen), no opponent gate. FirstTime$ True is this
		// shape's once-a-turn rider (btSelfFirstTime); any other shape
		// carrying it falls to the default refuse.
		tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
		if err := btSelfFirstTime(tp, tr); err != nil { // lower_bt_source.go
			return err
		}
	case tp.str("ValidSource") == "" && tp.str("ValidTarget") != "" &&
		!strings.Contains(tp.str("ValidTarget"), "Card.Self") &&
		!tp.is("Ward", "True") && !tp.is("Valiant", "True") && !tp.is("FirstTime", "True"):
		// the watcher with no source restriction (Daru Spiritualist:
		// "whenever a Cleric creature you control becomes the target of
		// a spell or ability"): the opponent-source watcher's arm
		// without its Whose gate; one trigger per matching chosen
		// target, the targeted permanent as the trigger object
		// (gorge becomesTargetReferent). ValidTarget$ may be a comma
		// list of permanent filters: the OR of the alternatives (gorge
		// matches the whole ValidTarget$ against each chosen target).
		if tr.F, err = lc.btTargetList(tr, strings.Split(tp.str("ValidTarget"), ",")); err != nil { // lower_bt_source.go
			return err
		}
		tr.Watch = true
	default:
		return refuse("BecomesTarget shape")
	}
	return nil
}
