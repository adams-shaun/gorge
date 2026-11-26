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
			r := &a.Repl
			if r.C&kop.ReplDmgSrcSpell != 0 && (srcSl != NoSlot || s.res.e.kind != seSpell) {
				continue // the source is the resolving spell (ValidSource$ Spell)
			}
			if r.C&kop.ReplDmgNonCombat != 0 && combat || r.C&kop.ReplDmgCombat != 0 && !combat {
				continue
			}
			if r.C&kop.ReplDmgSrcInPlay != 0 && srcSl == NoSlot {
				continue
			}
			switch {
			case sl == NoSlot:
				if !(r.C&kop.ReplDmgPlayer != 0) && !(r.C&kop.ReplDmgOppPlayer != 0 && rcp != hc) {
					continue // the recipient is no player the line names
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
					continue
				}
			}
			hits++
			switch {
			case r.C&kop.ReplDmgHalfDown != 0:
				n /= 2 // ReplaceCount$DamageAmount/HalfDown: half, rounded down (Ghosts of the Innocent)
			case n > int(r.A):
				n -= int(r.A) // CR 614.1: the rewrite replaces the event's amount
			default:
				n = 0
			}
		}
	}
	return n, hits
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
// kernel stalls there too rather than deal the damage without the ask.
func (s *State) dmgRewriteClash(minus, set, doublers int) {
	if minus >= 2 || set >= 2 || set == 1 && minus+doublers > 0 || minus > 0 && doublers > 0 {
		s.capFull(StallReplPlus)
	}
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
