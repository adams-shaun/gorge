package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplDamageMinus (gorge replacement_damage.go's Updated DamageAmount
// rewrites, the ReplaceWith$ DB$ ReplaceEffect | VarName$ DamageAmount body
// with VarValue$ ReplaceCount$DamageAmount/Minus.<n>): a battlefield line
// (Benevolent Unicorn, Lashknife Barrier) rewrites the event's damage to that
// much minus n, floored at none. The source read is the ValidSource$ word
// Spell (ReplDmgSrcSpell: the resolving spell has no battlefield source, the
// read spMatch's SPSrcSpell arm makes) or none (every source); the recipient
// the ValidTarget$ words (kop.ReplDmg* bits) or the single object filter
// alternative (Repl.Filter, read with the replacement's controller as You).
// The kernel reads it beside the plus-N and doubler rewrites in replDamage's
// pipeline (plus-then-minus), the fixed order gorge's Updated composition
// takes when the replacements do not meet an order ask; a minus beside a
// doubler of the same event is a gorge-asked competition (replDamage's
// dmgRewriteClash caps that meeting loudly).

// DebugDmgMinusOff skips the rewrite (the mutation: the kernel deals the
// event's full damage).
var DebugDmgMinusOff bool

// dmgMinusN is replDamage's minus rewrite: every battlefield ReplDamageMinus
// line whose source and recipient read matches takes n to max(n-A, 0). Only
// with prep's rb2DmgMinus.
//
//go:noinline
func (s *State) dmgMinusN(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool, n int) (int, int) {
	if DebugDmgMinusOff {
		return n, 0
	}
	g := k.gen
	hits := 0
	for m := s.statM; m.any(); m = m.rest() {
		h := m.first()
		if g.replBits2[s.cardAt(h)]&rb2DmgMinus == 0 || s.genLost(k, h) || s.faceDown(k, h) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(h)]
		hc := s.perms[h].ctrl
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDamageMinus {
				continue
			}
			if !s.dmgMinusMatch(k, &a.Repl, h, hc, srcSl, rcp, sl, combat) {
				continue
			}
			hits++
			if n > int(a.Repl.A) {
				n -= int(a.Repl.A) // CR 614.1: the rewrite replaces the event's amount
			} else {
				n = 0
			}
		}
	}
	return n, hits
}

// dmgMinusMatch reports whether the minus line r of host h (controlled by hc)
// matches an event from srcSl (NoSlot: the resolving spell) dealt to the
// player rcp (sl NoSlot) or the permanent at sl: dmgMinusN's per-line read,
// shared with the combat order walk's candidate collector (replOrderCandsSeat).
func (s *State) dmgMinusMatch(k *prep, r *kop.Repl, h Slot, hc uint8, srcSl Slot, rcp uint8, sl Slot, combat bool) bool {
	if r.C&kop.ReplDmgSrcSpell != 0 && (srcSl != NoSlot || s.res.e.kind != seSpell) {
		return false // the source is the resolving spell (ValidSource$ Spell)
	}
	if r.C&kop.ReplDmgNonCombat != 0 && combat || r.C&kop.ReplDmgCombat != 0 && !combat {
		return false
	}
	if r.C&kop.ReplDmgSrcInPlay != 0 && srcSl == NoSlot {
		return false
	}
	switch {
	case sl == NoSlot:
		if !(r.C&kop.ReplDmgPlayer != 0) && !(r.C&kop.ReplDmgOppPlayer != 0 && rcp != hc) {
			return false // the recipient is no player the line names
		}
	default:
		// the recipient is a permanent at sl: any named word or the
		// filter alternative (gorge's ValidTarget$ alternatives are
		// one big OR)
		ok := r.C&kop.ReplDmgPerm != 0 || r.C&kop.ReplDmgOppPerm != 0 && rcp != hc
		if !ok && r.Filter != 0 {
			ok = s.matchPermGen(k, r.Filter, sl, hc, h, layerFull)
		}
		if !ok {
			return false
		}
	}
	return true
}

// dmgMinusCand reports whether the permanent at h carries a ReplDamageMinus
// line that matches a combat event from srcSl to player rcp (sl NoSlot) or the
// permanent at sl: the combat order walk's candidate collector's read of
// dmgMinusN (replOrderCandsSeat).
func (s *State) dmgMinusCand(k *prep, h, srcSl Slot, rcp uint8, sl Slot) bool {
	if k.gen.replBits2[s.cardAt(h)]&rb2DmgMinus == 0 || s.genLost(k, h) || s.faceDown(k, h) {
		return false
	}
	cd := &k.t.Cards[s.cardAt(h)]
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplDamageMinus &&
			s.dmgMinusMatch(k, &a.Repl, h, s.perms[h].ctrl, srcSl, rcp, sl, true) {
			return true
		}
	}
	return false
}

// DebugDmgSetOff skips the set rewrite (the mutation: the kernel deals the
// event's full damage).
var DebugDmgSetOff bool

