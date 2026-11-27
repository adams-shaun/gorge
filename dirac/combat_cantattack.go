package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The Target$ scope of a CantAttack static (kop.StCantAtkPlayer,
// StCantAtkWalker; gorge combat.AttackBlocked over
// RestrictionTargetMatches): "You" forbids the attack on the static's
// controller, "Planeswalker.YouCtrl" one on a planeswalker that controller
// controls. A battle is neither, so a scoped static never blocks a battle
// pair -- a creature Rite of the Raging Storm's CantAttack holds back from
// its controller is still offered at that controller's Siege (CR 508.1b-c).
// A static with neither bit is the whole-defender shape and canAttack
// blocks its creature blind (combat.go).

// DebugCantAtkPodBlind is the lockstep mutation of the pod offers: a pod's
// attackOffersN offers every defender's pairs as if no scoped CantAttack
// static stood (TestFourSeatVowLockstep must then diverge).
var DebugCantAtkPodBlind bool

// prepCantAtkScope sets genPrep.anyCantAtk and genPrep.anyCantAtkScope: the
// first when a card or emblem static of t is a StatCantAttack at all (the
// blind read's cheap skip), the second when one carries a scope bit (the
// pair reads' cheap skip).
func prepCantAtkScope(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCantAttack {
				g.anyCantAtk = true
				if a.Static.StFlags&(kop.StCantAtkPlayer|kop.StCantAtkWalker|kop.StCantAtkDef) != 0 {
					g.anyCantAtkScope = true
				}
			}
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCantAttack {
				g.anyCantAtk = true
				if a.Static.StFlags&(kop.StCantAtkPlayer|kop.StCantAtkWalker|kop.StCantAtkDef) != 0 {
					g.anyCantAtkScope = true
				}
			}
		}
	}
}

// cantAtkBare reports whether a CantAttack static with NO Target$ scope
// applies to the creature at sl: without a scope every attack is forbidden,
// so canAttack drops the creature before any pair is enumerated. A scoped
// static contributes nothing here; its pairs are judged by cantAtkPair.
// Tables without a generated prep keep the whole-defender read.
func (s *State) cantAtkBare(k *prep, sl Slot) bool {
	if k.gen == nil {
		return s.staticFlag(k, kop.StatCantAttack, sl)
	}
	if !k.gen.anyCantAtk {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantAttack ||
			a.Static.StFlags&(kop.StCantAtkPlayer|kop.StCantAtkWalker) != 0 ||
			a.Static.StFlags&kop.StCantAtkDef != 0 && !s.duel() ||
			!s.statCond(k, st) || st.emb != 0 && !s.embCaptureOK(k, st.emb, sl) ||
			!s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		hit = true
		return false
	})
	return hit
}

// cantAtkPair reports whether a scoped CantAttack static forbids the
// (attacker at sl, defender seat def, attacked permanent walker) attack
// pair. walker NoSlot is the player attack: only StCantAtkPlayer ("You")
// can forbid it, and only at the static's controller. A walker attack is
// StCantAtkWalker (Planeswalker.YouCtrl), the walker's controller the
// static's controller (the walkers on offer are the defender's, so the
// controller test is the defender test). No bit forbids a battle pair.
// Tables without a generated prep keep the whole-defender read: nothing is
// pair-blocked because canAttack already dropped the creature.
func (s *State) cantAtkPair(k *prep, sl Slot, def uint8, walker Slot) bool {
	if k.gen == nil || !k.gen.anyCantAtkScope {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantAttack {
			return true
		}
		if a.Static.StFlags&kop.StCantAtkDef != 0 {
			if s.duel() {
				return true // the whole-defender shape: cantAtkBare
			}
			q := def
			if DebugUnlessDefBlind {
				q = s.nextAlive(s.active) // seat-ok: a test-only mutation
			}
			if s.unlessDefHolds(k, st, q) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull) {
				hit = true
				return false
			}
			return true
		}
		sc := a.Static.StFlags & (kop.StCantAtkPlayer | kop.StCantAtkWalker)
		if sc == 0 || !s.statCond(k, st) || st.emb != 0 && !s.embCaptureOK(k, st.emb, sl) ||
			!s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		if walker == NoSlot {
			if sc&kop.StCantAtkPlayer != 0 && def == st.ctrl {
				hit = true
				return false
			}
			return true
		}
		if sc&kop.StCantAtkWalker != 0 && s.typesAt(k, walker)&kop.TPlaneswalker != 0 &&
			s.perms[walker].ctrl == st.ctrl {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// cantAtkPairPlayer is cantAtkPair for the attack on defender seat def (the
// player, no permanent attacked).
func (s *State) cantAtkPairPlayer(k *prep, sl Slot, def uint8) bool {
	return s.cantAtkPair(k, sl, def, NoSlot)
}

// cantAtkEveryPair reports whether a scoped CantAttack static forbids every
// pair the attackers decision could offer the creature at sl, so gorge
// poses no decision at all: attackOffersCompute's option list comes out
// empty and askAttackers emits the silent empty declaration instead of
// asking (engine/rules/combat.go), leaving the step's CR 508.2 priority
// round. Each living opponent's player pair is its own test (a Vow of Duty
// forbids its controller alone, so a pod's other opponents stand); when
// every one is forbidden,
// attackOffersCompute's player-pair block skips that defender's
// planeswalker pairs with it, so only a battle pair can still carry an
// offer -- and a goad or an Encore duty is satisfied by attacking a player
// only (act_goad.go, combat_attacksifable.go), whose requirement keep-loop
// drops those pairs too. anyAttacker gates its candidates with this
// (combat.go), the way atkTaxAnyOffer gates a priced-out board.
func (s *State) cantAtkEveryPair(k *prep, sl Slot) bool {
	for q := uint8(0); q < s.nSeats(); q++ {
		if q != s.active && !s.out(q) && !s.cantAtkPairPlayer(k, sl, q) {
			return false // a player pair stands: it is an offer (each living opponent in a pod)
		}
	}
	if k.gen != nil && k.gen.anyBattle && !s.attackPlayerOnly(k, sl) &&
		!(goadOn(k) && s.goadedAt(k, sl)) && s.someBattleOffers(k, sl) {
		return false // a battle pair stands
	}
	return true
}

// cantAtkPairWalker is cantAtkPair for the attack on the planeswalker at q,
// the defender's permanent.
func (s *State) cantAtkPairWalker(k *prep, sl, q Slot) bool {
	return s.cantAtkPair(k, sl, s.perms[q].ctrl, q)
}

// DebugUnlessDefBlind is the lockstep mutation of the pod UnlessDefender$
// read: a pod judges every pair of a StCantAtkDef static on the next live
// seat, the two-seat reading of the defender (TestFourSeatUnlessDefenderLockstep
// must then diverge).
var DebugUnlessDefBlind bool

// unlessDefHolds reports whether a StCantAtkDef static's UnlessDefender$
// gate (its Cond2) restricts the attack on defender seat def: gorge
// AttackBlocked evaluates UnlessDefenderHolds on each (attacker, defending
// player) pair, a planeswalker's pair on the walker's controller. The
// condition's DefenderCtrl filters read g.atkDef (defendingSeatF2), bound
// to def for the read and put back after, as fireAttacksGen binds it for an
// attack event's triggers.
func (s *State) unlessDefHolds(k *prep, st statSrc, def uint8) bool {
	old := s.g.atkDef
	s.g.atkDef = def + 1
	ok := s.statCond(k, st)
	s.g.atkDef = old
	return ok
}
