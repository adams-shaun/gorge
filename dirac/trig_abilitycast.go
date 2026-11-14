package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Mode$ AbilityCast, "whenever you activate an exhaust /
// loyalty ability" (CR 602.2, 603.2; gorge trigmatch abilityCastMatches).
// gorge fires the trigger when the activation completes, after its costs
// are paid (the AbilityPush boundary), and judges ValidSA$ through
// abilityCastConstraintHolds: Activated.Exhaust is ab.ExhaustValue(),
// Activated.Loyalty a planeswalker loyalty ability, +OppCtrl the ability's
// controller an opponent of the listener's controller. The kernel's
// seAbility arm of finishBuild calls fireAbilityCast once the entry is on
// the stack and paid, behind genPrep.anyAbilityCast; a mana ability mints no
// stack entry, so the planner's tap loop calls fireManaAbilityCast for it
// (CR 605.1a: a mana ability is an activated ability). No State field: the
// trigger rides pend.

// AbilityCastStats counts the lifted mechanic; oracle tests only.
type AbilityCastStats struct {
	Fires      [8]int // TrigAbilityCast triggers queued, by Trigger.N class (kop.TrigAbCast*)
	OppFires   int    // queued for a listener whose controller activated nothing (the activator is its opponent)
	GraveFires int    // queued from a graveyard card
}

// DebugAbilityCastStats is the test hook; nil in normal play.
var DebugAbilityCastStats *AbilityCastStats

// DebugAbilityCastIgnoreClass makes the class test pass for every activated
// ability (the lockstep mutation test flips it and requires a divergence).
var DebugAbilityCastIgnoreClass bool

// prepAbilityCast sets anyAbilityCast when some printed trigger is an
// AbilityCast. A granted one (AddTrigger$) is not scanned (no corpus carrier).
func prepAbilityCast(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			g.anyAbilityCast = g.anyAbilityCast || abilityCastLine(a)
		}
	}
}

// abilityCastLine reports whether ability a listens for activations: an
// AbilityCast trigger, or a SpellCast one that also watches the activations
// of the sources with its chosen name (Tf4NamedCast, repl_choosename.go).
func abilityCastLine(a *kop.Ability) bool {
	if a.Kind != kop.AbTriggered {
		return false
	}
	return a.Trig.Event == kop.TrigAbilityCast || a.Trig.Event == kop.TrigSpellCast && a.Trig.Flags4&kop.Tf4NamedCast != 0
}

// abilityCastClass reports whether the activated ability ab of card c is of
// class cls (kop.TrigAbCast*).
func abilityCastClass(k *prep, e *stackEntry, cls int16) bool {
	if DebugAbilityCastIgnoreClass {
		return true
	}
	cd := &k.t.Cards[e.card]
	if int(e.ab) >= len(cd.Abilities) {
		return false
	}
	a := &cd.Abilities[e.ab]
	if cls == kop.TrigAbCastExhaust {
		return a.Exhaust
	}
	if cls == kop.TrigAbCastAny {
		return true // the target gate (abilityCastFires) is the whole test
	}
	// a planeswalker's [-X] carries CostLoyalty beside CostLoyaltyX; the
	// latter alone is an announced SubCounter<X/KIND>, not a loyalty ability
	return a.Cost.Flags&kop.CostLoyalty != 0
}

// abilityCastMatch is the listener test for one AbilityCast line.
func abilityCastMatch(k *prep, e *stackEntry, l trigListener, a *kop.Ability) bool {
	t := &a.Trig
	if !whoseOK(t.Whose, l.ctrl, e.ctrl) || t.N == kop.TrigAbCastLoyaltyOpp && e.ctrl == l.ctrl {
		return false
	}
	return abilityCastClass(k, e, t.N)
}

