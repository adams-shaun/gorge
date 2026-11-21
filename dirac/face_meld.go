package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Meld (CR 701.42, 712.4), cold: reached only when the table holds an
// OpExt ExtMeld (genPrep.anyMeld). gorge's representation (engine/state/
// meld.go) is one result object backed by two cards; dirac's is the same:
//
//   - the RESULT object's card is the melded face (kop.FaceMeld, Faces[1]);
//     every characteristic read sees it, and its def carries the pair's
//     mana-value sum (kgen lower_meld.go);
//   - the PARTNER object parks in kop.ZoneNone, on no zone list, with no
//     state of its own;
//   - the pair is one record of the cold meldLink region (gorge
//     Object.MeldedWith): result+1, partner+1, 4 B, zero length in a table
//     with no meld.
//
// ExtMeld follows gorge effMeld + effects.Meld: the source must be on the
// battlefield under the resolving controller, the partner is the first
// permanent of the partner filter (a named, controller-owned scan in seat
// then battlefield order); the pair is exiled as one zone batch, source then
// partner (a pair that is not a meld pair, a token or a copy among them,
// stays exiled: CR 701.42c); the partner is parked, the result turns to its
// melded face and returns to the battlefield under the controller (an
// ordinary entry: enters triggers, printed loyalty counters). When the result
// leaves the battlefield for any zone, meldSplit moves the partner there as
// the result is placed (gorge events.unmeld runs inside Move, before the
// result's own placement, so the partner lands first), silently: no event or
// trigger fires for the partner card (the Move fold emits none).
//
// A Commander pool refuses a meld result card at kgen (part 2 mirrors the
// CR 903.3 handling).

// maxMeldLinks is the cold region's records: the melded permanents at once.
const maxMeldLinks = 16

// meldRec is one melded permanent: result and partner object + 1 (0 free).
type meldRec struct{ res, partner objWord }

// coldMeld holds the links, for a table with a Meld op.
var coldMeld = newCold[meldRec]("meld", func(g *genPrep) int {
	if g.anyMeld {
		return maxMeldLinks
	}
	return 0
})

// DebugMeldKeep skips the split when a melded permanent leaves the
// battlefield: the partner stays parked (mutation testing only: the lockstep
// must catch it).
var DebugMeldKeep bool

// MeldStats counts the lifted mechanic; oracle tests only.
type MeldStats struct {
	Melds  int // pairs melded
	Splits int // melded permanents that left and split
	Stays  int // meld instructions that exiled a pair without melding (CR 701.42c)
}

// DebugMeldStats is the test hook; nil in normal play.
var DebugMeldStats *MeldStats

// prepMeld sets anyMeld: some program is ExtMeld.
func prepMeld(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				g.anyMeld = g.anyMeld || in.Ext() == kop.ExtMeld
			}
		}
	}
}

func init() { stallNames[StallMeld] = "meld" }

// meldFree is a free link record's index, -1 when all are taken.
func (s *State) meldFree() int {
	for i, r := range coldMeld.of(s) {
		if r.res == 0 {
			return i
		}
	}
	return -1
}

// meldTokenOrCopy reports whether object o is a token or a copy (CR 701.42c).
func (s *State) meldTokenOrCopy(o Obj) bool {
	ob := &s.objs[o]
	if ob.of&(ofToken|ofCopy) != 0 {
		return true
	}
	return ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot && s.perms[ob.slot].ovr.flags&ovCopy != 0
}

// meldExt is the //kmanifest:ext ExtMeld handler (meld below).
//
//kmanifest:ext ExtMeld
func (s *State) meldExt(k *prep, in *kop.Instr) (ok, done bool) {
	s.meld(k, in)
	return true, true
}