// dmgSetN is replDamage's set rewrite (ReplDamageSet, the printed DamageDone
// Updated body with a literal VarValue$ and a DamageAmount$ GE<k> gate: Divine
// Presence, Forethought Amulet): damage of the gate and up becomes Repl.A.
// The source read is the ValidSource$ filter (Repl.Filter2, read on the
// battlefield permanent or on the resolving spell's card), the recipient the
// ValidTarget$ words (kop.ReplDmg* bits) — the kernel's recipients read like
// dmgMinusN's. Damage of the gate and up sets again per matching line: two
// satisfied gates on one event are a gorge-asked CR 616.1 competition the
// focus pools avoid. Only with prep's rb2DmgSet.
//
//go:noinline
func (s *State) dmgSetN(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool, n int) (int, int) {
	if DebugDmgSetOff {
		return n, 0
	}
	g := k.gen
	hits := 0
	for m := s.statM; m.any(); m = m.rest() {
		h := m.first()
		if g.replBits2[s.cardAt(h)]&rb2DmgSet == 0 || s.genLost(k, h) || s.faceDown(k, h) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(h)]
		hc := s.perms[h].ctrl
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDamageSet {
				continue
			}
			r := &a.Repl
			if gate := kop.ReplDmgSetGateOf(r.C); n < gate {
				continue // damage under the gate is untouched
			}
			if r.Filter2 != 0 {
				ctx := evalCtx{src: s.perms[h].obj, srcSl: h, ctrl: hc}
				if srcSl != NoSlot {
					if !s.matchPermGen(k, r.Filter2, srcSl, hc, h, layerFull) {
						continue
					}
				} else if !s.matchSourceCard(k, r.Filter2, srcCard, srcCtrl, hc, &ctx) {
					continue
				}
			}
			switch {
			case sl == NoSlot:
				if !(r.C&kop.ReplDmgPlayer != 0) && !(r.C&kop.ReplDmgOppPlayer != 0 && rcp != hc) {
					continue // the recipient is no player the line names
				}
			default:
				ok := r.C&kop.ReplDmgPerm != 0 || r.C&kop.ReplDmgOppPerm != 0 && rcp != hc
				if !ok && r.Filter != 0 {
					ok = s.matchPermGen(k, r.Filter, sl, hc, h, layerFull)
				}
				if !ok {
					continue
				}
			}
			hits++
			n = int(r.A) // CR 614.1: the rewrite replaces the event's amount
		}
	}
	return n, hits
}

// dmgRewriteClash caps loudly when the minus/set rewrites that matched one
// event do not commute with each other or with a doubler (set-then-double is
// the gate times two, double-then-set the gate; minus-then-double is not
// double-then-minus): gorge asks the CR 616.1 order, whose answer changes the
// amount, and the kernel composes in one fixed order (StallReplPlus, the
// plus-N competition's cap). Two minus lines commute (their sum, floored), but
// gorge asks their order all the same (applyDamageClassReplacements poses the
// ask for every two or more matches that are not bare preventions), so the
// kernel stalls there too rather than deal the damage without the ask. A
// combat event the order walk collects (ordered: replOrderCandsSeat) is the
// exception: the walk poses the same no-op ask, so only other pairs stall.
func (s *State) dmgRewriteClash(minus, set, doublers int, ordered bool) {
	if minus >= 2 && !ordered || set >= 2 || set == 1 && minus+doublers > 0 || minus > 0 && doublers > 0 {
		s.capFull(StallReplPlus)
	}
}

// dmgRewriteClashAt is dmgRewriteClash for a damage event dealt by a source
// controlled by srcCtrl to a recipient controlled by rcp: a combat event the
// order walk collects (replOrderCandsSeat) is one to an opponent's recipient,
// or any recipient once a filtered replacement is in play.
func (s *State) dmgRewriteClashAt(k *prep, minus, set, doublers int, combat bool, rcp, srcCtrl uint8) {
	s.dmgRewriteClash(minus, set, doublers, combat && (rcp != srcCtrl || k.gen.anyFiltRepl))
}

// dmgRewritePrevClash caps loudly when a minus/set rewrite matched a combat
// event that a prevention also matches (the source's granted prevention, an
// Effect in gorge: Benalish Missionary; the recipient's printed or granted
// prevent-all, or its shield counter): gorge asks the CR 616.1 order between
// the rewrite and the prevention, and the kernel prevents after rewriting
// without the ask (StallReplPlus, as dmgRewriteClash).
func (s *State) dmgRewritePrevClash(k *prep, hits int, srcSl, sl Slot, combat bool) {
	if hits == 0 || !combat || k.gen == nil || srcSl == NoSlot {
		return
	}
	if s.prevFromGrant(k, srcSl) || sl != NoSlot && (s.prevCombatTo(k, sl) || k.gen.anyRepl&rbShield != 0 && s.shieldN(sl) > 0 && !s.noPrev(k, true)) {
		s.capFull(StallReplPlus)
	}
}