// abilityCastFires is abilityCastMatch plus the part that needs the state: a
// TrigAbCastAny line (ValidSA$ SpellAbility beside TargetsValid$) judges the
// targets the activation chose, which gorge reads once its answers are
// recorded (trigmatch abilityCastMatches' targetsvalid1 arm), and queues with
// the activated entry's stack identity as its amount so the trigger's
// CopySpellAbility finds the ability again (ExtCopyAbility, CopyAbTriggering).
// A mana ability has no targets, so it never satisfies the gate.
func (s *State) abilityCastFires(k *prep, e *stackEntry, l trigListener, a *kop.Ability) (int16, bool) {
	if !abilityCastMatch(k, e, l, a) {
		return 0, false
	}
	if a.Trig.N != kop.TrigAbCastAny {
		return 0, true
	}
	if e.kind != seAbility || a.Trig.Flags2&kop.Tf2TargetsFilter2 == 0 || !s.castTargetsAll(k, a.Trig.Filter2, e, l) {
		return 0, false
	}
	return int16(e.sid), true
}

// fireManaAbilityCast queues the AbilityCast listeners for mana ability ab of
// card, which ctrl activates (CR 605.1a: a mana ability is an activated
// ability, though it mints no stack entry). It is fireAbilityCast over a
// synthetic entry, so the exhaust class and the Tf4NamedCast source test read
// the same bodies a stack activation's do. Gorge announces the activation
// before the cost is paid (rules/mana_activation.go), so a caller fires it
// ahead of the tap and the sacrifice's own triggers.
func (s *State) fireManaAbilityCast(k *prep, card CardID, ab int8, ctrl uint8) {
	if k.gen == nil || !k.gen.anyAbilityCast || ab < 0 || int(card) >= len(k.t.Cards) || int(ab) >= len(k.t.Cards[card].Abilities) || k.t.Cards[card].Abilities[ab].Kind != kop.AbMana {
		return
	}
	e := stackEntry{card: card, ab: uint8(ab), ctrl: ctrl}
	s.fireAbilityCast(k, &e)
}

// fireViewAbilityCast is fireManaAbilityCast for the ability ab of the
// planner's view of sl (manaPCGen). A granted view carries the grantor's
// ability number, which names nothing on sl's own card, so a granted
// activation fires nothing here (its listener class has no printed ability to
// read).
func (s *State) fireViewAbilityCast(k *prep, p uint8, sl Slot, ab int8) {
	if k.gen == nil || !k.gen.anyAbilityCast || sl == NoSlot || s.manaGranted(k, sl) {
		return
	}
	s.fireManaAbilityCast(k, s.cardAt(sl), ab, p)
}

// fireAbilityCast queues the AbilityCast listeners for the just-activated
// ability entry e: battlefield listeners in gorge's walk order, then
// graveyard cards (TriggerZones$ Graveyard). No slot subscription.
//
//go:noinline
func (s *State) fireAbilityCast(k *prep, e *stackEntry) {
	st := DebugAbilityCastStats
	note := func(before uint8, l trigListener, a *kop.Ability, grave bool) {
		if st == nil || s.npend <= before {
			return
		}
		st.Fires[a.Trig.N&7]++
		if l.ctrl != e.ctrl {
			st.OppFires++
		}
		if grave {
			st.GraveFires++
		}
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, abilityCastLine, func(l trigListener, a *kop.Ability) {
			if a.Trig.Event == kop.TrigSpellCast {
				// a Tf4NamedCast line: the source of the activation has the chosen name
				if s.namedActivation(k, e, l, a) {
					b := s.npend
					s.playerTrigGen(k, kop.TrigAbilityCast, l, a, e.ctrl, 0)
					if s.npend > b {
						noteNamedCast(false)
					}
				}
				return
			}
			amt, ok := s.abilityCastFires(k, e, l, a)
			if !ok {
				return
			}
			b := s.npend
			s.playerTrigGen(k, kop.TrigAbilityCast, l, a, e.ctrl, amt)
			note(b, l, a, false)
		})
	}
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if s.objs[o].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind != kop.AbTriggered || a.Trig.Zone != kop.ZoneGraveyard || a.Trig.Event != kop.TrigAbilityCast {
						continue
					}
					l := trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}
					amt, ok := s.abilityCastFires(k, e, l, a)
					if !ok {
						continue
					}
					b := s.npend
					s.playerTrigGen(k, kop.TrigAbilityCast, l, a, e.ctrl, amt)
					note(b, l, a, true)
				}
			}
		}
	}
}
