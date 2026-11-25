package kgen

import (
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// srcInPlay reports whether a damage trigger's ValidSource$ demands
// inZoneBattlefield (gorge wordInZone: a live zone test, so a spell or an
// ability whose source left play never matches; the generic filter drops the
// word as implied by its battlefield contexts, which is false here). A comma
// list where only some alternatives carry it is refused.
func srcInPlay(vs string) (bool, error) {
	n, alts := 0, strings.Split(vs, ",")
	for _, a := range alts {
		if strings.Contains(a, "inZoneBattlefield") {
			n++
		}
	}
	if n != 0 && n != len(alts) {
		return false, refuse("damage ValidSource %q: inZoneBattlefield on some alternatives only", vs)
	}
	return n != 0, nil
}

// damageDoneOnce lowers Mode$ DamageDoneOnce: one trigger per damaged
// referent (the object or player a Damage event lands on) per damage batch,
// carrying the batch total of the matching events to that referent (gorge
// trigger_match.go damageBatchKey; a combat damage step, one DealDamage or
// DamageAll resolution is a batch, an event outside one a batch of one).
// ValidSource$ is F, ValidTarget$ is F2 (objects only or players only; no
// ValidTarget$ is every referent), CombatDamage$ is Combat, DamageAmount$
// <cmp><literal> gates the batch total (Tf2PerReferent, kernel
// trig_damagedonce.go). Every DamageDoneOnce line takes this one path.
func (lc *lowerCtx) damageDoneOnce(tp *params, tr *lTrigger, t cards.Trigger) error {
	var err error
	if v, ok := tp.get("ValidSource"); ok {
		if tr.SrcInPlay, err = srcInPlay(v); err != nil {
			return err
		}
		if tr.F, err = lc.filter(v); err != nil {
			return err
		}
	}
	if tp.is("FirstTime", "True") {
		return refuse("DamageDoneOnce FirstTime$")
	}
	if cd, ok := tp.get("CombatDamage"); ok {
		if cd == "True" {
			tr.Combat = 1
		} else {
			tr.Combat = -1
		}
	}
	if vt, ok := tp.get("ValidTarget"); ok {
		if tr.F2, err = lc.filter(vt); err != nil {
			return err
		}
		switch {
		case tr.F2.Players != 0 && len(tr.F2.Clauses) != 0:
			return refuse("DamageDoneOnce ValidTarget %q mixes players and objects", vt)
		case tr.F2.Players != 0 && tr.F2.Players != plBoth && tr.F2.Players != plOpp && tr.F2.Players != plYou:
			return refuse("DamageDoneOnce ValidTarget %q", vt)
		}
	}
	if ga, ok := tp.get("DamageAmount"); ok {
		ga = strings.TrimSpace(ga)
		cmp, ok := map[string]kop.DmgCmp{"GE": kop.DmgGE, "LE": kop.DmgLE, "EQ": kop.DmgEQ, "NE": kop.DmgNE, "GT": kop.DmgGT, "LT": kop.DmgLT}[ga[:min(2, len(ga))]]
		lit, err := strconv.Atoi(strings.TrimPrefix(ga, ga[:min(2, len(ga))]))
		if !ok || err != nil || lit < 0 || lit > 255 {
			return refuse("DamageDoneOnce DamageAmount %q", ga)
		}
		tr.DmgGate = kop.DmgGate(cmp, lit)
	}
	tr.Event = "DamageDone"
	if lc.ddoEvent {
		// an end-of-turn Effect's line: gorge's delayed-trigger path latches
		// the damage batch per damaged referent (trigger_delayed.go
		// EffectRepeat arm), so this is the per-referent line the emblem hook
		// fires (fireEmblemsDmg): Mode$ DamageDoneOnce without the object
		// Filter2/Tf2ToYou pair, marked Tf2PerReferent for the refLatch. Its
		// body reads still gate: the line's trigger object is the damage
		// source and its trigger player the damaged seat, so a read the
		// per-referent queue would carry (ddoReadOK) is not available here,
		// and an unconditional plain read (TriggeredCardController lowers to
		// the trigger player) would name the damaged seat where gorge reads
		// the source's controller. ddoEmblemReads admits exactly what this
		// line's plain lowering lowers exactly.
		if tr.DmgGate != 0 {
			return refuse("Effect DamageDoneOnce trigger with DamageAmount")
		}
		if r := lc.ddoEmblemReads(t.Effect); r != "" {
			return refuse("Effect DamageDoneOnce body reads %s", r)
		}
		// Batch marks the once-per-batch lowering and sets gen.anyBatch (the
		// combat damage step opens its batch); PerRef marks the per-referent
		// latch fireEmblemsDmg applies through the refLatch.
		tr.Batch, tr.PerRef = true, true
		return nil
	}
	tr.Batch, tr.PerRef = true, true
	if r := lc.ddoReads(t.Effect); r != "" {
		return refuse("DamageDoneOnce body reads %s", r)
	}
	return nil
}

// damageDoneSelfObjRef gives a plain DamageDone line from the card itself to
// an object, whose body reads the damaged creature, the referent the plain
// damage trigger does not carry (its trigger object is the damage SOURCE).
// gorge fires the line once per damage event, and one source deals at most
// one damage event to a given object per batch (a combat damage step, one
// fight, one DealDamage), so the kernel's per-referent queue carries the
// damaged permanent as the trigger object exactly. Any other trigger-context
// read that queue does not carry refuses by name. A line without the read
// keeps the plain trigger.
func (lc *lowerCtx) damageDoneSelfObjRef(tp *params, tr *lTrigger, t cards.Trigger) error {
	if t.Mode != "DamageDone" || tr.F2 == nil || !lc.ddoReadsTriggeredTarget(t.Effect) {
		return nil
	}
	objF2 := tr.F2.Players == 0 && len(tr.F2.Clauses) != 0
	// a host's attachment (Charisma, Sword of Kaldra) names one source object,
	// which deals at most one damage event to a referent per batch, as Self
	hostSrc := tp.is("ValidSource", "Card.AttachedBy") || tp.is("ValidSource", "Card.EquippedBy")
	if !(tp.is("ValidSource", "Card.Self") && (objF2 || selfPlayerRefShape(tr)) || hostSrc && objF2) {
		return nil
	}
	tr.Batch, tr.PerRef = true, true
	tr.SelfPlayerRef = selfPlayerRefShape(tr)
	if r := lc.ddoReads(t.Effect); r != "" {
		return refuse("DamageDone body reads %s", r)
	}
	return nil
}

// selfPlayerRefShape reports whether a plain DamageDone line from the card
// itself to any player (ValidTarget$ Player) that is not combat-only reads
// the damaged player (Abyssal Specter). The plain trigger's player is the
// source controller's opponent at two seats, wrong when the Specter hurts its
// own controller; the per-referent queue carries the damaged player itself
// (ddoRecipient), seat-exact at any seat count. A combat-only line keeps the plain trigger, whose damaged
// player is the defending player (damagedPlayerIsTriggerPlayer).
func selfPlayerRefShape(tr *lTrigger) bool {
	return tr.Combat != 1 && tr.F2 != nil && tr.F2.Players == plBoth && len(tr.F2.Clauses) == 0
}

// ddoPlayerReferent reports whether tr is a per-referent line over players
// only: the trigger player is the damaged player (ddoRecipient queues the
// damaged player for a player referent). selfPlayerRefShape gives the plain
// DamageDone one; a DamageDoneOnce line whose ValidTarget$ is Player, You
// or Opponent is the other (Rev, Tithe Extractor's "look at the top card of
// that player's library").
func ddoPlayerReferent(tr *lTrigger) bool {
	return tr != nil && tr.Event == "DamageDone" && tr.PerRef &&
		tr.F2 != nil && tr.F2.Players != 0 && len(tr.F2.Clauses) == 0
}

// ddoBodyReads lists every trigger-context spelling a DamageDone body reads,
// the SVars it names transitively included, in text order. A body of one
// admitted spelling and one refused spelling yields both.
func (lc *lowerCtx) ddoBodyReads(sa *cards.SA) []string {
	// read appends every trigger-context spelling in s, in text order: a
	// body can read an admitted spelling and a refused one, so a first-only
	// scan would miss the second.
	read := func(s string, out []string) []string {
		s = strings.ReplaceAll(s, "TriggerCount$DamageAmount", "")
		for i := 0; i < len(s); {
			n := 0
			switch {
			case strings.HasPrefix(s[i:], "TriggerCount"):
				n = len("TriggerCount")
			case strings.HasPrefix(s[i:], "Triggered"):
				n = len("Triggered")
			default:
				i++
				continue
			}
			j := i + n
			for j < len(s) && (s[j] == '$' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
				j++
			}
			out = append(out, s[i:j])
			i = j
		}
		return out
	}
	var reads []string
	seen := map[string]bool{}
	var work []string
	for ; sa != nil; sa = sa.Sub {
		reads = read(sa.Line, reads)
		for _, v := range sa.Params {
			work = append(work, v)
		}
	}
	// Only the SVars the body names (transitively) are this body's reads:
	// another ability's SVar on the same face (Yarus, Roar of the Old Gods'
	// ChangesZone return beside its DamageDoneOnce draw) is not, exactly as
	// saReadsBatch below scopes its own batch scan.
	for len(work) > 0 {
		w := work[len(work)-1]
		work = work[:len(work)-1]
		for _, name := range strings.FieldsFunc(w, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_')
		}) {
			if body, ok := lc.face.SVars[name]; ok && !seen[name] {
				seen[name] = true
				work = append(work, body)
			}
		}
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		reads = read(lc.face.SVars[k], reads)
	}
	return reads
}

