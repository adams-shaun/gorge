package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// the plotted designation granted by an effect (CR
// 702.170b-d; Kellan Joins Up's "it becomes plotted": AlterAttribute
// Defined$ Remembered Attributes$ Plotted). gorge effAlterAttribute gates
// Plotted on exile and emits the grant for each Defined$ object there; the
// fold stamps PlottedTurn = Turn, and exileCastsWalk offers the free
// sorcery-speed cast on a strictly later turn to ANY card carrying the
// designation, plot keyword or not. The kernel writes the same DurPlot
// record the plot action does (kw_plot.go), which offerPlotCasts
// reads card-agnostically, then queues the card's BecomesPlotted triggers
// (gorge trigmatch/plotted.go matches the grant event).

// prepPlotGrant arms the plot walker (genPrep.anyPlot) when some program
// grants the designation.
func prepPlotGrant(t *kop.Table, g *genPrep) {
	if tablePlotGrant(t) {
		g.anyPlot = true
	}
}

// tablePlotGrant reports whether some program holds an ExtPlotGrant.
func tablePlotGrant(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() == kop.ExtPlotGrant {
					return true
				}
			}
		}
	}
	return false
}

// plotGrant is ExtPlotGrant: each remembered object in exile becomes
// plotted this turn; any other zone is a no-op (gorge's zone gate).
//
//go:noinline
func (s *State) plotGrant(k *prep) {
	for i := 0; i < int(s.g.nrem); i++ {
		o := s.g.rem[i]
		if o == NoObj {
			continue
		}
		ob := &s.objs[o]
		if ob.zone != kop.ZoneExile {
			continue
		}
		if s.mayFull() {
			s.capFull(StallMayPlay)
			return
		}
		*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: ob.ver, ctrl: ob.owner(), until: kop.DurPlot, turn: s.turn}
		s.g.nmay++
		if k.cards[ob.card].plotTrig {
			s.fireBecomesPlotted(k, o) // kw_plot.go
		}
	}
}

// Plot (CR 702.170): a sorcery-speed special action from hand pays
// the printed plot cost and exiles the card with the plotted designation.
// On a later turn its owner may cast it from exile without paying its mana
// cost, at sorcery speed whatever the face's own type is.
//
// The designation and the turn it was plotted ride a mayPlay record
// (genState.may), exactly as warp's recast permission rides a DurWarp
// record: until = kop.DurPlot, turn = s.turn. The generic play surfaces
// skip it (ownWalkerRecord) and this file offers it alone.
//
// The action is not a cast: it pushes nothing, records no CastInfo and
// fires no cast trigger. The later cast is an ordinary free cast: the push
// bumps the card's ver, so its plot record goes stale, and a reversed cast
// does not refresh it (CR 733.1: gorge's abortCast restores the pre-cast
// state, and the push already cleared PlottedTurn).

// castPlotted is the AkCast mode of a plotted card's free cast from exile
// (castFree alone; no castMayPlay, so no permission record is spent).
const castPlotted = castFree

// DebugPlotSameTurn relaxes the later-turn rule on plot's free cast and the
// plot record's turn test (mutation testing only: the lockstep must catch
// it).
var DebugPlotSameTurn bool

// DebugPlotInstant relaxes the sorcery-timing rule on the plot special
// action (mutation testing only: the lockstep must catch it).
var DebugPlotInstant bool

// ownWalkerRecord reports whether a mayPlay record is offered only by its
// own walker (warp's recast, plot's free cast), never by the generic play
// surfaces.
func ownWalkerRecord(r mayPlay) bool {
	return r.until == kop.DurWarp || r.until == kop.DurPlot || r.until == kop.DurPrepared || r.until == kop.DurDefeated || r.until == kop.DurAirbend
}

// plotRecord reports whether p holds a live DurPlot record for o on a turn
// strictly after the plot action's (gorge PlottedTurn > 0 && Turn >
// PlottedTurn).
func (s *State) plotRecord(p uint8, o Obj) bool {
	ob := &s.objs[o]
	for i := 0; i < int(s.g.nmay); i++ {
		r := *s.mayAt(i)
		if r.obj == o && r.ctrl == p && r.ver == ob.ver && r.until == kop.DurPlot && (s.turn > r.turn || DebugPlotSameTurn) {
			return true
		}
	}
	return false
}

// plotLive reports whether a DurPlot record still names the exiled card
// (cleanupGen drops a stale one: the card was cast, moved or is a new
// object).
func (s *State) plotLive(r mayPlay) bool {
	ob := &s.objs[r.obj]
	return ob.ver == r.ver && ob.zone == kop.ZoneExile
}

