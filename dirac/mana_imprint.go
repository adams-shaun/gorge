package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The imprint mox (Chrome Mox: "{T}: Add one mana of any of the exiled
// card's colors"; kgen lower_imprintmox.go): a printed kop.AfManaReflect
// ability with no ManaAmong filter reads the colours of the cards its
// source's enters trigger exiled. gorge reads the source's persistent
// Imprinted list through the CR 607.2a exile gate (effects definedSpec
// Imprinted), cleared when the source leaves the battlefield and pruned of a
// card that leaves exile. The kernel reads the reverse exiled-with record
// (genState.exw, zone_exiledwith.go), which gorge also sets because the
// card's text names ExiledWithSource: live at the source's current ver (a
// departure moves the ver on, as gorge's departure clears the list), dropped
// when the card leaves exile or is exiled again. The forward bit is not
// read: it survives the source's departure, the Imprinted list does not.
//
// gorge's census lists the source with the reflected colours
// (rules.paymentPlanReflectedAlt), the alternative replacing the stand-in
// the shared census listed at the source's battlefield position
// (setPaymentPlanUnitAlt), so even a colour choice keeps that position
// (pcard.manaRefl: not appended with the choice-shaped sources); it lists
// no mana ability at all when the colours are none (considerReflected's
// candidate gate), and its activation asks a colour only among two or more
// (mana_activation.go). The kernel's planner view is therefore varPC[vi+m]
// for the live colours m: view 0 no mana ability, a one-colour view a fixed
// source, a wider one a colour choice among m (not manaMulti: one ability,
// not several).

// debugImprintMutate is the test-only mutation switch (0 none; 1 the
// imprinted colours are never read: the mox makes no mana; 2 a colour-choice
// view is appended with the choice-shaped sources, not kept at its
// battlefield position). TestChromeMoxLockstepMutation requires each to
// diverge.
var debugImprintMutate uint8

// SetDebugImprintMutate sets the mutation switch; tests only.
func SetDebugImprintMutate(m uint8) { debugImprintMutate = m }

// prepManaImprint builds card id's 32 views (prepManaRefl's arm for the
// imprint shape): view m makes one mana of a colour of m.
//
//go:noinline
func prepManaImprint(g *genPrep, id CardID, pc *pcard) {
	g.setFlag(id, gfManaVar|gfManaRefl)
	g.smallFor(id).manaVar = int16(len(g.varPC))
	for m := 0; m < 32; m++ {
		v := *pc
		v.manaCol, v.manaMulti, v.manaRefl = kop.ColorMask(m), false, true
		if m == 0 {
			v.manaAb = -1 // nothing to reflect: gorge lists no such ability
		}
		g.varPC = append(g.varPC, v)
	}
}

// imprintColours is the colours of the cards in exile whose reverse
// exiled-with record names the source at sl at its current ver.
func (s *State) imprintColours(k *prep, sl Slot) kop.ColorMask {
	if debugImprintMutate == 1 {
		return 0
	}
	src := s.perms[sl].obj
	ver := s.objs[src].ver
	var m kop.ColorMask
	for i := 0; i < int(s.g.nexw); i++ {
		r := &s.cold.exw[i]
		if r.src == src && r.fl&exwRev != 0 && r.sver == ver && s.objs[r.o].zone == kop.ZoneExile {
			m |= k.t.Cards[s.objs[r.o].card].Colors
		}
	}
	return m & kop.ColAny
}

// imprintMoxPick reports whether the resolving hand pick is the imprint
// mox's: gorge poses it as its own "imprint" ask (effects zone_change.go,
// Min = Max = ChangeNum$, after the trigger's optional yes), which kgen
// lowers as a mandatory pick (SrMandatory: unasked when no more qualify).
func (s *State) imprintMoxPick(k *prep) bool {
	if !s.res.active || k.gen == nil {
		return false
	}
	c := s.res.e.card
	if !k.gen.has(c, gfManaRefl) {
		return false
	}
	cd := &k.t.Cards[c]
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfManaReflect != 0 && a.AFlags&kop.AfLatent == 0 {
			return a.ManaAmong == 0
		}
	}
	return false
}