// ddoReads names the first trigger-context read of a DamageDone body the
// per-referent trigger does not carry exactly. The batch total
// (TriggerCount$DamageAmount, the pending amount) is carried; so are the
// referent shapes ddoReadOK admits (the damaged object for an object-only
// ValidTarget$, the one attacking player for combat damage to You). Every
// other Triggered*/TriggerCount* spelling refuses (the effect lowerer's own
// Remember* and Triggered* readers keep their rules).
func (lc *lowerCtx) ddoReads(sa *cards.SA) string {
	for _, r := range lc.ddoBodyReads(sa) {
		if !ddoReadOK(r, lc.ab.Trigger) {
			return r
		}
	}
	return ""
}

// ddoEmblemReads names the first trigger-context read of an end-of-turn
// Effect's DamageDoneOnce body that the per-referent emblem line
// (fireEmblemsDmg) does not carry exactly. The line is a plain DamageDone
// trigger: its trigger object is the damage source and its trigger player
// the damaged seat, and its ValidTarget$ is players only
// (eotCombatEmblemRefuse), so the carried reads are the batch amount
// (TriggerCount$DamageAmount, stripped by ddoBodyReads) and the damaged
// player, which the plain lowering lowers exactly only where the kernel's
// trigger player is that player (damagedPlayerIsTriggerPlayer) — the
// plain TriggeredTarget (Defined$ TriggeredTarget) and the LifeTotal
// property (TriggeredTarget$LifeTotal/HalfUp, the X of Aphelia's
// LoseLife). Everything else refuses: TriggeredCardController lowers to
// the trigger player — the damaged seat — where gorge reads the damage
// source's controller.
func (lc *lowerCtx) ddoEmblemReads(sa *cards.SA) string {
	dpt := lc.damagedPlayerIsTriggerPlayer()
	for _, r := range lc.ddoBodyReads(sa) {
		if dpt {
			switch r {
			case "TriggeredTarget", "TriggeredTargetLKICopy":
				continue
			}
			if p, ok := strings.CutPrefix(r, "TriggeredTarget$"); ok && p == "LifeTotal" {
				continue
			}
			if p, ok := strings.CutPrefix(r, "TriggeredTargetLKICopy$"); ok && p == "LifeTotal" {
				continue
			}
		}
		return r
	}
	return ""
}

