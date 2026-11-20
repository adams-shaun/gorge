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

// prepManaImprint builds card id's views (prepManaRefl's arm for the imprint
// shape): view m makes one mana of a colour of m. A ColorOrType$ Type Produce
// read (widen, Reflecting Pool) has 64 views: bit 5 is {C}, so a reflected
// {C} coexists with the coloured masks. A life-cost reflected ability
// (Horizon of Progress, life > 0) is gorge's last-resort Consequence{Life: N}
// with no normal alternative (pay PaymentPlanShapeTierOf's cost.Life arm), so
// its views carry the life as the kernel's pain-only unit's fields
// (mana_pain.go painUnit, manaLifePain) and name the ability themselves
// (kop.ReflLifeManaAb: the gen walk honours the cost, but the base pc prices
// no colours — the views are the unit).
//
//go:noinline
func prepManaImprint(g *genPrep, id CardID, pc *pcard, ident bool, ra int, life uint8, widen bool) {
	g.setFlag(id, gfManaVar|gfManaRefl)
	g.smallFor(id).manaVar = int16(len(g.varPC))
	n := 32
	if widen {
		n = 64
	}
	for m := 0; m < n; m++ {
		v := *pc
		v.manaCol, v.manaMulti, v.manaRefl, v.manaIdent = kop.ColorMask(m), false, !ident, ident
		if m == 0 {
			v.manaAb = -1 // nothing to reflect: gorge lists no such ability
		} else {
			v.manaAb = int8(ra)
			if life != 0 {
				// its {T} is an ordinary one: the life is the painful
				// alternative's cost (prepPainOnlyLife's read of Mana
				// Confluence)
				v.manaTap = true
				v.painCol, v.painN, v.painOnly, v.painLife = v.manaCol, life, true, true
			}
		}
		g.varPC = append(g.varPC, v)
	}
	if life != 0 && !ident && !widen {
		// the colourless view (reflColours' kop.ColC index): a land beside the
		// source could make {C} and nothing coloured (Horizon of Progress
		// beside a Dragon-Cursed Halls). gorge's fold keeps the {C} (ColorOrType$
		// Type), so the ability is no planner unit there and the gaps walk
		// plays it for {C}; the kernel prices it as the same last-resort pain
		// unit, of {C}. A Type read's 64 views already hold this view at index
		// 32.
		v := *pc
		v.manaCol, v.manaMulti, v.manaRefl, v.manaIdent = kop.ColC, false, true, false
		v.manaAb, v.manaTap = int8(ra), true
		v.painCol, v.painN, v.painOnly, v.painLife = kop.ColC, life, true, true
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
	r := s.cold.objSets
	for i := range r {
		if r[i].kind == 0 {
			break
		}
		rec := &r[i]
		if rec.src == src && rec.kind&exwRev != 0 && rec.sver == ver && s.objs[rec.o].zone == kop.ZoneExile {
			m |= k.t.Cards[s.objs[rec.o].card].Colors
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
			return a.ManaAmong == 0 && !a.ManaIdentity
		}
	}
	return false
}
