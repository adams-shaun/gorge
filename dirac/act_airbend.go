package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Airbend (CR 701.65a; gorge effects/airbend.go effAirbend,
// rules/airbend.go airbendCastAvailable, legal_walk_alt.go's "airbend_cast"
// offer): "Airbend [a permanent]" exiles it, and for as long as it remains
// exiled its owner may cast it by paying {2} rather than its mana cost.
//
//   - ExtAirbend exiles the battlefield permanent Obj (or the F set, minus
//     one target slot) through the ordinary leave path, and gives each card
//     that reached exile its owner's DurAirbend record (genState.may); a
//     token ceases to exist in exile and has none (CR 704.5d).
//   - The record is offered by offerAirbendCasts alone (ownWalkerRecord
//     keeps it off the generic play surfaces), on the card's own timing, to
//     its owner, when the floating pool alone pays {2} (as warp's recast,
//     faces.go FeatPoolPays). A stale record (the card left exile) is
//     dropped by cleanupGen's obj/ver/zone test.
//   - The cast is AkCast with mode castAirbend (castWarp|castWarpRecast,
//     which no other cast sets: every single mode bit is taken). beginCast
//     turns it into a warp-recast-shaped cast marked trigPlAirbend, and
//     buildCostGen charges {2} for that mark.

// castAirbend is the AkCast mode of an airbent card's cast from exile.
const castAirbend = castWarp | castWarpRecast

// FeatCastAirbend is castAirbend for the oracle adapter.
const FeatCastAirbend = castAirbend

// trigPlAirbend (trigpl.go) marks a spell entry cast for {2} from an airbend
// exile.

// DebugAirbendFullCost charges the card's mana cost for an airbend cast
// instead of {2} (mutation testing only: the lockstep must catch it).
var DebugAirbendFullCost bool

// prepAirbend sets anyAirbend: some program is ExtAirbend.
func prepAirbend(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				g.anyAirbend = g.anyAirbend || in.Ext() == kop.ExtAirbend
			}
		}
	}
}

// airbend is ExtAirbend. The objects are read once, before any leaves
// (gorge reads Defined first); one that has left by its turn is skipped.
func (s *State) airbend(k *prep, in *kop.Instr) {
	var objs [MaxPerm]Obj
	n := 0
	if in.Obj != kop.RefNone {
		if o := s.objRefObj(in.Obj); o != NoObj {
			objs[0], n = o, 1
		}
	} else {
		ex := NoObj
		if in.C > 0 {
			ex = s.objRefObj(tgtRef(int(in.C - 1)))
		}
		var ord [MaxPerm]Slot
		m := s.sweepHits(k, in.F, &ord)
		for _, sl := range ord[:m] {
			if o := s.perms[sl].obj; o != ex {
				objs[n] = o
				n++
			}
		}
	}
	var vers [MaxPerm]uint8
	for i := 0; i < n; i++ {
		vers[i] = s.objs[objs[i]].ver
	}
	for i := 0; i < n; i++ {
		o := objs[i]
		ob := &s.objs[o]
		if ob.zone != kop.ZoneBattlefield || ob.ver != vers[i] {
			continue
		}
		s.leave(k, ob.slot, kop.ZoneExile, false)
		if ob.zone != kop.ZoneExile || ob.of&(ofToken|ofCopy) != 0 {
			continue
		}
		if s.mayFull() {
			s.capFull(StallMayPlay)
			return
		}
		*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: ob.ver, ctrl: ob.owner(), until: kop.DurAirbend, turn: mayWord(s.turn)}
		s.g.nmay++
	}
}

// airbendCost is the cost of casting c from an airbend exile: {2}, less
// any reduction (CR 601.2f; gorge cast_begin.go applies cost modifiers).
func (s *State) airbendCost(k *prep, p uint8, c CardID, o Obj) kop.ManaCost {
	if DebugAirbendFullCost {
		return s.adjustCost(k, p, c, o, k.t.Cards[c].Cost)
	}
	return s.adjustCost(k, p, c, o, kop.ManaCost{Generic: 2})
}

// offerAirbendCasts offers casting p's airbent cards from exile (gorge
// legal_walk_alt.go exileCastsWalk, "airbend_cast"): the card's own timing,
// castable targets, {2} paid by the floating pool plus the restricted mana a
// cast from exile may spend (CR 106.6, 701.65a).
func (s *State) offerAirbendCasts(k *prep, d *Decision, p uint8, sorc bool) {
	pl := &s.pl[p]
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		c := s.objs[o].card
		pc := &k.cards[c]
		if !s.airbendRecord(p, o) {
			continue
		}
		if !pc.instant && !sorc && !s.castFlash(k, p, c) {
			continue
		}
		if s.castHeldOut(o) || k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) {
			continue
		}
		cost := s.airbendCost(k, p, c, o)
		var av avail
		s.availPool(p, &av)
		if s.rbAny(p) {
			s.restrAvail(k, p, c, s.restrOriginOfObj(o), &av)
		}
		if !av.affords(&cost, pipMaskOf(&cost), 0) {
			continue
		}
		d.add(MakeAction(AkCast, TCode(o), 0, castAirbend))
	}
}

// airbendRecord reports whether p holds a live DurAirbend record for o.
func (s *State) airbendRecord(p uint8, o Obj) bool {
	ob := &s.objs[o]
	for i := 0; i < int(s.g.nmay); i++ {
		r := *s.mayAt(i)
		if r.obj == o && r.ctrl == p && r.ver == ob.ver && r.until == kop.DurAirbend {
			return true
		}
	}
	return false
}

// refreshAirbend points the record at the card after a reversed recast put
// it back in exile (CR 733.1; as refreshWarpRecast).
func (s *State) refreshAirbend(o Obj) {
	ob := &s.objs[o]
	for i := 0; i < int(s.g.nmay); i++ {
		r := s.mayAt(i)
		if r.obj == o && r.ctrl == ob.owner() && r.until == kop.DurAirbend {
			r.ver = ob.ver
		}
	}
}