// ddoReadsTriggeredTarget reports whether the body reads the damaged object
// (TriggeredTarget / TriggeredTargetLKICopy): the shape a plain DamageDone
// line from the card itself may carry as its trigger object.
func (lc *lowerCtx) ddoReadsTriggeredTarget(sa *cards.SA) bool {
	for _, r := range lc.ddoBodyReads(sa) {
		if r == "TriggeredTarget" || r == "TriggeredTargetLKICopy" {
			return true
		}
	}
	return false
}

// saReadsBatch: some line or parameter of the body, or an SVar it names (transitively), reads the
// trigger's batch: its members (Triggered*, TriggerObjects*), its count (TriggerCount) or its
// remembered set (TriggerRemembered). A plain Remembered is not a batch read: gorge's
// rememberedExcludingCapture (engine/effects/count_ref.go) hides the capture closeZoneBatch seeds
// with the moved set, so inside the body it holds only what the body itself remembered; the
// kernel's trigger-body remembered set starts empty too.
func (lc *lowerCtx) saReadsBatch(sa0 *cards.SA) bool {
	sa := sa0
	reads := func(s string) bool {
		return strings.Contains(s, "Triggered") || strings.Contains(s, "TriggerCount") ||
			strings.Contains(s, "TriggerObjects") || strings.Contains(s, "TriggerRemembered")
	}
	for ; sa != nil; sa = sa.Sub {
		if reads(sa.Line) {
			return true
		}
	}
	// only the SVars the body names (transitively) count: another
	// ability's SVar reading its own trigger (Enduring Innocence's dies
	// trigger beside its ChangesZoneAll draw) is not this body's read
	seen := map[string]bool{}
	var work []string
	for sa := sa0; sa != nil; sa = sa.Sub {
		for _, v := range sa.Params {
			if reads(v) {
				return true
			}
			work = append(work, v)
		}
	}
	for len(work) > 0 {
		w := work[len(work)-1]
		work = work[:len(work)-1]
		for _, name := range strings.FieldsFunc(w, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_')
		}) {
			if body, ok := lc.face.SVars[name]; ok && !seen[name] {
				seen[name] = true
				if reads(body) {
					return true
				}
				work = append(work, body)
			}
		}
	}
	return false
}

