package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// changesZoneAll lowers Mode$ ChangesZoneAll ("whenever one or more ...
// enter / die / leave"). gorge (trigmatch/zone.go ZoneChangeMatches, one
// matcher for ChangesZone and ChangesZoneAll; trigger_match.go's zone batch
// latch) matches each move like ChangesZone and, inside an open zone batch,
// queues the line once, at its first matching move; outside a batch every
// move is its own batch. So the line is the ChangesZone event with the
// Batch flag: the kernel fires it per move and collapses it per batch
// (kernel trig_oneormore.go). Shapes: Destination$ Battlefield (any origin);
// Origin$ Battlefield to Graveyard (creatures: the kernel's dies event);
// Origin$ Battlefield to any zone, or to every zone but the graveyard
// ("without dying", Dour Port-Mage). A body reading the batch (its moved
// set, Remembered/TriggeredCards, or the moved count) refuses: the
// kernel's queued instance carries only the first member.
func (lc *lowerCtx) changesZoneAll(tp *params, tr *lTrigger, t cards.Trigger) error {
	v, ok := tp.get("ValidCards")
	if !ok {
		return refuse("ChangesZoneAll without ValidCards")
	}
	var err error
	lc.wasCastOK = tp.str("Destination") == "Battlefield" // wasCast reads the entering spell
	tr.F, err = lc.filter(v)
	lc.wasCastOK = false
	if err != nil {
		return err
	}
	if tr.F.Players != 0 || len(tr.F.Clauses) == 0 {
		return refuse("ChangesZoneAll ValidCards %s", tr.F)
	}
	for _, c := range tr.F.Clauses {
		if c.Flags&fSelf != 0 {
			return refuse("ChangesZoneAll ValidCards %s", tr.F)
		}
	}
	origin, dests := zAny, map[int]bool{}
	bfGrave := false
	if o, ok := tp.get("Origin"); ok {
		if origin, ok = zoneNames[o]; !ok {
			if bfGrave = isBattlefieldGraveSet(o); !bfGrave {
				return refuse("ChangesZoneAll Origin %q", o)
			}
		}
	}
	d, ok := tp.get("Destination")
	if !ok {
		return refuse("ChangesZoneAll without Destination")
	}
	for _, z := range strings.Split(d, ",") {
		zz, ok := zoneNames[strings.TrimSpace(z)]
		if !ok && strings.TrimSpace(z) != "Ante" {
			return refuse("ChangesZoneAll Destination %q", d)
		}
		dests[zz] = true
	}
	notGrave := origin == zBattlefield && len(dests) >= 3 && dests[zExile] && dests[zHand] && dests[zLibrary] && !dests[zGraveyard] && !dests[zAny] && !dests[zBattlefield]
	switch {
	case len(dests) == 1 && dests[zBattlefield] && origin == zAny:
		tr.Event = "ETB"
	case len(dests) == 1 && dests[zGraveyard] && origin == zBattlefield:
		for _, c := range tr.F.Clauses {
			if c.Types != tCreature {
				return refuse("ChangesZoneAll dies of %s", tr.F)
			}
		}
		tr.Event = "Dies"
	case len(dests) == 1 && dests[zAny] && origin == zBattlefield:
		tr.Event = "Leaves"
	case len(dests) == 1 && dests[zAny] && origin == zGraveyard:
		tr.Event, tr.From = "Leaves", zGraveyard
	case notGrave:
		tr.Event, tr.NotToGrave = "Leaves", true
	case bfGrave && len(dests) == 1 && dests[zExile]:
		// Origin$ Battlefield,Graveyard to exile (Ketramose, the New Dawn):
		// the Leaves line to exile plus the graveyard leave to exile, one
		// batch line (kop.Tf2LeavesGrave, kernel trig_leavesgrave.go)
		tr.Event, tr.N, tr.AlsoGrave = "Leaves", 1<<zoneBit(zExile), true
	default:
		return refuse("ChangesZoneAll %s->%s", zoneLabel[origin], d)
	}
	tr.Batch = true
	if lc.saReadsBatch(t.Effect) {
		// the one batch read that lowers: the exile line's body picks a card
		// among the batch's members (kayaBatchBody); every other read refuses
		if !tr.AlsoGrave || !kayaBatchBody(t.Effect) {
			return refuse("ChangesZoneAll body reads the batch")
		}
		lc.batchPoolOK = true
	}
	return nil
}

// kayaBatchBody reports exactly Kaya, Spirits' Justice's trigger body: one DB$
// Clone whose Choices$ pool is the exiled batch's creature cards
// (Card.TriggeredCards+Creature, ChoiceZone$ Exile), the target token
// (ValidTgts$ the battlefield, the becomer) copying the pick until end of
// turn with a keyword exception. The kernel's queued instance holds the
// batch's members for that pick only (trig_batchmembers.go).
func kayaBatchBody(sa *cards.SA) bool {
	if sa == nil || sa.API != "Clone" || sa.Sub != nil {
		return false
	}
	p := newParams(sa.Params)
	for _, k := range []string{"Optional", "CloneTarget", "Defined", "ChoiceOptional", "ExcludeChosen"} {
		if _, ok := p.get(k); ok {
			return false
		}
	}
	return p.is("Choices", "Card.TriggeredCards+Creature") && p.is("ChoiceZone", "Exile") &&
		p.is("ValidTgts", "Permanent.token+YouCtrl") && p.is("Duration", "UntilEndOfTurn") &&
		p.is("AddKeywords", "Flying")
}

// blockedByCreature lowers Mode$ AttackerBlockedByCreature (gorge
// queueAttackerBlockedTrigger's pair arm, attackerBlockedByPairCandidates):
// one instance per declared (attacker, blocker) pair whose attacker matches
// ValidCard$ and whose blocker matches ValidBlocker$. Only the two anchored
// shapes lower: ValidCard$ Card.Self ("becomes blocked by", the trigger
// object -- gorge's Remembered/TriggerCard -- is the blocker) and
// ValidBlocker$ Card.Self ("blocks", the trigger object is the attacker,
// TriggerBlocker the source). The kernel fires it with the become-blocked
// listeners (gorge checkAttackerBlockedTriggers walks both modes in face
// order).
func (lc *lowerCtx) blockedByCreature(tp *params, tr *lTrigger) error {
	vc, vb := tp.str("ValidCard"), tp.str("ValidBlocker")
	switch {
	case vc == "Card.Self" && vb != "" && !strings.Contains(vb, "Self") && !strings.Contains(vb, "By"):
	case vb == "Card.Self" && vc != "" && !strings.Contains(vc, "Self") && !strings.Contains(vc, "By"):
		tr.AnchorBlocker = true
	default:
		return refuse("AttackerBlockedByCreature ValidCard %q ValidBlocker %q", vc, vb)
	}
	var err error
	if tr.F, err = lc.filter(vc); err != nil {
		return err
	}
	if tr.F2, err = lc.filter(vb); err != nil {
		return err
	}
	tp.consume("ValidCard", "ValidBlocker")
	tr.Event = "AttackerBlocked"
	tr.ByCreature = true
	return nil
}

// isBattlefieldGraveSet reports an Origin$ list naming exactly the
// battlefield and the graveyard, in either order (gorge's ZoneList admits a
// move whose origin is in the set).
func isBattlefieldGraveSet(o string) bool {
	ws := strings.Split(o, ",")
	if len(ws) != 2 {
		return false
	}
	a, b := strings.TrimSpace(ws[0]), strings.TrimSpace(ws[1])
	return a == "Battlefield" && b == "Graveyard" || a == "Graveyard" && b == "Battlefield"
}
