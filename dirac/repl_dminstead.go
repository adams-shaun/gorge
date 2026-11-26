package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ReplDamageInstead (Angel of Suffering, Gloom Surgeon, Force Bubble, Panther
// Habit, Soul-Scar Mage, Swans of Bryn Argoll, Szadek, Undead Alchemist): a
// battlefield line whose body puts the damage to another use. gorge
// (rules/replacement.go applyNonMoveReplacements) runs the body when the
// damage event is proposed and hands back a stopped event: nothing is dealt
// (no lifelink, no damage trigger, no life loss), once per event (a combat
// pass is one event per source and recipient). A PreventionEffect$ True line
// (kop.ReplDmgInsPrevent) under "damage can't be prevented" still runs its
// body and lets the damage stand (CR 615.12); the other lines are not
// preventions and replace the damage regardless. The effects run in the
// order counters, mill, exile, draw.
//
// Beside another damage replacement that can match the same event gorge asks
// the CR 616.1 order: initStall refuses that pair (StallReplPlus), and two
// ReplDamageInstead lines matching one event stall here. A body that moves
// cards inside a replacement-order walk or beside a parked graveyard move
// is not modelled: the walk recomputes its pass, and the move would ask.

// DebugDmgInsteadOff skips the replacement (the oracle mutation: the damage is
// dealt).
var DebugDmgInsteadOff bool

// DmgInsteadStats counts the bodies run, one counter per kop.ReplDmgIns* effect
// bit (the oracle test's proof that its pools reach each replacement; the ask
// bodies' bits sit at 14 and 15, in Repl.C).
var DmgInsteadStats [16]atomic.Int64

// dmgInstead applies the matching ReplDamageInstead line to n damage a source
// controlled by srcCtrl (permanent srcSl, else the resolving spell srcCard)
// deals to the player rcp (sl == NoSlot) or the permanent at sl: true when the
// damage is replaced and nothing is dealt. Only with prep's rb2DmgInst. An
// ask body's match is handed to the caller (ask; repl_dmbody.go): damageTo
// parks the event on its decision, every other site stalls loudly, and a
// frozen pass's recomputed run defers to the walk.
//
//go:noinline
func (s *State) dmgInstead(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool, n int, ask *ncAsk) bool {
	if DebugDmgInsteadOff || n <= 0 {
		return false
	}
	host, line, hits := s.dmgBodyScan(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, false)
	if hits == 0 {
		return false
	}
	if hits > 1 {
		s.capFull(StallReplPlus) // gorge asks the CR 616.1 order of two lines
		return true
	}
	body := int(uint16(line.A))&0xff | int(uint16(line.C))&(kop.ReplDmgInsSac|kop.ReplDmgInsVind)
	for b := 0; b < len(DmgInsteadStats); b++ {
		// the ask bodies are counted where they run (repl_dmbody.go: the
		// combat path never reaches dmgInstead, it defers to the frozen walk)
		if body&(1<<b) != 0 && body&(kop.ReplDmgInsSac|kop.ReplDmgInsVind) == 0 {
			DmgInsteadStats[b].Add(1)
		}
	}
	if body&(kop.ReplDmgInsSac|kop.ReplDmgInsVind) != 0 {
		// an ask body: the choice is asked as an ordinary decision
		// (repl_dmbody.go); a frozen pass's recomputed run defers to the walk
		return s.dmgBody(k, host, line, body, rcp, sl, combat, n, ask)
	}
	if s.g.ro.on {
		s.capFull(StallReplPlus) // a walk's pass would run the body twice
		return true
	}
	if body&(kop.ReplDmgInsMill|kop.ReplDmgInsExileTop|kop.ReplDmgInsSac) != 0 && s.parkedAny() {
		s.capFull(StallReplPlus) // the moves would ask the graveyard-move competition's order
		return true
	}
	// CR 615.12: under "damage can't be prevented" a prevention's body still
	// runs and the damage stands
	stands := body&kop.ReplDmgInsPrevent != 0 && s.noPrev(k, combat)
	kind := int(uint16(line.A) >> kop.ReplDmgInsKindShift)
	hc := s.perms[host].ctrl
	if body&(kop.ReplDmgInsCtrHost|kop.ReplDmgInsCtrEquipped|kop.ReplDmgInsCtrTarget) != 0 {
		to := sl // the damaged permanent, and so the equipped one its filter matched
		if body&kop.ReplDmgInsCtrHost != 0 {
			to = host
		}
		if to != NoSlot && s.occ.has(to) {
			s.putCountersDmg(k, to, kind, n)
		}
	}
	if body&kop.ReplDmgInsMill != 0 {
		m := n
		if body&kop.ReplDmgInsDouble != 0 {
			m = 2 * n
		}
		s.millDmg(k, rcp, m)
	}
	if body&kop.ReplDmgInsExileTop != 0 {
		s.exileTopDmg(k, hc, n)
	}
	if body&kop.ReplDmgInsDrawSrc != 0 {
		for i := 0; i < n; i++ {
			s.draw(srcCtrl)
		}
	}
	return !stands
}

// dmgInsteadMatch: line r of the host at h reads the event: the combat word,
// the damaged recipient and the source.
func (s *State) dmgInsteadMatch(k *prep, r *kop.Repl, h Slot, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool) bool {
	if r.C&kop.ReplDmgNonCombat != 0 && combat || r.C&kop.ReplDmgCombat != 0 && !combat {
		return false
	}
	hc := s.perms[h].ctrl
	switch {
	case sl == NoSlot:
		if !(r.C&kop.ReplDmgPlayer != 0 || r.C&kop.ReplDmgYou != 0 && rcp == hc) {
			return false // the recipient is no player the line names
		}
	case r.C&kop.ReplDmgSelf != 0 && sl == h:
	case r.Filter != 0 && s.matchPermGen(k, r.Filter, sl, hc, h, layerFull):
	default:
		return false
	}
	if r.Filter2 == 0 {
		return true
	}
	if srcSl != NoSlot {
		return s.matchPermGen(k, r.Filter2, srcSl, hc, h, layerFull)
	}
	ctx := evalCtx{src: s.perms[h].obj, srcSl: h, ctrl: hc}
	return s.matchSourceCard(k, r.Filter2, srcCard, srcCtrl, hc, &ctx)
}

// millDmg mills player p's top n cards as OpMill does (one mill action; a
// mill replacement reaches it through millShuffling).
func (s *State) millDmg(k *prep, p uint8, n int) {
	if k.gen != nil && k.gen.anyBatch {
		defer s.batchClose(s.batchOpen()) // one mill action (gorge effMill)
	}
	s.millCostPay(k, p, n)
}

// exileTopDmg exiles player p's top n cards, fewer when the library is short
// (gorge Dig | ChangeNum$ All | DestinationZone$ Exile).
func (s *State) exileTopDmg(k *prep, p uint8, n int) {
	pl := &s.pl[p]
	for i := 0; i < n && pl.libLo < pl.libHi; i++ {
		o := pl.lib[pl.libLo]
		pl.libLo++
		s.touch(o)
		s.objs[o].zone = kop.ZoneNone
		s.placeFromK(k, o, kop.ZoneExile, false, kop.ZoneLibrary, false, false)
	}
}

// dmgInsteadCompete are the damage replacements that can match an event
// beside a ReplDamageInstead line: gorge asks their CR 616.1 order.
const (
	dmgInsteadCompete  = rbDblDmg | rbPlusDmg | rbPrevNC | rbPrevDmgTo | rbPrevCol | rbShield | rbPrevMill | rbPrevCombatTo | rbPrevCombatBy | rbSelfDmg
	dmgInsteadCompete2 = rb2DmgMinus | rb2DmgSet
)

// dmgInsteadStall: the table holds a ReplDamageInstead line beside another
// damage replacement, a static redirect or a redirect shield (initStall).
func dmgInsteadStall(g *genPrep) bool {
	return g.anyRepl2&rb2DmgInst != 0 &&
		(g.anyRepl&dmgInsteadCompete != 0 || g.anyRepl2&dmgInsteadCompete2 != 0 || g.anyRedir || g.uses.has(useStatRdr))
}

// dmgInsteadEarlier: an earlier initStall case already names the table's
// StallReplPlus (pairRefusal reads the cases in initStall's order).
func dmgInsteadEarlier(g *genPrep) bool {
	return g.anyRepl&rbPrevMill != 0 && g.anyRepl&(rbDblDmg|rbPlusDmg|rbPrevCol) != 0 ||
		g.anyRepl&rbSelfDmg != 0 && g.anyRepl&rbSelfDmgCompete != 0 && !selfPhLifted(g)
}

// dmgInsteadPair is pairRefusal's halves for dmgInsteadStall.
func dmgInsteadPair(g *genPrep) (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	text := "a replace-the-damage line (counters, mill, exile or draw instead) beside another damage replacement or a redirect: gorge asks the CR 616.1 order of the two"
	a := func(_ *kop.CardDef, i int) bool { return g.replBits2[i]&rb2DmgInst != 0 }
	b := func(c *kop.CardDef, i int) bool {
		return g.replBits[i]&dmgInsteadCompete != 0 || g.replBits2[i]&dmgInsteadCompete2 != 0 || abilityOn(func(x *kop.Ability) bool {
			return x.Kind == kop.AbReplacement && x.Repl.Kind == kop.ReplDamageRedirect ||
				progHas(x, func(in *kop.Instr) bool {
					e := in.Ext()
					return e == kop.ExtPreventColor || e == kop.ExtPreventDamageScope || e == kop.ExtPreventNext || e == kop.ExtDmgRedirect
				})
		})(c, i)
	}
	return text, a, b
}