// the trigger-context reads a per-referent DamageDone trigger
// (Mode$ DamageDoneOnce, and Mode$ DamageAll at a single player) carries
// exactly.
//
// The kernel's per-referent queue binds the trigger object and player from
// the damaged referent (trig_damagedonce.go ddoRecipient): for an object
// referent the damaged permanent and its controller; for a player referent
// the first source and the damaged player. A body may therefore read
// TriggeredTarget / TriggeredTargetLKICopy when the line's ValidTarget$ is
// an object filter (the referent is the damaged object), and
// TriggeredAttackingPlayer on a combat damage line whose ValidTarget$ is
// You (the damaged player is the listener's controller and, two players,
// the only possible attacking player; twoSeat-at-caller: lower_effect.go's
// TriggeredAttackingPlayer marks the card). Everything else still refuses.
//
// One shared predicate per shape: lowering (lower_effect.go defined), the
// body-read gate (lower_damagedoneonce.go ddoReads) and the DamageAll admission all ask
// these, so the shapes cannot drift apart.

// ddoObjectReferent reports whether tr is a per-referent DamageDone line
// whose referent is the damaged object: ValidTarget$ is an object filter
// (players == 0) with clauses, so the kernel's trigger object is the
// damaged permanent.
func ddoObjectReferent(tr *lTrigger) bool {
	return tr != nil && tr.Event == "DamageDone" && tr.PerRef &&
		tr.F2 != nil && tr.F2.Players == 0 && len(tr.F2.Clauses) != 0
}

// ddoAttackingPlayer reports whether tr is a per-referent combat-damage
// line whose ValidTarget$ is exactly You: the damage source's controller is
// gorge's TriggeredAttackingPlayer. The kernel's per-referent DamageDone
// queue carries the source as the trigger object and the damaged seat as its
// trigger player (dirac dmgSeat), so the lowering reads the trigger object's
// controller (rTriggeredTgtCtrl, seat-exact), not the one opponent and not
// rTriggeredPlayer (which is the damaged seat there).
func ddoAttackingPlayer(tr *lTrigger) bool {
	return tr != nil && tr.Event == "DamageDone" && tr.PerRef && tr.Combat == 1 &&
		tr.F2 != nil && tr.F2.Players == plYou && len(tr.F2.Clauses) == 0
}

// ddoReadOK reports whether a DamageDoneOnce body may read spelling: the
// referent shapes above are carried exactly by the per-referent queue;
// every other Triggered*/TriggerCount* spelling is not.
func ddoReadOK(spelling string, tr *lTrigger) bool {
	switch spelling {
	case "TriggeredTarget", "TriggeredTargetLKICopy":
		return ddoObjectReferent(tr) || ddoPlayerReferent(tr)
	case "TriggeredTargetController":
		// gorge definedSpecTriggeredTargetController: the controller of the
		// damaged object, read live at resolution (the kernel lowers it to
		// rTriggeredTgtCtrl, whose read covers a referent that left the
		// battlefield since), so the role is exact only on an object
		// referent.
		return ddoObjectReferent(tr)
	case "TriggeredAttackingPlayer":
		return ddoAttackingPlayer(tr)
	}
	return false
}