// meld is ExtMeld (gorge effMeld): Obj the source, F the partner's filter.
//
//go:noinline
func (s *State) meld(k *prep, in *kop.Instr) {
	src := s.objRefObj(in.Obj)
	if src == NoObj {
		return
	}
	e := &s.res.e
	sob := &s.objs[src]
	if sob.zone != kop.ZoneBattlefield || s.perms[sob.slot].ctrl != e.ctrl {
		return
	}
	var ord [MaxPerm]Slot
	n := s.sweepHits(k, in.F, &ord)
	partner := NoObj
	for _, sl := range ord[:n] {
		if o := s.perms[sl].obj; o != src {
			partner = o
			break
		}
	}
	if partner == NoObj {
		return
	}
	ri := s.meldFree()
	if ri < 0 {
		s.capFull(StallMeld)
		return
	}
	paired := !s.meldTokenOrCopy(src) && !s.meldTokenOrCopy(partner)
	cd := &k.t.Cards[sob.card]
	paired = paired && cd.FaceMode == kop.FaceMeld && cd.Face == 0 && len(cd.Faces) == 2

	// CR 701.42a: exile both as one zone batch, the source first.
	order := [2]Slot{sob.slot, s.objs[partner].slot}
	hr := s.holdOpen(k)
	if k.gen != nil && k.gen.anyBatch {
		s.leaveAllBatched(k, order[:], kop.ZoneExile)
	} else {
		for _, sl := range order {
			s.fireLeave(k, sl, kop.ZoneExile)
		}
		for _, sl := range order {
			s.removePerm(k, sl, kop.ZoneExile, false)
		}
	}
	s.holdClose(k, hr)
	pob := &s.objs[partner]
	if !paired || sob.zone != kop.ZoneExile || pob.zone != kop.ZoneExile {
		if DebugMeldStats != nil {
			DebugMeldStats.Stays++
		}
		return // CR 701.42c: the exiled cards stay in exile
	}
	// The pairing fold: park the partner, turn the result.
	s.detach(partner)
	pob.zone = kop.ZoneNone
	s.turnFace(k, src, 1)
	coldMeld.of(s)[ri] = meldRec{res: objWord(src) + 1, partner: objWord(partner) + 1}
	if DebugMeldStats != nil {
		DebugMeldStats.Melds++
	}
	// The melded permanent's entry, under the melder's control.
	from := s.detach(src)
	s.enterMoved(k, src, e.ctrl, false, from)
}

// meldLeave is events.unmeld: result o is leaving the battlefield for z, so
// it turns back to its front face and its partner card (parked in ZoneNone)
// goes to z too, in its own owner's zone. It runs before the result is
// placed, as gorge's Move runs unmeld ahead of the result's own insertion.
// A graveyard move under an any-zone exile replacement (Rest in Peace) would
// send the result to exile inside placeFromK while gorge's replacement has
// already made both moves exile: that case stalls (StallLowering) rather
// than split the pair wrongly.
//
//go:noinline
func (s *State) meldLeave(k *prep, o Obj, z kop.Zone, bottom bool, wasCreature bool) {
	recs := coldMeld.of(s)
	for i := range recs {
		if recs[i].res != objWord(o)+1 {
			continue
		}
		p := Obj(recs[i].partner - 1)
		recs[i] = meldRec{}
		if DebugMeldKeep {
			return
		}
		s.turnFace(k, o, 0)
		if s.objs[p].zone != kop.ZoneNone {
			return
		}
		if z == kop.ZoneGraveyard && k.gen.exileAnyC != nil {
			s.stall(StallLowering)
			return
		}
		if DebugMeldStats != nil {
			DebugMeldStats.Splits++
		}
		s.meldPlace(k, p, z, bottom, wasCreature)
		return
	}
}

// meldPlace puts parked card p into z of its owner: placeFromK's list
// insertion with none of its replacement or trigger arms (gorge's Move fold
// emits no event for the partner).
func (s *State) meldPlace(k *prep, p Obj, z kop.Zone, bottom bool, wasCreature bool) {
	s.touch(p)
	pb := &s.objs[p]
	pb.ver++
	pb.slot = NoSlot
	s.g.zoneTurn.set(p)
	pl := &s.pl[pb.owner()]
	if pl.zoneFull(z) {
		s.capFull(StallZone)
		return
	}
	pb.zone = z
	switch z {
	case kop.ZoneLibrary:
		if bottom {
			pl.libBottom(p)
		} else {
			pl.libTop(p)
		}
	case kop.ZoneHand:
		pl.hand[pl.handN] = p
		pb.of |= ofInHand
		pl.handN++
	case kop.ZoneGraveyard:
		pl.grave[pl.graveN] = p
		pl.graveN++
	case kop.ZoneExile:
		pl.exile[pl.exileN] = p
		pl.exileN++
	default:
		pb.zone = kop.ZoneNone
		return
	}
	if s.g.enteredOn {
		s.noteEnter(p, z, kop.ZoneBattlefield, wasCreature)
	}
}

// meldLeaveOnLeave is the removePerm hook: a melded permanent's partner
// leaves with it (face_meld.go). The gate lives here so removePerm's hot
// path grows no branch (codeshape).
func (s *State) meldLeaveOnLeave(k *prep, o Obj, z kop.Zone, bottom bool, wasCreature bool) {
	if k.gen != nil && k.gen.anyMeld {
		s.meldLeave(k, o, z, bottom, wasCreature)
	}
}
