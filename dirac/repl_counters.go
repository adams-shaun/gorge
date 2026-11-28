package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Replacement grammar (dies-to-exile for others' permanents, filtered counter
// doublers and plus/minus replacements) and keyword counter grants.

// ctrKw is the keywords p's keyword counters grant (CR 122.1b).
func ctrKw(s *State, p *perm) kop.KwMask {
	var m kop.KwMask
	if s.g.nmay != 0 && s.suspendHasteAt(p) {
		m = kop.KwHaste // suspend's grant (kw_suspend.go)
	}
	for i := range p.ck {
		k := p.ck[i]
		if k == 0 {
			return m // slots are packed; CtrP1P1 never takes one
		}
		if p.cn[i] != 0 { // a seen kind stays in its slot after its last counter is removed
			m |= ctrKindKw(k)
		}
	}
	for _, e := range s.ctrOverOf(p.obj, 0) {
		if e.n != 0 {
			m |= ctrKindKw(e.kind)
		}
	}
	return m
}

// ctrKwNewer is the keywords p's keyword counters grant that are newer than
// timestamp ts (CR 613.7): a loses-all-abilities or keyword-removal effect at
// ts takes every older grant and leaves these (gorge
// appendNewerCounterKeywords: a counter's stamp strictly above the effect's).
func ctrKwNewer(s *State, p *perm, ts uint16) kop.KwMask {
	var m kop.KwMask
	for i := range p.ck {
		k := p.ck[i]
		if k == 0 {
			return m
		}
		if p.cn[i] != 0 && p.cs[i] > ts {
			m |= ctrKindKw(k)
		}
	}
	for _, e := range s.ctrOverOf(p.obj, 0) {
		if e.n != 0 && e.ts > ts {
			m |= ctrKindKw(e.kind)
		}
	}
	return m
}

// ctrKindKw is the keyword a counter of kind k grants. Flying is the
// featurized CtrFlying; the other supported kinds sit above CtrCharge.
func ctrKindKw(k uint16) kop.KwMask {
	switch kop.CounterKind(k) {
	case kop.CtrFlying:
		return kop.KwFlying
	case kop.CtrFirstStrike:
		return kop.KwFirstStrike
	case kop.CtrDoubleStrike:
		return kop.KwDoubleStrike
	case kop.CtrDeathtouch:
		return kop.KwDeathtouch
	case kop.CtrHaste:
		return kop.KwHaste
	case kop.CtrHexproof:
		return kop.KwHexproof
	case kop.CtrIndestructible:
		return kop.KwIndestructible
	case kop.CtrLifelink:
		return kop.KwLifelink
	case kop.CtrMenace:
		return kop.KwMenace
	case kop.CtrReach:
		return kop.KwReach
	case kop.CtrShadow:
		return kop.KwShadow
	case kop.CtrTrample:
		return kop.KwTrample
	case kop.CtrVigilance:
		return kop.KwVigilance
	}
	return 0
}

// useExileOnLeaveOverride arms the Override.ExileOnLeave scan (an Animate
// Replacements$ record on the dying permanent). A table without one keeps the
// printed-source fast path.
var useExileOnLeaveOverride = newGenUse(tableHasExileOnLeaveOverride)

// tableHasExileOnLeaveOverride reports whether some override record carries
// the leave-to-exile replacement.
func tableHasExileOnLeaveOverride(t *kop.Table) bool {
	for i := range t.Overrides {
		if t.Overrides[i].ExileOnLeave {
			return true
		}
	}
	return false
}

// overrideExileOnLeaveN counts the active Override.ExileOnLeave records on the
// permanent at sl (an Animate Replacements$ line: one continuous replacement
// each, gorge registers one per animation).
func (s *State) overrideExileOnLeaveN(k *prep, sl Slot) int {
	n := 0
	if k.gen != nil && k.gen.uses.has(useExileOnLeaveOverride) && s.fxM.has(sl) {
		s.eachOvRec(sl, func(r ovRec) {
			if k.t.Overrides[r.idx-1].ExileOnLeave {
				n += r.reps() // identical applications merge into one record (addOvRecMk)
			}
		})
	}
	return n
}

// stallBatchLeave stops the game on a batch departure to a hand or library
// that bypassed leave: the kernel does not redirect it past an Animate
// Replacements$ record, so it stalls rather than diverge (overrideLeaveZone).
func (s *State) stallBatchLeave(k *prep, sl Slot, z kop.Zone) {
	if z == kop.ZoneGraveyard || z == kop.ZoneExile || k.gen == nil || !k.gen.uses.has(useExileOnLeaveOverride) {
		return
	}
	if s.overrideExileOnLeaveN(k, sl) > 0 {
		s.capFull(StallReplLeave)
	}
}

// overrideLeaveZone is the zone a permanent leaving the battlefield for z (a
// hand or library move: a bounce, a tuck) really goes to under an Animate
// Replacements$ record on it. The carriers' R: line names no Destination$
// ("if this permanent would leave the battlefield, exile it instead": Geth's
// "instead of putting it anywhere else"), so gorge's matcher fires it for every
// departure and the permanent is exiled. The graveyard move is dieZone's.
// Two or more such records are CR 616.1 competing effects: a stall, as
// dieZone's.
func (s *State) overrideLeaveZone(k *prep, sl Slot, z kop.Zone) kop.Zone {
	if z == kop.ZoneGraveyard || z == kop.ZoneExile || k.gen == nil || !k.gen.uses.has(useExileOnLeaveOverride) {
		return z
	}
	switch n := s.overrideExileOnLeaveN(k, sl); {
	case n == 1:
		return kop.ZoneExile
	case n > 1:
		s.capFull(StallReplLeave)
		return kop.ZoneExile
	}
	return z
}

// exileOnLeaveN counts the dies-to-exile replacements that apply to
// sl, a permanent about to leave the battlefield for a graveyard. Each
// matching source is a separate CR 616.1 replacement: gorge asks the
// affected player to order two or more of them even when they agree on
// Exile, and the kernel has no death order ask, so dieZone stalls. The
// second and third results are the counter a matching source leaves on the
// exiled card (0 when none): dieZone reads them only when one source applies.
// The fourth is the last matching source's slot (the one source when n is 1;
// NoSlot when none): its rider runs after the move (repl_exilerider.go).
func (s *State) exileOnLeaveN(k *prep, sl Slot) (int, kop.CounterKind, uint8, Slot) {
	g := k.gen
	if g == nil || g.anyRepl&rbExileOnLeave == 0 && !(g.uses.has(useExileOnLeaveOverride) && s.fxM.has(sl)) {
		return 0, 0, 0, NoSlot
	}
	n := 0
	src := NoSlot
	var ctr kop.CounterKind
	var ctrN uint8
	if g.exileOnLeave != nil {
		for m := s.statM; m.any(); m = m.rest() {
			b := m.first()
			// a seat that has left the game's effects no longer apply (CR 800.4a):
			// gorge's replacement walk (rules/object_query.go forEachObject) visits
			// only living seats' zones, so its permanent is not consulted even
			// though the sweep has not moved it yet (zone_leavegame.go)
			if s.genLost(k, b) || s.out(s.perms[b].ctrl) {
				continue
			}
			s.eachPileCard(k, b, func(c CardID) bool {
				if g.replBits[c]&rbExileOnLeave == 0 {
					return true
				}
				if s.matchPerm(k, g.exileOnLeave[c], sl, s.perms[b].ctrl, b) {
					n++
					src = b
					if kind, amount := exileCounterKind(k.t, c, g.exileOnLeave[c]); kind != 0 {
						ctr, ctrN = kind, amount
					}
				}
				return true
			})
		}
	}
	// an emblem's dies-to-exile replacement joins the count (Repl.A 0: no
	// counter, no rider); two or more matches, or one beside a card source,
	// are dieZone's StallReplLeave below
	cardN := n
	// an Animate Replacements$ record on the permanent itself (Llanowar
	// Greenwidow, Realmbreaker, Geth): the override's ExileOnLeave rides its
	// lifetime, so it is a battlefield-only replacement with no counter and
	// no rider. It competes with any printed source by the same CR 616.1
	// count.
	n += s.overrideExileOnLeaveN(k, sl)
	if embN := s.embExileLeaveN(k, sl); embN > 0 {
		n += embN
		if cardN == 0 && embN == 1 {
			src = NoSlot // the emblem is the only source: no rider, no counter
		}
	}
	if cardN > 0 && g.anyExileAny && s.isCreature(k, sl) {
		exileAnyDied() // repl_exileonleave.go
	}
	return n, ctr, ctrN, src
}

// entryCounterSource reports whether the AddCounter replacement sourced at
// src with recipient filter f applies to the entry counters being settled for
// the entrant in s.g.ctrSettle. CR 614.12 (gorge's ownEntryCounterExcluded):
// a line the entrant itself carries applies only when it selects the entrant
// alone, FSelf with no alternative. A general subset (Yoshimaru's "a creature
// you control", a filter-less doubler) does not, so it never modifies its own
// entry.
func (s *State) entryCounterSource(k *prep, src Slot, f kop.FilterID) bool {
	if s.g.ctrSettle == 0 || src != Slot(s.g.ctrSettle-1) {
		return true
	}
	if f == 0 {
		return false
	}
	fl := &k.t.Filters[f]
	return fl.Or == 0 && fl.Flags&kop.FSelf != 0
}

// replCtrOn applies the filtered counter replacements to n counters
// of kind being put on sl: each matching plus/minus one adds its amount,
// each matching doubler doubles. A plus/minus and a doubler do not commute
// (CR 616.1's order ask), so the game stalls instead of guessing.
func (s *State) replCtrOn(k *prep, sl Slot, kind, n int) int {
	return s.replCtrOnX(k, sl, kind, n, false)
}

// entryCtrSelfExcluded reports whether an AddCounter replacement whose source
// is the permanent at src must stay out of the entry-counter placement on recv
// (CR 614.12; gorge ownEntryCounterExcluded): a replacement that modifies how
// a permanent enters may come from the permanent itself only if it affects
// that permanent alone (Card.Self), not a general subset that includes it.
func (s *State) entryCtrSelfExcluded(k *prep, src, recv Slot, f kop.FilterID) bool {
	return s.g.entrySelf != 0 && src == recv && int(s.g.entrySelf) == int(recv)+1 &&
		!filterSelfOnly(k.t, f)
}

// dblCtrN counts the any-kind counter doublers (ReplDoubleCounters, Doubling
// Season) whose source is a permanent controlled by ctrl. Outside an entry it
// is replN's count; during the entrant's entry fold a doubler on the entrant
// itself is left out unless it affects the entrant alone (CR 614.12).
func (s *State) dblCtrN(k *prep, sl Slot, ctrl uint8) int {
	g := k.gen
	n := 0
	for m := s.statM.and(s.ctrlM[ctrl]); m.any(); m = m.rest() {
		b := m.first()
		if s.genLost(k, b) {
			continue
		}
		s.eachPileCard(k, b, func(c CardID) bool {
			if g.replBits[c]&rbDblCtr == 0 {
				return true
			}
			if b == sl && s.g.entrySelf == uint8(sl)+1 {
				selfOnly := false
				cd := &k.t.Cards[c]
				for i := range cd.Abilities {
					a := &cd.Abilities[i]
					if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplDoubleCounters &&
						filterSelfOnly(k.t, a.Repl.Filter) {
						selfOnly = true
						break
					}
				}
				if !selfOnly {
					return true
				}
			}
			n++
			return true
		})
	}
	return n
}

// replCtrOnX is replCtrOn with the EffectOnly$ doubler (Doubling Season)
// left out when noEO: a cost's counters are no effect's (replCostCtr,
// repl_addcounter.go).
func (s *State) replCtrOnX(k *prep, sl Slot, kind, n int, noEO bool) int {
	g := k.gen
	ctrl := s.perms[sl].ctrl
	dbl := 0
	if g.anyRepl&rbDblCtr != 0 && !noEO {
		dbl = s.dblCtrN(k, sl, ctrl)
	}
	if g.anyRepl&rbDblCtrYou != 0 {
		dbl += s.dblCtrYou(k, s.counterAdder()) // trig_leavereturn.go
	}
	plus, plusN, plusPos, plusNeg := 0, 0, 0, 0
	if g.anyRepl&rbCtrOn != 0 {
		for m := s.statM; m.any(); m = m.rest() {
			b := m.first()
			if s.genLost(k, b) {
				continue
			}
			lc := s.perms[b].ctrl
			s.eachPileCard(k, b, func(c CardID) bool {
				if g.replBits[c]&rbCtrOn == 0 {
					return true
				}
				cd := &k.t.Cards[c]
				for i := range cd.Abilities {
					a := &cd.Abilities[i]
					if a.Kind != kop.AbReplacement || !s.entryCounterSource(k, b, a.Repl.Filter) {
						continue
					}
					switch a.Repl.Kind {
					case kop.ReplDoubleCountersOn:
						if s.entryCtrSelfExcluded(k, b, sl, a.Repl.Filter) || !s.ctrKindMatch(a, kind, lc) { // repl_addcounter.go
							continue
						}
						if s.matchPerm(k, a.Repl.Filter, sl, lc, b) {
							dbl++
						}
					case kop.ReplAddCountersOn:
						if s.entryCtrSelfExcluded(k, b, sl, a.Repl.Filter) || !s.ctrKindMatch(a, kind, lc) { // repl_addcounter.go
							continue
						}
						if s.matchPerm(k, a.Repl.Filter, sl, lc, b) {
							plus += int(a.Repl.A)
							plusN++
							if a.Repl.A > 0 {
								plusPos++
							} else if a.Repl.A < 0 {
								plusNeg++
							}
						}
					}
				}
				return true
			})
		}
	}
	eotPlus := 0
	if !DebugCtrPlusEOTOff && g.anyRepl&rbPlusCtrEOT != 0 {
		eotPlus = int(s.g.plusCtr[ctrl])
		if g.anyPlusCre {
			cn, _ := s.plusCreN(k, sl, kind) // Prairie Dog's (repl_turnrepl.go)
			eotPlus += int(cn)
		}
		plus += eotPlus
		plusN += eotPlus
		plusPos += eotPlus
	}
	if (dbl > 0 && plusN > 0) || (plusPos > 0 && plusNeg > 0) {
		if m, ok := s.ctrOrderApply(k, sl, kind, n); ok {
			return m // the order ask was answered (repl_ctrorder.go)
		}
		s.capFull(StallReplCtr)
		return n
	}
	if eotPlus > 0 {
		CtrPlusEOTStats.Applied.Add(int64(eotPlus))
		if g.anyPlusCre {
			cn, _ := s.plusCreN(k, sl, kind)
			TurnReplStats.Plus.Add(int64(cn))
		}
	}
	if plus != 0 {
		n += plus
		if n < 0 {
			n = 0
		}
	}
	for c := dbl; c > 0 && n <= 0xffff; c-- {
		n *= 2
	}
	return n
}

// Innkeeper's Talent level 3, "If you would put one or more
// counters on a permanent or player, put twice that many ... instead"
// (ReplDoubleCountersYou, gated by its ClassBand$ 3 Repl.Cond). gorge reads
// the putter from its in-flight counter adder (rules/engine_damage.go
// inFlightCounterAdder): a publication at entry counters (the entrant's
// controller), the turn-based lore counter (the Saga's controller), infect
// damage (the source's controller) and costs, else the resolving stack
// object's controller; with none known the line fails closed. The kernel
// publishes the same through genState.ctrAdder (replCountersBy), else reads
// the resolving or building entry's controller.

// counterAdder is the player putting the counters now being placed (255:
// none known).
func (s *State) counterAdder() uint8 {
	if s.g.ctrAdder != 0 {
		return s.g.ctrAdder - 1
	}
	if s.res.active {
		return s.res.e.ctrl
	}
	if s.ctl == ctlBuild {
		return s.bld().e.ctrl
	}
	return 255
}

// replCountersBy is replCounters with the putter published as adder.
func (s *State) replCountersBy(k *prep, sl Slot, kind, n int, adder uint8) int {
	if k.gen == nil || n <= 0 || k.gen.anyRepl&rbDblCtrYou == 0 {
		return s.replCounters(k, sl, kind, n)
	}
	saved := s.g.ctrAdder
	s.g.ctrAdder = adder + 1
	n = s.replCounters(k, sl, kind, n)
	s.g.ctrAdder = saved
	return n
}

// dblCtrYou counts the ReplDoubleCountersYou sources whose controller is
// adder and whose condition (the Class level) holds.
//
//go:noinline
func (s *State) dblCtrYou(k *prep, adder uint8) int {
	if adder > 1 {
		return 0
	}
	n := 0
	for m := s.statM; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].ctrl != adder || s.genLost(k, sl) {
			continue
		}
		s.eachPileCard(k, sl, func(c CardID) bool {
			if k.gen.replBits[c]&rbDblCtrYou == 0 {
				return true
			}
			cd := &k.t.Cards[c]
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplDoubleCountersYou &&
					s.entryCounterSource(k, sl, a.Repl.Filter) &&
					s.statCond(k, statSrc{a: a, sl: sl, ctrl: adder, obj: s.perms[sl].obj}) {
					n++
				}
			}
			return true
		})
	}
	return n
}