// offerPlotHand offers hand card o's plot special action (gorge rules
// legal_walk_hand.go's "plot" cast option). The caller has already checked
// the card's cast prohibition and that it is sorcery timing. The action is
// offered on potential affordability, like the warp cast, so a pool that
// must be tapped still sees it; the action itself taps then pays.
func (s *State) offerPlotHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	cd := &k.t.Cards[c]
	pc := s.adjustCost(k, p, c, o, cd.PlotCost) // a Spell-scope modifier applies (CR 601.2f)
	if len(k.gen.grantPC) != 0 && s.g.availX&availWalk != 0 {
		// gorge's potential walk counts a restricted mana ability only for a
		// payment its restriction admits (potentialRestrictionAdmits): a
		// hand-barred grant puts no cast on the walk grantCap reads
		var ha avail
		s.walkHandAvail(k, p, payDesc{class: pdOther, src: NoSlot}, &ha)
		av = &ha
	}
	if !av.affords(&pc, pipMaskOf(&pc), 0) && (!s.rbAny(p) && !k.restrSrc || !s.plotRestrFallback(k, p, &pc, av)) {
		return
	}
	if s.castHeldOut(o) || k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
		return // gorge castSuppressed / castRestricted gate the hand card first
	}
	d.add(MakeAction(AkPlot, uint8(o), 0, 0))
}

// plotAction performs the plot special action (CR 702.170a): pay the plot
// cost, move the card from hand to exile and arm the later free cast. Not a
// cast: nothing is pushed and no cast trigger fires.
func (s *State) plotAction(k *prep, o Obj, p uint8) {
	ob := &s.objs[o]
	c := ob.card
	cd := &k.t.Cards[c]
	cost := s.adjustCost(k, p, c, o, cd.PlotCost)
	if !s.poolPays(p, &cost) {
		// gorge's potential witness: the plot action's plan taps the
		// sources at priority, then the action pays from the floating pool
		short := cost
		if m := s.restrPoolNonSpell(k, p); m != 0 {
			short = s.restrShortAdm(p, m, cost) // mana_specialaction.go: only a non-spell restriction funds it
		}
		s.tapFor(k, p, &short, 0, NoObj)
	}
	if s.rbAny(p) {
		s.payPlotRestr(k, p, &cost, o) // mana_specialaction.go
	} else {
		s.payFromPool(p, &cost, 0)
	}
	from := s.detach(o)
	s.placeFrom(o, kop.ZoneExile, false, from, false)
	if s.mayFull() {
		s.capFull(StallMayPlay)
		return
	}
	*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: ob.ver, ctrl: ob.owner(), until: kop.DurPlot, turn: s.turn}
	s.g.nmay++
	s.passes = 0
	if pc := &k.cards[c]; pc.plotTrig {
		s.fireBecomesPlotted(k, o) // kw_plot.go
	}
}

// offerPlotCasts offers p's plotted cards' free cast from exile (gorge
// rules legal_walk_alt.go exileCastsWalk's "plot_cast"): only on a later
// turn than the plot action, only at sorcery speed whatever the face's
// type, and only while the DurPlot record is live (same object, still
// exiled).
func (s *State) offerPlotCasts(k *prep, d *Decision, p uint8, sorc bool) {
	if !sorc && !DebugPlotSameTurn {
		return
	}
	pl := &s.pl[p]
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		c := s.objs[o].card
		if !s.plotRecord(p, o) {
			continue
		}
		if s.castHeldOut(o) || k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		// The free cast waives the mana cost, not the spell's additional
		// costs (CR 118.9): a plotted Corrupted Conviction is offered only
		// while a creature can be sacrificed for it (CR 601.2h).
		if !s.spellCostsFeasible(k, c, p, o) || !s.spellTargetsFeasible(k, c, p) {
			continue
		}
		d.add(MakeAction(AkCast, uint8(o), 0, castPlotted))
	}
}

// "When this card becomes plotted" (CR 702.170a; gorge Mode$
// BecomesPlotted, trigmatch/plotted.go: the AlterAttribute Plotted grant
// the plot action emits after the card is exiled, matched against the card
// itself from exile; Aloe Alchemist). The kernel's plot action queues the
// card's own triggers, its owner controlling them; they go on the stack
// before the next priority, as any queued trigger.

// prepPlotTrig marks the cards with a TrigBecomesPlotted trigger.
func prepPlotTrig(t *kop.Table, p *prep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigBecomesPlotted {
				p.cards[i].plotTrig = true
			}
		}
	}
}

// fireBecomesPlotted queues plotted card o's BecomesPlotted triggers.
//
//go:noinline
func (s *State) fireBecomesPlotted(k *prep, o Obj) {
	ob := &s.objs[o]
	cd := &k.t.Cards[ob.card]
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigBecomesPlotted {
			s.enqueue(o, ob.ver, ob.card, uint8(i), ob.owner(), o, ob.owner(), 0)
		}
	}
}
